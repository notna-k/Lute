import { useCallback, useEffect, useState } from 'react';
import {
    Alert,
    Box,
    Button,
    Chip,
    CircularProgress,
    LinearProgress,
    FormControl,
    IconButton,
    InputLabel,
    MenuItem,
    Paper,
    Select,
    Stack,
    Table,
    TableBody,
    TableCell,
    TableContainer,
    TableHead,
    TablePagination,
    TableRow,
    TextField,
    Tooltip,
    Typography,
} from '@mui/material';
import { Add as AddIcon, Refresh as RefreshIcon } from '@mui/icons-material';
import { Link, useNavigate } from 'react-router-dom';
import { EnqueueJobDialog } from '../components/EnqueueJobDialog';
import { executionService, type JobExecution } from '../services/executionService';

const PAGE_SIZE = 25;

function formatFinished(iso: string): string {
    if (!iso) return '—';
    const d = Date.parse(iso);
    if (Number.isNaN(d)) return iso;
    return new Date(d).toLocaleString();
}

function formatDuration(ms: number): string {
    if (ms < 1000) return `${ms} ms`;
    if (ms < 60_000) return `${(ms / 1000).toFixed(1)} s`;
    return `${Math.floor(ms / 60_000)}m ${Math.round((ms % 60_000) / 1000)}s`;
}

