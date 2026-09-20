import { useEffect, useState } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import {
  Alert,
  Button,
  Dialog,
  Field,
  IconButton,
  Input,
  Select,
  Textarea,
} from '@/components/ui';
import { jobService, type EnqueueRequest } from '@/services/jobService';

const RUNTIME_OPTIONS = [
  { label: 'Node 25', value: 'node:25' },
  { label: 'Python:3.13.0 - Latest stable', value: 'python:3.13' },
  { label: 'Python:3.12.0 - LTS', value: 'python:3.12' },
  { label: 'Python:3.11.0 - LTS', value: 'python:3.11' },
  { label: 'Python:2.7.18 - LTS (legacy)', value: 'python:2.7' },
  { label: 'JVM:25.0.1 - Latest stable', value: 'eclipse-temurin:25-jdk' },
  { label: 'JVM:21.0.9 - LTS', value: 'eclipse-temurin:21-jdk' },
  { label: 'JVM:17.0.17 - LTS', value: 'eclipse-temurin:17-jdk' },
  { label: 'JVM:11.0.29 - LTS', value: 'eclipse-temurin:11-jdk' },
  { label: 'JVM:8u471 - LTS', value: 'eclipse-temurin:8-jdk' },
  { label: 'JVM:7 - Legacy', value: 'eclipse-temurin:7-jdk' },
];

const TYPE_OPTIONS = [
  { value: 'container', label: 'container' },
  { value: 'noop', label: 'noop' },
];

type ParamRow = { key: string; value: string };

const INITIAL_FORM: EnqueueRequest = {
  queue: 'default',
  type: 'container',
  timeout_sec: 300,
  max_retries: 3,
};

const makeRow = (): ParamRow => ({ key: '', value: '' });

export interface EnqueueJobDialogProps {
  open: boolean;
  onClose: () => void;
  defaultQueue?: string;
  onEnqueued: (jobId: string) => void;
}

