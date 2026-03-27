import { useMemo, useState, type RefObject } from 'react';
import {
    Box,
    Checkbox,
    CircularProgress,
    FormControl,
    InputLabel,
    ListItemText,
    MenuItem,
    OutlinedInput,
    Select,
    type SelectChangeEvent,
    Typography,
} from '@mui/material';
import {
    ALL_LOG_SEVERITIES,
    parseSlogLogLine,
    SEVERITY_LIGHT_UI_SWATCH,
    SEVERITY_STYLE,
    SOURCE_BADGE_LABEL,
    SOURCE_BADGE_TITLE,
    SOURCE_STYLE,
    type LogSeverity,
} from '../utils/slogLogLine';

const ITEM_HEIGHT = 48;
const ITEM_PADDING_TOP = 8;
const MenuProps = {
    PaperProps: {
        sx: {
            maxHeight: ITEM_HEIGHT * 6 + ITEM_PADDING_TOP,
            width: 280,
            bgcolor: 'background.paper',
        },
    },
};

export interface JobLogConsoleProps {
    lines: string[];
    logBoxRef: RefObject<HTMLDivElement | null>;
    onScroll: () => void;
    /** Hide scroll area (initial load) */
    hideScrollArea: boolean;
    logsLoading: boolean;
    loadingOlder: boolean;
    logHasMore: boolean;
}

