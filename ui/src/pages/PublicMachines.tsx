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
import { usePublicMachines } from '../hooks/useMachines';

const PublicMachines = () => {
  const { data: machines, isLoading, error, refetch } = usePublicMachines();

  const getStatusColor = (status: string): 'success' | 'error' | 'warning' | 'default' => {
    switch (status) {
      case 'running':
        return 'success';
      case 'stopped':
        return 'error';
      case 'paused':
        return 'warning';
      default:
        return 'default';
    }
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
          Failed to load public machines: {error instanceof Error ? error.message : 'Unknown error'}
        </Alert>
        <Button onClick={() => refetch()} variant="outlined">
          Retry
        </Button>
      </Box>
    );
  }

  return (
    <Box>
      <Box sx={{ mb: 4 }}>
        <Typography variant="h4" component="h1" gutterBottom fontWeight="bold">
          Public Machines
        </Typography>
        <Typography variant="body2" color="text.secondary">
          Browse shared virtual machines from the community
        </Typography>
      </Box>

      {machines && machines.length > 0 ? (
        <Paper>
          <List>
            {machines.map((machine, index) => (
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
                        bgcolor: 'secondary.light',
                        color: 'secondary.main',
                        width: 40,
                        height: 40,
                      }}
                    >
                      {machine.name.charAt(0).toUpperCase()}
                    </Avatar>
                    <Box>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <Typography variant="subtitle1" fontWeight="medium">
                          {machine.name}
                        </Typography>
                        <Chip
                          label="Public"
                          size="small"
                          color="secondary"
                          sx={{ height: 20 }}
                        />
                      </Box>
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
                    <Button
                      variant="text"
                      color="primary"
                      size="small"
                    >
                      View
                    </Button>
                  </Stack>
                </Box>
              </ListItem>
            ))}
          </List>
        </Paper>
      ) : (
        <Box sx={{ textAlign: 'center', py: 6 }}>
          <Typography variant="body1" color="text.secondary">
            No public machines available
          </Typography>
        </Box>
      )}
    </Box>
  );
};

export default PublicMachines;
