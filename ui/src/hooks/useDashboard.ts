import { useQuery } from '@tanstack/react-query';
import { dashboardService, DashboardUptimePeriod } from '../services/dashboardService';

export const dashboardKeys = {
    uptime: (period: DashboardUptimePeriod) => ['dashboard', 'uptime', period] as const,
};

export const useDashboardUptime = (period: DashboardUptimePeriod = '7d') => {
    return useQuery({
        queryKey: dashboardKeys.uptime(period),
        queryFn: () => dashboardService.getUptime(period),
        staleTime: 60 * 1000, // 1 minute
    });
};
