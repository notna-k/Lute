import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { ArrowLeft, Power } from "lucide-react";
import { useReEnableWorker, useWorker } from "@/hooks/useWorkers";
import { useDashboardUptime } from "@/hooks/useDashboard";
import type { ChartPoint, DashboardUptimePeriod } from "@/services/dashboardService";
import {
  Alert,
  Badge,
  Button,
  PageHeader,
  Skeleton,
  Tabs,
} from "@/components/ui";
import {
  MetricsChart,
  type MetricKey,
} from "@/features/workers/MetricsChart";
import { statusTone } from "@/features/workers/utils";

const PERIOD_ITEMS: { value: DashboardUptimePeriod; label: string }[] = [
  { value: "10m", label: "10 min" },
  { value: "1h", label: "1 hour" },
  { value: "24h", label: "24 hours" },
  { value: "7d", label: "7 days" },
];

function buildTickFormatter(period: DashboardUptimePeriod) {
  return (ts: number) => {
    const d = new Date(ts);
    if (period === "10m" || period === "1h" || period === "24h") {
      return d.toLocaleTimeString([], {
        hour: "2-digit",
        minute: "2-digit",
        second: period === "10m" ? "2-digit" : undefined,
        hour12: false,
      });
    }
    return d.toLocaleDateString([], { month: "short", day: "numeric" });
  };
}

const WorkerDetail = () => {
  const { id } = useParams<{ id: string }>();
  const [period, setPeriod] = useState<DashboardUptimePeriod>("7d");

  const {
    data: worker,
    isLoading: workerLoading,
    isError: workerError,
    refetch: refetchWorker,
  } = useWorker(id ?? "");
  const { data: chartData, isLoading: uptimeLoading } = useDashboardUptime(
    period,
    id ?? undefined
  );
  const reEnable = useReEnableWorker();

  if (!id) {
    return (
      <Alert tone="danger" title="Missing worker ID">
        <Link to="/workers" className="text-primary underline">
          Back to workers
        </Link>
      </Alert>
    );
  }

  if (workerLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-6 w-48" />
        <Skeleton className="h-10 w-72" />
        <Skeleton className="h-60 w-full rounded-lg" />
      </div>
    );
  }

  if (workerError || !worker) {
    return (
      <Alert tone="danger" title="Worker not found">
        <Link to="/workers" className="text-primary underline">
          Back to workers
        </Link>
      </Alert>
    );
  }

  const points: ChartPoint[] = chartData?.points ?? [];
  const domain: [number, number] = chartData
    ? [chartData.period_start_ms, chartData.period_end_ms]
    : [0, Date.now()];
  const diskYDomain: [number, number] | undefined = chartData?.disk_y_domain;
  const empty = points.length === 0 && !uptimeLoading;
  const tickFormatter = buildTickFormatter(period);
  const tickCount = period === "24h" ? 6 : 8;

  const pointsByMetric = (k: MetricKey) =>
    points.filter((p) => p[k] != null);

  return (
    <>
      <Link
        to="/workers"
        className="mb-3 inline-flex items-center gap-1 text-sm text-fg-muted hover:text-fg"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to workers
      </Link>

      <PageHeader
        title={worker.name}
        description="Uptime, CPU, memory and disk usage over time."
        actions={
          <div className="flex items-center gap-2">
            <Badge tone={statusTone(worker.status)} dot>
              {worker.status}
            </Badge>
          </div>
        }
      />

      {worker.status === "dead" && (
        <Alert
          tone="warning"
          className="mb-4"
          title="This worker is marked dead"
          action={
            <Button
              variant="outline"
              size="sm"
              leftIcon={<Power className="h-4 w-4" />}
              loading={reEnable.isPending}
              onClick={() =>
                reEnable.mutate(id, {
                  onSuccess: () => refetchWorker(),
                })
              }
            >
              Re-enable
            </Button>
          }
        >
          Re-enable to allow the agent to connect again.
        </Alert>
      )}

      <div className="mb-4">
        <Tabs<DashboardUptimePeriod>
          value={period}
          onChange={setPeriod}
          items={PERIOD_ITEMS}
          variant="pill"
        />
      </div>

      {empty ? (
        <Alert tone="info" title="No metrics yet">
          Data is collected every few minutes. Come back soon.
        </Alert>
      ) : (
        <div className="flex flex-col gap-4">
          <MetricsChart
            title="CPU load"
            data={pointsByMetric("cpu_load")}
            metric="cpu_load"
            domain={domain}
            loading={uptimeLoading}
            tickFormatter={tickFormatter}
            tickCount={tickCount}
            valueFormatter={(v) => (v != null ? v.toFixed(2) : "—")}
          />
          <MetricsChart
            title="Memory (MB)"
            data={pointsByMetric("mem_usage_mb")}
            metric="mem_usage_mb"
            domain={domain}
            loading={uptimeLoading}
            tickFormatter={tickFormatter}
            tickCount={tickCount}
            valueFormatter={(v) => (v != null ? v.toFixed(1) : "—")}
          />
          <MetricsChart
            title="Disk used (GB)"
            data={pointsByMetric("disk_used_gb")}
            metric="disk_used_gb"
            domain={domain}
            yDomain={diskYDomain}
            loading={uptimeLoading}
            tickFormatter={tickFormatter}
            tickCount={tickCount}
            valueFormatter={(v) =>
              v != null ? `${v.toFixed(2)} GB` : "—"
            }
            yTickFormatter={(v) => `${Number(v).toFixed(0)} GB`}
          />
        </div>
      )}
    </>
  );
};

export default WorkerDetail;
