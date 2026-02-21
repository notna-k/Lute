import { apiClient } from './api';

/** Canonical metrics (same as Machine.metrics and snapshot). */
export interface DashboardMetrics {
    cpu_load?: number;
    mem_usage_mb?: number;
    disk_used_gb?: number;
    disk_total_gb?: number;
}

export interface UptimePoint {
    at: string;
    uptime_pct?: number;
    metrics?: DashboardMetrics;
}

export interface DashboardUptimeResponse {
    points: UptimePoint[];
}

export type DashboardUptimePeriod = '24h' | '7d';

export const dashboardService = {
    getUptime: async (period: DashboardUptimePeriod): Promise<DashboardUptimeResponse> => {
        return apiClient.get<DashboardUptimeResponse>(`/api/v1/dashboard/uptime?period=${period}`);
    },
};
