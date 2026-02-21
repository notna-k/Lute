import { useState } from 'react';
import { useParams, Link as RouterLink } from 'react-router-dom';
import {
  Box,
  Typography,
  Paper,
  Skeleton,
  ToggleButton,
  ToggleButtonGroup,
  Link,
  Button,
  Alert,
  useTheme,
} from '@mui/material';
import { ArrowBack as ArrowBackIcon } from '@mui/icons-material';
import {
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Area,
  AreaChart,
} from 'recharts';
import { useMachine, useReEnableMachine } from '../hooks/useMachines';
import { useDashboardUptime } from '../hooks/useDashboard';
import type { DashboardUptimePeriod } from '../services/dashboardService';
import type { ChartPoint } from '../services/dashboardService';

const chartHeight = 240;

const MachineMetrics = () => {
  const theme = useTheme();
  const { id } = useParams<{ id: string }>();
  const [period, setPeriod] = useState<DashboardUptimePeriod>('7d');
  const { data: machine, isLoading: machineLoading, isError: machineError, refetch: refetchMachine } = useMachine(id ?? '');
  const { data: chartData, isLoading: uptimeLoading } = useDashboardUptime(period, id ?? undefined);
  const reEnableMutation = useReEnableMachine();

  const points: ChartPoint[] = chartData?.points ?? [];
  const domain: [number, number] = chartData
    ? [chartData.period_start_ms, chartData.period_end_ms]
    : [0, Date.now()];
  const diskYDomain: [number, number] = chartData?.disk_y_domain ?? [0, 1];
  const chartDataCpu = points.filter((p) => p.cpu_load != null);
  const chartDataMemory = points.filter((p) => p.mem_usage_mb != null);
  const chartDataDisk = points.filter((p) => p.disk_used_gb != null || p.disk_total_gb != null);

  const tickFormatter = (ts: number) => {
    const d = new Date(ts);
    if (period === '10m' || period === '1h' || period === '24h') {
      return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: period === '10m' ? '2-digit' : undefined, hour12: false });
    }
    return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
  };

  if (!id) {
    return (
      <Box>
        <Typography color="error">Missing machine ID</Typography>
        <Link component={RouterLink} to="/machines">Back to My Machines</Link>
      </Box>
    );
  }

  if (machineLoading) {
    return (
      <Box>
        <Skeleton variant="text" width={200} height={40} />
        <Skeleton variant="rectangular" height={chartHeight} sx={{ mt: 2, borderRadius: 1 }} />
      </Box>
    );
  }

  if (machineError || !machine) {
    return (
      <Box>
        <Typography color="error">Machine not found</Typography>
        <Link component={RouterLink} to="/machines" sx={{ mt: 1, display: 'inline-block' }}>
          Back to My Machines
        </Link>
      </Box>
    );
  }

  const empty = points.length === 0;

  return (
    <Box>
      <Box sx={{ mb: 3, display: 'flex', alignItems: 'center', gap: 2 }}>
        <Link
          component={RouterLink}
          to="/machines"
          sx={{ display: 'flex', alignItems: 'center', color: 'text.secondary', textDecoration: 'none', '&:hover': { color: 'primary.main' } }}
        >
          <ArrowBackIcon sx={{ mr: 0.5 }} /> Back
        </Link>
      </Box>
      <Box sx={{ mb: 2 }}>
        <Typography variant="h4" component="h1" fontWeight="bold">
          {machine.name}
        </Typography>
        <Typography variant="body2" color="text.secondary">
          Uptime, CPU, memory and disk usage
        </Typography>
      </Box>
      {machine.status === 'dead' && id && (
        <Alert severity="warning" sx={{ mb: 2 }} action={
          <Button
            color="inherit"
            size="small"
            disabled={reEnableMutation.isPending}
            onClick={() => reEnableMutation.mutate(id, { onSuccess: () => refetchMachine() })}
          >
            {reEnableMutation.isPending ? 'Re-enabling…' : 'Re-enable'}
          </Button>
        }>
          This machine is marked dead. Re-enable to allow the agent to connect again.
        </Alert>
      )}
      <Box sx={{ mb: 2 }}>
        <ToggleButtonGroup
          value={period}
          exclusive
          onChange={(_, v) => v != null && setPeriod(v)}
          size="small"
        >
          <ToggleButton value="10m">10 min</ToggleButton>
          <ToggleButton value="1h">1 hour</ToggleButton>
          <ToggleButton value="24h">24h</ToggleButton>
          <ToggleButton value="7d">7 days</ToggleButton>
        </ToggleButtonGroup>
      </Box>

      {empty && !uptimeLoading && (
        <Typography variant="body2" color="text.secondary" sx={{ py: 4 }}>
          No metrics yet; data is collected every few minutes.
        </Typography>
      )}

      {!empty && (
        <Box sx={{ display: 'flex', flexDirection: 'column', gap: 3 }}>
          {/* CPU load */}
          <Paper sx={{ p: 2 }}>
            <Typography variant="subtitle1" fontWeight="medium" gutterBottom>CPU load</Typography>
            {uptimeLoading ? (
              <Skeleton variant="rectangular" height={chartHeight} sx={{ borderRadius: 1 }} />
            ) : (
              <ResponsiveContainer width="100%" height={chartHeight}>
                <AreaChart data={chartDataCpu} margin={{ top: 20, right: 16, left: 16, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis type="number" dataKey="t" domain={domain} tickFormatter={tickFormatter} tickCount={period === '24h' ? 6 : 8} />
                  <YAxis domain={[0, 'auto']} allowDataOverflow />
                  <Tooltip
                    formatter={(value: number | undefined) => [value != null ? value.toFixed(2) : '—', 'CPU load']}
                    labelFormatter={(label) => new Date(typeof label === 'number' ? label : label).toLocaleString()}
                  />
                  <Area type="monotone" dataKey="cpu_load" stroke={theme.palette.secondary.main} fill={theme.palette.secondary.main} fillOpacity={0.2} isAnimationActive={false} />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </Paper>

          {/* Memory */}
          <Paper sx={{ p: 2 }}>
            <Typography variant="subtitle1" fontWeight="medium" gutterBottom>Memory (MB)</Typography>
            {uptimeLoading ? (
              <Skeleton variant="rectangular" height={chartHeight} sx={{ borderRadius: 1 }} />
            ) : (
              <ResponsiveContainer width="100%" height={chartHeight}>
                <AreaChart data={chartDataMemory} margin={{ top: 20, right: 16, left: 16, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis type="number" dataKey="t" domain={domain} tickFormatter={tickFormatter} tickCount={period === '24h' ? 6 : 8} />
                  <YAxis />
                  <Tooltip
                    formatter={(value: number | undefined) => [value != null ? value.toFixed(1) : '—', 'Memory (MB)']}
                    labelFormatter={(label) => new Date(typeof label === 'number' ? label : label).toLocaleString()}
                  />
                  <Area type="monotone" dataKey="mem_usage_mb" stroke={theme.palette.info.main} fill={theme.palette.info.main} fillOpacity={0.2} isAnimationActive={false} />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </Paper>

          {/* Disk used (GB), Y max = machine disk size */}
          <Paper sx={{ p: 2 }}>
            <Typography variant="subtitle1" fontWeight="medium" gutterBottom>Disk used (GB)</Typography>
            {uptimeLoading ? (
              <Skeleton variant="rectangular" height={chartHeight} sx={{ borderRadius: 1 }} />
            ) : (
              <ResponsiveContainer width="100%" height={chartHeight}>
                <AreaChart data={chartDataDisk} margin={{ top: 20, right: 16, left: 16, bottom: 0 }}>
                  <CartesianGrid strokeDasharray="3 3" />
                  <XAxis type="number" dataKey="t" domain={domain} tickFormatter={tickFormatter} tickCount={period === '24h' ? 6 : 8} />
                  <YAxis domain={diskYDomain} tickFormatter={(v) => `${Number(v).toFixed(0)} GB`} width={56} />
                  <Tooltip
                    formatter={(value: number | undefined) => [value != null ? `${value.toFixed(2)} GB` : '—', 'Disk used']}
                    labelFormatter={(label) => new Date(typeof label === 'number' ? label : label).toLocaleString()}
                  />
                  <Area type="monotone" dataKey="disk_used_gb" stroke={theme.palette.success.main} fill={theme.palette.success.main} fillOpacity={0.2} isAnimationActive={false} />
                </AreaChart>
              </ResponsiveContainer>
            )}
          </Paper>
        </Box>
      )}
    </Box>
  );
};

export default MachineMetrics;