export default function ExecutionsHistory() {
    const navigate = useNavigate();
    const [rows, setRows] = useState<JobExecution[]>([]);
    const [total, setTotal] = useState(0);
    const [page, setPage] = useState(0);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [queueFilter, setQueueFilter] = useState('');
    const [typeFilter, setTypeFilter] = useState('');
    const [statusFilter, setStatusFilter] = useState<'' | 'success' | 'failed'>('');
    const [sort, setSort] = useState<'finished_at_desc' | 'finished_at_asc'>('finished_at_desc');
    const [queueOptions, setQueueOptions] = useState<string[]>([]);
    const [typeOptions, setTypeOptions] = useState<string[]>([]);
    const [dialogOpen, setDialogOpen] = useState(false);

    const loadFilterOptions = useCallback(async () => {
        try {
            const o = await executionService.filterOptions();
            setQueueOptions(o.queues ?? []);
            setTypeOptions(o.types ?? []);
        } catch {
            /* optional */
        }
    }, []);

    const fetchExecutions = useCallback(async () => {
        setLoading(true);
        setError(null);
        try {
            const res = await executionService.list({
                queue: queueFilter.trim() || undefined,
                type: typeFilter.trim() || undefined,
                status: statusFilter || undefined,
                offset: page * PAGE_SIZE,
                limit: PAGE_SIZE,
                sort,
            });
            setRows(res.executions ?? []);
            setTotal(res.total ?? 0);
        } catch (e) {
            setError(e instanceof Error ? e.message : 'Failed to load executions');
            setRows([]);
            setTotal(0);
        } finally {
            setLoading(false);
        }
    }, [queueFilter, typeFilter, statusFilter, sort, page]);

    useEffect(() => {
        loadFilterOptions();
    }, [loadFilterOptions]);

    useEffect(() => {
        fetchExecutions();
    }, [fetchExecutions]);

    const handleEnqueue = (jobId: string) => {
        navigate(`/jobs/${jobId}`);
    };

    return (
        <Box>
            <Box sx={{ mb: 3, display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', flexWrap: 'wrap', gap: 2 }}>
                <Box>
                    <Typography variant="h4" component="h1" fontWeight={700} gutterBottom>
                        Executions history
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                        Completed job runs from the database (newest first by default)
                    </Typography>
                </Box>
                <Stack direction="row" spacing={1}>
                    <Tooltip title="Refresh">
                        <span>
                            <IconButton onClick={() => void fetchExecutions()} disabled={loading} size="small">
                                <RefreshIcon />
                            </IconButton>
                        </span>
                    </Tooltip>
                    <Button variant="contained" startIcon={<AddIcon />} onClick={() => setDialogOpen(true)}>
                        Trigger job
                    </Button>
                </Stack>
            </Box>

            <Paper
                elevation={0}
                sx={{
                    p: 2,
                    mb: 2,
                    borderRadius: 2,
                    border: 1,
                    borderColor: 'divider',
                }}
            >
                <Stack direction={{ xs: 'column', md: 'row' }} spacing={2} alignItems={{ md: 'center' }} flexWrap="wrap" useFlexGap>
                    <FormControl size="small" sx={{ minWidth: 140 }}>
                        <InputLabel>Status</InputLabel>
                        <Select
                            label="Status"
                            value={statusFilter}
                            onChange={(e) => {
                                setStatusFilter(e.target.value as '' | 'success' | 'failed');
                                setPage(0);
                            }}
                        >
                            <MenuItem value="">All</MenuItem>
                            <MenuItem value="success">Success</MenuItem>
                            <MenuItem value="failed">Failed</MenuItem>
                        </Select>
                    </FormControl>
                    <TextField
                        size="small"
                        label="Queue"
                        value={queueFilter}
                        onChange={(e) => {
                            setQueueFilter(e.target.value);
                            setPage(0);
                        }}
                        placeholder="Exact match"
                        sx={{ minWidth: 160 }}
                        InputProps={{ inputProps: { list: 'exec-queue-options' } }}
                    />
                    <datalist id="exec-queue-options">
                        {queueOptions.map((q) => (
                            <option key={q} value={q} />
                        ))}
                    </datalist>
                    <TextField
                        size="small"
                        label="Type"
                        value={typeFilter}
                        onChange={(e) => {
                            setTypeFilter(e.target.value);
                            setPage(0);
                        }}
                        placeholder="Exact match"
                        sx={{ minWidth: 140 }}
                        InputProps={{ inputProps: { list: 'exec-type-options' } }}
                    />
                    <datalist id="exec-type-options">
                        {typeOptions.map((t) => (
                            <option key={t} value={t} />
                        ))}
                    </datalist>
                    <FormControl size="small" sx={{ minWidth: 200 }}>
                        <InputLabel>Sort</InputLabel>
                        <Select
                            label="Sort"
                            value={sort}
                            onChange={(e) => {
                                setSort(e.target.value as 'finished_at_desc' | 'finished_at_asc');
                                setPage(0);
                            }}
                        >
                            <MenuItem value="finished_at_desc">Newest first</MenuItem>
                            <MenuItem value="finished_at_asc">Oldest first</MenuItem>
                        </Select>
                    </FormControl>
                </Stack>
            </Paper>

            {error && (
                <Alert severity="error" sx={{ mb: 2 }}>
                    {error}
                </Alert>
            )}

            {loading && rows.length === 0 ? (
                <Box sx={{ display: 'flex', justifyContent: 'center', py: 8 }}>
                    <CircularProgress />
                </Box>
            ) : (
                <TableContainer
                    component={Paper}
                    elevation={0}
                    sx={{
                        borderRadius: 2,
                        border: 1,
                        borderColor: 'divider',
                        overflow: 'hidden',
                        position: 'relative',
                    }}
                >
                    {loading && rows.length > 0 && (
                        <LinearProgress sx={{ position: 'absolute', top: 0, left: 0, right: 0, zIndex: 1 }} />
                    )}
                    <Table size="small" sx={{ '& tbody tr:hover': { bgcolor: 'action.hover' } }}>
                        <TableHead>
                            <TableRow sx={{ bgcolor: 'grey.50' }}>
                                <TableCell sx={{ fontWeight: 600 }}>Finished</TableCell>
                                <TableCell sx={{ fontWeight: 600 }}>Status</TableCell>
                                <TableCell sx={{ fontWeight: 600 }}>Job</TableCell>
                                <TableCell sx={{ fontWeight: 600 }}>Queue</TableCell>
                                <TableCell sx={{ fontWeight: 600 }}>Type</TableCell>
                                <TableCell sx={{ fontWeight: 600 }}>Worker</TableCell>
                                <TableCell sx={{ fontWeight: 600 }} align="right">
                                    Duration
                                </TableCell>
                                <TableCell sx={{ fontWeight: 600 }}>Error</TableCell>
                            </TableRow>
                        </TableHead>
                        <TableBody>
                            {rows.length === 0 ? (
                                <TableRow>
                                    <TableCell colSpan={8} align="center" sx={{ py: 6, color: 'text.secondary' }}>
                                        No executions match your filters.
                                    </TableCell>
                                </TableRow>
                            ) : (
                                rows.map((ex) => (
                                    <TableRow
                                        key={ex.id}
                                        hover
                                        sx={{ cursor: 'pointer' }}
                                        onClick={() => navigate(`/jobs/${ex.job_id}`)}
                                    >
                                        <TableCell sx={{ whiteSpace: 'nowrap', fontVariantNumeric: 'tabular-nums' }}>
                                            {formatFinished(ex.finished_at)}
                                        </TableCell>
                                        <TableCell>
                                            <Chip
                                                label={ex.success ? 'Success' : 'Failed'}
                                                color={ex.success ? 'success' : 'error'}
                                                size="small"
                                                variant={ex.success ? 'filled' : 'filled'}
                                                sx={{ fontWeight: 600, height: 22 }}
                                            />
                                        </TableCell>
                                        <TableCell>
                                            <Typography
                                                component={Link}
                                                to={`/jobs/${ex.job_id}`}
                                                variant="body2"
                                                onClick={(e) => e.stopPropagation()}
                                                sx={{
                                                    fontFamily: 'monospace',
                                                    color: 'primary.main',
                                                    textDecoration: 'none',
                                                    '&:hover': { textDecoration: 'underline' },
                                                }}
                                            >
                                                {ex.job_id.length > 12 ? `${ex.job_id.slice(0, 10)}…` : ex.job_id}
                                            </Typography>
                                        </TableCell>
                                        <TableCell>{ex.queue}</TableCell>
                                        <TableCell>
                                            <Typography variant="body2" sx={{ fontFamily: 'monospace' }}>
                                                {ex.type}
                                            </Typography>
                                        </TableCell>
                                        <TableCell>
                                            <Typography variant="caption" sx={{ fontFamily: 'monospace', color: 'text.secondary' }}>
                                                {ex.worker_id
                                                    ? ex.worker_id.length > 10
                                                        ? `${ex.worker_id.slice(0, 8)}…`
                                                        : ex.worker_id
                                                    : '—'}
                                            </Typography>
                                        </TableCell>
                                        <TableCell align="right" sx={{ fontVariantNumeric: 'tabular-nums' }}>
                                            {formatDuration(ex.elapsed_ms)}
                                        </TableCell>
                                        <TableCell
                                            sx={{
                                                maxWidth: 220,
                                                overflow: 'hidden',
                                                textOverflow: 'ellipsis',
                                                whiteSpace: 'nowrap',
                                                color: ex.error ? 'error.main' : 'text.disabled',
                                            }}
                                            title={ex.error || ''}
                                        >
                                            {ex.error || '—'}
                                        </TableCell>
                                    </TableRow>
                                ))
                            )}
                        </TableBody>
                    </Table>
                    <TablePagination
                        component="div"
                        count={total}
                        page={page}
                        onPageChange={(_, p) => setPage(p)}
                        rowsPerPage={PAGE_SIZE}
                        rowsPerPageOptions={[PAGE_SIZE]}
                        onRowsPerPageChange={() => {}}
                    />
                </TableContainer>
            )}

            <EnqueueJobDialog
                open={dialogOpen}
                onClose={() => setDialogOpen(false)}
                defaultQueue="default"
                onEnqueued={handleEnqueue}
            />
        </Box>
    );
}
