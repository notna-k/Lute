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
} from '@mui/material';
import { Link } from 'react-router-dom';
import AddMachineDialog from '../components/AddMachineDialog';
import { useUserMachines, usePublicMachines } from '../hooks/useMachines';

const statCards = [
  { key: 'total', name: 'Total Machines' },
  { key: 'alive', name: 'Running' },
  { key: 'dead', name: 'Stopped' },
  { key: 'public', name: 'Public Machines' },
] as const;

const Dashboard = () => {
  const { user } = useAuth();
  const [addDialogOpen, setAddDialogOpen] = useState(false);
  const { data: userMachinesData, isLoading: userLoading, isError: userError } = useUserMachines();
  const { data: publicMachinesData, isLoading: publicLoading, isError: publicError } = usePublicMachines();

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
                  '&:hover': { borderColor: 'primary.main', bgcolor: 'action.hover' },
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
                  '&:hover': { borderColor: 'primary.main', bgcolor: 'action.hover' },
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
                '&:hover': { borderColor: 'primary.main', bgcolor: 'action.hover' },
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
