import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Plus, RefreshCw } from "lucide-react";
import {
  Alert,
  Badge,
  Button,
  Card,
  EmptyState,
  Field,
  IconButton,
  Input,
  PageHeader,
  Pagination,
  Select,
  Skeleton,
  TBody,
  Td,
  Th,
  THead,
  Table,
  Tooltip,
  Tr,
} from "@/components/ui";
import { EnqueueJobDialog } from "@/features/jobs/EnqueueJobDialog";
import {
  executionService,
  type JobExecution,
} from "@/services/executionService";
import { cn } from "@/lib/cn";

const PAGE_SIZE = 25;

function formatFinished(iso: string): string {
  if (!iso) return "—";
  const ms = Date.parse(iso);
  if (Number.isNaN(ms)) return iso;
  return new Date(ms).toLocaleString();
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms} ms`;
  if (ms < 60_000) return `${(ms / 1000).toFixed(1)} s`;
  return `${Math.floor(ms / 60_000)}m ${Math.round((ms % 60_000) / 1000)}s`;
}

type StatusFilter = "" | "success" | "failed";
type SortOption = "finished_at_desc" | "finished_at_asc";

const STATUS_OPTIONS = [
  { value: "", label: "All statuses" },
  { value: "success", label: "Success" },
  { value: "failed", label: "Failed" },
] as const;

const SORT_OPTIONS = [
  { value: "finished_at_desc", label: "Newest first" },
  { value: "finished_at_asc", label: "Oldest first" },
] as const;

const Executions = () => {
  const navigate = useNavigate();
  const [rows, setRows] = useState<JobExecution[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [queueFilter, setQueueFilter] = useState("");
  const [typeFilter, setTypeFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusFilter>("");
  const [sort, setSort] = useState<SortOption>("finished_at_desc");

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
      setError(e instanceof Error ? e.message : "Failed to load executions");
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

  return (
    <>
      <PageHeader
        title="Executions"
        description="Completed job runs (newest first by default)."
        actions={
          <div className="flex items-center gap-2">
            <Tooltip content="Refresh">
              <IconButton
                label="Refresh"
                variant="outline"
                onClick={() => void fetchExecutions()}
                disabled={loading}
              >
                <RefreshCw
                  className={cn("h-4 w-4", loading && "animate-spin")}
                />
              </IconButton>
            </Tooltip>
            <Button
              leftIcon={<Plus className="h-4 w-4" />}
              onClick={() => setDialogOpen(true)}
            >
              Trigger job
            </Button>
          </div>
        }
      />

      <Card className="mb-4 p-4">
        <div className="grid gap-3 md:grid-cols-4">
          <Field label="Status">
            <Select<StatusFilter>
              value={statusFilter}
              onChange={(v) => {
                setStatusFilter(v);
                setPage(0);
              }}
              options={STATUS_OPTIONS.map((o) => ({
                value: o.value,
                label: o.label,
              }))}
            />
          </Field>
          <Field label="Queue">
            <Input
              value={queueFilter}
              placeholder="Exact match"
              onChange={(e) => {
                setQueueFilter(e.target.value);
                setPage(0);
              }}
              list="exec-queue-options"
            />
            <datalist id="exec-queue-options">
              {queueOptions.map((q) => (
                <option key={q} value={q} />
              ))}
            </datalist>
          </Field>
          <Field label="Type">
            <Input
              value={typeFilter}
              placeholder="Exact match"
              onChange={(e) => {
                setTypeFilter(e.target.value);
                setPage(0);
              }}
              list="exec-type-options"
            />
            <datalist id="exec-type-options">
              {typeOptions.map((t) => (
                <option key={t} value={t} />
              ))}
            </datalist>
          </Field>
          <Field label="Sort">
            <Select<SortOption>
              value={sort}
              onChange={(v) => {
                setSort(v);
                setPage(0);
              }}
              options={SORT_OPTIONS.map((o) => ({
                value: o.value,
                label: o.label,
              }))}
            />
          </Field>
        </div>
      </Card>

      {error && (
        <Alert tone="danger" className="mb-4">
          {error}
        </Alert>
      )}

      {loading && rows.length === 0 ? (
        <Card>
          <div className="p-4">
            <div className="space-y-2">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          </div>
        </Card>
      ) : rows.length === 0 ? (
        <EmptyState
          title="No executions match your filters"
          description="Try loosening the filters or trigger a new job."
          action={
            <Button
              leftIcon={<Plus className="h-4 w-4" />}
              onClick={() => setDialogOpen(true)}
            >
              Trigger job
            </Button>
          }
        />
      ) : (
        <Card className="overflow-hidden">
          <Table>
            <THead>
              <Tr>
                <Th>Finished</Th>
                <Th>Status</Th>
                <Th>Job</Th>
                <Th>Queue</Th>
                <Th>Type</Th>
                <Th>Worker</Th>
                <Th className="text-right">Duration</Th>
                <Th>Error</Th>
              </Tr>
            </THead>
            <TBody>
              {rows.map((ex) => (
                <Tr
                  key={ex.id}
                  className="cursor-pointer"
                  onClick={() => navigate(`/executions/${ex.job_id}`)}
                >
                  <Td className="whitespace-nowrap tabular-nums">
                    {formatFinished(ex.finished_at)}
                  </Td>
                  <Td>
                    <Badge tone={ex.success ? "success" : "danger"} dot>
                      {ex.success ? "Success" : "Failed"}
                    </Badge>
                  </Td>
                  <Td>
                    <Link
                      to={`/executions/${ex.job_id}`}
                      onClick={(e) => e.stopPropagation()}
                      className="font-mono text-primary hover:underline"
                    >
                      {ex.job_id.length > 12
                        ? `${ex.job_id.slice(0, 10)}…`
                        : ex.job_id}
                    </Link>
                  </Td>
                  <Td>{ex.queue}</Td>
                  <Td className="font-mono text-xs">{ex.type}</Td>
                  <Td className="font-mono text-xs text-fg-muted">
                    {ex.worker_id
                      ? ex.worker_id.length > 10
                        ? `${ex.worker_id.slice(0, 8)}…`
                        : ex.worker_id
                      : "—"}
                  </Td>
                  <Td className="text-right tabular-nums">
                    {formatDuration(ex.elapsed_ms)}
                  </Td>
                  <Td
                    className={cn(
                      "max-w-[220px] truncate",
                      ex.error ? "text-danger-fg" : "text-fg-subtle"
                    )}
                    title={ex.error || ""}
                  >
                    {ex.error || "—"}
                  </Td>
                </Tr>
              ))}
            </TBody>
          </Table>
          <Pagination
            total={total}
            page={page}
            pageSize={PAGE_SIZE}
            onPageChange={setPage}
          />
        </Card>
      )}

      <EnqueueJobDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        defaultQueue="default"
        onEnqueued={(jobId) => navigate(`/executions/${jobId}`)}
      />
    </>
  );
};

export default Executions;
