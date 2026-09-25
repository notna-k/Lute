// The workbench's edit-mode columns: the configure panel, the parameter list, and the YAML to commit.
import { useState, type ReactNode } from 'react';
import {
  Check,
  ChevronDown,
  ChevronUp,
  Copy,
  Download,
  GripVertical,
  Plus,
  RotateCcw,
  Trash2,
} from 'lucide-react';
import { cn } from '@/lib/cn';
import { useCopy } from '@/hooks/useCopy';
import { ParamEditor } from '@/features/params/ParamEditor';
import { ParamField } from '@/features/params/ParamField';
import { typeDef } from '@/features/params/registry';
import type { DraftField, SchemaDraft as Draft } from '@/features/params/useSchemaDraft';
import { downloadYaml, type YamlDoc } from '@/features/params/yaml';
import { Button } from '@/components/ui/Button';
import { Card, CardHeader, CardTitle } from '@/components/ui/Card';
import { IconButton } from '@/components/ui/IconButton';
import type { JobDefinition } from '@/types/jobs';

export function ConfigurePanel({ draft, selected }: { draft: Draft; selected: DraftField | null }) {
  // No inner scroll container: it would trap the date picker and other popovers.
  return (
    <Card className='sticky top-0'>
      <CardHeader>
        <CardTitle>Configure{selected ? ` · ${selected.name}` : ''}</CardTitle>
      </CardHeader>
      <div className='p-4'>
        {selected ? (
          <ParamEditor
            field={selected}
            onChange={(patch) => draft.updateField(selected.id, patch)}
            siblings={draft.fields.filter((f) => f.id !== selected.id).map((f) => f.name)}
          />
        ) : (
          <p className='py-8 text-center text-[12.5px] text-fg-subtle'>
            Pick a parameter to configure it
          </p>
        )}
      </div>
    </Card>
  );
}

export function SchemaList({
  draft,
  selectedId,
  onSelect,
  driftNote,
}: {
  draft: Draft;
  selectedId: string | null;
  onSelect: (id: string | null) => void;
  driftNote: ReactNode;
}) {
  const { fields, values, setValue } = draft;
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [overIndex, setOverIndex] = useState<number | null>(null);

  function endDrag() {
    setDragIndex(null);
    setOverIndex(null);
  }

  function action(label: string, icon: ReactNode, run: () => void, disabled = false) {
    return (
      <IconButton
        label={label}
        variant='ghost'
        size='sm'
        disabled={disabled}
        onClick={(e) => {
          e.stopPropagation();
          run();
        }}
      >
        {icon}
      </IconButton>
    );
  }

  return (
    <Card>
      <CardHeader className='flex items-center gap-2.5'>
        <CardTitle>Parameters · click one to configure</CardTitle>
        {driftNote}
        <span className='ml-auto font-mono text-[11.5px] text-fg-subtle'>{fields.length}</span>
      </CardHeader>

      <div className='space-y-1 p-3'>
        {fields.map((field, i) => {
          const active = field.id === selectedId;
          const Icon = typeDef(field.type).icon;
          return (
            <div
              key={field.id}
              onClick={() => onSelect(field.id)}
              onDragOver={(e) => {
                e.preventDefault();
                setOverIndex(i);
              }}
              onDrop={() => {
                if (dragIndex !== null) draft.reorderField(dragIndex, i);
                endDrag();
              }}
              className={cn(
                'group relative cursor-pointer border p-3 pl-8 transition-colors',
                active
                  ? 'border-fg bg-bg-subtle'
                  : 'border-transparent hover:border-border hover:bg-surface-hover',
                dragIndex === i && 'opacity-40',
                overIndex === i && dragIndex !== null && dragIndex !== i && 'border-fg',
              )}
            >
              {/* Only the grip is draggable, so text selection inside the controls keeps working. */}
              <span
                draggable
                onDragStart={() => setDragIndex(i)}
                onDragEnd={endDrag}
                title='Drag to reorder'
                className='absolute left-1.5 top-3.5 cursor-grab text-fg-subtle opacity-0 transition-opacity group-hover:opacity-100'
              >
                <GripVertical className='h-4 w-4' />
              </span>

              <ParamField
                field={field}
                value={values[field.name]}
                onChange={(v) => setValue(field.name, v)}
              />

              <div
                className={cn(
                  'absolute right-2 top-2 flex items-center gap-0.5 border border-border bg-bg-elevated p-0.5 text-fg-subtle transition-opacity',
                  active ? 'opacity-100' : 'opacity-0 group-hover:opacity-100',
                )}
              >
                <span className='px-1'>
                  <Icon className='h-3.5 w-3.5' />
                </span>
                {action(
                  'Move up',
                  <ChevronUp className='h-3.5 w-3.5' />,
                  () => draft.moveField(field.id, -1),
                  i === 0,
                )}
                {action(
                  'Move down',
                  <ChevronDown className='h-3.5 w-3.5' />,
                  () => draft.moveField(field.id, 1),
                  i === fields.length - 1,
                )}
                {action('Duplicate', <Copy className='h-3.5 w-3.5' />, () =>
                  draft.duplicateField(field.id),
                )}
                {action('Remove', <Trash2 className='h-3.5 w-3.5' />, () => {
                  draft.removeField(field.id);
                  if (active) onSelect(null);
                })}
              </div>
            </div>
          );
        })}

        {fields.length === 0 && (
          <p className='border border-dashed border-border py-10 text-center text-[12.5px] text-fg-subtle'>
            No parameters yet
          </p>
        )}

        {/* A new parameter starts as text; its type is chosen in the configure panel. */}
        <button
          type='button'
          onClick={() => onSelect(draft.addField('string'))}
          className='flex w-full items-center justify-center gap-2 border border-dashed border-border py-3 text-[12.5px] text-fg-muted transition-colors hover:border-fg hover:text-fg'
        >
          <Plus className='h-3.5 w-3.5' /> Add input
        </button>
      </div>
    </Card>
  );
}

