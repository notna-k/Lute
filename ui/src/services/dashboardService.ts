import { apiClient } from './api';

/** One bucket-aligned chart point. Null metrics = worker was down in this bucket (gap). */
export interface ChartPoint {
  t: number;
  cpu_load?: number | null;
  mem_usage_mb?: number | null;
  disk_used_gb?: number | null;
  disk_total_gb?: number | null;
}

/** Chart-ready response: backend-bucketed points + domain info. */
export interface DashboardUptimeResponse {
  points: ChartPoint[];
  period_start_ms: number;
  period_end_ms: number;
  disk_y_domain: [number, number];
}

export type DashboardUptimePeriod = '10m' | '1h' | '24h' | '7d';

/** Server config: snapshot interval in seconds (METRICS_SNAPSHOT_INTERVAL). */
export interface DashboardConfigResponse {
  metrics_poll_interval_seconds: number;
}

export const dashboardService = {
  getConfig: async (): Promise<DashboardConfigResponse> => {
    return apiClient.get<DashboardConfigResponse>('/api/v1/dashboard/config', {
      cache: 'no-store',
    });
  },
  getUptime: async (
    period: DashboardUptimePeriod,
    workerId?: string,
  ): Promise<DashboardUptimeResponse> => {
    const params = new URLSearchParams({ period });
    if (workerId) params.set('worker_id', workerId);
    return apiClient.get<DashboardUptimeResponse>(`/api/v1/dashboard/uptime?${params.toString()}`, {
      cache: 'no-store',
    });
  },
};
