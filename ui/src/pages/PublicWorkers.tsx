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
import { usePublicWorkers } from '../hooks/useWorkers';

const PublicWorkers = () => {
  const { data: workers, isLoading, error, refetch } = usePublicWorkers();

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
          Failed to load public workers: {error instanceof Error ? error.message : 'Unknown error'}
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
          Public Workers
        </Typography>
        <Typography variant="body2" color="text.secondary">
          Browse shared workers from the community
        </Typography>
      </Box>

      {workers && workers.length > 0 ? (
        <Paper>
          <List>
            {workers.map((w, index) => (
              <ListItem
                key={w.id}
                sx={{
                  borderBottom: index < workers.length - 1 ? 1 : 0,
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
                      {w.name.charAt(0).toUpperCase()}
                    </Avatar>
                    <Box>
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <Typography variant="subtitle1" fontWeight="medium">
                          {w.name}
                        </Typography>
                        <Chip
                          label="Public"
                          size="small"
                          color="secondary"
                          sx={{ height: 20 }}
                        />
                      </Box>
                      <Typography variant="body2" color="text.secondary">
                        {w.description || 'No description'}
                      </Typography>
                    </Box>
                  </Box>
                  <Stack direction="row" spacing={2} alignItems="center">
                    <Chip
                      label={w.status}
                      color={getStatusColor(w.status)}
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
            No public workers available
          </Typography>
        </Box>
      )}
    </Box>
  );
};

export default PublicWorkers;
