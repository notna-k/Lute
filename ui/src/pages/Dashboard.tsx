import { useState, useMemo } from 'react';
import { useAuth } from '../contexts/AuthContext';
import {
  Box,
  Typography,
  Grid,
  Card,
  CardContent,
  Paper,
  Link as MuiLink,
  Skeleton,
  ToggleButton,
  ToggleButtonGroup,
  useTheme,
} from '@mui/material';
import { Link } from 'react-router-dom';
import {
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Area,
  AreaChart,
} from 'recharts';
import AddMachineDialog from '../components/AddMachineDialog';
import { useUserMachines, usePublicMachines } from '../hooks/useMachines';
import { useDashboardUptime } from '../hooks/useDashboard';
import type { DashboardUptimePeriod } from '../services/dashboardService';

const statCards = [
  { key: 'total', name: 'Total Machines' },
  { key: 'alive', name: 'Running' },
  { key: 'dead', name: 'Stopped' },
  { key: 'public', name: 'Public Machines' },
] as const;

const Dashboard = () => {
  const theme = useTheme();
  const { user } = useAuth();
  const [addDialogOpen, setAddDialogOpen] = useState(false);
  const [uptimePeriod, setUptimePeriod] = useState<DashboardUptimePeriod>('7d');
  const { data: userMachinesData, isLoading: userLoading, isError: userError } = useUserMachines();
  const { data: publicMachinesData, isLoading: publicLoading, isError: publicError } = usePublicMachines();
  const { data: uptimeData, isLoading: uptimeLoading } = useDashboardUptime(uptimePeriod);

  const userMachines = userMachinesData ?? [];
  const publicMachines = publicMachinesData ?? [];

  const stats = useMemo(() => {
    const alive = userMachines.filter((m) => m.status === 'alive').length;
    const dead = userMachines.filter((m) => m.status === 'dead').length;
    const total = userMachines.length;
    const publicCount = publicMachines.length;
    return { total, alive, dead, public: publicCount };
  }, [userMachines, publicMachines]);

  const loading = userLoading || publicLoading;
  const error = userError || publicError;

  const formatStatValue = (key: (typeof statCards)[number]['key']): string => {
    if (loading || error) return '—';
    return String(stats[key]);
  };

  return (
    <Box>
      <Box sx={{ mb: 4 }}>
        <Typography variant="h4" component="h1" gutterBottom fontWeight="bold">
          Welcome back, {user?.displayName || user?.email}!
        </Typography>
        <Typography variant="body2" color="text.secondary">
          Here's an overview of your virtual machines
        </Typography>
      </Box>

      {/* Stats Grid */}
      <Grid container spacing={3} sx={{ mb: 4 }}>
        {statCards.map(({ key, name }) => (
          <Grid item xs={12} sm={6} lg={3} key={name}>
            <Card>
              <CardContent>
                <Typography variant="body2" color="text.secondary" gutterBottom>
                  {name}
                </Typography>
                <Box sx={{ display: 'flex', alignItems: 'baseline', mt: 1 }}>
                  {loading ? (
                    <Skeleton variant="text" width={48} height={40} />
                  ) : (
                    <Typography variant="h4" component="div" fontWeight="semibold">
                      {formatStatValue(key)}
                    </Typography>
                  )}
                </Box>
              </CardContent>
            </Card>
          </Grid>
        ))}
      </Grid>

      {/* Quick Actions */}
      <Paper sx={{ p: 3, mb: 4 }}>
        <Typography variant="h6" gutterBottom fontWeight="medium">
          Quick Actions
        </Typography>
        <Grid container spacing={3} sx={{ mt: 1 }}>
          <Grid item xs={12} sm={4}>
            <MuiLink
              component={Link}
              to="/machines"
              underline="none"
              sx={{ display: 'block' }}
            >
              <Paper
                elevation={1}
                sx={{
                  p: 3,
                  border: 1,
                  borderColor: 'divider',
                  '&:hover': {
                    borderColor: 'primary.main',
                    bgcolor: 'action.hover',
                  },
                  cursor: 'pointer',
                }}
              >
                <Typography variant="subtitle1" fontWeight="medium" gutterBottom>
                  My Machines
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  View and manage your VMs
                </Typography>
              </Paper>
            </MuiLink>
          </Grid>
          <Grid item xs={12} sm={4}>
            <MuiLink
              component={Link}
              to="/public-machines"
              underline="none"
              sx={{ display: 'block' }}
            >
              <Paper
                elevation={1}
                sx={{
                  p: 3,
                  border: 1,
                  borderColor: 'divider',
                  '&:hover': {
                    borderColor: 'primary.main',
                    bgcolor: 'action.hover',
                  },
                  cursor: 'pointer',
                }}
              >
                <Typography variant="subtitle1" fontWeight="medium" gutterBottom>
                  Public Machines
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  Browse shared VMs
                </Typography>
              </Paper>
            </MuiLink>
          </Grid>
          <Grid item xs={12} sm={4}>
            <Paper
              elevation={1}
              sx={{
                p: 3,
                border: 1,
                borderColor: 'divider',
                '&:hover': {
                  borderColor: 'primary.main',
                  bgcolor: 'action.hover',
                },
                cursor: 'pointer',
              }}
              onClick={() => setAddDialogOpen(true)}
            >
              <Typography variant="subtitle1" fontWeight="medium" gutterBottom>
                Add Machine
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Install the agent on a new VM
              </Typography>
            </Paper>
          </Grid>
        </Grid>
      </Paper>

      {/* Average uptime graph */}
      <Paper sx={{ p: 3, mb: 4 }}>
        <Box sx={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', justifyContent: 'space-between', gap: 2, mb: 2 }}>
          <Typography variant="h6" fontWeight="medium">
            Machines uptime
          </Typography>
          <ToggleButtonGroup
            value={uptimePeriod}
            exclusive
            onChange={(_, v) => v != null && setUptimePeriod(v)}
            size="small"
          >
            <ToggleButton value="24h">24h</ToggleButton>
            <ToggleButton value="7d">7 days</ToggleButton>
          </ToggleButtonGroup>
        </Box>
        {uptimeLoading ? (
          <Skeleton variant="rectangular" height={280} sx={{ borderRadius: 1 }} />
        ) : !uptimeData?.points?.length ? (
          <Typography variant="body2" color="text.secondary" sx={{ py: 4 }}>
            No uptime data yet; data is collected every few minutes.
          </Typography>
        ) : (
          <Box sx={{ width: '100%', height: 280 }}>
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={uptimeData.points} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis
                  dataKey="at"
                  tickFormatter={(v) => {
                    const d = new Date(v);
                    return uptimePeriod === '24h' ? d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : d.toLocaleDateString([], { month: 'short', day: 'numeric' });
                  }}
                />
                <YAxis domain={[0, 100]} tickFormatter={(v) => `${v}%`} />
                <Tooltip
                  formatter={(value: number | undefined) => [value != null ? `${value.toFixed(1)}%` : '—', 'Uptime']}
                  labelFormatter={(label) => new Date(label).toLocaleString()}
                  content={({ active, payload, label }) => {
                    if (!active || !payload?.length || !label) return null;
                    const point = payload[0]?.payload as { at: string; uptime_pct?: number; metrics?: { cpu_load?: number; mem_usage_mb?: number; disk_used_gb?: number; disk_total_gb?: number } };
                    const m = point?.metrics;
                    const diskPct = m && m.disk_total_gb != null && m.disk_total_gb > 0 && m.disk_used_gb != null
                      ? ((m.disk_used_gb / m.disk_total_gb) * 100).toFixed(1) + '%'
                      : null;
                    return (
                      <Paper sx={{ p: 1.5, minWidth: 160 }} elevation={2}>
                        <Typography variant="caption" color="text.secondary">{new Date(label).toLocaleString()}</Typography>
                        <Typography variant="body2">Uptime: {point?.uptime_pct != null ? `${point.uptime_pct.toFixed(1)}%` : '—'}</Typography>
                        {m && (m.cpu_load != null || m.mem_usage_mb != null || diskPct) && (
                          <>
                            {m.cpu_load != null && <Typography variant="caption" display="block">CPU load: {m.cpu_load.toFixed(2)}</Typography>}
                            {m.mem_usage_mb != null && <Typography variant="caption" display="block">Mem: {m.mem_usage_mb.toFixed(1)} MB</Typography>}
                            {diskPct != null && <Typography variant="caption" display="block">Disk: {diskPct}</Typography>}
                          </>
                        )}
                      </Paper>
                    );
                  }}
                />
                <Area type="monotone" dataKey="uptime_pct" stroke={theme.palette.primary.main} fill={theme.palette.primary.main} fillOpacity={0.2} />
              </AreaChart>
            </ResponsiveContainer>
          </Box>
        )}
      </Paper>

      {/* Recent Activity */}
      <Paper>
        <Box sx={{ p: 3, borderBottom: 1, borderColor: 'divider' }}>
          <Typography variant="h6" fontWeight="medium">
            Recent Activity
          </Typography>
        </Box>
        <Box sx={{ p: 3 }}>
          <Typography variant="body2" color="text.secondary">
            No recent activity
          </Typography>
        </Box>
      </Paper>

      <AddMachineDialog
        open={addDialogOpen}
        onClose={() => setAddDialogOpen(false)}
      />
    </Box>
  );
};

export default Dashboard;
