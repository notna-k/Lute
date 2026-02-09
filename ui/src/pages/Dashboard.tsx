import { useState } from 'react';
import { useAuth } from '../contexts/AuthContext';
import {
  Box,
  Typography,
  Grid,
  Card,
  CardContent,
  Paper,
  Link as MuiLink,
} from '@mui/material';
import { Link } from 'react-router-dom';
import AddMachineDialog from '../components/AddMachineDialog';

const Dashboard = () => {
  const { user } = useAuth();
  const [addDialogOpen, setAddDialogOpen] = useState(false);

  // Mock stats - replace with actual data from your API
  const stats = [
    { name: 'Total Machines', value: '12', change: '+2', changeType: 'positive' },
    { name: 'Running', value: '8', change: '+1', changeType: 'positive' },
    { name: 'Stopped', value: '4', change: '-1', changeType: 'negative' },
    { name: 'Public Machines', value: '24', change: '+5', changeType: 'positive' },
  ];

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
        {stats.map((stat) => (
          <Grid item xs={12} sm={6} lg={3} key={stat.name}>
            <Card>
              <CardContent>
                <Typography variant="body2" color="text.secondary" gutterBottom>
                  {stat.name}
                </Typography>
                <Box sx={{ display: 'flex', alignItems: 'baseline', mt: 1 }}>
                  <Typography variant="h4" component="div" fontWeight="semibold">
                    {stat.value}
                  </Typography>
                  <Typography
                    variant="body2"
                    sx={{
                      ml: 1,
                      fontWeight: 'semibold',
                      color: stat.changeType === 'positive' ? 'success.main' : 'error.main',
                    }}
                  >
                    {stat.change}
                  </Typography>
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
