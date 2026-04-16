import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { useTheme } from "@/contexts/ThemeContext";
import { Card, Skeleton } from "@/components/ui";
import type { ChartPoint } from "@/services/dashboardService";

const THEME_TOKENS = {
  light: {
    grid: "rgba(148, 163, 184, 0.3)",
    axis: "rgb(100, 116, 139)",
    tooltipBg: "rgb(255 255 255)",
    tooltipBorder: "rgb(226 232 240)",
    tooltipText: "rgb(15 23 42)",
    cpu: "rgb(37, 99, 235)",
    memory: "rgb(14, 165, 233)",
    disk: "rgb(34, 197, 94)",
  },
  dark: {
    grid: "rgba(71, 85, 105, 0.35)",
    axis: "rgb(148, 163, 184)",
    tooltipBg: "rgb(15 23 42)",
    tooltipBorder: "rgb(51 65 85)",
    tooltipText: "rgb(241 245 249)",
    cpu: "rgb(96, 165, 250)",
    memory: "rgb(56, 189, 248)",
    disk: "rgb(74, 222, 128)",
  },
} as const;

export type MetricKey = "cpu_load" | "mem_usage_mb" | "disk_used_gb";

const METRIC_COLOR_KEY: Record<MetricKey, "cpu" | "memory" | "disk"> = {
  cpu_load: "cpu",
  mem_usage_mb: "memory",
  disk_used_gb: "disk",
};

export interface MetricsChartProps {
  title: string;
  data: ChartPoint[];
  metric: MetricKey;
  domain: [number, number];
  yDomain?: [number, number];
  height?: number;
  loading?: boolean;
  valueFormatter: (v: number | undefined) => string;
  yTickFormatter?: (v: number) => string;
  tickFormatter: (ts: number) => string;
  tickCount?: number;
}

export function MetricsChart({
  title,
  data,
  metric,
  domain,
  yDomain,
  height = 240,
  loading,
  valueFormatter,
  yTickFormatter,
  tickFormatter,
  tickCount = 8,
}: MetricsChartProps) {
  const { resolved } = useTheme();
  const tokens = THEME_TOKENS[resolved];
  const color = tokens[METRIC_COLOR_KEY[metric]];

  return (
    <Card className="p-4">
      <h3 className="mb-3 text-sm font-semibold text-fg">{title}</h3>
      {loading ? (
        <Skeleton className="h-[240px] w-full rounded-md" />
      ) : (
        <ResponsiveContainer width="100%" height={height}>
          <AreaChart
            data={data}
            margin={{ top: 10, right: 12, left: 8, bottom: 0 }}
          >
            <defs>
              <linearGradient
                id={`gradient-${metric}`}
                x1="0"
                y1="0"
                x2="0"
                y2="1"
              >
                <stop offset="5%" stopColor={color} stopOpacity={0.45} />
                <stop offset="95%" stopColor={color} stopOpacity={0.02} />
              </linearGradient>
            </defs>
            <CartesianGrid
              strokeDasharray="3 3"
              stroke={tokens.grid}
              vertical={false}
            />
            <XAxis
              type="number"
              dataKey="t"
              domain={domain}
              tickFormatter={tickFormatter}
              tickCount={tickCount}
              stroke={tokens.axis}
              tick={{ fill: tokens.axis, fontSize: 11 }}
            />
            <YAxis
              domain={yDomain ?? [0, "auto"]}
              tickFormatter={yTickFormatter}
              stroke={tokens.axis}
              tick={{ fill: tokens.axis, fontSize: 11 }}
              width={yTickFormatter ? 60 : 44}
            />
            <Tooltip
              contentStyle={{
                backgroundColor: tokens.tooltipBg,
                border: `1px solid ${tokens.tooltipBorder}`,
                borderRadius: 6,
                color: tokens.tooltipText,
                fontSize: 12,
              }}
              labelStyle={{ color: tokens.tooltipText, fontWeight: 600 }}
              formatter={(value: number | undefined) => [
                valueFormatter(value),
                title,
              ]}
              labelFormatter={(label) =>
                new Date(
                  typeof label === "number" ? label : label
                ).toLocaleString()
              }
            />
            <Area
              type="monotone"
              dataKey={metric}
              stroke={color}
              strokeWidth={2}
              fill={`url(#gradient-${metric})`}
              isAnimationActive={false}
            />
          </AreaChart>
        </ResponsiveContainer>
      )}
    </Card>
  );
}