export function EnqueueJobDialog({
  open,
  onClose,
  defaultQueue = 'default',
  onEnqueued,
}: EnqueueJobDialogProps) {
  const [form, setForm] = useState<EnqueueRequest>({
    ...INITIAL_FORM,
    queue: defaultQueue,
  });
  const [sourceRepository, setSourceRepository] = useState('');
  const [runtime, setRuntime] = useState('python:3.12');
  const [command, setCommand] = useState('');
  const [paramsRows, setParamsRows] = useState<ParamRow[]>([makeRow()]);
  const [selectorRows, setSelectorRows] = useState<ParamRow[]>([]);
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setForm({ ...INITIAL_FORM, queue: defaultQueue });
    setSourceRepository('');
    setRuntime('python:3.12');
    setCommand('');
    setParamsRows([makeRow()]);
    setSelectorRows([]);
    setSubmitError(null);
  }, [open, defaultQueue]);

  const addRow = () => setParamsRows((r) => [...r, makeRow()]);
  const removeRow = (i: number) =>
    setParamsRows((r) => r.filter((_, idx) => idx !== i));
  const updateRow = (i: number, field: 'key' | 'value', value: string) =>
    setParamsRows((r) =>
      r.map((row, idx) => (idx === i ? { ...row, [field]: value } : row))
    );

  const addSelectorRow = () => setSelectorRows((r) => [...r, makeRow()]);
  const removeSelectorRow = (i: number) =>
    setSelectorRows((r) => r.filter((_, idx) => idx !== i));
  const updateSelectorRow = (i: number, field: 'key' | 'value', value: string) =>
    setSelectorRows((r) =>
      r.map((row, idx) => (idx === i ? { ...row, [field]: value } : row))
    );

  const buildSelector = (): Record<string, string> | undefined => {
    const sel: Record<string, string> = {};
    selectorRows.forEach(({ key, value }) => {
      if (key.trim()) sel[key.trim()] = value;
    });
    return Object.keys(sel).length > 0 ? sel : undefined;
  };

  const buildPayload = (): unknown => {
    if (form.type === 'noop') return {};
    const request_params: Record<string, string> = {};
    paramsRows.forEach(({ key, value }) => {
      if (key.trim()) request_params[key.trim()] = value;
    });
    return {
      source_repository: sourceRepository.trim() || undefined,
      runtime: runtime.trim() || undefined,
      command: command.trim() || undefined,
      request_params,
    };
  };

  const handleSubmit = async () => {
    setSubmitting(true);
    setSubmitError(null);
    try {
      const payload = form.type === 'noop' ? {} : buildPayload();
      const res = await jobService.enqueueJob({ ...form, payload, selector: buildSelector() });
      onClose();
      onEnqueued(res.job_id);
    } catch (e) {
      setSubmitError(e instanceof Error ? e.message : 'Failed to enqueue job');
    } finally {
      setSubmitting(false);
    }
  };

  const disabled =
    submitting ||
    !form.queue ||
    !form.type ||
    (form.type === 'container' && (!runtime.trim() || !command.trim()));

  return (
    <Dialog
      open={open}
      onClose={onClose}
      size='lg'
      title='Trigger new job'
      description='Enqueue a job for a worker to pick up.'
      footer={
        <>
          <Button variant='ghost' onClick={onClose} disabled={submitting}>
            Cancel
          </Button>
          <Button
            onClick={handleSubmit}
            loading={submitting}
            disabled={disabled}
          >
            Trigger
          </Button>
        </>
      }
    >
      <div className='flex flex-col gap-4'>
        {submitError && <Alert tone='danger'>{submitError}</Alert>}

        <div className='grid gap-4 sm:grid-cols-2'>
          <Field label='Queue' required htmlFor='queue'>
            <Input
              id='queue'
              value={form.queue}
              onChange={(e) =>
                setForm((f) => ({ ...f, queue: e.target.value }))
              }
            />
          </Field>
          <Field label='Type' required>
            <Select
              value={form.type}
              onChange={(v) => setForm((f) => ({ ...f, type: v }))}
              options={TYPE_OPTIONS}
            />
          </Field>
        </div>

        {form.type === 'container' && (
          <>
            <Field
              label='Source repository'
              hint='Optional — https://github.com/owner/repo.git'
              htmlFor='repo'
            >
              <Input
                id='repo'
                value={sourceRepository}
                onChange={(e) => setSourceRepository(e.target.value)}
                placeholder='Optional git URL'
              />
            </Field>
            <Field label='Runtime' required>
              <Select
                value={runtime}
                onChange={setRuntime}
                options={RUNTIME_OPTIONS}
              />
            </Field>
            <Field label='Command' required htmlFor='cmd'>
              <Textarea
                id='cmd'
                value={command}
                onChange={(e) => setCommand(e.target.value)}
                rows={3}
                placeholder='bash script to run in the container'
              />
            </Field>

            <div>
              <p className='mb-2 text-sm font-medium text-fg'>
                Params (key-value → env vars)
              </p>
              <div className='flex flex-col gap-2'>
                {paramsRows.map((row, i) => (
                  <div key={i} className='flex items-center gap-2'>
                    <Input
                      placeholder='Key'
                      value={row.key}
                      onChange={(e) => updateRow(i, 'key', e.target.value)}
                      className='flex-1'
                    />
                    <Input
                      placeholder='Value'
                      value={row.value}
                      onChange={(e) => updateRow(i, 'value', e.target.value)}
                      className='flex-1'
                    />
                    <IconButton
                      label='Remove row'
                      variant='ghost'
                      onClick={() => removeRow(i)}
                      className='text-danger hover:bg-danger/10'
                    >
                      <Trash2 className='h-4 w-4' />
                    </IconButton>
                  </div>
                ))}
              </div>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                leftIcon={<Plus className='h-4 w-4' />}
                onClick={addRow}
                className='mt-2'
              >
                Add row
              </Button>
            </div>
          </>
        )}

        <div>
          <p className='mb-1 text-sm font-medium text-fg'>
            Worker selector{' '}
            <span className='font-normal text-fg-muted'>(optional — route to labelled workers only)</span>
          </p>
          {selectorRows.length > 0 && (
            <div className='mb-2 flex flex-col gap-2'>
              {selectorRows.map((row, i) => (
                <div key={i} className='flex items-center gap-2'>
                  <Input
                    placeholder='Label key'
                    value={row.key}
                    onChange={(e) => updateSelectorRow(i, 'key', e.target.value)}
                    className='flex-1'
                  />
                  <Input
                    placeholder='Label value'
                    value={row.value}
                    onChange={(e) => updateSelectorRow(i, 'value', e.target.value)}
                    className='flex-1'
                  />
                  <IconButton
                    label='Remove selector'
                    variant='ghost'
                    onClick={() => removeSelectorRow(i)}
                    className='text-danger hover:bg-danger/10'
                  >
                    <Trash2 className='h-4 w-4' />
                  </IconButton>
                </div>
              ))}
            </div>
          )}
          <Button
            type='button'
            variant='ghost'
            size='sm'
            leftIcon={<Plus className='h-4 w-4' />}
            onClick={addSelectorRow}
          >
            Add selector
          </Button>
        </div>

        <div className='grid gap-4 sm:grid-cols-2'>
          <Field label='Timeout (sec)' htmlFor='to'>
            <Input
              id='to'
              type='number'
              value={form.timeout_sec ?? 300}
              onChange={(e) =>
                setForm((f) => ({
                  ...f,
                  timeout_sec: Number(e.target.value),
                }))
              }
            />
          </Field>
          <Field label='Max retries' htmlFor='mr'>
            <Input
              id='mr'
              type='number'
              value={form.max_retries ?? 3}
              onChange={(e) =>
                setForm((f) => ({
                  ...f,
                  max_retries: Number(e.target.value),
                }))
              }
            />
          </Field>
        </div>
      </div>
    </Dialog>
  );
}

export default EnqueueJobDialog;
