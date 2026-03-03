import type { ReactNode } from "react";
import { useMemo, useState } from "react";
import { ChevronDown, ChevronUp } from "lucide-react";

import {
  ChartContainer,
  RechartsPrimitive,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from "@/components/ui/chart";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { KvmDiskChartSample } from "@/pages/HypervisorPage/KvmDetailPage/sections/resource/kvm-node-lite-metrics";

type KvmDiskRealtimeSectionProps = {
  panelClass: string;
  textPrimary: string;
  textMuted: string;
  samples: KvmDiskChartSample[];
  diskCount?: number;
  primaryDiskLabel?: string;
  currentMBps: number;
  averageMBps: number;
  peakMBps: number;
  currentIops: number;
  readMBps: number;
  writeMBps: number;
  rangeControl?: ReactNode;
};

type KvmDiskMetricKey =
  | "throughputMBps"
  | "readMBps"
  | "writeMBps"
  | "totalIops"
  | "utilPercent";

type KvmDiskMetricDefinition = {
  key: KvmDiskMetricKey;
  label: string;
  unit: string;
  decimals: number;
};

const DISK_METRICS: KvmDiskMetricDefinition[] = [
  { key: "throughputMBps", label: "Throughput", unit: "MB/s", decimals: 2 },
  { key: "readMBps", label: "Read", unit: "MB/s", decimals: 2 },
  { key: "writeMBps", label: "Write", unit: "MB/s", decimals: 2 },
  { key: "totalIops", label: "IOPS", unit: "", decimals: 0 },
  { key: "utilPercent", label: "Disk util", unit: "%", decimals: 2 },
];

function formatDiskMetricValue(
  value: number,
  metric: KvmDiskMetricDefinition,
): string {
  const normalized = Number.isFinite(value) ? value : 0;
  const body =
    metric.decimals <= 0
      ? Math.round(normalized).toLocaleString()
      : normalized.toFixed(metric.decimals);
  return metric.unit ? `${body} ${metric.unit}` : body;
}

export function KvmDiskRealtimeSection({
  panelClass,
  textPrimary,
  textMuted,
  samples,
  diskCount = 0,
  primaryDiskLabel = "",
  currentMBps,
  averageMBps,
  peakMBps,
  currentIops,
  readMBps,
  writeMBps,
  rangeControl,
}: KvmDiskRealtimeSectionProps) {
  const [isExpanded, setIsExpanded] = useState(true);
  const [selectedMetricKey, setSelectedMetricKey] =
    useState<KvmDiskMetricKey>("throughputMBps");

  const selectedMetric = useMemo(
    () =>
      DISK_METRICS.find((item) => item.key === selectedMetricKey) ??
      DISK_METRICS[0],
    [selectedMetricKey],
  );

  const chartData = useMemo(
    () =>
      samples.map((sample) => ({
        timestamp: sample.timestamp,
        value: Math.max(0, Number(sample[selectedMetric.key])),
      })),
    [samples, selectedMetric],
  );

  const latestRow = samples.length > 0 ? samples[samples.length - 1] : null;

  const fallbackMetricValues: Record<KvmDiskMetricKey, number> = {
    throughputMBps: currentMBps,
    readMBps,
    writeMBps,
    totalIops: currentIops,
    utilPercent: 0,
  };

  return (
    <Card className={cn("shadow-lg", panelClass)}>
      <CardHeader className="space-y-2">
        <div className="flex items-start justify-between gap-3">
          <button
            type="button"
            onClick={() => setIsExpanded((prev) => !prev)}
            className={cn(
              "inline-flex items-center gap-2 text-left text-base font-semibold transition-opacity hover:opacity-80",
              textPrimary,
            )}
          >
            <span>Disk Chart</span>
            {isExpanded ? (
              <ChevronUp className="h-4 w-4" />
            ) : (
              <ChevronDown className="h-4 w-4" />
            )}
          </button>
          {isExpanded && rangeControl ? (
            <div className="shrink-0">{rangeControl}</div>
          ) : null}
        </div>
        {isExpanded ? (
          <CardDescription className={cn(textMuted)}>
            {diskCount > 0
              ? `${diskCount} disk(s)${primaryDiskLabel ? ` • ${primaryDiskLabel}` : ""}`
              : "Realtime disk metrics from node telemetry"}
          </CardDescription>
        ) : null}
      </CardHeader>
      {isExpanded ? (
        <CardContent>
          {samples.length === 0 ? (
            <p className={cn("text-sm", textMuted)}>
              Chua co du lieu Disk realtime.
            </p>
          ) : (
            <div className="grid items-stretch gap-4 lg:grid-cols-[minmax(0,3fr)_minmax(0,0.8fr)]">
              <div className="h-full">
                <ChartContainer
                  config={
                    {
                      value: {
                        label: selectedMetric.label,
                        color: "hsl(var(--chart-3))",
                      },
                    } satisfies ChartConfig
                  }
                  className="!aspect-auto h-full min-h-[420px] w-full rounded-lg border border-black/10 bg-gradient-to-b from-black/[0.02] via-black/[0.05] to-black/[0.12] p-2 dark:border-white/10 dark:bg-gradient-to-b dark:from-white/[0.02] dark:via-white/[0.05] dark:to-white/[0.12]"
                >
                  <RechartsPrimitive.AreaChart
                    accessibilityLayer
                    data={chartData}
                    margin={{ left: 10, right: 10, top: 10, bottom: 0 }}
                  >
                    <defs>
                      <linearGradient id="diskFill" x1="0" y1="0" x2="0" y2="1">
                        <stop
                          offset="0%"
                          stopColor="#f59e0b"
                          stopOpacity={0.14}
                        />
                        <stop
                          offset="60%"
                          stopColor="#d97706"
                          stopOpacity={0.26}
                        />
                        <stop
                          offset="100%"
                          stopColor="#fbbf24"
                          stopOpacity={0.44}
                        />
                      </linearGradient>
                    </defs>
                    <RechartsPrimitive.CartesianGrid
                      vertical={false}
                      stroke="hsl(var(--border))"
                      strokeDasharray="4 4"
                    />
                    <RechartsPrimitive.XAxis
                      dataKey="timestamp"
                      tickLine
                      axisLine={{
                        stroke: "#9ca3af",
                        strokeWidth: 2.2,
                      }}
                      tickMargin={8}
                      minTickGap={28}
                      tick={{ fontSize: 11 }}
                      tickFormatter={(value) =>
                        new Date(String(value)).toLocaleTimeString([], {
                          hour: "2-digit",
                          minute: "2-digit",
                          second: "2-digit",
                        })
                      }
                    />
                    <RechartsPrimitive.YAxis
                      domain={[0, "auto"]}
                      tickLine
                      axisLine={{
                        stroke: "#9ca3af",
                        strokeWidth: 2.2,
                      }}
                      tickMargin={8}
                      tick={{ fontSize: 11 }}
                      tickFormatter={(value) =>
                        formatDiskMetricValue(Number(value), selectedMetric)
                      }
                    />
                    <ChartTooltip
                      cursor={false}
                      content={
                        <ChartTooltipContent
                          labelFormatter={(_, payload) => {
                            const row = payload?.[0]?.payload as
                              | { timestamp?: string }
                              | undefined;
                            return row?.timestamp
                              ? new Date(row.timestamp).toLocaleString()
                              : "Disk metric";
                          }}
                          formatter={(value) =>
                            formatDiskMetricValue(Number(value), selectedMetric)
                          }
                        />
                      }
                    />
                    <RechartsPrimitive.Area
                      type="linear"
                      dataKey="value"
                      stroke="hsl(var(--chart-3))"
                      strokeWidth={2.8}
                      fill="url(#diskFill)"
                      fillOpacity={1}
                      dot={false}
                      connectNulls
                      isAnimationActive={false}
                    />
                  </RechartsPrimitive.AreaChart>
                </ChartContainer>
              </div>

              <div className="grid grid-cols-2 gap-3">
                {DISK_METRICS.map((metricDef) => {
                  const isSelected = metricDef.key === selectedMetric.key;
                  const value = latestRow
                    ? Number(latestRow[metricDef.key])
                    : fallbackMetricValues[metricDef.key];

                  return (
                    <button
                      key={metricDef.key}
                      type="button"
                      onClick={() => setSelectedMetricKey(metricDef.key)}
                      className={cn(
                        "rounded-md border border-black/10 bg-black/[0.03] px-2 py-2 text-left transition-colors dark:border-white/10 dark:bg-white/[0.03]",
                        isSelected &&
                          "border-amber-500/60 bg-amber-500/10 dark:border-amber-400/60 dark:bg-amber-400/10",
                      )}
                    >
                      <p className={cn("text-sm pl-2", textMuted)}>
                        {metricDef.label}
                      </p>
                      <p
                        className={cn(
                          "text-xl pl-2 font-semibold",
                          textPrimary,
                        )}
                      >
                        {formatDiskMetricValue(value, metricDef)}
                      </p>
                    </button>
                  );
                })}

                <div className="rounded-md border border-black/10 bg-black/[0.03] px-2 py-2 dark:border-white/10 dark:bg-white/[0.03]">
                  <p className={cn("text-sm pl-2", textMuted)}>
                    Average throughput
                  </p>
                  <p className={cn("text-xl pl-2 font-semibold", textPrimary)}>
                    {averageMBps.toFixed(2)} MB/s
                  </p>
                </div>
                <div className="rounded-md border border-black/10 bg-black/[0.03] px-2 py-2 dark:border-white/10 dark:bg-white/[0.03]">
                  <p className={cn("text-sm pl-2", textMuted)}>
                    Peak throughput
                  </p>
                  <p className={cn("text-xl pl-2 font-semibold", textPrimary)}>
                    {peakMBps.toFixed(2)} MB/s
                  </p>
                </div>
              </div>
            </div>
          )}
        </CardContent>
      ) : null}
    </Card>
  );
}
