import { useState } from 'react';
import {
  Dialog,
  DialogTitle,
  DialogContent,
  DialogActions,
  Button,
  Typography,
  Box,
  Stepper,
  Step,
  StepLabel,
  StepContent,
  IconButton,
  Tooltip,
  Paper,
  Alert,
} from '@mui/material';
import {
  ContentCopy as CopyIcon,
  Check as CheckIcon,
  Terminal as TerminalIcon,
} from '@mui/icons-material';

interface AddMachineDialogProps {
  open: boolean;
  onClose: () => void;
}

const API_URL = import.meta.env.VITE_API_URL || 'http://localhost:8080';

const AddMachineDialog = ({ open, onClose }: AddMachineDialogProps) => {
  const [copiedStep, setCopiedStep] = useState<number | null>(null);

  const installCommand = `curl -sSL ${API_URL}/api/v1/agent/install.sh | bash`;
  const setupCommand = `lute-agent --setup --api ${API_URL}`;

  const handleCopy = async (text: string, step: number) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedStep(step);
      setTimeout(() => setCopiedStep(null), 2000);
    } catch {
      // fallback
      const textarea = document.createElement('textarea');
      textarea.value = text;
      document.body.appendChild(textarea);
      textarea.select();
      document.execCommand('copy');
      document.body.removeChild(textarea);
      setCopiedStep(step);
      setTimeout(() => setCopiedStep(null), 2000);
    }
  };

  const CodeBlock = ({ code, step }: { code: string; step: number }) => (
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
        alignItems: 'center',
        gap: 1,
      }}
    >
      <Typography
        component="span"
        sx={{
          color: 'success.light',
          fontFamily: 'monospace',
          fontSize: '0.85rem',
          mr: 0.5,
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
        {code}
      </Box>
      <Tooltip title={copiedStep === step ? 'Copied!' : 'Copy'}>
        <IconButton
          size="small"
          onClick={() => handleCopy(code, step)}
          sx={{ color: 'grey.400', '&:hover': { color: 'grey.100' } }}
        >
          {copiedStep === step ? (
            <CheckIcon fontSize="small" sx={{ color: 'success.light' }} />
          ) : (
            <CopyIcon fontSize="small" />
          )}
        </IconButton>
      </Tooltip>
    </Paper>
  );

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
        <Alert severity="info" sx={{ mb: 3 }}>
          Run these commands on the target VM to install and register the agent.
          The agent will collect system information and create the machine automatically.
        </Alert>

        <Stepper orientation="vertical" activeStep={-1}>
          {/* Step 1 */}
          <Step active>
            <StepLabel>
              <Typography fontWeight="medium">Install the agent</Typography>
            </StepLabel>
            <StepContent>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
                Download and install the Lute agent binary on your VM.
                This auto-detects the OS and architecture.
              </Typography>
              <CodeBlock code={installCommand} step={1} />
            </StepContent>
          </Step>

          {/* Step 2 */}
          <Step active>
            <StepLabel>
              <Typography fontWeight="medium">Run setup</Typography>
            </StepLabel>
            <StepContent>
              <Typography variant="body2" color="text.secondary" sx={{ mb: 1.5 }}>
                The agent will ask for a service name, collect system info
                (hostname, OS, CPU, memory, IP), and register with the server.
              </Typography>
              <CodeBlock code={setupCommand} step={2} />
            </StepContent>
          </Step>

          {/* Step 3 */}
          <Step active>
            <StepLabel>
              <Typography fontWeight="medium">Done!</Typography>
            </StepLabel>
            <StepContent>
              <Typography variant="body2" color="text.secondary">
                The machine will appear in your list automatically.
                The agent will keep running and send heartbeats to the server.
              </Typography>
            </StepContent>
          </Step>
        </Stepper>
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

