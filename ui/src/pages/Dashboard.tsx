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
import AddWorkerDialog from '../components/AddWorkerDialog';
import { useUserWorkers, usePublicWorkers } from '../hooks/useWorkers';

const statCards = [
  { key: 'total', name: 'Total Workers' },
  { key: 'alive', name: 'Running' },
  { key: 'dead', name: 'Stopped' },
  { key: 'public', name: 'Public Workers' },
] as const;

const Dashboard = () => {
  const { user } = useAuth();
  const [addDialogOpen, setAddDialogOpen] = useState(false);
  const { data: userWorkersData, isLoading: userLoading, isError: userError } = useUserWorkers();
  const { data: publicWorkersData, isLoading: publicLoading, isError: publicError } = usePublicWorkers();

  const userWorkers = userWorkersData ?? [];
  const publicWorkers = publicWorkersData ?? [];

  const stats = useMemo(() => {
    const alive = userWorkers.filter((w) => w.status === 'alive').length;
    const dead = userWorkers.filter((w) => w.status === 'dead').length;
    const total = userWorkers.length;
    const publicCount = publicWorkers.length;
    return { total, alive, dead, public: publicCount };
  }, [userWorkers, publicWorkers]);

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
          Here&apos;s an overview of your workers
        </Typography>
      </Box>

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

      <Paper sx={{ p: 3, mb: 4 }}>
        <Typography variant="h6" gutterBottom fontWeight="medium">
          Quick Actions
        </Typography>
        <Grid container spacing={3} sx={{ mt: 1 }}>
          <Grid item xs={12} sm={4}>
            <MuiLink
              component={Link}
              to="/workers"
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
                  My Workers
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  View and manage your workers
                </Typography>
              </Paper>
            </MuiLink>
          </Grid>
          <Grid item xs={12} sm={4}>
            <MuiLink
              component={Link}
              to="/public-workers"
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
                  Public Workers
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  Browse shared workers
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
                Add Worker
              </Typography>
              <Typography variant="body2" color="text.secondary">
                Install the agent on a new host
              </Typography>
            </Paper>
          </Grid>
        </Grid>
      </Paper>

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

      <AddWorkerDialog
        open={addDialogOpen}
        onClose={() => setAddDialogOpen(false)}
      />
    </Box>
  );
};

export default Dashboard;
