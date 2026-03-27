import { useEffect, useState } from 'react';
import {
    Alert,
    Box,
    Button,
    Dialog,
    DialogActions,
    DialogContent,
    DialogTitle,
    FormControl,
    IconButton,
    InputLabel,
    MenuItem,
    Select,
    Stack,
    TextField,
    Typography,
} from '@mui/material';
import { Add as AddIcon, Delete as DeleteIcon } from '@mui/icons-material';
import { jobService, EnqueueRequest } from '../services/jobService';

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

const INITIAL_FORM: EnqueueRequest = {
    queue: 'default',
    type: 'container',
    timeout_sec: 300,
    max_retries: 3,
};

type ParamRow = { key: string; value: string };
const initialParamRow = (): ParamRow => ({ key: '', value: '' });

export interface EnqueueJobDialogProps {
    open: boolean;
    onClose: () => void;
    defaultQueue?: string;
    onEnqueued: (jobId: string) => void;
}

export function EnqueueJobDialog({ open, onClose, defaultQueue = 'default', onEnqueued }: EnqueueJobDialogProps) {
    const [form, setForm] = useState<EnqueueRequest>({ ...INITIAL_FORM, queue: defaultQueue });

    useEffect(() => {
        if (!open) return;
        setForm({ ...INITIAL_FORM, queue: defaultQueue });
        setSourceRepository('');
        setRuntime('python:3.12');
        setCommand('');
        setParamsRows([initialParamRow()]);
        setSubmitError(null);
    }, [open, defaultQueue]);
    const [sourceRepository, setSourceRepository] = useState('');
    const [runtime, setRuntime] = useState('python:3.12');
    const [command, setCommand] = useState('');
    const [paramsRows, setParamsRows] = useState<ParamRow[]>([initialParamRow()]);
    const [submitting, setSubmitting] = useState(false);
    const [submitError, setSubmitError] = useState<string | null>(null);

    const handleClose = () => {
        onClose();
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
            onClose();
            onEnqueued(res.job_id);
        } catch (e) {
            setSubmitError(e instanceof Error ? e.message : 'Failed to enqueue job');
        } finally {
            setSubmitting(false);
        }
    };

    return (
        <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
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
                                label="Source repository (optional)"
                                value={sourceRepository}
                                onChange={(e) => setSourceRepository(e.target.value)}
                                size="small"
                                fullWidth
                                placeholder="Optional — https://github.com/owner/repo.git"
                            />
                            <FormControl size="small" fullWidth>
                                <InputLabel>Runtime</InputLabel>
                                <Select label="Runtime" value={runtime} onChange={(e) => setRuntime(e.target.value)}>
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
                <Button onClick={handleClose}>Cancel</Button>
                <Button
                    variant="contained"
                    onClick={handleSubmit}
                    disabled={
                        submitting ||
                        !form.queue ||
                        !form.type ||
                        (form.type === 'container' && (!runtime.trim() || !command.trim()))
                    }
                >
                    {submitting ? 'Triggering…' : 'Trigger'}
                </Button>
            </DialogActions>
        </Dialog>
    );
}
