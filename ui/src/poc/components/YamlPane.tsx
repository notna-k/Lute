/**
 * The YAML the panel would have you commit — the single artifact of the
 * schema builder (PRODUCT.md §6). Kept read-only on purpose: Git is the source
 * of truth, so the panel's job is to produce a diff, not to own the file.
 */
import { useMemo, useState } from 'react';
import { Check, Copy, Download } from 'lucide-react';
import { cn } from '@/lib/cn';
import { toYaml } from '../params/yaml';
import type { JobDef } from '../params/types';

/** Minimal YAML colouring — enough to read, not a parser. */
function Line({ text, highlight }: { text: string; highlight: boolean }) {
  const comment = text.trimStart().startsWith('#');
  const match = /^(\s*(?:- )?)([\w-]+)(:)(.*)$/.exec(text);
  return (
    <div className={cn('px-3 -mx-3', highlight && 'bg-primary-subtle')}>
      {comment ? (
        <span className='text-fg-subtle'>{text}</span>
      ) : match ? (
        <>
          <span>{match[1]}</span>
          <span className='text-info'>{match[2]}</span>
          <span className='text-fg-subtle'>{match[3]}</span>
          <span className='text-fg'>{match[4]}</span>
        </>
      ) : (
        <span className='text-fg'>{text || ' '}</span>
      )}
    </div>
  );
}

export interface YamlPaneProps {
  job: JobDef;
  /** Param id whose lines should be highlighted. */
  focusId?: string | null;
  className?: string;
  title?: string;
  /** When set, offers the document as a file download under this name. */
  downloadName?: string;
}

export function YamlPane({ job, focusId, className, title, downloadName }: YamlPaneProps) {
  const doc = useMemo(() => toYaml(job), [job]);
  const [copied, setCopied] = useState(false);
  const range = focusId ? doc.ranges[focusId] : undefined;

  async function copy() {
    await navigator.clipboard.writeText(doc.text);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  function download() {
    const url = URL.createObjectURL(new Blob([`${doc.text}\n`], { type: 'text/yaml' }));
    const a = document.createElement('a');
    a.href = url;
    a.download = downloadName ?? 'job.yaml';
    a.click();
    URL.revokeObjectURL(url);
  }

  return (
    <div className={cn('flex min-h-0 flex-col rounded-xl border border-border bg-bg-subtle', className)}>
      <div className='flex shrink-0 items-center gap-2 border-b border-border px-3 py-2'>
        <span className='font-mono text-xxs uppercase tracking-wider text-fg-muted'>
          {title ?? job.source.path}
        </span>
        <div className='ml-auto flex items-center gap-1.5'>
          <button
            onClick={copy}
            className='inline-flex items-center gap-1.5 rounded border border-border px-2 py-1 font-mono text-xxs text-fg-muted hover:border-border-strong hover:text-fg'
          >
            {copied ? <Check className='h-3 w-3 text-success' /> : <Copy className='h-3 w-3' />}
            {copied ? 'copied' : 'copy'}
          </button>
          {downloadName && (
            <button
              onClick={download}
              title={`Download ${downloadName}`}
              className='inline-flex items-center gap-1.5 rounded border border-border px-2 py-1 font-mono text-xxs text-fg-muted hover:border-border-strong hover:text-fg'
            >
              <Download className='h-3 w-3' />
              .yaml
            </button>
          )}
        </div>
      </div>
      <pre className='min-h-0 flex-1 overflow-auto px-3 py-2.5 font-mono text-xs leading-[1.6]'>
        {doc.text.split('\n').map((line, i) => (
          <Line
            key={i}
            text={line}
            highlight={Boolean(range && i >= range[0] && i <= range[1])}
          />
        ))}
      </pre>
    </div>
  );
}
