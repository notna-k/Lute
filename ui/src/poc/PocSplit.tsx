/**
 * PoC C — "Split".
 *
 * Thesis: there are no modes. One surface, always split — the form on the
 * left, its consequences on the right. Whatever you touch on the left
 * highlights on the right, whether that is the YAML you would commit, the env
 * the container receives, or the API payload a script would post.
 *
 * The bet: for a GitOps product, the strongest thing the panel can do is make
 * the file and the run permanently visible side by side. Authoring is then
 * just a disclosure on each field rather than a separate mode.
 *
 * Cost: needs the width; the right pane is dead weight on a laptop screen
 * unless it collapses.
 */
import { useMemo, useState } from 'react';
import {
  Check,
  ChevronDown,
  ChevronUp,
  FileCode2,
  GitCompare,
  Play,
  Plus,
  Settings2,
  Terminal,
  Trash2,
} from 'lucide-react';
import { cn } from '@/lib/cn';
import { PocSwitcher } from './components/PocFrame';
import { RunField } from './components/RunField';
import { ParamEditor } from './components/ParamEditor';
import { EnvPane } from './components/EnvPane';
import { YamlPane } from './components/YamlPane';
import { toEnvPairs, typeDef, TYPE_ORDER } from './params/registry';
import { MOCK_JOB } from './mock';
import { useJobDraft } from './useJobDraft';

type Pane = 'yaml' | 'env' | 'api';

const PANES: { id: Pane; label: string; icon: typeof FileCode2 }[] = [
  { id: 'yaml', label: 'Definition', icon: FileCode2 },
  { id: 'env', label: 'Environment', icon: Terminal },
  { id: 'api', label: 'API call', icon: GitCompare },
];

