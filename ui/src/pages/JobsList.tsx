import { useState, useCallback, useEffect } from 'react';
import {
    Box,
    Typography,
    Paper,
    Button,
    Chip,
    CircularProgress,
    Alert,
    Table,
    TableBody,
    TableCell,
    TableContainer,
    TableHead,
    TableRow,
    Dialog,
    DialogTitle,
    DialogContent,
    DialogActions,
    TextField,
    MenuItem,
    Stack,
    FormControl,
    InputLabel,
    Select,
    IconButton,
    Tooltip,
} from '@mui/material';
import { Add as AddIcon, Refresh as RefreshIcon, Delete as DeleteIcon } from '@mui/icons-material';
import { Link, useNavigate } from 'react-router-dom';
import { jobService, Job, EnqueueRequest } from '../services/jobService';

const STATUS_COLORS: Record<string, 'success' | 'warning' | 'error' | 'default' | 'info'> = {
    done: 'success',
    running: 'info',
    pending: 'warning',
    dead: 'error',
    cancelled: 'default',
};

const RUNTIME_OPTIONS: { label: string; value: string }[] = [
    { label: 'Node:25.0.0 - Latest stable', value: 'node:25' },
    { label: 'Node:24.0.1 - LTS', value: 'node:24' },
    { label: 'Node:22.0.0 - LTS', value: 'node:22' },
    { label: 'Node:20.0.0 - LTS', value: 'node:20' },
    { label: 'Python:3.13.0 - Latest stable', value: 'python:3.13' },
    { label: 'Python:3.12.0 - LTS', value: 'python:3.12' },
    { label: 'Python:3.11.0 - LTS', value: 'python:3.11' },
    { label: 'Python:2.7.18 - LTS (legacy)', value: 'python:2.7' },
    { label: 'JVM:25.0.1 - Latest stable', value: 'eclipse-temurin:25-jdk' },
    { label: 'JVM:21.0.9 - LTS', value: 'eclipse-temurin:21-jdk' },
    { label: 'JVM:17.0.17 - LTS', value: 'eclipse-temurin:17-jdk' },
    { label: 'JVM:11.0.29 - LTS', value: 'eclipse-temurin:11-jdk' },
    { label: 'JVM:8u471 - LTS', value: 'eclipse-temurin:8-jdk' },
    { label: 'JVM:7 - Legacy', value: 'eclipse-temurin:7-jdk' },
];

function formatTs(unix: number | undefined): string {
    if (!unix) return '—';
    return new Date(unix * 1000).toLocaleString();
}

const DEFAULT_QUEUE = 'default';

const INITIAL_FORM: EnqueueRequest = {
    queue: DEFAULT_QUEUE,
    type: 'container',
    timeout_sec: 300,
    max_retries: 3,
};

type ParamRow = { key: string; value: string };
const initialParamRow = (): ParamRow => ({ key: '', value: '' });

