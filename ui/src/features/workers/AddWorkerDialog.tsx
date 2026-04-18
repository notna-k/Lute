import { useEffect, useState } from "react";
import { Check, Copy, Terminal } from "lucide-react";
import { apiClient } from "@/services/api";
import {
  Alert,
  Button,
  Dialog,
  IconButton,
  Spinner,
  Tooltip,
} from "@/components/ui";

interface AddWorkerDialogProps {
  open: boolean;
  onClose: () => void;
}

interface ClaimCodeResponse {
  code: string;
  expires_at: string;
}

function apiHttpOrigin(): string {
  const env = import.meta.env.VITE_API_URL;
  if (env !== undefined && env !== null && String(env).trim() !== "") {
    return String(env).replace(/\/$/, "");
  }
  if (typeof window !== "undefined") {
    return window.location.origin;
  }
  return "http://localhost:8080";
}

function useClaimCode(open: boolean) {
  const [code, setCode] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!open) return;
    setCode(null);
    setError(null);
    setLoading(true);
    apiClient
      .post<ClaimCodeResponse>("/api/v1/workers/claim-code")
      .then((res) => setCode(res.code))
      .catch((err: Error) =>
        setError(err.message || "Failed to get claim code")
      )
      .finally(() => setLoading(false));
  }, [open]);

  return { code, error, loading };
}

async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    const ta = document.createElement("textarea");
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    try {
      document.execCommand("copy");
    } finally {
      document.body.removeChild(ta);
    }
    return true;
  }
}

export function AddWorkerDialog({ open, onClose }: AddWorkerDialogProps) {
  const { code, error, loading } = useClaimCode(open);
  const [copied, setCopied] = useState(false);

  const origin = apiHttpOrigin();
  const installCommand = `curl -sSL ${origin}/api/public/v1/workers/bootstrap/install.sh | bash`;
  const setupCommand = `lute-worker setup --api ${origin}`;
  const fullCommand = code
    ? `${installCommand} && ${setupCommand} --claim-code ${code}`
    : "";

  const handleCopy = async () => {
    if (!fullCommand) return;
    await copyText(fullCommand);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <Dialog
      open={open}
      onClose={onClose}
      size="md"
      title={
        <span className="flex items-center gap-2">
          <Terminal className="h-5 w-5 text-primary" />
          Add new worker
        </span>
      }
      description="Run the command below on the target host. It will install the agent and register the worker to your account."
      footer={
        <Button variant="secondary" onClick={onClose}>
          Close
        </Button>
      }
    >
      <div className="flex flex-col gap-4">
        {loading && (
          <div className="flex items-center gap-2 text-sm text-fg-muted">
            <Spinner size={16} />
            Generating your claim code…
          </div>
        )}

        {error && (
          <Alert tone="danger" title="Could not generate a claim code">
            {error}. Make sure you are logged in and try again.
          </Alert>
        )}

        {code && (
          <>
            <p className="text-sm text-fg-muted">
              Your claim code expires in 15 minutes. Run the command on the host
              before it expires.
            </p>
            <div className="relative rounded-md border border-border bg-bg-inverse p-3 font-mono text-sm text-fg-inverse">
              <div className="flex items-start gap-2 pr-8">
                <span className="select-none text-success">$</span>
                <code className="whitespace-pre-wrap break-all">
                  {fullCommand}
                </code>
              </div>
              <div className="absolute right-2 top-2">
                <Tooltip content={copied ? "Copied!" : "Copy"}>
                  <IconButton
                    label="Copy command"
                    variant="ghost"
                    size="sm"
                    onClick={handleCopy}
                    className="text-fg-inverse hover:bg-white/10"
                  >
                    {copied ? (
                      <Check className="h-4 w-4 text-success" />
                    ) : (
                      <Copy className="h-4 w-4" />
                    )}
                  </IconButton>
                </Tooltip>
              </div>
            </div>
          </>
        )}

        <p className="text-sm text-fg-muted">
          The agent will prompt for a service name, collect system info, then
          start in the background and send heartbeats to the server.
        </p>
      </div>
    </Dialog>
  );
}

export default AddWorkerDialog;
