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
import type { KvmRamChartSample } from "@/pages/HypervisorPage/KvmDetailPage/sections/resource/kvm-node-raw-metrics";

type KvmRamRealtimeSectionProps = {
  panelClass: string;
  textPrimary: string;
  textMuted: string;
  samples: KvmRamChartSample[];
  rangeControl?: ReactNode;
};

type KvmRamMetricKey = Exclude<keyof KvmRamChartSample, "timestamp">;

type KvmRamMetricDefinition = {
  key: KvmRamMetricKey;
  label: string;
  mode: "bytes";
};

const RAM_METRICS: KvmRamMetricDefinition[] = [
  { key: "usedBytes", label: "RAM used", mode: "bytes" },
  { key: "availableBytes", label: "RAM available", mode: "bytes" },
  { key: "totalBytes", label: "RAM total", mode: "bytes" },
  { key: "swapUsedBytes", label: "Swap used", mode: "bytes" },
  { key: "swapTotalBytes", label: "Swap total", mode: "bytes" },
];

function formatRamValue(
  value: number,
  definition: KvmRamMetricDefinition,
): string {
  const normalized = Number.isFinite(value) && value > 0 ? value : 0;
  if (definition.mode !== "bytes") {
    return Math.round(normalized).toLocaleString();
  }
  if (normalized >= 1024 ** 3) {
    return `${(normalized / 1024 ** 3).toFixed(2)} GiB`;
  }
  if (normalized >= 1024 ** 2) {
    return `${(normalized / 1024 ** 2).toFixed(2)} MiB`;
  }
  if (normalized >= 1024) {
    return `${(normalized / 1024).toFixed(2)} KiB`;
  }
  return `${Math.round(normalized)} B`;
}

export function KvmRamRealtimeSection({
  panelClass,
  textPrimary,
  textMuted,
  samples,
  rangeControl,
}: KvmRamRealtimeSectionProps) {
  const [isExpanded, setIsExpanded] = useState(true);
  const [selectedMetricKey, setSelectedMetricKey] =
    useState<KvmRamMetricKey>("usedBytes");

  const chartConfig = {
    value: {
      label: "RAM metric",
      color: "hsl(var(--chart-2))",
    },
  } satisfies ChartConfig;

  const selectedMetric = useMemo(
    () =>
      RAM_METRICS.find((item) => item.key === selectedMetricKey) ??
      RAM_METRICS[0],
    [selectedMetricKey],
  );

  const chartData = samples.map((row) => ({
    timestamp: row.timestamp,
    value: Math.max(0, Number(row[selectedMetric.key])),
  }));

  const latestRow =
    samples.length > 0
      ? samples[samples.length - 1]
      : null;

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
            <span>RAM Chart</span>
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
            System RAM and swap counters
          </CardDescription>
        ) : null}
      </CardHeader>
      {isExpanded ? (
        <CardContent>
          {samples.length === 0 ? (
            <p className={cn("text-sm", textMuted)}>
              Chua co du lieu RAM realtime.
            </p>
          ) : (
            <div className="grid items-stretch gap-4 lg:grid-cols-[minmax(0,3fr)_minmax(0,0.8fr)]">
              <div className="h-full">
                <ChartContainer
                  config={chartConfig}
                  className="!aspect-auto h-full min-h-[420px] w-full rounded-lg border border-black/10 bg-gradient-to-b from-black/[0.02] via-black/[0.05] to-black/[0.12] p-2 dark:border-white/10 dark:bg-gradient-to-b dark:from-white/[0.02] dark:via-white/[0.05] dark:to-white/[0.12]"
                >
                  <RechartsPrimitive.AreaChart
                    accessibilityLayer
                    data={chartData}
                    margin={{ left: 10, right: 10, top: 10, bottom: 0 }}
                  >
                    <defs>
                      <linearGradient id="ramFill" x1="0" y1="0" x2="0" y2="1">
                        <stop
                          offset="0%"
                          stopColor="#10b981"
                          stopOpacity={0.14}
                        />
                        <stop
                          offset="60%"
                          stopColor="#059669"
                          stopOpacity={0.26}
                        />
                        <stop
                          offset="100%"
                          stopColor="#34d399"
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
                        formatRamValue(Number(value), selectedMetric)
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
                              : "RAM metric";
                          }}
                          formatter={(value) =>
                            formatRamValue(Number(value), selectedMetric)
                          }
                        />
                      }
                    />
                    <RechartsPrimitive.Area
                      dataKey="value"
                      type="linear"
                      stroke="hsl(var(--chart-2))"
                      strokeWidth={2.8}
                      fill="url(#ramFill)"
                      fillOpacity={1}
                      dot={false}
                      connectNulls
                      isAnimationActive={false}
                    />
                  </RechartsPrimitive.AreaChart>
                </ChartContainer>
              </div>

              <div className="grid grid-cols-2 gap-3">
                {RAM_METRICS.map((metricDef) => {
                  const isSelected = metricDef.key === selectedMetric.key;
                  const value = latestRow
                    ? Number(latestRow[metricDef.key])
                    : 0;
                  return (
                    <button
                      key={metricDef.key}
                      type="button"
                      onClick={() => setSelectedMetricKey(metricDef.key)}
                      className={cn(
                        "rounded-md border border-black/10 bg-black/[0.03] px-2 py-2 text-left transition-colors dark:border-white/10 dark:bg-white/[0.03]",
                        isSelected &&
                          "border-emerald-500/60 bg-emerald-500/10 dark:border-emerald-400/60 dark:bg-emerald-400/10",
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
                        {formatRamValue(value, metricDef)}
                      </p>
                    </button>
                  );
                })}
              </div>
            </div>
          )}
        </CardContent>
      ) : null}
    </Card>
  );
}