export function JobLogConsole({
    lines: linesIn,
    logBoxRef,
    onScroll,
    hideScrollArea,
    logsLoading,
    loadingOlder,
    logHasMore,
}: JobLogConsoleProps) {
    const lines = linesIn ?? [];
    const [enabledSeverities, setEnabledSeverities] = useState<Set<LogSeverity>>(
        () => new Set(ALL_LOG_SEVERITIES)
    );

    const parsedRows = useMemo(
        () => lines.map((raw) => ({ raw, parsed: parseSlogLogLine(raw) })),
        [lines]
    );

    const visibleRows = useMemo(
        () => parsedRows.filter((row) => enabledSeverities.has(row.parsed.severity)),
        [parsedRows, enabledSeverities]
    );

    const handleSeverityChange = (event: SelectChangeEvent<LogSeverity[]>) => {
        const v = event.target.value;
        if (v == null) return;
        const next = (typeof v === 'string' ? v.split(',') : v) as LogSeverity[];
        setEnabledSeverities(new Set(Array.isArray(next) ? next : []));
    };

    const allSelected = enabledSeverities.size === ALL_LOG_SEVERITIES.length;

    return (
        <Box sx={{ position: 'relative' }}>
            <FormControl size="small" sx={{ minWidth: 280, mb: 2 }} fullWidth>
                <InputLabel id="job-log-severity-label">Severity</InputLabel>
                <Select
                    labelId="job-log-severity-label"
                    multiple
                    value={Array.from(enabledSeverities)}
                    onChange={handleSeverityChange}
                    input={<OutlinedInput label="Severity" />}
                    renderValue={(selected) => {
                        const s = selected as LogSeverity[] | null | undefined;
                        if (s == null || !Array.isArray(s)) {
                            return 'All levels';
                        }
                        return s.length === ALL_LOG_SEVERITIES.length
                            ? 'All levels'
                            : [...s].sort().join(', ');
                    }}
                    MenuProps={MenuProps}
                >
                    {ALL_LOG_SEVERITIES.map((sev) => (
                        <MenuItem key={sev} value={sev} dense sx={{ py: 0.5 }}>
                            <Checkbox checked={enabledSeverities.has(sev)} size="small" sx={{ color: 'text.secondary' }} />
                            <Box
                                aria-hidden
                                sx={{
                                    width: 10,
                                    height: 10,
                                    borderRadius: 0.5,
                                    bgcolor: SEVERITY_LIGHT_UI_SWATCH[sev],
                                    mr: 1.25,
                                    flexShrink: 0,
                                    border: 1,
                                    borderColor: 'divider',
                                }}
                            />
                            <ListItemText
                                primary={sev}
                                primaryTypographyProps={{
                                    sx: {
                                        color: 'text.primary',
                                        fontFamily: 'monospace',
                                        fontSize: 13,
                                        fontWeight: 600,
                                    },
                                }}
                            />
                        </MenuItem>
                    ))}
                </Select>
            </FormControl>

            {logsLoading && lines.length === 0 && (
                <Box sx={{ display: 'flex', justifyContent: 'center', py: 3 }}>
                    <CircularProgress size={28} />
                </Box>
            )}
            <Box
                ref={logBoxRef}
                onScroll={onScroll}
                sx={{
                    display: hideScrollArea ? 'none' : 'block',
                    maxHeight: 420,
                    overflow: 'auto',
                    bgcolor: 'grey.900',
                    borderRadius: 1,
                    p: 2,
                    fontSize: 12,
                    fontFamily: 'monospace',
                }}
            >
                {loadingOlder && (
                    <Typography variant="caption" color="grey.500" display="block" sx={{ mb: 1 }}>
                        Loading older…
                    </Typography>
                )}
                {lines.length === 0 && !logsLoading ? (
                    <Typography variant="body2" color="grey.500">
                        No log lines in this window yet.
                    </Typography>
                ) : visibleRows.length === 0 && lines.length > 0 ? (
                    <Typography variant="body2" color="grey.500">
                        No lines match the selected severities.
                        {!allSelected && ' Adjust the filter above.'}
                    </Typography>
                ) : (
                    visibleRows.map((row, i) => {
                        const { parsed } = row;
                        const sevSt = SEVERITY_STYLE[parsed.severity];
                        const srcSt = SOURCE_STYLE[parsed.source];
                        return (
                            <Box
                                key={`${i}-${row.raw.slice(0, 48)}`}
                                sx={{
                                    display: 'grid',
                                    gridTemplateColumns: {
                                        xs: '1fr',
                                        sm: 'minmax(140px,168px) minmax(64px,auto) minmax(52px,auto) minmax(0,1fr)',
                                    },
                                    gap: { xs: 0.35, sm: 0.75 },
                                    alignItems: 'start',
                                    py: 0.4,
                                    borderBottom: '1px solid',
                                    borderColor: 'rgba(255,255,255,0.06)',
                                    '&:last-of-type': { borderBottom: 'none' },
                                }}
                            >
                                <Typography
                                    component="span"
                                    variant="caption"
                                    sx={{
                                        color: 'grey.500',
                                        fontFamily: 'monospace',
                                        fontSize: '0.7rem',
                                        whiteSpace: 'nowrap',
                                        overflow: 'hidden',
                                        textOverflow: 'ellipsis',
                                    }}
                                    title={parsed.timestampDisplay}
                                >
                                    {parsed.timestampDisplay}
                                </Typography>
                                <Box
                                    component="span"
                                    title={SOURCE_BADGE_TITLE[parsed.source]}
                                    sx={{
                                        color: srcSt.color,
                                        bgcolor: srcSt.labelBg,
                                        fontWeight: 700,
                                        fontSize: '0.62rem',
                                        letterSpacing: 0.03,
                                        px: 0.6,
                                        py: 0.15,
                                        borderRadius: 0.5,
                                        textAlign: 'center',
                                        justifySelf: 'start',
                                        whiteSpace: 'nowrap',
                                    }}
                                >
                                    {SOURCE_BADGE_LABEL[parsed.source]}
                                </Box>
                                <Box
                                    component="span"
                                    sx={{
                                        color: sevSt.color,
                                        bgcolor: sevSt.labelBg,
                                        fontWeight: 700,
                                        fontSize: '0.65rem',
                                        letterSpacing: 0.04,
                                        px: 0.75,
                                        py: 0.15,
                                        borderRadius: 0.5,
                                        textAlign: 'center',
                                        justifySelf: 'start',
                                    }}
                                >
                                    {parsed.severity}
                                </Box>
                                <Typography
                                    component="span"
                                    variant="body2"
                                    sx={{
                                        color: sevSt.color,
                                        fontFamily: 'monospace',
                                        fontSize: '0.75rem',
                                        lineHeight: 1.45,
                                        wordBreak: 'break-all',
                                        whiteSpace: 'pre-wrap',
                                    }}
                                >
                                    {parsed.message}
                                </Typography>
                            </Box>
                        );
                    })
                )}
            </Box>
            {!logHasMore && lines.length > 0 && (
                <Typography variant="caption" color="text.secondary" sx={{ mt: 1, display: 'block' }}>
                    Start of log file
                </Typography>
            )}
        </Box>
    );
}
