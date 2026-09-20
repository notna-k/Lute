/**
 * One worker: what it is labelled with, and what it has been doing.
 *
 * Two tabs rather than one long scroll — labels are edited rarely and read
 * often, metrics are the opposite, and stacking them meant scrolling past the
 * editor every time to reach a chart.
 */
import { useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { Power } from 'lucide-react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useReEnableWorker, useWorker } from '@/hooks/useWorkers';
import { useDashboardUptime } from '@/hooks/useDashboard';
import type { ChartPoint, DashboardUptimePeriod } from '@/services/dashboardService';
import {
  Alert,
  Button,
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
  Fact,
  SegmentedControl,
  Skeleton,
  StatusBadge,
  Tabs,
} from '@/components/ui';
import { DetailHeader, PageBody, PageScroll, Section } from '@/components/layout';
import { MetricsChart, type MetricKey } from '@/features/workers/MetricsChart';
import { LabelEditor } from '@/features/workers/LabelEditor';
import { workerState } from '@/features/workers/utils';
import { workerService } from '@/services/workerService';
import { relativeTime, toEpochMs } from '@/lib/format';

type View = 'metrics' | 'labels';

const PERIOD_ITEMS: { value: DashboardUptimePeriod; label: string }[] = [
  { value: '10m', label: '10 min' },
  { value: '1h', label: '1 hour' },
  { value: '24h', label: '24 hours' },
  { value: '7d', label: '7 days' },
];

function buildTickFormatter(period: DashboardUptimePeriod) {
  return (ts: number) => {
    const d = new Date(ts);
    if (period === '10m' || period === '1h' || period === '24h') {
      return d.toLocaleTimeString([], {
        hour: '2-digit',
        minute: '2-digit',
        second: period === '10m' ? '2-digit' : undefined,
        hour12: false,
      });
    }
    return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
  };
}

export default function WorkerDetail() {
  const { id } = useParams<{ id: string }>();
  const [period, setPeriod] = useState<DashboardUptimePeriod>('7d');
  const [view, setView] = useState<View>('metrics');

  const {
    data: worker,
    isLoading: workerLoading,
    isError: workerError,
    refetch: refetchWorker,
  } = useWorker(id ?? '');
  const { data: chartData, isLoading: uptimeLoading } = useDashboardUptime(
    period,
    id ?? undefined
  );
  const reEnable = useReEnableWorker();
  const queryClient = useQueryClient();
  const updateLabels = useMutation({
    mutationFn: (labels: Record<string, string>) =>
      workerService.updateLabels(id!, labels),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['worker', id] }),
  });

  if (workerLoading) {
    return (
      <PageScroll>
        <PageBody className='space-y-3'>
          <Skeleton className='h-6 w-48' />
          <Skeleton className='h-60 w-full' />
        </PageBody>
      </PageScroll>
    );
  }

  if (!id || workerError || !worker) {
    return (
      <PageScroll>
        <PageBody>
          <Alert tone='danger' title='Worker not found'>
            <Link to='/workers' className='underline'>
              Back to workers
            </Link>
          </Alert>
        </PageBody>
      </PageScroll>
    );
  }

  const points: ChartPoint[] = chartData?.points ?? [];
  const domain: [number, number] = chartData
    ? [chartData.period_start_ms, chartData.period_end_ms]
    : [0, Date.now()];
  const empty = points.length === 0 && !uptimeLoading;
  const tickFormatter = buildTickFormatter(period);
  const tickCount = period === '24h' ? 6 : 8;
  const pointsByMetric = (k: MetricKey) => points.filter((p) => p[k] != null);
  const seen = toEpochMs(worker.last_seen);

  return (
    <>
      <DetailHeader
        crumbs={[{ label: 'Workers', to: '/workers' }]}
        title={worker.name}
        subtitle={worker.description}
        tags={
          <StatusBadge state={workerState(worker.status)}>
            {worker.status}
          </StatusBadge>
        }
        actions={
          worker.status === 'dead' ? (
            <Button
              variant='outline'
              size='sm'
              disabled={reEnable.isPending}
              onClick={() => reEnable.mutate(id, { onSuccess: () => void refetchWorker() })}
            >
              <Power className='h-3.5 w-3.5' /> Re-enable
            </Button>
          ) : undefined
        }
        tabs={
          <Tabs<View>
            value={view}
            onChange={setView}
            items={[
              { value: 'metrics', label: 'Metrics' },
              {
                value: 'labels',
                label: 'Labels',
                count: Object.keys(worker.labels ?? {}).length,
              },
            ]}
          />
        }
        meta={
          <>
            {seen && <Fact title='Last heartbeat'>{relativeTime(seen)}</Fact>}
            {worker.agent_ip && (
              <Fact>
                <span className='font-mono'>{worker.agent_ip}</span>
              </Fact>
            )}
            {worker.agent_version && (
              <Fact title='Agent version'>
                <span className='font-mono'>{worker.agent_version}</span>
              </Fact>
            )}
          </>
        }
      />

      <PageScroll>
        <PageBody>
          {worker.status === 'dead' && (
            <Alert tone='warning' title='This worker is marked dead' className='mb-6'>
              It stopped sending heartbeats. Re-enable it to let the agent connect
              again.
            </Alert>
          )}

          {view === 'labels' ? (
            <Card>
              <CardHeader>
                <CardTitle>Routing labels</CardTitle>
                <CardDescription>
                  A build is dispatched here when every key in its selector matches
                  a label below.
                </CardDescription>
              </CardHeader>
              <div className='p-4'>
                <LabelEditor
                  initialLabels={worker.labels}
                  onSave={(labels) => updateLabels.mutate(labels)}
                  saving={updateLabels.isPending}
                />
                {updateLabels.isError && (
                  <p className='mt-2 text-xs text-danger'>
                    Failed to save labels. Please try again.
                  </p>
                )}
              </div>
            </Card>
          ) : (
            <Section
              title='Resource usage'
              aside={
                <SegmentedControl<DashboardUptimePeriod>
                  label='Metrics window'
                  value={period}
                  onChange={setPeriod}
                  options={PERIOD_ITEMS}
                />
              }
            >
              {empty ? (
                <Alert tone='info' title='No metrics yet'>
                  The agent reports every few minutes. Come back shortly.
                </Alert>
              ) : (
                <div className='flex flex-col gap-4'>
                  <MetricsChart
                    title='CPU load'
                    data={pointsByMetric('cpu_load')}
                    metric='cpu_load'
                    domain={domain}
                    loading={uptimeLoading}
                    tickFormatter={tickFormatter}
                    tickCount={tickCount}
                    valueFormatter={(v) => (v != null ? v.toFixed(2) : '—')}
                  />
                  <MetricsChart
                    title='Memory (MB)'
                    data={pointsByMetric('mem_usage_mb')}
                    metric='mem_usage_mb'
                    domain={domain}
                    loading={uptimeLoading}
                    tickFormatter={tickFormatter}
                    tickCount={tickCount}
                    valueFormatter={(v) => (v != null ? v.toFixed(1) : '—')}
                  />
                  <MetricsChart
                    title='Disk used (GB)'
                    data={pointsByMetric('disk_used_gb')}
                    metric='disk_used_gb'
                    domain={domain}
                    yDomain={chartData?.disk_y_domain}
                    loading={uptimeLoading}
                    tickFormatter={tickFormatter}
                    tickCount={tickCount}
                    valueFormatter={(v) => (v != null ? `${v.toFixed(2)} GB` : '—')}
                    yTickFormatter={(v) => `${Number(v).toFixed(0)} GB`}
                  />
                </div>
              )}
            </Section>
          )}
        </PageBody>
      </PageScroll>
    </>
  );
}