export default function PocSplit() {
  const draft = useJobDraft(MOCK_JOB);
  const [pane, setPane] = useState<Pane>('yaml');
  const [focusId, setFocusId] = useState<string | null>(null);
  const [configId, setConfigId] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [queued, setQueued] = useState<number | null>(null);

  const { job, values, setValue, errors } = draft;
  const focused = job.params.find((p) => p.id === focusId) ?? null;

  // Whether the draft still matches what Git has. In the real thing this is a
  // diff against the synced commit; here, any edit at all counts.
  const dirty = useMemo(
    () => JSON.stringify(job.params) !== JSON.stringify(MOCK_JOB.params),
    [job.params]
  );

  const payload = useMemo(() => {
    const body = Object.fromEntries(
      job.params
        .filter((p) => p.type !== 'secret')
        .map((p) => [p.name, values[p.name] ?? null])
    );
    return JSON.stringify({ params: body }, null, 2);
  }, [job.params, values]);

  function run() {
    draft.setSubmitted(true);
    if (Object.keys(draft.allErrors).length) return;
    setQueued(413);
    setTimeout(() => setQueued(null), 3000);
  }

  return (
    <div className='mx-auto max-w-[86rem]'>
      <PocSwitcher />

      <div className='mb-4 flex flex-wrap items-baseline gap-3'>
        <h1 className='font-mono text-xl font-semibold text-fg'>{job.name}</h1>
        <span className='text-sm text-fg-muted'>{job.description}</span>
        <span
          className={cn(
            'ml-auto inline-flex items-center gap-1.5 rounded px-2 py-1 font-mono text-xxs',
            dirty
              ? 'bg-warning-subtle text-warning-fg'
              : 'bg-success-subtle text-success-fg'
          )}
        >
          {dirty ? (
            <>
              <GitCompare className='h-3 w-3' /> drifted from {job.source.commit}
            </>
          ) : (
            <>
              <Check className='h-3 w-3' /> in sync with {job.source.commit}
            </>
          )}
        </span>
      </div>

      <div className='grid gap-4 xl:grid-cols-2'>
        {/* --- left: the form ---------------------------------------------- */}
        <div className='rounded-xl border border-border bg-surface'>
          <div className='flex items-center gap-2 border-b border-border px-4 py-2.5'>
            <span className='font-mono text-xxs uppercase tracking-wider text-fg-muted'>
              parameters
            </span>
            <span className='ml-auto font-mono text-xxs text-fg-subtle'>
              hover a field to see it on the right
            </span>
          </div>

          <div className='divide-y divide-dashed divide-border px-4'>
            {job.params.map((spec, i) => {
              const open = configId === spec.id;
              const Icon = typeDef(spec.type).icon;
              return (
                <div
                  key={spec.id}
                  onMouseEnter={() => setFocusId(spec.id)}
                  onFocusCapture={() => setFocusId(spec.id)}
                  className={cn('py-4 transition-colors', focusId === spec.id && 'bg-primary-subtle/20')}
                >
                  <div className='mb-2 flex items-baseline gap-2'>
                    <Icon className='h-3.5 w-3.5 shrink-0 self-center text-fg-subtle' />
                    <span className='text-sm font-semibold text-fg'>{spec.label}</span>
                    {spec.required && (
                      <span className='text-xxs font-medium uppercase tracking-wide text-warning'>
                        required
                      </span>
                    )}
                    <div className='ml-auto flex items-center gap-0.5'>
                      <button
                        onClick={() => draft.moveParam(spec.id, -1)}
                        disabled={i === 0}
                        className='rounded p-1 text-fg-subtle hover:bg-surface-hover hover:text-fg disabled:opacity-30'
                        aria-label='Move up'
                      >
                        <ChevronUp className='h-3.5 w-3.5' />
                      </button>
                      <button
                        onClick={() => draft.moveParam(spec.id, 1)}
                        disabled={i === job.params.length - 1}
                        className='rounded p-1 text-fg-subtle hover:bg-surface-hover hover:text-fg disabled:opacity-30'
                        aria-label='Move down'
                      >
                        <ChevronDown className='h-3.5 w-3.5' />
                      </button>
                      <button
                        onClick={() => {
                          setConfigId(open ? null : spec.id);
                          setPane('yaml');
                        }}
                        className={cn(
                          'rounded p-1 transition-colors',
                          open
                            ? 'bg-primary-subtle text-primary'
                            : 'text-fg-subtle hover:bg-surface-hover hover:text-fg'
                        )}
                        aria-label='Configure field'
                      >
                        <Settings2 className='h-3.5 w-3.5' />
                      </button>
                    </div>
                  </div>

                  {open ? (
                    <div className='rounded-lg border border-primary/30 bg-bg-subtle p-4'>
                      <ParamEditor
                        spec={spec}
                        onChange={(patch) => draft.updateParam(spec.id, patch)}
                        siblings={job.params.filter((p) => p.id !== spec.id).map((p) => p.name)}
                      />
                      <button
                        onClick={() => {
                          draft.removeParam(spec.id);
                          setConfigId(null);
                        }}
                        className='mt-3 inline-flex items-center gap-1.5 rounded border border-border px-2 py-1 font-mono text-xxs text-fg-muted hover:border-danger hover:text-danger'
                      >
                        <Trash2 className='h-3 w-3' /> remove parameter
                      </button>
                    </div>
                  ) : (
                    <RunField
                      spec={spec}
                      value={values[spec.name]}
                      onChange={(v) => setValue(spec.name, v)}
                      error={errors[spec.name]}
                      showEnv={false}
                    />
                  )}
                </div>
              );
            })}
          </div>

          <div className='px-4 py-3'>
            {adding ? (
              <div className='flex flex-wrap gap-1.5 rounded-lg border border-border bg-bg-subtle p-2'>
                {TYPE_ORDER.map((id) => {
                  const t = typeDef(id);
                  const Icon = t.icon;
                  return (
                    <button
                      key={id}
                      onClick={() => {
                        setConfigId(draft.addParam(id));
                        setAdding(false);
                      }}
                      className='inline-flex items-center gap-1.5 rounded-md border border-border px-2 py-1 text-xs text-fg-muted hover:border-primary hover:text-fg'
                    >
                      <Icon className='h-3.5 w-3.5' /> {t.label}
                    </button>
                  );
                })}
              </div>
            ) : (
              <button
                onClick={() => setAdding(true)}
                className='flex w-full items-center justify-center gap-2 rounded-md border border-dashed border-border py-2 text-xs text-fg-muted hover:border-primary hover:text-primary'
              >
                <Plus className='h-3.5 w-3.5' /> Add parameter
              </button>
            )}
          </div>

          <div className='flex flex-wrap items-center gap-3 rounded-b-xl border-t border-border bg-bg-subtle px-4 py-3'>
            <button
              onClick={run}
              className='inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-fg-onPrimary hover:bg-primary-hover'
            >
              <Play className='h-4 w-4' /> Run build
            </button>
            {dirty && (
              <button className='inline-flex items-center gap-2 rounded-md border border-warning px-3 py-2 text-sm text-warning-fg hover:bg-warning-subtle'>
                <GitCompare className='h-4 w-4' /> Commit schema change
              </button>
            )}
            {queued && (
              <span className='rounded bg-success-subtle px-2 py-1 font-mono text-xs text-success-fg'>
                queued #{queued}
              </span>
            )}
            {draft.submitted && !draft.valid && (
              <span className='ml-auto font-mono text-xs text-danger'>
                {Object.keys(draft.allErrors).length} field(s) need attention
              </span>
            )}
          </div>
        </div>

        {/* --- right: consequences ------------------------------------------ */}
        <div className='flex min-h-[34rem] flex-col'>
          <div className='mb-2 flex overflow-hidden rounded-md border border-border'>
            {PANES.map((p) => {
              const Icon = p.icon;
              return (
                <button
                  key={p.id}
                  onClick={() => setPane(p.id)}
                  className={cn(
                    'inline-flex flex-1 items-center justify-center gap-1.5 py-1.5 text-xs transition-colors',
                    pane === p.id
                      ? 'bg-primary text-fg-onPrimary'
                      : 'text-fg-muted hover:bg-surface-hover hover:text-fg'
                  )}
                >
                  <Icon className='h-3.5 w-3.5' />
                  {p.label}
                </button>
              );
            })}
          </div>

          {pane === 'yaml' && <YamlPane job={job} focusId={focusId} className='min-h-0 flex-1' />}

          {pane === 'env' && (
            <div className='min-h-0 flex-1 overflow-auto'>
              <EnvPane
                specs={job.params}
                values={values}
                command={job.command}
                runtime={job.runtime}
                focusName={focused?.name ?? null}
              />
              <p className='mt-2 px-1 text-xxs text-fg-subtle'>
                Exactly what the worker will export before running{' '}
                <code className='font-mono text-fg-muted'>{job.command}</code>. Secrets are
                resolved on the worker and never leave the store.
              </p>
              <div className='mt-3 grid grid-cols-2 gap-2'>
                {toEnvPairs(job.params, values).map((p) => (
                  <div
                    key={p.key}
                    className='truncate rounded border border-border bg-bg-subtle px-2 py-1 font-mono text-xxs'
                  >
                    <span className='text-info'>{p.key}</span>
                    <span className='text-fg-subtle'>=</span>
                    <span className={p.masked ? 'text-warning' : 'text-fg'}>
                      {p.masked ? '••••' : p.value || "''"}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {pane === 'api' && (
            <div className='min-h-0 flex-1 rounded-xl border border-border bg-bg-subtle'>
              <div className='border-b border-border px-3 py-2 font-mono text-xxs uppercase tracking-wider text-fg-muted'>
                equivalent request
              </div>
              <pre className='overflow-auto px-3 py-2.5 font-mono text-xs leading-relaxed text-fg'>
{`curl -X POST \\
  $CORE/api/public/v1/jobs/${job.slug}/builds \\
  -H 'X-API-Key: $LUTE_KEY' \\
  -H 'Idempotency-Key: '"$(uuidgen)" \\
  -d '${payload}'`}
              </pre>
              <p className='px-3 pb-3 text-xxs text-fg-subtle'>
                The panel and this call go through the same schema validation — anything the
                form rejects, the API rejects.
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
