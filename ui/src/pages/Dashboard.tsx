import { useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  ArrowRight,
  Plus,
  Server,
  Terminal,
} from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { useUserWorkers } from "@/hooks/useWorkers";
import {
  Alert,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  EmptyState,
  PageHeader,
  Skeleton,
} from "@/components/ui";
import { AddWorkerDialog } from "@/features/workers/AddWorkerDialog";

interface StatCardProps {
  label: string;
  value: string | number;
  tone?: "neutral" | "success" | "danger" | "primary";
  loading?: boolean;
  hint?: string;
}

const TONE_STYLES: Record<NonNullable<StatCardProps["tone"]>, string> = {
  neutral: "text-fg",
  success: "text-success-fg",
  danger: "text-danger-fg",
  primary: "text-info-fg",
};

function StatCard({ label, value, tone = "neutral", loading, hint }: StatCardProps) {
  return (
    <Card>
      <CardContent className="flex flex-col gap-2 py-5">
        <p className="text-xs font-medium uppercase tracking-wide text-fg-muted">
          {label}
        </p>
        {loading ? (
          <Skeleton className="h-9 w-16" />
        ) : (
          <p className={`text-3xl font-bold tabular-nums ${TONE_STYLES[tone]}`}>
            {value}
          </p>
        )}
        {hint && <p className="text-xs text-fg-muted">{hint}</p>}
      </CardContent>
    </Card>
  );
}

interface QuickActionProps {
  title: string;
  description: string;
  icon: typeof Server;
  to?: string;
  onClick?: () => void;
}

function QuickAction({ title, description, icon: Icon, to, onClick }: QuickActionProps) {
  const inner = (
    <div className="group flex h-full items-start gap-4 rounded-lg border border-border bg-surface p-5 shadow-card transition-colors hover:border-primary hover:bg-surface-hover">
      <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-primary-subtle text-info-fg transition-colors group-hover:bg-primary group-hover:text-fg-onPrimary">
        <Icon className="h-5 w-5" />
      </span>
      <div className="min-w-0 flex-1">
        <div className="flex items-center justify-between gap-2">
          <h3 className="text-sm font-semibold text-fg">{title}</h3>
          <ArrowRight className="h-4 w-4 text-fg-subtle transition-transform group-hover:translate-x-0.5 group-hover:text-primary" />
        </div>
        <p className="mt-1 text-sm text-fg-muted">{description}</p>
      </div>
    </div>
  );

  if (to) {
    return (
      <Link to={to} className="block h-full">
        {inner}
      </Link>
    );
  }
  return (
    <button type="button" onClick={onClick} className="block h-full w-full text-left">
      {inner}
    </button>
  );
}

const Dashboard = () => {
  const { user } = useAuth();
  const [addOpen, setAddOpen] = useState(false);
  const { data: userWorkers = [], isLoading: loading, isError: hasError } =
    useUserWorkers();

  const stats = useMemo(() => {
    const alive = userWorkers.filter((w) => w.status === "alive").length;
    const dead = userWorkers.filter((w) => w.status === "dead").length;
    const total = userWorkers.length;
    return {
      total,
      alive,
      dead,
    };
  }, [userWorkers]);

  return (
    <>
      <PageHeader
        title={`Welcome back, ${user?.displayName || user?.email?.split("@")[0] || "there"}`}
        description="Here's an overview of your distributed compute fleet."
      />

      {hasError && (
        <Alert tone="danger" className="mb-4">
          Failed to load worker stats. Please refresh the page.
        </Alert>
      )}

      <div className="grid gap-4 sm:grid-cols-3">
        <StatCard label="Total workers" value={stats.total} loading={loading} />
        <StatCard
          label="Running"
          value={stats.alive}
          tone="success"
          loading={loading}
        />
        <StatCard
          label="Stopped"
          value={stats.dead}
          tone={stats.dead > 0 ? "danger" : "neutral"}
          loading={loading}
        />
      </div>

      <Card className="mt-6">
        <CardHeader>
          <CardTitle>Quick actions</CardTitle>
          <CardDescription>Jump into the most common workflows.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-4 sm:grid-cols-2">
            <QuickAction
              title="My workers"
              description="View and manage your registered agents."
              icon={Server}
              to="/workers"
            />
            <QuickAction
              title="Add a worker"
              description="Install the agent on a new host."
              icon={Plus}
              onClick={() => setAddOpen(true)}
            />
          </div>
        </CardContent>
      </Card>

      <Card className="mt-6">
        <CardHeader className="flex flex-row items-center justify-between">
          <div>
            <CardTitle>Recent activity</CardTitle>
            <CardDescription>
              Latest job executions across your workers.
            </CardDescription>
          </div>
          <Link
            to="/executions"
            className="inline-flex items-center gap-1 text-sm font-medium text-primary hover:underline"
          >
            View all
            <ArrowRight className="h-4 w-4" />
          </Link>
        </CardHeader>
        <CardContent>
          <EmptyState
            icon={<Terminal className="h-5 w-5" />}
            title="No recent activity"
            description="Trigger a job from the executions page to see it appear here."
            action={
              <Link
                to="/executions"
                className="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-sm font-medium text-fg-onPrimary hover:bg-primary-hover"
              >
                Go to executions
                <ArrowRight className="h-4 w-4" />
              </Link>
            }
          />
        </CardContent>
      </Card>

      <AddWorkerDialog open={addOpen} onClose={() => setAddOpen(false)} />
    </>
  );
};

export default Dashboard;