export default function JobsList() {
    const navigate = useNavigate();
    const [jobs, setJobs] = useState<Job[]>([]);
    const [queues, setQueues] = useState<string[]>([DEFAULT_QUEUE]);
    const [selectedQueue, setSelectedQueue] = useState(DEFAULT_QUEUE);
    const [loading, setLoading] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const [dialogOpen, setDialogOpen] = useState(false);
    const [form, setForm] = useState<EnqueueRequest>(INITIAL_FORM);
    const [sourceRepository, setSourceRepository] = useState('');
    const [runtime, setRuntime] = useState('python:3.12');
    const [command, setCommand] = useState('');
    const [paramsRows, setParamsRows] = useState<ParamRow[]>([initialParamRow()]);
    const [submitting, setSubmitting] = useState(false);
    const [submitError, setSubmitError] = useState<string | null>(null);

    const fetchQueues = useCallback(async () => {
        try {
            const res = await jobService.listQueues();
            const names = res.queues.map((q) => q.name);
            if (names.length > 0) setQueues(names);
        } catch {
            // keep default
        }
    }, []);

    const fetchJobs = useCallback(async (queue: string) => {
        setLoading(true);
        setError(null);
        try {
            const res = await jobService.listJobs(queue, { limit: 100 });
            setJobs(res.jobs ?? []);
        } catch (e) {
            setError(e instanceof Error ? e.message : 'Failed to load jobs');
        } finally {
            setLoading(false);
        }
    }, []);

    useEffect(() => {
        fetchQueues();
    }, [fetchQueues]);

    useEffect(() => {
        fetchJobs(selectedQueue);
    }, [selectedQueue, fetchJobs]);

    const handleOpenDialog = () => {
        setForm({ ...INITIAL_FORM, queue: selectedQueue });
        setSourceRepository('');
        setRuntime('python:3.12');
        setCommand('');
        setParamsRows([initialParamRow()]);
        setSubmitError(null);
        setDialogOpen(true);
    };

    const addParamRow = () => setParamsRows((r) => [...r, initialParamRow()]);
    const removeParamRow = (index: number) => setParamsRows((r) => r.filter((_, i) => i !== index));
    const updateParamRow = (index: number, field: 'key' | 'value', value: string) => {
        setParamsRows((r) => r.map((row, i) => (i === index ? { ...row, [field]: value } : row)));
    };

    const buildPayload = (): unknown => {
        if (form.type === 'noop') return {};
        const request_params: Record<string, string> = {};
        paramsRows.forEach(({ key, value }) => {
            if (key.trim()) request_params[key.trim()] = value;
        });
        return {
            source_repository: sourceRepository.trim() || undefined,
            runtime: runtime.trim() || undefined,
            command: command.trim() || undefined,
            request_params,
        };
    };

    const handleSubmit = async () => {
        setSubmitting(true);
        setSubmitError(null);
        try {
            const payload = form.type === 'noop' ? {} : buildPayload();
            const res = await jobService.enqueueJob({ ...form, payload });
            setDialogOpen(false);
            await fetchJobs(selectedQueue);
            navigate(`/jobs/${res.job_id}`);
        } catch (e) {
            setSubmitError(e instanceof Error ? e.message : 'Failed to enqueue job');
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <Box>
            {/* Header */}
            <Box sx={{ mb: 4, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                <Box>
                    <Typography variant="h4" component="h1" gutterBottom fontWeight="bold">
                        Jobs
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                        View and trigger worker jobs
                    </Typography>
                </Box>
                <Stack direction="row" spacing={1} alignItems="center">
                    <Tooltip title="Refresh">
                        <IconButton onClick={() => fetchJobs(selectedQueue)} disabled={loading}>
                            <RefreshIcon />
                        </IconButton>
                    </Tooltip>
                    <Button variant="contained" startIcon={<AddIcon />} onClick={handleOpenDialog}>
                        Trigger new
                    </Button>
                </Stack>
            </Box>

            {/* Queue selector */}
            <Box sx={{ mb: 2, maxWidth: 240 }}>
                <FormControl size="small" fullWidth>
                    <InputLabel>Queue</InputLabel>
                    <Select
                        label="Queue"
                        value={selectedQueue}
                        onChange={(e) => setSelectedQueue(e.target.value)}
                    >
                        {queues.map((q) => (
                            <MenuItem key={q} value={q}>{q}</MenuItem>
                        ))}
                    </Select>
                </FormControl>
            </Box>

            {/* Error */}
            {error && (
                <Alert severity="error" sx={{ mb: 2 }}>
                    {error}
                </Alert>
            )}

            {/* Loading */}
            {loading && (
                <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
                    <CircularProgress />
                </Box>
            )}

            {/* Table */}
            {!loading && (
                jobs.length === 0 ? (
                    <Box sx={{ textAlign: 'center', py: 8 }}>
                        <Typography color="text.secondary">
                            No jobs in queue <strong>{selectedQueue}</strong>
                        </Typography>
                    </Box>
                ) : (
                    <TableContainer component={Paper}>
                        <Table size="small">
                            <TableHead>
                                <TableRow>
                                    <TableCell>ID</TableCell>
                                    <TableCell>Type</TableCell>
                                    <TableCell>Queue</TableCell>
                                    <TableCell>Status</TableCell>
                                    <TableCell>Enqueued</TableCell>
                                    <TableCell>Started</TableCell>
                                    <TableCell>Done</TableCell>
                                    <TableCell>Attempts</TableCell>
                                </TableRow>
                            </TableHead>
                            <TableBody>
                                {jobs.map((job) => (
                                    <TableRow
                                        key={job.id}
                                        hover
                                        sx={{ cursor: 'pointer' }}
                                        onClick={() => navigate(`/jobs/${job.id}`)}
                                    >
                                        <TableCell>
                                            <Typography
                                                component={Link}
                                                to={`/jobs/${job.id}`}
                                                variant="body2"
                                                sx={{ textDecoration: 'none', color: 'primary.main', fontFamily: 'monospace' }}
                                                onClick={(e) => e.stopPropagation()}
                                            >
                                                {job.id.slice(0, 8)}…
                                            </Typography>
                                        </TableCell>
                                        <TableCell>
                                            <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                                                {job.type}
                                            </Typography>
                                        </TableCell>
                                        <TableCell>{job.queue}</TableCell>
                                        <TableCell>
                                            <Chip
                                                label={job.status}
                                                color={STATUS_COLORS[job.status] ?? 'default'}
                                                size="small"
                                            />
                                        </TableCell>
                                        <TableCell>
                                            <Typography variant="caption">{formatTs(job.enqueued_at)}</Typography>
                                        </TableCell>
                                        <TableCell>
                                            <Typography variant="caption">{formatTs(job.started_at)}</Typography>
                                        </TableCell>
                                        <TableCell>
                                            <Typography variant="caption">{formatTs(job.done_at)}</Typography>
                                        </TableCell>
                                        <TableCell>{job.attempts}</TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </TableContainer>
                )
            )}

            {/* Trigger new dialog */}
            <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} maxWidth="sm" fullWidth>
                <DialogTitle>Trigger new job</DialogTitle>
                <DialogContent>
                    <Stack spacing={2} sx={{ mt: 1 }}>
                        {submitError && <Alert severity="error">{submitError}</Alert>}
                        <TextField
                            label="Queue"
                            value={form.queue}
                            onChange={(e) => setForm((f) => ({ ...f, queue: e.target.value }))}
                            size="small"
                            fullWidth
                        />
                        <TextField
                            label="Type"
                            select
                            value={form.type}
                            onChange={(e) => setForm((f) => ({ ...f, type: e.target.value }))}
                            size="small"
                            fullWidth
                        >
                            <MenuItem value="noop">noop</MenuItem>
                            <MenuItem value="container">container</MenuItem>
                        </TextField>

                        {form.type === 'container' && (
                            <>
                                <TextField
                                    label="Source Repository"
                                    value={sourceRepository}
                                    onChange={(e) => setSourceRepository(e.target.value)}
                                    size="small"
                                    fullWidth
                                    placeholder="https://github.com/owner/repo.git"
                                />
                                <FormControl size="small" fullWidth>
                                    <InputLabel>Runtime</InputLabel>
                                    <Select
                                        label="Runtime"
                                        value={runtime}
                                        onChange={(e) => setRuntime(e.target.value)}
                                    >
                                        {RUNTIME_OPTIONS.map((opt) => (
                                            <MenuItem key={opt.value} value={opt.value}>
                                                {opt.label}
                                            </MenuItem>
                                        ))}
                                    </Select>
                                </FormControl>
                                <TextField
                                    label="Command"
                                    value={command}
                                    onChange={(e) => setCommand(e.target.value)}
                                    size="small"
                                    fullWidth
                                    multiline
                                    minRows={2}
                                    placeholder="bash script to run in the container"
                                />
                                <Box>
                                    <Typography variant="subtitle2" sx={{ mb: 1 }}>
                                        Params (key-value → env vars)
                                    </Typography>
                                    {paramsRows.map((row, index) => (
                                        <Stack key={index} direction="row" spacing={1} sx={{ mb: 1 }} alignItems="center">
                                            <TextField
                                                placeholder="Key"
                                                value={row.key}
                                                onChange={(e) => updateParamRow(index, 'key', e.target.value)}
                                                size="small"
                                                sx={{ flex: 1 }}
                                            />
                                            <TextField
                                                placeholder="Value"
                                                value={row.value}
                                                onChange={(e) => updateParamRow(index, 'value', e.target.value)}
                                                size="small"
                                                sx={{ flex: 1 }}
                                            />
                                            <IconButton
                                                size="small"
                                                onClick={() => removeParamRow(index)}
                                                aria-label="Remove row"
                                                color="error"
                                            >
                                                <DeleteIcon fontSize="small" />
                                            </IconButton>
                                        </Stack>
                                    ))}
                                    <Button size="small" startIcon={<AddIcon />} onClick={addParamRow}>
                                        Add row
                                    </Button>
                                </Box>
                            </>
                        )}

                        <Stack direction="row" spacing={2}>
                            <TextField
                                label="Timeout (sec)"
                                type="number"
                                value={form.timeout_sec ?? 300}
                                onChange={(e) => setForm((f) => ({ ...f, timeout_sec: Number(e.target.value) }))}
                                size="small"
                                sx={{ flex: 1 }}
                            />
                            <TextField
                                label="Max retries"
                                type="number"
                                value={form.max_retries ?? 3}
                                onChange={(e) => setForm((f) => ({ ...f, max_retries: Number(e.target.value) }))}
                                size="small"
                                sx={{ flex: 1 }}
                            />
                        </Stack>
                    </Stack>
                </DialogContent>
                <DialogActions>
                    <Button onClick={() => setDialogOpen(false)}>Cancel</Button>
                    <Button
                        variant="contained"
                        onClick={handleSubmit}
                        disabled={submitting || !form.queue || !form.type || (form.type === 'container' && (!sourceRepository.trim() || !runtime.trim() || !command.trim()))}
                    >
                        {submitting ? 'Triggering…' : 'Trigger'}
                    </Button>
                </DialogActions>
            </Dialog>
        </Box>
    );
}
