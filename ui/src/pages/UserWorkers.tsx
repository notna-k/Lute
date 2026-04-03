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
  IconButton,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
} from '@mui/material';
import { Add as AddIcon, Delete as DeleteIcon } from '@mui/icons-material';
import { Link } from 'react-router-dom';
import { useUserWorkers, useReEnableWorker, useDeleteWorker } from '../hooks/useWorkers';
import { Worker } from '../types';
import AddWorkerDialog from '../components/AddWorkerDialog';

const UserWorkers = () => {
  const { data: workers, isLoading, error, refetch } = useUserWorkers();
  const [addDialogOpen, setAddDialogOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Worker | null>(null);
  const reEnableMutation = useReEnableWorker();
  const deleteMutation = useDeleteWorker();

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

  const handleReEnable = (workerId: string) => {
    reEnableMutation.mutate(workerId, { onSuccess: () => refetch() });
  };

  const handleDeleteClick = (w: Worker) => {
    setDeleteTarget(w);
  };

  const handleDeleteConfirm = () => {
    if (!deleteTarget) return;
    deleteMutation.mutate(deleteTarget.id, {
      onSuccess: () => {
        setDeleteTarget(null);
        refetch();
      },
    });
  };

  const handleDeleteCancel = () => {
    setDeleteTarget(null);
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
          Failed to load workers: {error instanceof Error ? error.message : 'Unknown error'}
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
            My Workers
          </Typography>
          <Typography variant="body2" color="text.secondary">
            Manage your registered workers and agents
          </Typography>
        </Box>
        <Button
          variant="contained"
          startIcon={<AddIcon />}
          onClick={() => setAddDialogOpen(true)}
        >
          Add Worker
        </Button>
      </Box>

      {workers && workers.length > 0 ? (
        <Paper>
          <List>
            {workers.map((w: Worker, index: number) => (
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
                        bgcolor: 'primary.light',
                        color: 'primary.main',
                        width: 40,
                        height: 40,
                      }}
                    >
                      {w.name.charAt(0).toUpperCase()}
                    </Avatar>
                    <Box>
                      <Typography variant="subtitle1" fontWeight="medium">
                        {w.name}
                      </Typography>
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
                    {w.is_public && (
                      <Chip
                        label="Public"
                        size="small"
                        color="secondary"
                      />
                    )}
                    {w.status === 'dead' && (
                      <Button
                        variant="outlined"
                        color="primary"
                        size="small"
                        disabled={reEnableMutation.isPending && reEnableMutation.variables === w.id}
                        onClick={() => handleReEnable(w.id)}
                      >
                        {reEnableMutation.isPending && reEnableMutation.variables === w.id ? 'Re-enabling…' : 'Re-enable'}
                      </Button>
                    )}
                    <Button
                      component={Link}
                      to={`/workers/${w.id}`}
                      variant="text"
                      color="primary"
                      size="small"
                    >
                      Manage
                    </Button>
                    <IconButton
                      aria-label={`Delete ${w.name}`}
                      color="error"
                      size="small"
                      onClick={() => handleDeleteClick(w)}
                      disabled={deleteMutation.isPending && deleteMutation.variables === w.id}
                    >
                      <DeleteIcon fontSize="small" />
                    </IconButton>
                  </Stack>
                </Box>
              </ListItem>
            ))}
          </List>
        </Paper>
      ) : (
        <Box sx={{ textAlign: 'center', py: 6 }}>
          <Typography variant="body1" color="text.secondary" gutterBottom>
            No workers found
          </Typography>
          <Button
            variant="contained"
            startIcon={<AddIcon />}
            sx={{ mt: 2 }}
            onClick={() => setAddDialogOpen(true)}
          >
            Add your first worker
          </Button>
        </Box>
      )}

      <AddWorkerDialog
        open={addDialogOpen}
        onClose={() => setAddDialogOpen(false)}
      />

      <Dialog
        open={!!deleteTarget}
        onClose={handleDeleteCancel}
        aria-labelledby="delete-worker-title"
        aria-describedby="delete-worker-description"
      >
        <DialogTitle id="delete-worker-title">
          Delete worker?
        </DialogTitle>
        <DialogContent>
          <DialogContentText id="delete-worker-description">
            {deleteTarget && (
              <>
                Are you sure you want to delete <strong>{deleteTarget.name}</strong>?
                This will remove the worker and its history. This action cannot be undone.
              </>
            )}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={handleDeleteCancel} autoFocus>
            Cancel
          </Button>
          <Button
            onClick={handleDeleteConfirm}
            color="error"
            variant="contained"
            disabled={deleteMutation.isPending}
          >
            {deleteMutation.isPending ? 'Deleting…' : 'Delete'}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
};

export default UserWorkers;
