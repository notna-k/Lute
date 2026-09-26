// Enrols a host: pick or create a registration token, then copy the `docker run` for it.
import { useEffect, useState } from 'react';
import { Check, Copy, Plus, Terminal } from 'lucide-react';
import { useCopy } from '@/hooks/useCopy';
import { useCreateWorkerToken, useInstallInfo, useWorkerTokens } from '@/hooks/useWorkers';
import { Alert } from '@/components/ui/Alert';
import { Button } from '@/components/ui/Button';
import { Dialog } from '@/components/ui/Dialog';
import { IconButton } from '@/components/ui/IconButton';
import { Field, Input } from '@/components/ui/Input';
import { SegmentedControl } from '@/components/ui/SegmentedControl';
import { Select } from '@/components/ui/Select';
import { Tooltip } from '@/components/ui/Tooltip';
import { maskToken, runCommand } from './utils';

interface AddWorkerDialogProps {
  open: boolean;
  onClose: () => void;
}

type TokenSource = 'new' | 'existing';

export function AddWorkerDialog({ open, onClose }: AddWorkerDialogProps) {
  const [source, setSource] = useState<TokenSource>('new');
  const [tokenName, setTokenName] = useState('');
  const [created, setCreated] = useState<{ id: string; token: string; prefix: string } | null>(
    null,
  );
  const [existingId, setExistingId] = useState('');
  const [workerName, setWorkerName] = useState('');
  const [copied, copy] = useCopy();

  const install = useInstallInfo(open);
  const tokens = useWorkerTokens();
  const create = useCreateWorkerToken();
  const active = (tokens.data ?? []).filter((t) => !t.revoked_at);

  useEffect(() => {
    if (!open) return;
    setCreated(null);
    setTokenName('');
    setWorkerName('');
    create.reset();
    // eslint-disable-next-line react-hooks/exhaustive-deps -- reset once per opening
  }, [open]);

  useEffect(() => {
    if (source === 'existing' && !existingId && active.length) setExistingId(active[0].id);
  }, [source, existingId, active]);

  const existing = active.find((t) => t.id === existingId);
  // A new token goes into the copied command in full but stays masked on screen; for an
  // existing one the panel only knows its start, so the user pastes the rest.
  const token =
    source === 'new' ? created?.token : existing ? `<your ${existing.prefix}… token>` : undefined;
  const shown = source === 'new' && token ? maskToken(token, created?.prefix.length) : token;
  const build = (t: string) =>
    install.data
      ? runCommand({ ...install.data, token: t, name: workerName.trim() || undefined })
      : '';
  const command = token ? build(token) : '';
  const display = shown ? build(shown) : '';

  return (
    <Dialog
      open={open}
      onClose={onClose}
      size='lg'
      title={
        <span className='flex items-center gap-2'>
          <Terminal className='h-5 w-5 text-primary' />
          Add a worker
        </span>
      }
      description='A worker is a container on a Linux host with rootless Docker. It registers itself with a token on its first start and keeps its identity in the mounted directory.'
      footer={
        <Button variant='secondary' onClick={onClose}>
          Close
        </Button>
      }
    >
      <div className='flex flex-col gap-4'>
        <Field label='Registration token'>
          <SegmentedControl<TokenSource>
            label='Token'
            value={source}
            onChange={setSource}
            options={[
              { value: 'new', label: 'New token' },
              { value: 'existing', label: 'Existing token' },
            ]}
          />
        </Field>

        {source === 'new' && !created && (
          <div className='flex items-end gap-2'>
            <Field
              label='Token name'
              htmlFor='token-name'
              hint='Name it after the machines it will enrol.'
              className='flex-1'
            >
              <Input
                id='token-name'
                value={tokenName}
                onChange={(e) => setTokenName(e.target.value)}
                placeholder='e.g. build-farm'
              />
            </Field>
            <Button
              variant='primary'
              disabled={!tokenName.trim() || create.isPending}
              loading={create.isPending}
              onClick={() =>
                create.mutate(tokenName.trim(), {
                  onSuccess: (t) => setCreated({ id: t.id, token: t.token, prefix: t.prefix }),
                })
              }
              className='mb-[22px]'
            >
              <Plus className='h-3.5 w-3.5' /> Create
            </Button>
          </div>
        )}
        {create.isError && (
          <Alert tone='danger' title='Could not create a token'>
            {create.error instanceof Error ? create.error.message : 'Unknown error'}
          </Alert>
        )}
        {source === 'new' && created && (
          <Alert tone='warning' title='Copy the command now'>
            It carries the new token, hidden on screen. Lute keeps only a hash, so once this dialog
            closes the token cannot be copied again.
          </Alert>
        )}

        {source === 'existing' &&
          (active.length ? (
            <Field
              label='Token'
              hint='Only a token’s start is stored. Paste the full token you saved into LUTE_TOKEN.'
            >
              <Select
                value={existingId}
                onChange={setExistingId}
                options={active.map((t) => ({
                  value: t.id,
                  label: (
                    <span className='flex gap-2'>
                      {t.name}
                      <span className='font-mono text-fg-subtle'>{t.prefix}…</span>
                    </span>
                  ),
                }))}
              />
            </Field>
          ) : (
            <p className='text-sm text-fg-muted'>No active tokens. Create a new one.</p>
          ))}

        <Field
          label='Worker name'
          htmlFor='worker-name'
          code='LUTE_NAME'
          hint='Optional. Defaults to the host name; names are unique across Lute.'
        >
          <Input
            id='worker-name'
            value={workerName}
            onChange={(e) => setWorkerName(e.target.value)}
            placeholder='the host name'
          />
        </Field>

        {command && (
          <div className='relative rounded-md border border-border bg-bg-inverse p-3 font-mono text-[12.5px] text-fg-inverse'>
            <pre className='overflow-x-auto whitespace-pre pr-8'>{display}</pre>
            <div className='absolute right-2 top-2'>
              <Tooltip content={copied ? 'Copied!' : 'Copy'}>
                <IconButton
                  label='Copy command'
                  variant='ghost'
                  size='sm'
                  onClick={() => void copy(command)}
                  className='text-fg-inverse hover:bg-white/10'
                >
                  {copied ? (
                    <Check className='h-4 w-4 text-success' />
                  ) : (
                    <Copy className='h-4 w-4' />
                  )}
                </IconButton>
              </Tooltip>
            </div>
          </div>
        )}

        <p className='text-sm text-fg-muted'>
          Paste it into a shell as the unprivileged user that owns rootless Docker. Host setup is in{' '}
          <code className='font-mono text-[12px]'>docs/worker.md</code>.
        </p>
      </div>
    </Dialog>
  );
}
