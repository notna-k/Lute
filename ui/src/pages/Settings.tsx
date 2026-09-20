/**
 * Operator settings.
 *
 * One layout throughout: a labelled row on the left, its control on the right.
 * Every setting reads the same way, and a new one is a row rather than a new card
 * with its own idea of how a form looks.
 */
import { useState, type ReactNode } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { KeyRound, Plus, Trash2 } from 'lucide-react';
import {
  Alert,
  Button,
  Card,
  CardHeader,
  CardTitle,
  EmptyState,
  Input,
  Kbd,
  PageHeader,
  SegmentedControl,
  Switch,
} from '@/components/ui';
import { PageBody, PageScroll, Section } from '@/components/layout';
import { useTheme, type ThemeMode } from '@/contexts/ThemeContext';
import { useUiPreferences, type Density } from '@/contexts/UiPreferencesContext';
import { getSettings, updateSettings } from '@/services/settingsService';
import {
  apiKeyService,
  type APIKeySummary,
  type CreateAPIKeyResponse,
} from '@/services/apiKeyService';
import { timestamp, toEpochMs } from '@/lib/format';

/** A labelled setting row: what it does on the left, the control on the right. */
function SettingRow({
  label,
  hint,
  children,
}: {
  label: ReactNode;
  hint?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className='flex flex-wrap items-start gap-4 border-b border-border-subtle px-4 py-3.5 last:border-b-0'>
      <div className='min-w-[14rem] flex-1'>
        <p className='text-[13px] font-medium text-fg'>{label}</p>
        {hint && (
          <p className='mt-0.5 max-w-[60ch] text-xs leading-relaxed text-fg-subtle'>
            {hint}
          </p>
        )}
      </div>
      <div className='flex shrink-0 items-center gap-2'>{children}</div>
    </div>
  );
}

function publicApiBase(): string {
  const configured = import.meta.env.VITE_API_URL;
  if (configured !== undefined && String(configured).trim() !== '') {
    return `${String(configured).replace(/\/$/, '')}/api/public/v1`;
  }
  if (typeof window !== 'undefined') {
    return `${window.location.origin}/api/public/v1`;
  }
  return '/api/public/v1';
}

