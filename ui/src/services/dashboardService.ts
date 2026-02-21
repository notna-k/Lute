import { apiClient } from './api';

/** One bucket-aligned chart point. Null metrics = machine was down in this bucket (gap). */
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

const TARGET_CHART_POINTS = 80;

/** Mirror of backend bucketDuration: returns bucket size in ms for the given period. */
export function bucketDurationMs(period: DashboardUptimePeriod, snapshotIntervalSeconds: number): number {
    if (snapshotIntervalSeconds <= 0) snapshotIntervalSeconds = 60;
    const snapshotMs = snapshotIntervalSeconds * 1000;
    const periodMs =
        period === '10m' ? 10 * 60 * 1000 :
        period === '1h'  ? 60 * 60 * 1000 :
        period === '24h' ? 24 * 60 * 60 * 1000 :
                           7 * 24 * 60 * 60 * 1000;
    const bucket = Math.floor(periodMs / TARGET_CHART_POINTS);
    return Math.max(bucket, snapshotMs);
}

export const dashboardService = {
    getConfig: async (): Promise<DashboardConfigResponse> => {
        return apiClient.get<DashboardConfigResponse>('/api/v1/dashboard/config', { cache: 'no-store' });
    },
    getUptime: async (period: DashboardUptimePeriod, machineId?: string): Promise<DashboardUptimeResponse> => {
        const params = new URLSearchParams({ period });
        if (machineId) params.set('machine_id', machineId);
        return apiClient.get<DashboardUptimeResponse>(`/api/v1/dashboard/uptime?${params.toString()}`, { cache: 'no-store' });
    },
};
