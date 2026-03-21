import { useState, useCallback, useEffect } from 'react';
import {
    Box,
    Typography,
    Paper,
    Chip,
    Button,
    CircularProgress,
    Alert,
    Stack,
    Divider,
    Tooltip,
} from '@mui/material';
import {
    ArrowBack as ArrowBackIcon,
    Refresh as RefreshIcon,
    Replay as RetryIcon,
    Cancel as CancelIcon,
} from '@mui/icons-material';
import { Link, useParams } from 'react-router-dom';
import { jobService, Job } from '../services/jobService';

const STATUS_COLORS: Record<string, 'success' | 'warning' | 'error' | 'default' | 'info'> = {
    done: 'success',
    running: 'info',
    pending: 'warning',
    dead: 'error',
    cancelled: 'default',
};

function formatTs(unix: number | undefined): string {
    if (!unix) return '—';
    return new Date(unix * 1000).toLocaleString();
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
    return (
        <Box>
            <Typography variant="caption" color="text.secondary" fontWeight="medium" sx={{ textTransform: 'uppercase', letterSpacing: 0.5 }}>
                {label}
            </Typography>
            <Box sx={{ mt: 0.25 }}>{children}</Box>
        </Box>
    );
}

export default function JobDetail() {
    const { id } = useParams<{ id: string }>();

    const [job, setJob] = useState<Job | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [actionError, setActionError] = useState<string | null>(null);
    const [retrying, setRetrying] = useState(false);
    const [cancelling, setCancelling] = useState(false);

    const fetchJob = useCallback(async () => {
        if (!id) return;
        setLoading(true);
        setError(null);
        try {
            const j = await jobService.getJob(id);
            setJob(j);
        } catch (e) {
            setError(e instanceof Error ? e.message : 'Failed to load job');
        } finally {
            setLoading(false);
        }
    }, [id]);

    useEffect(() => {
        fetchJob();
    }, [fetchJob]);

    const handleRetry = async () => {
        if (!id) return;
        setRetrying(true);
        setActionError(null);
        try {
            await jobService.retryJob(id);
            await fetchJob();
        } catch (e) {
            setActionError(e instanceof Error ? e.message : 'Retry failed');
        } finally {
            setRetrying(false);
        }
    };

    const handleCancel = async () => {
        if (!id) return;
        setCancelling(true);
        setActionError(null);
        try {
            await jobService.cancelJob(id);
            await fetchJob();
        } catch (e) {
            setActionError(e instanceof Error ? e.message : 'Cancel failed');
        } finally {
            setCancelling(false);
        }
    };

    if (loading) {
        return (
            <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '400px' }}>
                <CircularProgress />
            </Box>
        );
    }

    if (error || !job) {
        return (
            <Box>
                <Button component={Link} to="/jobs" startIcon={<ArrowBackIcon />} sx={{ mb: 2 }}>
                    Back to Jobs
                </Button>
                <Alert severity="error">{error ?? 'Job not found'}</Alert>
            </Box>
        );
    }

    const canRetry = job.status === 'dead' || job.status === 'done';
    const canCancel = job.status === 'pending';

    return (
        <Box>
            {/* Back */}
            <Button component={Link} to="/jobs" startIcon={<ArrowBackIcon />} sx={{ mb: 3 }}>
                Back to Jobs
            </Button>

            {/* Title + actions */}
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', mb: 3 }}>
                <Box>
                    <Stack direction="row" spacing={2} alignItems="center">
                        <Typography variant="h5" fontWeight="bold" sx={{ fontFamily: 'monospace' }}>
                            {job.id}
                        </Typography>
                        <Chip
                            label={job.status}
                            color={STATUS_COLORS[job.status] ?? 'default'}
                            size="small"
                        />
                    </Stack>
                    <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
                        {job.type} · {job.queue}
                    </Typography>
                </Box>
                <Stack direction="row" spacing={1}>
                    <Tooltip title="Refresh">
                        <Button variant="outlined" size="small" onClick={fetchJob} startIcon={<RefreshIcon />}>
                            Refresh
                        </Button>
                    </Tooltip>
                    {canRetry && (
                        <Button
                            variant="outlined"
                            size="small"
                            startIcon={<RetryIcon />}
                            onClick={handleRetry}
                            disabled={retrying}
                        >
                            {retrying ? 'Retrying…' : 'Retry'}
                        </Button>
                    )}
                    {canCancel && (
                        <Button
                            variant="outlined"
                            color="error"
                            size="small"
                            startIcon={<CancelIcon />}
                            onClick={handleCancel}
                            disabled={cancelling}
                        >
                            {cancelling ? 'Cancelling…' : 'Cancel'}
                        </Button>
                    )}
                </Stack>
            </Box>

            {actionError && (
                <Alert severity="error" sx={{ mb: 2 }}>{actionError}</Alert>
            )}

            {/* Overview */}
            <Paper sx={{ p: 3, mb: 2 }}>
                <Typography variant="subtitle1" fontWeight="bold" gutterBottom>
                    Overview
                </Typography>
                <Divider sx={{ mb: 2 }} />
                <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', sm: '1fr 1fr', md: '1fr 1fr 1fr' }, gap: 2.5 }}>
                    <Field label="Queue">
                        <Typography variant="body2">{job.queue}</Typography>
                    </Field>
                    <Field label="Type">
                        <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>{job.type}</Typography>
                    </Field>
                    <Field label="Status">
                        <Chip label={job.status} color={STATUS_COLORS[job.status] ?? 'default'} size="small" />
                    </Field>
                    <Field label="Enqueued">
                        <Typography variant="body2">{formatTs(job.enqueued_at)}</Typography>
                    </Field>
                    <Field label="Started">
                        <Typography variant="body2">{formatTs(job.started_at)}</Typography>
                    </Field>
                    <Field label="Completed">
                        <Typography variant="body2">{formatTs(job.done_at)}</Typography>
                    </Field>
                    <Field label="Attempts">
                        <Typography variant="body2">{job.attempts} / {job.max_retries}</Typography>
                    </Field>
                    <Field label="Timeout">
                        <Typography variant="body2">{job.timeout_sec}s</Typography>
                    </Field>
                    {job.worker_id && (
                        <Field label="Worker">
                            <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>{job.worker_id}</Typography>
                        </Field>
                    )}
                </Box>
            </Paper>

            {/* Error */}
            {job.error && (
                <Paper sx={{ p: 3, mb: 2 }}>
                    <Typography variant="subtitle1" fontWeight="bold" gutterBottom>
                        Error
                    </Typography>
                    <Divider sx={{ mb: 2 }} />
                    <Alert severity="error" sx={{ fontFamily: 'monospace', whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
                        {job.error}
                    </Alert>
                </Paper>
            )}

            {/* Payload */}
            {job.payload != null && (
                <Paper sx={{ p: 3, mb: 2 }}>
                    <Typography variant="subtitle1" fontWeight="bold" gutterBottom>
                        Payload
                    </Typography>
                    <Divider sx={{ mb: 2 }} />
                    <Box
                        component="pre"
                        sx={{
                            m: 0,
                            p: 2,
                            bgcolor: 'grey.50',
                            borderRadius: 1,
                            fontSize: 12,
                            fontFamily: 'monospace',
                            overflowX: 'auto',
                            whiteSpace: 'pre-wrap',
                            wordBreak: 'break-all',
                        }}
                    >
                        {JSON.stringify(job.payload, null, 2)}
                    </Box>
                </Paper>
            )}

            {/* Logs */}
            <Paper sx={{ p: 3 }}>
                <Typography variant="subtitle1" fontWeight="bold" gutterBottom>
                    Logs
                </Typography>
                <Divider sx={{ mb: 2 }} />
                {job.error ? (
                    <Box
                        component="pre"
                        sx={{
                            m: 0,
                            p: 2,
                            bgcolor: 'grey.900',
                            color: 'error.light',
                            borderRadius: 1,
                            fontSize: 12,
                            fontFamily: 'monospace',
                            overflowX: 'auto',
                            whiteSpace: 'pre-wrap',
                            wordBreak: 'break-all',
                        }}
                    >
                        {job.error}
                    </Box>
                ) : (
                    <Box
                        component="pre"
                        sx={{
                            m: 0,
                            p: 2,
                            bgcolor: 'grey.900',
                            color: 'grey.400',
                            borderRadius: 1,
                            fontSize: 12,
                            fontFamily: 'monospace',
                        }}
                    >
                        {job.status === 'done'
                            ? '(job completed successfully — execution stdout/stderr are not yet captured)'
                            : job.status === 'running'
                                ? '(job is running — live logs are not yet supported)'
                                : '(no logs available)'}
                    </Box>
                )}
            </Paper>
        </Box>
    );
}