/** Minimal YAML colouring: enough to read, not a parser. */
function YamlLine({ text, highlight }: { text: string; highlight: boolean }) {
  const comment = text.trimStart().startsWith('#');
  const match = /^(\s*(?:- )?)([\w-]+)(:)(.*)$/.exec(text);
  return (
    <div className={cn('-mx-3 px-3', highlight && 'bg-warning-subtle')}>
      {comment ? (
        <span className='text-fg-subtle'>{text}</span>
      ) : match ? (
        <>
          <span>{match[1]}</span>
          <span className='text-fg-muted'>{match[2]}</span>
          <span className='text-fg-subtle'>{match[3]}</span>
          <span className='text-fg'>{match[4]}</span>
        </>
      ) : (
        <span className='text-fg'>{text || ' '}</span>
      )}
    </div>
  );
}

export function YamlPanel({
  job,
  doc,
  selectedName,
  dirty,
  onDiscard,
}: {
  job: JobDefinition;
  doc: YamlDoc;
  selectedName?: string;
  dirty: boolean;
  onDiscard: () => void;
}) {
  const [copied, copy] = useCopy();
  const range = selectedName ? doc.ranges[selectedName] : undefined;

  return (
    <div className='sticky top-0'>
      <Card className='bg-bg-subtle'>
        <CardHeader className='flex items-center gap-2'>
          <CardTitle className='truncate normal-case tracking-normal'>
            {job.source.path || `${job.slug}.yaml`}
          </CardTitle>
          <div className='ml-auto flex shrink-0 items-center gap-1.5'>
            <Button variant='outline' size='xs' onClick={() => copy(doc.text)}>
              {copied ? <Check className='h-3 w-3 text-success' /> : <Copy className='h-3 w-3' />}
              {copied ? 'copied' : 'copy'}
            </Button>
            <Button
              variant='outline'
              size='xs'
              title={`Download ${job.slug}.yaml`}
              onClick={() => downloadYaml(doc.text, `${job.slug}.yaml`)}
            >
              <Download className='h-3 w-3' /> .yaml
            </Button>
          </div>
        </CardHeader>
        <pre className='scrollbar-thin max-h-[30rem] overflow-auto px-3 py-2.5 font-mono text-[12px] leading-[1.6]'>
          {doc.text.split('\n').map((line, i) => (
            <YamlLine
              key={i}
              text={line}
              highlight={Boolean(range && i >= range[0] && i <= range[1])}
            />
          ))}
        </pre>
      </Card>

      <p className='mt-2 text-[11.5px] leading-relaxed text-fg-subtle'>
        Git is the source of truth. Commit this file to{' '}
        <span className='font-mono text-fg-muted'>
          {job.source.repo || 'the job-definitions repo'}
        </span>{' '}
        and Core picks the change up on its next sync.
      </p>
      {dirty && (
        <Button variant='outline' size='xs' className='mt-2' onClick={onDiscard}>
          <RotateCcw className='h-3 w-3' /> discard changes
        </Button>
      )}
    </div>
  );
}
