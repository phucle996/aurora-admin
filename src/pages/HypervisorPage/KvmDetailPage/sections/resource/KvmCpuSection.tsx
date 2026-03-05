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
import type { KvmCpuChartSample } from "@/pages/HypervisorPage/KvmDetailPage/sections/resource/kvm-node-lite-metrics";

type KvmCpuMetricKey = Exclude<keyof KvmCpuChartSample, "timestamp">;

type KvmCpuMetricDefinition = {
  key: KvmCpuMetricKey;
  label: string;
  unit: string;
  decimals?: number;
};

const CPU_METRICS: KvmCpuMetricDefinition[] = [
  { key: "cpuUsagePct", label: "CPU usage", unit: "%", decimals: 2 },
  { key: "load1", label: "Load 1", unit: "", decimals: 2 },
  { key: "load5", label: "Load 5", unit: "", decimals: 2 },
  { key: "load15", label: "Load 15", unit: "", decimals: 2 },
  { key: "runQueueLength", label: "Run queue", unit: "", decimals: 0 },
  { key: "processCount", label: "Processes", unit: "", decimals: 0 },
  { key: "threadCount", label: "Threads", unit: "", decimals: 0 },
  { key: "systemLoadPercent", label: "System load", unit: "%", decimals: 2 },
];

function formatMetricValue(
  value: number,
  definition: KvmCpuMetricDefinition,
): string {
  const decimals = definition.decimals ?? 2;
  const normalized = Number.isFinite(value) ? value : 0;
  const body =
    decimals <= 0
      ? Math.round(normalized).toLocaleString()
      : normalized.toFixed(decimals);
  return definition.unit ? `${body} ${definition.unit}` : body;
}

type KvmCpuRealtimeSectionProps = {
  panelClass: string;
  textPrimary: string;
  textMuted: string;
  cpuModel: string;
  samples: KvmCpuChartSample[];
  rangeControl?: ReactNode;
};

export function KvmCpuSection({
  panelClass,
  textPrimary,
  textMuted,
  cpuModel,
  samples,
  rangeControl,
}: KvmCpuRealtimeSectionProps) {
  const [isExpanded, setIsExpanded] = useState(true);
  const [selectedMetricKey, setSelectedMetricKey] =
    useState<KvmCpuMetricKey>("cpuUsagePct");

  const hasAnyData = useMemo(
    () =>
      samples.some((sample) =>
        CPU_METRICS.some((metricDef) => {
          const value = sample[metricDef.key];
          return typeof value === "number" && Number.isFinite(value);
        }),
      ),
    [samples],
  );

  const effectiveMetricKey: KvmCpuMetricKey = hasAnyData
    ? selectedMetricKey
    : "cpuUsagePct";

  const selectedMetric = useMemo(
    () =>
      CPU_METRICS.find((item) => item.key === effectiveMetricKey) ??
      CPU_METRICS[0],
    [effectiveMetricKey],
  );

  const latestSample = samples.length > 0 ? samples[samples.length - 1] : null;
  const cpuModelDescription = useMemo(() => {
    const model = cpuModel.trim();
    const cores = Math.max(0, Math.round(latestSample?.cpuCores ?? 0));
    const coresLabel = cores > 0 ? `${cores} cores` : "";
    if (!model) {
      return coresLabel || "-";
    }
    if (!coresLabel) {
      return model;
    }
    return `${model} • ${coresLabel}`;
  }, [cpuModel, latestSample]);

  const chartConfig = {
    value: {
      label: selectedMetric.label,
      color: "hsl(var(--chart-1))",
    },
  } satisfies ChartConfig;

  const chartData = samples.map((sample) => ({
    timestamp: sample.timestamp,
    value: Math.max(0, Number(sample[selectedMetric.key] ?? 0)),
  }));

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
            <span>CPU Chart</span>
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
            {cpuModelDescription}
          </CardDescription>
        ) : null}
      </CardHeader>
      {isExpanded ? (
        <CardContent>
          {samples.length === 0 ? (
            <p className={cn("text-sm", textMuted)}>
              Chua co du lieu CPU realtime.
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
                      <linearGradient id="cpuFill" x1="0" y1="0" x2="0" y2="1">
                        <stop
                          offset="0%"
                          stopColor="#0572ba"
                          stopOpacity={0.16}
                        />
                        <stop
                          offset="60%"
                          stopColor="#0052a4"
                          stopOpacity={0.28}
                        />
                        <stop
                          offset="100%"
                          stopColor="#6366f1"
                          stopOpacity={0.46}
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
                        formatMetricValue(Number(value), selectedMetric)
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
                              : "CPU metric";
                          }}
                          formatter={(value) =>
                            formatMetricValue(Number(value), selectedMetric)
                          }
                        />
                      }
                    />
                    <RechartsPrimitive.Area
                      type="linear"
                      dataKey="value"
                      stroke="hsl(var(--chart-1))"
                      strokeWidth={2.8}
                      fill="url(#cpuFill)"
                      fillOpacity={1}
                      dot={false}
                      isAnimationActive={false}
                      connectNulls
                    />
                  </RechartsPrimitive.AreaChart>
                </ChartContainer>
              </div>

              <div className="grid grid-cols-2 gap-3">
                {CPU_METRICS.map((metricDef) => {
                  const isSelected = metricDef.key === selectedMetric.key;
                  const rawValue =
                    latestSample &&
                    typeof latestSample[metricDef.key] === "number"
                      ? Number(latestSample[metricDef.key])
                      : 0;
                  return (
                    <button
                      key={metricDef.key}
                      type="button"
                      onClick={() => setSelectedMetricKey(metricDef.key)}
                      className={cn(
                        "rounded-md border border-black/10 bg-black/[0.03] px-2 py-2 text-left transition-colors",

                        isSelected &&
                          "border-blue-500/60 bg-blue-500/10 dark:border-blue-400/60 dark:bg-blue-400/10",
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
                        {formatMetricValue(rawValue, metricDef)}
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
