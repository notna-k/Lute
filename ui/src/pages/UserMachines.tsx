import { useState } from 'react';
import {
  Box,
  Typography,
  Paper,
  List,
  ListItem,
  Avatar,
  Chip,
  Button,
  Stack,
  CircularProgress,
  Alert,
} from '@mui/material';
import { Add as AddIcon } from '@mui/icons-material';
import { Link } from 'react-router-dom';
import { useUserMachines, useReEnableMachine } from '../hooks/useMachines';
import { Machine } from '../types';
import AddMachineDialog from '../components/AddMachineDialog';

const UserMachines = () => {
  const { data: machines, isLoading, error, refetch } = useUserMachines();
  const [addDialogOpen, setAddDialogOpen] = useState(false);
  const reEnableMutation = useReEnableMachine();

  const getStatusColor = (status: string): 'success' | 'error' | 'warning' | 'default' => {
    switch (status) {
      case 'alive':
      case 'running':
        return 'success';
      case 'dead':
      case 'stopped':
        return 'error';
      case 'pending':
      case 'registered':
      case 'paused':
        return 'warning';
      default:
        return 'default';
    }
  };

  const handleReEnable = (machineId: string) => {
    reEnableMutation.mutate(machineId, { onSuccess: () => refetch() });
  };

  if (isLoading) {
    return (
      <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '400px' }}>
        <CircularProgress />
      </Box>
    );
  }

  if (error) {
    return (
      <Box>
        <Alert severity="error" sx={{ mb: 2 }}>
          Failed to load machines: {error instanceof Error ? error.message : 'Unknown error'}
        </Alert>
        <Button onClick={() => refetch()} variant="outlined">
          Retry
        </Button>
      </Box>
    );
  }

  return (
    <Box>
      <Box sx={{ mb: 4, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <Box>
          <Typography variant="h4" component="h1" gutterBottom fontWeight="bold">
            My Machines
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Manage your virtual machines
          </Typography>
        </Box>
        <Button
          variant="contained"
          startIcon={<AddIcon />}
          onClick={() => setAddDialogOpen(true)}
        >
          Add Machine
        </Button>
      </Box>

      {machines && machines.length > 0 ? (
        <Paper>
          <List>
            {machines.map((machine: Machine, index: number) => (
              <ListItem
                key={machine.id}
                sx={{
                  borderBottom: index < machines.length - 1 ? 1 : 0,
                  borderColor: 'divider',
                  '&:hover': {
                    bgcolor: 'action.hover',
                  },
                }}
              >
                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%', py: 2 }}>
                  <Box sx={{ display: 'flex', alignItems: 'center', gap: 2 }}>
                    <Avatar
                      sx={{
                        bgcolor: 'primary.light',
                        color: 'primary.main',
                        width: 40,
                        height: 40,
                      }}
                    >
                      {machine.name.charAt(0).toUpperCase()}
                    </Avatar>
                    <Box>
                      <Typography variant="subtitle1" fontWeight="medium">
                        {machine.name}
                      </Typography>
                      <Typography variant="body2" color="text.secondary">
                        {machine.description || 'No description'}
                      </Typography>
                    </Box>
                  </Box>
                  <Stack direction="row" spacing={2} alignItems="center">
                    <Chip
                      label={machine.status}
                      color={getStatusColor(machine.status)}
                      size="small"
                    />
                    {machine.is_public && (
                      <Chip
                        label="Public"
                        size="small"
                        color="secondary"
                      />
                    )}
                    {machine.status === 'dead' && (
                      <Button
                        variant="outlined"
                        color="primary"
                        size="small"
                        disabled={reEnableMutation.isPending && reEnableMutation.variables === machine.id}
                        onClick={() => handleReEnable(machine.id)}
                      >
                        {reEnableMutation.isPending && reEnableMutation.variables === machine.id ? 'Re-enabling…' : 'Re-enable'}
                      </Button>
                    )}
                    <Button
                      component={Link}
                      to={`/machines/${machine.id}`}
                      variant="text"
                      color="primary"
                      size="small"
                    >
                      Manage
                    </Button>
                  </Stack>
                </Box>
              </ListItem>
            ))}
          </List>
        </Paper>
      ) : (
        <Box sx={{ textAlign: 'center', py: 6 }}>
          <Typography variant="body1" color="text.secondary" gutterBottom>
            No machines found
          </Typography>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            sx={{ mt: 2 }}
            onClick={() => setAddDialogOpen(true)}
          >
            Add your first machine
          </Button>
        </Box>
      )}

      <AddMachineDialog
        open={addDialogOpen}
        onClose={() => setAddDialogOpen(false)}
      />
    </Box>
  );
};

export default UserMachines;
