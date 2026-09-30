// Reads api/openapi.yaml at build time and shapes it for the API reference pages.
import { parseDocument } from 'yaml';
import raw from '../../../api/openapi.yaml?raw';

type Json = null | boolean | number | string | Json[] | { [k: string]: Json };
type Obj = Record<string, any>;

const doc = parseDocument(raw);
const spec: Obj = doc.toJS();

/** Response codes in the order the spec lists them; a JS object would sort them numerically. */
function responseOrder(path: string, method: string): string[] {
	const node = doc.getIn(['paths', path, method, 'responses']) as { items?: { key: { value: unknown } }[] } | undefined;
	return node?.items?.map((i) => String(i.key.value)) ?? [];
}

export const baseUrl: string = spec.servers[0].url;
export const specVersion: string = spec.info.version;

export interface Tag {
	name: string;
	title: string;
	description: string;
	object?: string;
}

export interface Param {
	name: string;
	in: 'path' | 'query';
	required: boolean;
	type: string;
	description: string;
	example?: Json;
}

export interface Field {
	name: string;
	type: string;
	required: boolean;
	description: string;
	children: Field[];
}

export interface Response {
	status: string;
	description: string;
	example?: Json;
	fields: Field[];
}

export interface Operation {
	id: string;
	/** URL segment of the endpoint's page: createRun → create-run. */
	slug: string;
	tag: string;
	method: string;
	path: string;
	summary: string;
	description: string;
	params: Param[];
	body?: { fields: Field[]; example?: Json };
	responses: Response[];
}

const methods = ['get', 'post', 'put', 'patch', 'delete'];

/** Follows local $refs and folds allOf, so callers see plain schemas. */
function deref(node: any): any {
	if (Array.isArray(node)) return node.map(deref);
	if (!node || typeof node !== 'object') return node;
	if (typeof node.$ref === 'string') {
		const target = node.$ref
			.replace(/^#\//, '')
			.split('/')
			.reduce((o: Obj, k: string) => o[k], spec);
		return deref(target);
	}
	if (Array.isArray(node.allOf)) {
		const parts = node.allOf.map(deref);
		return {
			type: 'object',
			required: parts.flatMap((p: Obj) => p.required ?? []),
			properties: Object.assign({}, ...parts.map((p: Obj) => p.properties ?? {})),
			example: parts.find((p: Obj) => p.example)?.example,
		};
	}
	const out: Obj = {};
	for (const [k, v] of Object.entries(node)) out[k] = deref(v);
	return out;
}

export function typeOf(s: Obj | undefined): string {
	if (!s) return 'any';
	if (s.enum) return s.enum.map((v: string) => JSON.stringify(v)).join(' | ');
	if (s.type === 'array') return s.items?.enum ? `(${typeOf(s.items)})[]` : `${typeOf(s.items)}[]`;
	if (s.type === 'object' && s.additionalProperties && !s.properties) {
		const v = s.additionalProperties === true ? 'any' : typeOf(s.additionalProperties);
		return `map<string, ${v}>`;
	}
	if (s.type === 'integer') return 'int';
	if (s.type === 'boolean') return 'bool';
	return s.type ?? 'object';
}

export function fieldsOf(schema: Obj | undefined): Field[] {
	if (!schema?.properties) return [];
	const required: string[] = schema.required ?? [];
	return Object.entries<Obj>(schema.properties).map(([name, s]) => ({
		name,
		type: typeOf(s),
		required: required.includes(name),
		description: s.description ?? '',
		children: fieldsOf(s.type === 'array' ? s.items : s),
	}));
}

/** An example for a schema: its own, else one built from its properties. */
function exampleOf(s: Obj | undefined): Json | undefined {
	if (!s) return undefined;
	if (s.example !== undefined) return s.example;
	if (s.enum) return s.enum[0];
	if (s.properties) {
		const out: Record<string, Json> = {};
		for (const [k, v] of Object.entries<Obj>(s.properties)) {
			const ex = exampleOf(v);
			if (ex !== undefined) out[k] = ex;
		}
		return out;
	}
	if (s.type === 'array') {
		const item = exampleOf(s.items);
		return item === undefined ? [] : [item];
	}
	return s.default;
}

export const tags: Tag[] = spec.tags.map((t: Obj) => ({
	name: t.name,
	title: t['x-displayName'] ?? t.name,
	description: t.description ?? '',
	object: t['x-object'],
}));

export function objectSchema(name: string): Obj {
	return deref(spec.components.schemas[name]);
}

export function operationsFor(tag: string): Operation[] {
	const ops: Operation[] = [];
	for (const [path, rawItem] of Object.entries<Obj>(spec.paths)) {
		const item = deref(rawItem);
		// Methods in the order the spec lists them.
		for (const method of Object.keys(rawItem).filter((k) => methods.includes(k))) {
			const op = item[method];
			if (!op || !op.tags?.includes(tag)) continue;
			const params: Param[] = [...(item.parameters ?? []), ...(op.parameters ?? [])].map((p: Obj) => ({
				name: p.name,
				in: p.in,
				required: !!p.required,
				type: typeOf(p.schema) + (p.schema?.default !== undefined ? ` = ${p.schema.default}` : ''),
				description: p.description ?? '',
				example: p.example,
			}));
			const media = op.requestBody?.content?.['application/json'];
			ops.push({
				id: op.operationId,
				slug: op.operationId.replace(/([a-z])([A-Z])/g, '$1-$2').toLowerCase(),
				tag,
				method: method.toUpperCase(),
				path,
				summary: op.summary,
				description: op.description ?? '',
				params,
				body: media ? { fields: fieldsOf(media.schema), example: media.example ?? exampleOf(media.schema) } : undefined,
				responses: responseOrder(path, method).map((status) => {
					const r: Obj = op.responses[status];
					const m = r.content?.['application/json'];
					const schema = m?.schema?.type === 'array' ? m.schema.items : m?.schema;
					return {
						status,
						description: r.description ?? '',
						example: m ? (m.example ?? exampleOf(m.schema)) : undefined,
						fields: fieldsOf(schema),
					};
				}),
			});
		}
	}
	return ops;
}

function urlFor(op: Operation): string {
	return (
		baseUrl +
		op.path.replace(/\{([^}]+)\}/g, (_, name) => {
			const p = op.params.find((x) => x.name === name);
			return p?.example !== undefined ? String(p.example) : `{${name}}`;
		})
	);
}