export default function Settings() {
  const qc = useQueryClient();
  const [name, setName] = useState('');
  const [newToken, setNewToken] = useState<CreateAPIKeyResponse | null>(null);
  const { mode, setMode } = useTheme();
  const { density, setDensity, sidebarExpanded, setSidebarExpanded } =
    useUiPreferences();

  const keysQuery = useQuery({
    queryKey: ['api-keys'],
    queryFn: () => apiKeyService.list(),
  });
  const settingsQuery = useQuery({ queryKey: ['settings'], queryFn: getSettings });

  const settingsMut = useMutation({
    mutationFn: (allowAdhocBuilds: boolean) => updateSettings({ allowAdhocBuilds }),
    onSuccess: (data) => qc.setQueryData(['settings'], data),
  });

  const createMut = useMutation({
    mutationFn: (n: string) => apiKeyService.create(n),
    onSuccess: (data) => {
      setNewToken(data);
      setName('');
      void qc.invalidateQueries({ queryKey: ['api-keys'] });
    },
  });

  const revokeMut = useMutation({
    mutationFn: (id: string) => apiKeyService.revoke(id),
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['api-keys'] }),
  });

  const keys: APIKeySummary[] = keysQuery.data?.api_keys ?? [];

  return (
    <>
      <PageHeader
        title='Settings'
        description='How this panel looks, how builds are allowed to run, and who can call the API.'
      />

      <PageScroll>
        <PageBody className='max-w-[64rem]'>
          <Section title='Appearance'>
            <Card>
              <SettingRow
                label='Theme'
                hint='System follows your operating system’s setting.'
              >
                <SegmentedControl<ThemeMode>
                  label='Theme'
                  value={mode}
                  onChange={setMode}
                  options={[
                    { value: 'light', label: 'Light' },
                    { value: 'dark', label: 'Dark' },
                    { value: 'system', label: 'System' },
                  ]}
                />
              </SettingRow>
              <SettingRow
                label='Density'
                hint='Compact tightens table rows so more builds fit on screen.'
              >
                <SegmentedControl<Density>
                  label='Density'
                  value={density}
                  onChange={setDensity}
                  options={[
                    { value: 'roomy', label: 'Roomy' },
                    { value: 'compact', label: 'Compact' },
                  ]}
                />
              </SettingRow>
              <SettingRow
                label='Expanded sidebar'
                hint={
                  <>
                    Show labels beside the navigation icons. Toggle any time with{' '}
                    <Kbd>[</Kbd>.
                  </>
                }
              >
                <Switch
                  checked={sidebarExpanded}
                  onChange={(e) => setSidebarExpanded(e.target.checked)}
                  aria-label='Expanded sidebar'
                />
              </SettingRow>
            </Card>
          </Section>

          <Section title='Builds'>
            <Card>
              <SettingRow
                label='Allow ad-hoc builds'
                hint='Let the panel run a schema that differs from the committed definition — edited in the workbench, or authored here. Turn it off to require every build to come from Git.'
              >
                <Switch
                  checked={settingsQuery.data?.allowAdhocBuilds ?? true}
                  disabled={settingsQuery.isLoading || settingsMut.isPending}
                  onChange={(e) => settingsMut.mutate(e.target.checked)}
                  aria-label='Allow ad-hoc builds'
                />
              </SettingRow>
            </Card>
            {settingsMut.isError && (
              <Alert tone='danger' className='mt-3'>
                {settingsMut.error instanceof Error
                  ? settingsMut.error.message
                  : 'Could not save the setting.'}
              </Alert>
            )}
          </Section>

          <Section title='Public API'>
            <Card>
              <SettingRow
                label='Endpoint'
                hint={
                  <>
                    Send <code>Authorization: Bearer &lt;token&gt;</code> to this base
                    URL.
                  </>
                }
              >
                <code className='break-all border border-border bg-bg-subtle px-2 py-1 font-mono text-[11.5px]'>
                  {publicApiBase()}
                </code>
              </SettingRow>
              <SettingRow
                label='New key'
                hint='Name it after where it will live, so a revoke later is obvious.'
              >
                <Input
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder='e.g. CI, laptop, production'
                  disabled={createMut.isPending}
                  className='w-56'
                  aria-label='New key name'
                />
                <Button
                  variant='primary'
                  size='sm'
                  disabled={!name.trim() || createMut.isPending}
                  onClick={() => createMut.mutate(name.trim())}
                >
                  <Plus className='h-3.5 w-3.5' /> Create
                </Button>
              </SettingRow>
            </Card>

            {createMut.isError && (
              <Alert tone='danger' className='mt-3'>
                {createMut.error instanceof Error
                  ? createMut.error.message
                  : 'Create failed'}
              </Alert>
            )}

            {newToken && (
              <Alert tone='warning' title='Copy this key now' className='mt-3'>
                <p>The secret is shown once. Store it in a password manager.</p>
                <code className='mt-2 block break-all border border-border bg-bg px-2 py-1.5 font-mono text-[11.5px]'>
                  {newToken.token}
                </code>
                <Button
                  variant='secondary'
                  size='sm'
                  className='mt-3'
                  onClick={() => setNewToken(null)}
                >
                  I have saved it
                </Button>
              </Alert>
            )}
          </Section>

          <Section title='Your API keys'>
            {keysQuery.isError && (
              <Alert tone='danger' className='mb-3'>
                {keysQuery.error instanceof Error
                  ? keysQuery.error.message
                  : 'Failed to load API keys'}
              </Alert>
            )}
            {keysQuery.isLoading ? (
              <p className='text-[12.5px] text-fg-subtle'>Loading…</p>
            ) : keys.length === 0 ? (
              <Card>
                <div className='py-10'>
                  <EmptyState
                    icon={<KeyRound className='h-5 w-5' />}
                    title='No API keys yet'
                    description='Create one above to call the public API from scripts or CI.'
                  />
                </div>
              </Card>
            ) : (
              <Card>
                <CardHeader>
                  <CardTitle>{keys.length} keys</CardTitle>
                </CardHeader>
                {keys.map((k) => {
                  const created = toEpochMs(k.created_at);
                  const used = toEpochMs(k.last_used_at);
                  return (
                    <SettingRow
                      key={k.id}
                      label={
                        <span className='flex items-center gap-2'>
                          {k.name}
                          <span className='font-mono text-[11.5px] text-fg-subtle'>
                            {k.prefix}
                          </span>
                          {k.revoked && (
                            <span className='text-[11.5px] text-fg-subtle'>revoked</span>
                          )}
                        </span>
                      }
                      hint={
                        <>
                          Created {created ? timestamp(created) : '—'}
                          {used ? ` · last used ${timestamp(used)}` : ' · never used'}
                        </>
                      }
                    >
                      {!k.revoked && (
                        <Button
                          variant='danger'
                          size='sm'
                          disabled={revokeMut.isPending}
                          // Revoking cannot be undone, and whatever is holding
                          // the key stops working the moment it lands.
                          onClick={() => {
                            if (
                              window.confirm(
                                `Revoke "${k.name}"? Anything using it stops working immediately.`
                              )
                            ) {
                              revokeMut.mutate(k.id);
                            }
                          }}
                        >
                          <Trash2 className='h-3.5 w-3.5' /> Revoke
                        </Button>
                      )}
                    </SettingRow>
                  );
                })}
              </Card>
            )}
            {revokeMut.isError && (
              <Alert tone='danger' className='mt-3'>
                {revokeMut.error instanceof Error
                  ? revokeMut.error.message
                  : 'Revoke failed'}
              </Alert>
            )}
          </Section>
        </PageBody>
      </PageScroll>
    </>
  );
}
