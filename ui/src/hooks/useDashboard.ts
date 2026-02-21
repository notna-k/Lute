import { useState, useEffect, useCallback } from 'react';
import { dashboardService, type DashboardUptimePeriod, type DashboardUptimeResponse } from '../services/dashboardService';

const metricsUpdateIntervalMs = (() => {
    const raw = import.meta.env.VITE_METRICS_UPDATE_INTERVAL_SECONDS;
    if (raw === undefined || raw === '') return 60 * 1000;
    const sec = Number(raw);
    return Number.isFinite(sec) && sec > 0 ? sec * 1000 : 60 * 1000;
})();

export const useDashboardUptime = (period: DashboardUptimePeriod = '7d', machineId?: string) => {
    const enabled = machineId === undefined || machineId.length > 0;

    const [data, setData] = useState<DashboardUptimeResponse | undefined>(undefined);
    const [isLoading, setIsLoading] = useState(true);
    const [isError, setIsError] = useState(false);
    const [error, setError] = useState<Error | null>(null);

    const fetchUptime = useCallback(() => {
        if (!enabled) return;
        return dashboardService
            .getUptime(period, machineId)
            .then((res) => {
                setData(res);
                setIsError(false);
                setError(null);
            })
            .catch((err) => {
                setIsError(true);
                setError(err instanceof Error ? err : new Error(String(err)));
            })
            .finally(() => setIsLoading(false));
    }, [period, machineId, enabled]);

    useEffect(() => {
        if (!enabled) {
            setIsLoading(false);
            return;
        }
        setIsLoading(true);
        fetchUptime();
        const id = setInterval(fetchUptime, metricsUpdateIntervalMs);
        return () => clearInterval(id);
    }, [fetchUptime, enabled]);

    return { data, isLoading, isError, error };
};
