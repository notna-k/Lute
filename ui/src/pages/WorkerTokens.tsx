// Registration tokens: what lets a new host enrol itself as a worker.
import { useState } from 'react';
import { Link } from 'react-router-dom';
import { KeyRound, Plus, Trash2 } from 'lucide-react';
import { useCreateWorkerToken, useRevokeWorkerToken, useWorkerTokens } from '@/hooks/useWorkers';
import { Alert } from '@/components/ui/Alert';
import { Button } from '@/components/ui/Button';
import { EmptyState } from '@/components/ui/EmptyState';
import { Input } from '@/components/ui/Input';
import { PageHeader } from '@/components/ui/PageHeader';
import { TBody, Table, Td, Th, THead, Tr } from '@/components/ui/Table';
import { PageBody, PageScroll } from '@/components/layout/Page';
import { relativeTime, toEpochMs } from '@/lib/format';

export default function WorkerTokens() {
  const [name, setName] = useState('');
  const [plaintext, setPlaintext] = useState<string | null>(null);
  const tokens = useWorkerTokens();
  const create = useCreateWorkerToken();
  const revoke = useRevokeWorkerToken();
  const list = tokens.data ?? [];

  return (
    <>
      <PageHeader
        breadcrumb={
          <Link to='/workers' className='hover:text-fg'>
            Workers
          </Link>
        }
        title='Registration tokens'
        description='A worker presents a token once, on its first start, and gets its own secret back. Revoking a token stops new registrations; workers that already registered keep working.'
      />
      <PageScroll>
        <PageBody className='max-w-[64rem] space-y-4'>
          <div className='flex flex-wrap items-center gap-2'>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder='Token name, e.g. build-farm'
              aria-label='New token name'
              className='w-64'
            />
            <Button
              variant='primary'
              size='sm'
              disabled={!name.trim() || create.isPending}
              onClick={() =>
                create.mutate(name.trim(), {
                  onSuccess: (t) => {
                    setPlaintext(t.token);
                    setName('');
                  },
                })
              }
            >
              <Plus className='h-3.5 w-3.5' /> Create token
            </Button>
          </div>

          {create.isError && (
            <Alert tone='danger'>
              {create.error instanceof Error ? create.error.message : 'Create failed'}
            </Alert>
          )}
          {plaintext && (
            <Alert tone='warning' title='Copy this token now'>
              <p>It is shown once. Pass it to a new worker as LUTE_TOKEN.</p>
              <code className='mt-2 block break-all border border-border bg-bg px-2 py-1.5 font-mono text-[11.5px]'>
                {plaintext}
              </code>
              <Button
                variant='secondary'
                size='sm'
                className='mt-3'
                onClick={() => setPlaintext(null)}
              >
                I have saved it
              </Button>
            </Alert>
          )}
          {tokens.isError && (
            <Alert tone='danger' title='Failed to load tokens'>
              {tokens.error instanceof Error ? tokens.error.message : 'Unknown error'}
            </Alert>
          )}
          {revoke.isError && (
            <Alert tone='danger'>
              {revoke.error instanceof Error ? revoke.error.message : 'Revoke failed'}
            </Alert>
          )}

          {!tokens.isLoading && list.length === 0 ? (
            <div className='py-12'>
              <EmptyState
                icon={<KeyRound className='h-5 w-5' />}
                title='No registration tokens'
                description='Create one to enrol your first worker.'
              />
            </div>
          ) : (
            <Table>
              <THead>
                <Tr>
                  <Th>Name</Th>
                  <Th>Token</Th>
                  <Th>Created</Th>
                  <Th>Last used</Th>
                  <Th className='text-right'>State</Th>
                </Tr>
              </THead>
              <TBody>
                {list.map((t) => {
                  const used = toEpochMs(t.last_used_at);
                  const created = toEpochMs(t.created_at);
                  return (
                    <Tr key={t.id}>
                      <Td className='font-medium'>{t.name}</Td>
                      <Td className='font-mono text-fg-subtle'>{t.prefix}…</Td>
                      <Td className='text-fg-muted tabular-nums'>
                        {created ? relativeTime(created) : '—'}
                      </Td>
                      <Td className='text-fg-muted tabular-nums'>
                        {used ? relativeTime(used) : 'never'}
                      </Td>
                      <Td className='text-right'>
                        {t.revoked_at ? (
                          <span className='text-fg-subtle'>revoked</span>
                        ) : (
                          <Button
                            variant='danger'
                            size='xs'
                            disabled={revoke.isPending}
                            onClick={() => {
                              if (
                                window.confirm(
                                  `Revoke "${t.name}"? New workers can no longer register with it.`,
                                )
                              ) {
                                revoke.mutate(t.id);
                              }
                            }}
                          >
                            <Trash2 className='h-3 w-3' /> Revoke
                          </Button>
                        )}
                      </Td>
                    </Tr>
                  );
                })}
              </TBody>
            </Table>
          )}
        </PageBody>
      </PageScroll>
    </>
  );
}
