import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { ArrowLeft, RefreshCw, RotateCcw, X } from "lucide-react";
import {
  Alert,
  Badge,
  Button,
  Card,
  CardContent,
  CardHeader,
  CardTitle,
  PageHeader,
  Skeleton,
  Tooltip,
} from "@/components/ui";
import { jobService, type Job } from "@/services/jobService";
import { LogViewer } from "@/features/jobs/LogViewer";
import type { BadgeTone } from "@/components/ui";

const JOB_LOG_PAGE = 200;

const STATUS_TONE: Record<string, BadgeTone> = {
  done: "success",
  running: "info",
  pending: "warning",
  dead: "danger",
  cancelled: "neutral",
};

function formatTs(unix: number | undefined): string {
  if (!unix) return "—";
  return new Date(unix * 1000).toLocaleString();
}

function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div className="flex flex-col gap-1">
      <span className="text-xxs font-semibold uppercase tracking-wider text-fg-muted">
        {label}
      </span>
      <div className="text-sm text-fg">{children}</div>
    </div>
  );
}

const ExecutionDetail = () => {
  const { id } = useParams<{ id: string }>();

  const [job, setJob] = useState<Job | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [retrying, setRetrying] = useState(false);
  const [cancelling, setCancelling] = useState(false);

  const logBoxRef = useRef<HTMLDivElement>(null);
  const loadingOlderRef = useRef(false);
  const [logLines, setLogLines] = useState<string[]>([]);
  const [logNextCursor, setLogNextCursor] = useState<string | null>(null);
  const [logHasMore, setLogHasMore] = useState(false);
  const [logsLoading, setLogsLoading] = useState(false);
  const [logsError, setLogsError] = useState<string | null>(null);
  const [loadingOlder, setLoadingOlder] = useState(false);

  const fetchJob = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      const j = await jobService.getJob(id);
      setJob(j);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Failed to load job");
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    fetchJob();
  }, [fetchJob]);

  const loadLogsTail = useCallback(
    async (cursor?: string) => {
      if (!id) return;
      const prepend = Boolean(cursor);
      if (prepend) {
        if (loadingOlderRef.current) return;
        loadingOlderRef.current = true;
        setLoadingOlder(true);
      } else {
        setLogsLoading(true);
      }
      setLogsError(null);
      try {
        const r = await jobService.getJobLogs(id, {
          direction: "tail",
          limit: JOB_LOG_PAGE,
          cursor,
        });
        const el = logBoxRef.current;
        const prevScrollHeight = prepend && el ? el.scrollHeight : 0;
        const prevScrollTop = prepend && el ? el.scrollTop : 0;

        const chunk = Array.isArray(r.lines) ? r.lines : [];
        if (prepend) {
          setLogLines((prev) => [...chunk, ...prev]);
        } else {
          setLogLines(chunk);
        }
        setLogNextCursor(r.next_cursor ?? null);
        setLogHasMore(r.has_more);
        if (r.error) setLogsError(r.error);

        requestAnimationFrame(() => {
          const box = logBoxRef.current;
          if (!box) return;
          if (prepend) {
            box.scrollTop =
              box.scrollHeight - prevScrollHeight + prevScrollTop;
          } else {
            box.scrollTop = box.scrollHeight;
          }
        });
      } catch (e) {
        setLogsError(e instanceof Error ? e.message : "Failed to load logs");
        if (!prepend) {
          setLogLines([]);
          setLogHasMore(false);
          setLogNextCursor(null);
        }
      } finally {
        setLogsLoading(false);
        setLoadingOlder(false);
        loadingOlderRef.current = false;
      }
    },
    [id]
  );

  useEffect(() => {
    if (!id) return;
    setLogLines([]);
    setLogNextCursor(null);
    setLogHasMore(false);
    setLogsError(null);
    void loadLogsTail();
  }, [id, loadLogsTail]);

  const onLogScroll = () => {
    const el = logBoxRef.current;
    if (!el || logsLoading || loadingOlderRef.current || !logHasMore || !logNextCursor)
      return;
    if (el.scrollTop < 64) {
      void loadLogsTail(logNextCursor);
    }
  };

  const handleRetry = async () => {
    if (!id) return;
    setRetrying(true);
    setActionError(null);
    try {
      await jobService.retryJob(id);
      await fetchJob();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : "Retry failed");
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
      setActionError(e instanceof Error ? e.message : "Cancel failed");
    } finally {
      setCancelling(false);
    }
  };

  if (loading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-6 w-48" />
        <Skeleton className="h-10 w-80" />
        <Skeleton className="h-40 w-full rounded-lg" />
      </div>
    );
  }

  if (error || !job) {
    return (
      <>
        <Link
          to="/executions"
          className="mb-3 inline-flex items-center gap-1 text-sm text-fg-muted hover:text-fg"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to executions
        </Link>
        <Alert tone="danger">{error ?? "Job not found"}</Alert>
      </>
    );
  }

  const canRetry = job.status === "dead" || job.status === "done";
  const canCancel = job.status === "pending";

  return (
    <>
      <Link
        to="/executions"
        className="mb-3 inline-flex items-center gap-1 text-sm text-fg-muted hover:text-fg"
      >
        <ArrowLeft className="h-4 w-4" />
        Back to executions
      </Link>

      <PageHeader
        title={<span className="font-mono text-xl sm:text-2xl">{job.id}</span>}
        description={
          <span>
            {job.type} · {job.queue}
          </span>
        }
        actions={
          <div className="flex items-center gap-2">
            <Badge tone={STATUS_TONE[job.status] ?? "neutral"} dot>
              {job.status}
            </Badge>
            <Tooltip content="Refresh job and logs">
              <Button
                variant="outline"
                size="sm"
                leftIcon={<RefreshCw className="h-4 w-4" />}
                onClick={async () => {
                  await fetchJob();
                  void loadLogsTail();
                }}
              >
                Refresh
              </Button>
            </Tooltip>
            {canRetry && (
              <Button
                variant="outline"
                size="sm"
                leftIcon={<RotateCcw className="h-4 w-4" />}
                loading={retrying}
                onClick={handleRetry}
              >
                Retry
              </Button>
            )}
            {canCancel && (
              <Button
                variant="danger"
                size="sm"
                leftIcon={<X className="h-4 w-4" />}
                loading={cancelling}
                onClick={handleCancel}
              >
                Cancel
              </Button>
            )}
          </div>
        }
      />

      {actionError && (
        <Alert tone="danger" className="mb-4">
          {actionError}
        </Alert>
      )}

      <Card className="mb-4">
        <CardHeader>
          <CardTitle>Overview</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 sm:grid-cols-2 md:grid-cols-3">
            <Field label="Queue">{job.queue}</Field>
            <Field label="Type">
              <span className="font-mono text-sm">{job.type}</span>
            </Field>
            <Field label="Status">
              <Badge tone={STATUS_TONE[job.status] ?? "neutral"} dot>
                {job.status}
              </Badge>
            </Field>
            <Field label="Enqueued">{formatTs(job.enqueued_at)}</Field>
            <Field label="Started">{formatTs(job.started_at)}</Field>
            <Field label="Completed">{formatTs(job.done_at)}</Field>
            <Field label="Attempts">
              {job.attempts} / {job.max_retries}
            </Field>
            <Field label="Timeout">{job.timeout_sec}s</Field>
            {job.worker_id && (
              <Field label="Worker">
                <span className="font-mono text-sm">{job.worker_id}</span>
              </Field>
            )}
          </div>
        </CardContent>
      </Card>

      {job.error && (
        <Card className="mb-4">
          <CardHeader>
            <CardTitle>Error</CardTitle>
          </CardHeader>
          <CardContent>
            <Alert tone="danger">
              <pre className="whitespace-pre-wrap break-all font-mono text-xs">
                {job.error}
              </pre>
            </Alert>
          </CardContent>
        </Card>
      )}

      {job.payload != null && (
        <Card className="mb-4">
          <CardHeader>
            <CardTitle>Payload</CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="overflow-x-auto whitespace-pre-wrap break-all rounded-md bg-bg-subtle p-3 font-mono text-xs text-fg">
              {JSON.stringify(job.payload, null, 2)}
            </pre>
          </CardContent>
        </Card>
      )}

      <Card>
        <CardHeader>
          <CardTitle>Logs</CardTitle>
        </CardHeader>
        <CardContent>
          {logsError && (
            <Alert tone="warning" className="mb-3">
              {logsError}
            </Alert>
          )}
          <LogViewer
            lines={logLines}
            logBoxRef={logBoxRef}
            onScroll={onLogScroll}
            hideScrollArea={logLines.length === 0 && logsLoading}
            logsLoading={logsLoading}
            loadingOlder={loadingOlder}
            logHasMore={logHasMore}
          />
        </CardContent>
      </Card>
    </>
  );
};

export default ExecutionDetail;