const json = (v: Json) => JSON.stringify(v, null, 2);
const indent = (s: string, pad: string) => s.replace(/\n/g, `\n${pad}`);

function pyLiteral(v: Json, pad = ''): string {
	if (v === null) return 'None';
	if (v === true) return 'True';
	if (v === false) return 'False';
	if (typeof v !== 'object') return JSON.stringify(v);
	const inner = pad + '    ';
	if (Array.isArray(v)) {
		if (v.length === 0) return '[]';
		return `[\n${v.map((x) => inner + pyLiteral(x, inner)).join(',\n')},\n${pad}]`;
	}
	const entries = Object.entries(v);
	if (entries.length === 0) return '{}';
	return `{\n${entries.map(([k, x]) => `${inner}${JSON.stringify(k)}: ${pyLiteral(x, inner)}`).join(',\n')},\n${pad}}`;
}

export interface Sample {
	lang: 'sh' | 'js' | 'python' | 'go';
	label: string;
	code: string;
}

export function samplesFor(op: Operation): Sample[] {
	const url = urlFor(op);
	const body = op.body?.example;
	const m = op.method;

	const curl = [
		`curl${m === 'GET' ? '' : ` -X ${m}`} ${url} \\`,
		`  -H "Authorization: Bearer $LUTE_API_KEY"${body !== undefined ? ' \\' : ''}`,
		...(body !== undefined ? [`  -H "Content-Type: application/json" \\`, `  -d '${indent(json(body), '  ')}'`] : []),
	].join('\n');

	const js = [
		`const res = await fetch("${url}", {`,
		...(m === 'GET' ? [] : [`  method: "${m}",`]),
		`  headers: {`,
		`    Authorization: "Bearer " + process.env.LUTE_API_KEY,`,
		...(body !== undefined ? [`    "Content-Type": "application/json",`] : []),
		`  },`,
		...(body !== undefined ? [`  body: JSON.stringify(${indent(json(body), '  ')}),`] : []),
		`});`,
		...(op.responses.some((r) => r.example !== undefined) ? ['const data = await res.json();'] : []),
	].join('\n');

	const py = [
		'import os, requests',
		'',
		`res = requests.${m.toLowerCase()}(`,
		`    "${url}",`,
		`    headers={"Authorization": "Bearer " + os.environ["LUTE_API_KEY"]},`,
		...(body !== undefined ? [`    json=${pyLiteral(body, '    ')},`] : []),
		')',
	].join('\n');

	const go = [
		...(body !== undefined ? [`body := strings.NewReader(\`${json(body)}\`)`] : []),
		`req, _ := http.NewRequest("${m}", "${url}", ${body !== undefined ? 'body' : 'nil'})`,
		`req.Header.Set("Authorization", "Bearer "+os.Getenv("LUTE_API_KEY"))`,
		...(body !== undefined ? [`req.Header.Set("Content-Type", "application/json")`] : []),
		'res, err := http.DefaultClient.Do(req)',
	].join('\n');

	return [
		{ lang: 'sh', label: 'cURL', code: curl },
		{ lang: 'js', label: 'JavaScript', code: js },
		{ lang: 'python', label: 'Python', code: py },
		{ lang: 'go', label: 'Go', code: go },
	];
}

/** The object as JSON, each described field preceded by its description as a comment. */
export function annotatedObject(schema: Obj): string {
	const example = (exampleOf(schema) ?? {}) as Record<string, Json>;
	const props: Record<string, Obj> = schema.properties ?? {};
	const keys = Object.keys(props).filter((k) => example[k] !== undefined);
	const lines = keys.flatMap((k, i) => {
		const note = firstSentence(props[k].description);
		const line = `  "${k}": ${JSON.stringify(example[k])}${i < keys.length - 1 ? ',' : ''}`;
		return note ? [`  // ${note}`, line] : [line];
	});
	return `{\n${lines.join('\n')}\n}`;
}

function firstSentence(s: string | undefined): string {
	if (!s) return '';
	return s.split(/(?<=\.)\s/)[0].replace(/`/g, '').replace(/\.$/, '');
}

/** Just enough Markdown for spec descriptions: paragraphs, `code` and **bold**. */
export function md(s: string): string {
	const esc = s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
	return esc
		.trim()
		.split(/\n\s*\n/)
		.map((p) => `<p>${p.replace(/\n/g, ' ').replace(/`([^`]+)`/g, '<code>$1</code>').replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')}</p>`)
		.join('');
}

/** Where an endpoint's page lives, relative to the site base. */
export const opPath = (op: Operation) => `docs/api/${op.tag}/${op.slug}/`;
