import { useState, useEffect } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Typography,
  Box,
  IconButton,
  Tooltip,
  Paper,
  Alert,
  CircularProgress,
} from '@mui/material';
import {
  ContentCopy as CopyIcon,
  Check as CheckIcon,
  Terminal as TerminalIcon,
} from '@mui/icons-material';
import { apiClient } from '../services/api';

interface AddMachineDialogProps {
  open: boolean;
  onClose: () => void;
}

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

interface ClaimCodeResponse {
  code: string;
  expires_at: string;
}

const AddMachineDialog = ({ open, onClose }: AddMachineDialogProps) => {
  const [copied, setCopied] = useState(false);
  const [claimCode, setClaimCode] = useState<string | null>(null);
  const [claimError, setClaimError] = useState<string | null>(null);
  const [claimLoading, setClaimLoading] = useState(false);

  useEffect(() => {
    if (!open) return;
    setClaimCode(null);
    setClaimError(null);
    setClaimLoading(true);
    apiClient
      .post<ClaimCodeResponse>('/api/v1/agent/claim-code')
      .then((res) => {
        setClaimCode(res.code);
        setClaimError(null);
      })
      .catch((err: Error) => {
        setClaimCode(null);
        setClaimError(err.message || 'Failed to get claim code');
      })
      .finally(() => setClaimLoading(false));
  }, [open]);

  const installCommand = `curl -sSL ${API_URL}/api/v1/agent/install.sh | bash`;
  const setupCommand = `lute-agent --setup --api ${API_URL}`;
  const fullCommand =
    claimCode != null
      ? `${installCommand} && ${setupCommand} --claim-code ${claimCode}`
      : '';

  const handleCopy = async () => {
    if (!fullCommand) return;
    try {
      await navigator.clipboard.writeText(fullCommand);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      const textarea = document.createElement('textarea');
      textarea.value = fullCommand;
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="sm"
      fullWidth
      PaperProps={{ sx: { borderRadius: 2 } }}
    >
      <DialogTitle sx={{ pb: 1 }}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
          <TerminalIcon color="primary" />
          <Typography variant="h6" fontWeight="bold">
            Add New Machine
          </Typography>
        </Box>
      </DialogTitle>

      <DialogContent>
        <Alert severity="info" sx={{ mb: 2 }}>
          Run the command below on the target VM. It will install the agent and
          register the machine to your account. The machine will appear in your
          list automatically.
        </Alert>

        {claimLoading && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 2 }}>
            <CircularProgress size={20} />
            <Typography variant="body2" color="text.secondary">
              Generating your claim code…
            </Typography>
          </Box>
        )}

        {claimError && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {claimError}. A claim code is required to add a machine. Make sure
            you are logged in and try again.
          </Alert>
        )}

        {claimCode && (
          <>
            <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
              Your claim code expires in 15 minutes. Run the command on the VM
              before it expires.
            </Typography>

            <Paper
              elevation={0}
              sx={{
                bgcolor: 'grey.900',
                color: 'grey.100',
                p: 2,
                borderRadius: 1,
                fontFamily: 'monospace',
                fontSize: '0.85rem',
                position: 'relative',
                display: 'flex',
                alignItems: 'flex-start',
                gap: 1,
              }}
            >
              <Typography
                component="span"
                sx={{
                  color: 'success.light',
                  fontFamily: 'monospace',
                  fontSize: '0.85rem',
                  flexShrink: 0,
                }}
              >
                $
              </Typography>
              <Box
                component="code"
                sx={{
                  flex: 1,
                  wordBreak: 'break-all',
                  whiteSpace: 'pre-wrap',
                }}
              >
                {fullCommand}
              </Box>
              <Tooltip title={copied ? 'Copied!' : 'Copy'}>
                <IconButton
                  size="small"
                  onClick={handleCopy}
                  sx={{ color: 'grey.400', '&:hover': { color: 'grey.100' } }}
                >
                  {copied ? (
                    <CheckIcon fontSize="small" sx={{ color: 'success.light' }} />
                  ) : (
                    <CopyIcon fontSize="small" />
                  )}
                </IconButton>
              </Tooltip>
            </Paper>
          </>
        )}

        <Typography variant="body2" color="text.secondary" sx={{ mt: 2 }}>
          The agent will prompt for a service name, collect system info, then
          start in the background and send heartbeats to the server.
        </Typography>
      </DialogContent>

      <DialogActions sx={{ px: 3, pb: 2 }}>
        <Button onClick={onClose} variant="outlined">
          Close
        </Button>
      </DialogActions>
    </Dialog>
  );
};

export default AddMachineDialog;
