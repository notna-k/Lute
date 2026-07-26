import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { KeyRound, Plus, Trash2 } from 'lucide-react';
import {
  Alert,
  Button,
  Card,
  EmptyState,
  Input,
  PageHeader,
} from '@/components/ui';
import { getSettings, updateSettings } from '@/services/settingsService';
import {
  apiKeyService,
  type APIKeySummary,
  type CreateAPIKeyResponse,
} from '@/services/apiKeyService';

function publicApiBase(): string {
  const env = import.meta.env.VITE_API_URL;
  if (env !== undefined && env !== null && String(env).trim() !== '') {
    return `${String(env).replace(/\/$/, '')}/api/public/v1`;
  }
  if (typeof window !== 'undefined') {
    return `${window.location.origin}/api/public/v1`;
  }
  return '/api/public/v1';
}

const Settings = () => {
  const qc = useQueryClient();
  const [name, setName] = useState('');
  const [newToken, setNewToken] = useState<CreateAPIKeyResponse | null>(null);

  const keysQuery = useQuery({
    queryKey: ['api-keys'],
    queryFn: () => apiKeyService.list(),
  });

  const createMut = useMutation({
    mutationFn: (n: string) => apiKeyService.create(n),
    onSuccess: (data) => {
      setNewToken(data);
      setName('');
      void qc.invalidateQueries({ queryKey: ['api-keys'] });
    },
  });

  const settingsQuery = useQuery({
    queryKey: ['settings'],
    queryFn: getSettings,
  });

  const settingsMut = useMutation({
    mutationFn: (allowAdhocBuilds: boolean) => updateSettings({ allowAdhocBuilds }),
    onSuccess: (data) => qc.setQueryData(['settings'], data),
  });

  const revokeMut = useMutation({
    mutationFn: (id: string) => apiKeyService.revoke(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['api-keys'] });
    },
  });

  const keys: APIKeySummary[] = keysQuery.data?.api_keys ?? [];

  return (
    <>
      <PageHeader
        title='Settings'
        description='API keys for the public REST API (runs, workers, and integrations).'
      />

      <Card className='mb-6 border-border bg-surface p-4 sm:p-5'>
        <h2 className='text-sm font-semibold text-fg'>Public API</h2>
        <p className='mt-1 text-sm text-fg-muted'>
          Send{' '}
          <code className='rounded bg-bg px-1 py-0.5 text-xs'>
            Authorization: Bearer &lt;token&gt;
          </code>{' '}
          to:
        </p>
        <code className='mt-2 block break-all rounded-md bg-bg px-2 py-1.5 text-xs text-fg'>
          {publicApiBase()}
        </code>
      </Card>

      <Card className='mb-6 border-border bg-surface p-4 sm:p-5'>
        <h2 className='text-sm font-semibold text-fg'>Builds</h2>
        <label className='mt-3 flex items-start gap-3'>
          <input
            type='checkbox'
            className='mt-0.5 h-4 w-4 shrink-0 accent-accent'
            checked={settingsQuery.data?.allowAdhocBuilds ?? true}
            disabled={settingsQuery.isLoading || settingsMut.isPending}
            onChange={(e) => settingsMut.mutate(e.target.checked)}
          />
          <span>
            <span className='block text-sm font-medium text-fg'>Allow ad-hoc builds</span>
            <span className='mt-0.5 block text-sm text-fg-muted'>
              Let the panel run templates that differ from Git — edited in the
              workbench, or created from scratch. Turn this off to require every
              build to come from a committed definition.
            </span>
          </span>
        </label>
        {settingsMut.isError && (
          <Alert tone='danger' className='mt-3'>
            {settingsMut.error instanceof Error
              ? settingsMut.error.message
              : 'Could not save the setting.'}
          </Alert>
        )}
      </Card>

      {newToken && (
        <Alert tone='warning' title='Copy your new key now' className='mb-6'>
          <p>
            This secret is shown only once. Store it in a password manager or
            secret store.
          </p>
          <code className='mt-2 block break-all rounded bg-bg px-2 py-1.5 text-xs'>
            {newToken.token}
          </code>
          <Button
            variant='secondary'
            className='mt-3'
            type='button'
            onClick={() => setNewToken(null)}
          >
            I have saved it
          </Button>
        </Alert>
      )}

      {keysQuery.isError && (
        <Alert tone='danger' className='mb-4'>
          {keysQuery.error instanceof Error
            ? keysQuery.error.message
            : 'Failed to load API keys'}
        </Alert>
      )}

      <Card className='border-border bg-surface p-4 sm:p-5'>
        <div className='flex flex-col gap-4 sm:flex-row sm:items-end'>
          <div className='min-w-0 flex-1'>
            <label htmlFor='key-name' className='text-sm font-medium text-fg'>
              New key name
            </label>
            <Input
              id='key-name'
              className='mt-1'
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder='e.g. CI, laptop, production'
              disabled={createMut.isPending}
            />
          </div>
          <Button
            type='button'
            leftIcon={<Plus className='h-4 w-4' />}
            disabled={!name.trim() || createMut.isPending}
            onClick={() => createMut.mutate(name.trim())}
          >
            Create key
          </Button>
        </div>
        {createMut.isError && (
          <Alert tone='danger' className='mt-3'>
            {createMut.error instanceof Error
              ? createMut.error.message
              : 'Create failed'}
          </Alert>
        )}
      </Card>

      <section className='mt-8'>
        <h2 className='mb-3 text-sm font-semibold text-fg'>Your API keys</h2>
        {keysQuery.isLoading ? (
          <p className='text-sm text-fg-muted'>Loading…</p>
        ) : keys.length === 0 ? (
          <EmptyState
            icon={<KeyRound className='h-5 w-5' />}
            title='No API keys yet'
            description='Create a key to call the public API from scripts or tools.'
          />
        ) : (
          <ul className='divide-y divide-border rounded-lg border border-border bg-surface'>
            {keys.map((k) => (
              <li
                key={k.id}
                className='flex flex-col gap-2 px-4 py-3 sm:flex-row sm:items-center sm:justify-between'
              >
                <div className='min-w-0'>
                  <p className='font-medium text-fg'>{k.name}</p>
                  <p className='truncate text-xs text-fg-muted'>
                    <span className='font-mono'>{k.prefix}</span>
                    {k.revoked ? (
                      <span className='ml-2 text-fg-muted'>Revoked</span>
                    ) : null}
                  </p>
                  <p className='text-xs text-fg-muted'>
                    Created {new Date(k.created_at).toLocaleString()}
                    {k.last_used_at
                      ? ` · Last used ${new Date(k.last_used_at).toLocaleString()}`
                      : ''}
                  </p>
                </div>
                {!k.revoked && (
                  <Button
                    type='button'
                    variant='danger'
                    leftIcon={<Trash2 className='h-4 w-4' />}
                    disabled={revokeMut.isPending}
                    onClick={() => {
                      if (
                        window.confirm(
                          `Revoke key "${k.name}"? Integrations using it will stop working.`,
                        )
                      ) {
                        revokeMut.mutate(k.id);
                      }
                    }}
                  >
                    Revoke
                  </Button>
                )}
              </li>
            ))}
          </ul>
        )}
        {revokeMut.isError && (
          <Alert tone='danger' className='mt-3'>
            {revokeMut.error instanceof Error
              ? revokeMut.error.message
              : 'Revoke failed'}
          </Alert>
        )}
      </section>
    </>
  );
};

export default Settings;
