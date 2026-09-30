// API reference pages get their own sidebar, like the guides get theirs: the API's
// how-to pages, then one group per resource listing its endpoints behind method badges.
import { defineRouteMiddleware, type StarlightRouteData } from '@astrojs/starlight/route-data';
import { opPath, operationsFor, tags } from './lib/openapi';

type Entry = StarlightRouteData['sidebar'][number];
type Link = Extract<Entry, { type: 'link' }>;

const base = import.meta.env.BASE_URL;

function link(label: string, path: string, current: string, method?: string): Link {
	const href = `${base}${path}`;
	return {
		type: 'link',
		label,
		href,
		isCurrent: href === current,
		badge: method ? { text: method, variant: 'default', class: `lute-badge lute-badge-${method.toLowerCase()}` } : undefined,
		attrs: {},
	};
}

function apiSidebar(current: string): Entry[] {
	return [
		{
			type: 'group',
			label: 'Using the API',
			collapsed: false,
			badge: undefined,
			entries: [link('Overview', 'docs/api/', current), link('Webhooks', 'docs/api/webhooks/', current)],
		},
		...tags.map(
			(tag): Entry => ({
				type: 'group',
				label: tag.title,
				// Starlight still opens the group holding the current page.
				collapsed: true,
				badge: undefined,
				entries: [
					link(`About ${tag.title.toLowerCase()}`, `docs/api/${tag.name}/`, current),
					...operationsFor(tag.name).map((op) => link(op.summary, opPath(op), current, op.method)),
				],
			}),
		),
		{
			type: 'group',
			label: 'Guides',
			collapsed: false,
			badge: undefined,
			entries: [link('Back to the guides', 'docs/', current)],
		},
	];
}

const flatten = (entries: Entry[]): Link[] => entries.flatMap((e) => (e.type === 'link' ? [e] : flatten(e.entries)));

export const onRequest = defineRouteMiddleware((context) => {
	const route = context.locals.starlightRoute;
	const path = context.url.pathname;
	if (!path.startsWith(`${base}docs/api/`)) return;

	route.sidebar = apiSidebar(path);
	const links = flatten(route.sidebar).filter((l) => l.href !== `${base}docs/`);
	const i = links.findIndex((l) => l.isCurrent);
	route.pagination = { prev: i > 0 ? links[i - 1] : undefined, next: i >= 0 ? links[i + 1] : undefined };
});
