import { ArrowLeft, CheckCircle2, Cpu, HardDrive, RefreshCcw, Server } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useTheme } from "next-themes";
import { useNavigate, useParams } from "react-router-dom";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  RechartsPrimitive,
  type ChartConfig,
} from "@/components/ui/chart";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { useEnabledModules } from "@/state/enabled-modules-context";
import {
  formatDate,
  getPlatformClusterDetail,
  getPlatformClusterMetrics,
  isPlatformModule,
  resolvePlatformBaseURL,
  statusBadgeClass,
  type PlatformCluster,
  type PlatformClusterCapability,
  type PlatformNodeMetric,
} from "@/pages/K8sPage/k8s-page.api";

type ClusterMetricSample = {
  timestamp: string;
  totalCpuMilli: number;
  totalMemoryBytes: number;
  nodeCount: number;
};

const METRIC_HISTORY_LIMIT = 120;
const METRIC_POLL_INTERVAL_MS = 10_000;

function formatMilliCPU(value: number): string {
  const normalized = Math.max(0, value);
  if (normalized >= 1000) {
    return `${(normalized / 1000).toFixed(2)} cores`;
  }
  return `${Math.round(normalized)} mCPU`;
}

function formatBytes(value: number): string {
  const normalized = Math.max(0, value);
  if (normalized >= 1024 ** 4) {
    return `${(normalized / 1024 ** 4).toFixed(2)} TiB`;
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

function yesNoBadge(value: boolean): string {
  return value ? "Yes" : "No";
}

type TrendCardProps = {
  title: string;
  description: string;
  colorVar: "hsl(var(--chart-1))" | "hsl(var(--chart-2))";
  gradientId: string;
  panelClass: string;
  textPrimary: string;
  textMuted: string;
  data: Array<{ timestamp: string; value: number }>;
  formatter: (value: number) => string;
};

function TrendCard({
  title,
  description,
  colorVar,
  gradientId,
  panelClass,
  textPrimary,
  textMuted,
  data,
  formatter,
}: TrendCardProps) {
  const chartConfig = {
    value: {
      label: title,
      color: colorVar,
    },
  } satisfies ChartConfig;

  return (
    <Card className={cn("shadow-lg", panelClass)}>
      <CardHeader>
        <CardTitle className={textPrimary}>{title}</CardTitle>
        <CardDescription className={textMuted}>{description}</CardDescription>
      </CardHeader>
      <CardContent>
        {data.length === 0 ? (
          <p className={cn("text-sm", textMuted)}>Chua co du lieu metrics.</p>
        ) : (
          <ChartContainer
            config={chartConfig}
            className="!aspect-auto h-[320px] w-full rounded-lg border border-black/10 bg-gradient-to-b from-black/[0.02] via-black/[0.05] to-black/[0.12] p-2 dark:border-white/10 dark:bg-gradient-to-b dark:from-white/[0.02] dark:via-white/[0.05] dark:to-white/[0.12]"
          >
            <RechartsPrimitive.AreaChart
              accessibilityLayer
              data={data}
              margin={{ left: 10, right: 10, top: 10, bottom: 0 }}
            >
              <defs>
                <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                  <stop offset="0%" stopColor={colorVar} stopOpacity={0.15} />
                  <stop offset="100%" stopColor={colorVar} stopOpacity={0.45} />
                </linearGradient>
              </defs>
              <RechartsPrimitive.CartesianGrid vertical={false} stroke="hsl(var(--border))" strokeDasharray="4 4" />
              <RechartsPrimitive.XAxis
                dataKey="timestamp"
                tickLine
                axisLine={{ stroke: "#9ca3af", strokeWidth: 2.2 }}
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
                axisLine={{ stroke: "#9ca3af", strokeWidth: 2.2 }}
                tickMargin={8}
                tick={{ fontSize: 11 }}
                tickFormatter={(value) => formatter(Number(value))}
              />
              <ChartTooltip
                cursor={false}
                content={
                  <ChartTooltipContent
                    labelFormatter={(_, payload) => {
                      const row = payload?.[0]?.payload as { timestamp?: string } | undefined;
                      return row?.timestamp ? new Date(row.timestamp).toLocaleString() : title;
                    }}
                    formatter={(value) => formatter(Number(value))}
                  />
                }
              />
              <RechartsPrimitive.Area
                type="linear"
                dataKey="value"
                stroke={colorVar}
                strokeWidth={2.8}
                fill={`url(#${gradientId})`}
                fillOpacity={1}
                dot={false}
                connectNulls
                isAnimationActive={false}
              />
            </RechartsPrimitive.AreaChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}

export default function K8sDetailPage() {
  const { resolvedTheme } = useTheme();
  const navigate = useNavigate();
  const { clusterId = "" } = useParams();
  const { items } = useEnabledModules();

  const [cluster, setCluster] = useState<PlatformCluster | null>(null);
  const [capability, setCapability] = useState<PlatformClusterCapability | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState("");

  const [nodes, setNodes] = useState<PlatformNodeMetric[]>([]);
  const [metricHistory, setMetricHistory] = useState<ClusterMetricSample[]>([]);
  const [metricsLoading, setMetricsLoading] = useState(false);
  const [metricsError, setMetricsError] = useState("");
  const [lastMetricsAt, setLastMetricsAt] = useState("");

  const platformBaseURL = useMemo(() => resolvePlatformBaseURL(items), [items]);
  const hasPlatformModule = useMemo(
    () => items.some((item) => item.installed && isPlatformModule(item)),
    [items],
  );

  const loadDetail = useCallback(async () => {
    if (!platformBaseURL || !clusterId) {
      return;
    }
    setDetailLoading(true);
    setDetailError("");
    try {
      const out = await getPlatformClusterDetail(platformBaseURL, clusterId);
      setCluster(out.cluster);
      setCapability(out.capability);
    } catch (err) {
      setDetailError(err instanceof Error ? err.message : "Load cluster detail failed");
      setCluster(null);
      setCapability(null);
    } finally {
      setDetailLoading(false);
    }
  }, [clusterId, platformBaseURL]);

  const loadMetrics = useCallback(async () => {
    if (!platformBaseURL || !clusterId) {
      return;
    }
    setMetricsLoading(true);
    setMetricsError("");
    try {
      const out = await getPlatformClusterMetrics(platformBaseURL, clusterId);
      setNodes(out.nodes);
      const totalCpuMilli = out.nodes.reduce((sum, node) => sum + node.cpu_milli, 0);
      const totalMemoryBytes = out.nodes.reduce((sum, node) => sum + node.memory_bytes, 0);
      const sample: ClusterMetricSample = {
        timestamp: new Date().toISOString(),
        totalCpuMilli,
        totalMemoryBytes,
        nodeCount: out.nodes.length,
      };
      setMetricHistory((prev) => {
        const next = [...prev, sample];
        if (next.length > METRIC_HISTORY_LIMIT) {
          return next.slice(next.length - METRIC_HISTORY_LIMIT);
        }
        return next;
      });
      setLastMetricsAt(new Date().toISOString());
    } catch (err) {
      setMetricsError(err instanceof Error ? err.message : "Load metrics failed");
      setNodes([]);
    } finally {
      setMetricsLoading(false);
    }
  }, [clusterId, platformBaseURL]);

  useEffect(() => {
    if (!platformBaseURL || !clusterId) {
      return;
    }
    void loadDetail();
  }, [clusterId, loadDetail, platformBaseURL]);

  useEffect(() => {
    if (!platformBaseURL || !clusterId) {
      return;
    }
    void loadMetrics();
    const id = window.setInterval(() => {
      void loadMetrics();
    }, METRIC_POLL_INTERVAL_MS);
    return () => window.clearInterval(id);
  }, [clusterId, loadMetrics, platformBaseURL]);

  const isDark = resolvedTheme !== "light";
  const panelClass = isDark ? "border-white/10 bg-slate-950/60" : "border-black/10 bg-white/85";
  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";

  const totalCpuMilli = nodes.reduce((sum, node) => sum + node.cpu_milli, 0);
  const totalMemoryBytes = nodes.reduce((sum, node) => sum + node.memory_bytes, 0);
  const cpuTrendData = metricHistory.map((item) => ({ timestamp: item.timestamp, value: item.totalCpuMilli }));
  const memoryTrendData = metricHistory.map((item) => ({ timestamp: item.timestamp, value: item.totalMemoryBytes }));

  return (
    <main className="space-y-4 py-3 lg:py-1">
      <header className="space-y-3">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div className="flex items-center gap-3">
            <Button
              type="button"
              variant="outline"
              onClick={() => navigate("/orchestration/k8s")}
              className={cn("mt-0.5", isDark && "border-white/15 bg-white/5")}
            >
              <ArrowLeft className="mr-2 h-4 w-4" />
              Back to Clusters
            </Button>

            <div className="space-y-1">
              <p
                className={cn(
                  "inline-flex rounded-full border px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.14em]",
                  isDark
                    ? "border-indigo-300/30 bg-indigo-500/10 text-indigo-100"
                    : "border-indigo-200 bg-indigo-50 text-indigo-700",
                )}
              >
                K8s Detail
              </p>
              <h1 className={cn("text-3xl font-semibold tracking-tight", textPrimary)}>
                {cluster?.name || "Kubernetes Cluster Detail"}
              </h1>
              <p className={cn("text-sm", textMuted)}>
                Xem thong tin cluster, capability va metrics runtime theo node.
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              className={cn(isDark && "border-white/20 bg-white/5")}
              onClick={() => {
                void loadDetail();
                void loadMetrics();
              }}
              disabled={!platformBaseURL || detailLoading || metricsLoading}
            >
              <RefreshCcw className={cn("mr-2 h-4 w-4", (detailLoading || metricsLoading) && "animate-spin")} />
              Refresh
            </Button>
          </div>
        </div>
      </header>

      {!hasPlatformModule ? (
        <Card className={cn("border-dashed", panelClass)}>
          <CardHeader>
            <CardTitle className={textPrimary}>Platform module chua duoc install</CardTitle>
            <CardDescription className={textMuted}>
              Vao Runtime Module Status Board de install module <code>platform</code> truoc.
            </CardDescription>
          </CardHeader>
        </Card>
      ) : !platformBaseURL ? (
        <Card className={cn("border-dashed", panelClass)}>
          <CardHeader>
            <CardTitle className={textPrimary}>Thieu endpoint cua platform module</CardTitle>
            <CardDescription className={textMuted}>
              Kiem tra key <code>/endpoint/platform</code> trong etcd.
            </CardDescription>
          </CardHeader>
        </Card>
      ) : (
        <>
          <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            {[
              { label: "Nodes", value: nodes.length, icon: Server },
              { label: "Total CPU", value: formatMilliCPU(totalCpuMilli), icon: Cpu },
              { label: "Total Memory", value: formatBytes(totalMemoryBytes), icon: HardDrive },
              { label: "Last Metrics Sync", value: formatDate(lastMetricsAt), icon: CheckCircle2 },
            ].map((item) => (
              <Card key={item.label} className={cn("shadow-lg", panelClass)}>
                <CardHeader className="pb-1">
                  <div className="flex items-center justify-between">
                    <CardDescription className={textMuted}>{item.label}</CardDescription>
                    <item.icon className={cn("h-4 w-4", isDark ? "text-slate-300" : "text-slate-500")} />
                  </div>
                  <CardTitle className={cn("text-2xl", textPrimary)}>{item.value}</CardTitle>
                </CardHeader>
              </Card>
            ))}
          </section>

          <section className="grid gap-4 xl:grid-cols-[minmax(0,0.95fr)_minmax(0,2.05fr)]">
            <div className="space-y-4">
              <Card className={cn("shadow-lg", panelClass)}>
                <CardHeader>
                  <CardTitle className={textPrimary}>Cluster Overview</CardTitle>
                  <CardDescription className={textMuted}>Thong tin co ban cua cluster.</CardDescription>
                </CardHeader>
                <CardContent className="space-y-3">
                  {detailError ? <p className="text-sm text-rose-500">{detailError}</p> : null}
                  {detailLoading ? (
                    <p className={cn("text-sm", textMuted)}>Loading cluster detail...</p>
                  ) : (
                    <>
                      <div>
                        <p className={cn("text-xs", textMuted)}>Cluster ID</p>
                        <p className={cn("text-sm font-medium break-all", textPrimary)}>{cluster?.id || "-"}</p>
                      </div>
                      <div>
                        <p className={cn("text-xs", textMuted)}>API Endpoint</p>
                        <p className={cn("text-sm font-medium break-all", textPrimary)}>{cluster?.api_endpoint || "-"}</p>
                      </div>
                      <div className="grid grid-cols-2 gap-3">
                        <div>
                          <p className={cn("text-xs", textMuted)}>Region</p>
                          <p className={cn("text-sm font-medium", textPrimary)}>{cluster?.region || "-"}</p>
                        </div>
                        <div>
                          <p className={cn("text-xs", textMuted)}>Environment</p>
                          <p className={cn("text-sm font-medium", textPrimary)}>{cluster?.environment || "-"}</p>
                        </div>
                      </div>
                      <div className="grid grid-cols-2 gap-3">
                        <div>
                          <p className={cn("text-xs", textMuted)}>Default Namespace</p>
                          <p className={cn("text-sm font-medium", textPrimary)}>{cluster?.default_namespace || "-"}</p>
                        </div>
                        <div>
                          <p className={cn("text-xs", textMuted)}>Status</p>
                          <Badge variant="outline" className={cn("rounded-full", statusBadgeClass(cluster?.status || ""))}>
                            {cluster?.status || "unknown"}
                          </Badge>
                        </div>
                      </div>
                      <div>
                        <p className={cn("text-xs", textMuted)}>Last Health Check</p>
                        <p className={cn("text-sm font-medium", textPrimary)}>
                          {formatDate(cluster?.last_health_check_at)}
                        </p>
                      </div>
                    </>
                  )}
                </CardContent>
              </Card>

              <Card className={cn("shadow-lg", panelClass)}>
                <CardHeader>
                  <CardTitle className={textPrimary}>Cluster Capability</CardTitle>
                  <CardDescription className={textMuted}>Tinh nang ma cluster hien tai ho tro.</CardDescription>
                </CardHeader>
                <CardContent className="space-y-3">
                  {capability ? (
                    <>
                      <div>
                        <p className={cn("text-xs", textMuted)}>Kubernetes Version</p>
                        <p className={cn("text-sm font-medium", textPrimary)}>
                          {capability.kubernetes_version || "-"}
                        </p>
                      </div>
                      <div className="grid grid-cols-2 gap-2">
                        {[
                          { label: "Metrics API", value: capability.supports_metrics_api },
                          { label: "Ingress", value: capability.supports_ingress },
                          { label: "StatefulSet", value: capability.supports_statefulset },
                          { label: "PVC", value: capability.supports_pvc },
                        ].map((item) => (
                          <div key={item.label} className="rounded-md border border-black/10 px-3 py-2 dark:border-white/10">
                            <p className={cn("text-xs", textMuted)}>{item.label}</p>
                            <p className={cn("text-sm font-medium", textPrimary)}>{yesNoBadge(item.value)}</p>
                          </div>
                        ))}
                      </div>
                      <div>
                        <p className={cn("text-xs", textMuted)}>Ingress Classes</p>
                        <p className={cn("text-sm font-medium break-words", textPrimary)}>
                          {capability.ingress_classes.length > 0
                            ? capability.ingress_classes.join(", ")
                            : "-"}
                        </p>
                      </div>
                      <div>
                        <p className={cn("text-xs", textMuted)}>Storage Classes</p>
                        <p className={cn("text-sm font-medium break-words", textPrimary)}>
                          {capability.storage_classes.length > 0
                            ? capability.storage_classes.join(", ")
                            : "-"}
                        </p>
                      </div>
                    </>
                  ) : (
                    <p className={cn("text-sm", textMuted)}>
                      Chua co du lieu capability. Chay sync-capabilities tu backend de cap nhat.
                    </p>
                  )}
                </CardContent>
              </Card>
            </div>

            <div className="space-y-4">
              <TrendCard
                title="CPU Trend"
                description="Tong CPU usage cua cluster theo node metrics."
                colorVar="hsl(var(--chart-1))"
                gradientId="k8sCpuTrendFill"
                panelClass={panelClass}
                textPrimary={textPrimary}
                textMuted={textMuted}
                data={cpuTrendData}
                formatter={(value) => formatMilliCPU(value)}
              />

              <TrendCard
                title="Memory Trend"
                description="Tong memory usage cua cluster theo node metrics."
                colorVar="hsl(var(--chart-2))"
                gradientId="k8sMemoryTrendFill"
                panelClass={panelClass}
                textPrimary={textPrimary}
                textMuted={textMuted}
                data={memoryTrendData}
                formatter={(value) => formatBytes(value)}
              />

              <Card className={cn("shadow-lg", panelClass)}>
                <CardHeader>
                  <CardTitle className={textPrimary}>Node Metrics Snapshot</CardTitle>
                  <CardDescription className={textMuted}>
                    {metricsError
                      ? "Khong lay duoc metrics."
                      : `Cap nhat moi nhat: ${formatDate(lastMetricsAt)}`}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  {metricsError ? <p className="mb-3 text-sm text-rose-500">{metricsError}</p> : null}
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>Node</TableHead>
                        <TableHead>CPU</TableHead>
                        <TableHead>Memory</TableHead>
                        <TableHead>Timestamp</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {metricsLoading && nodes.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={4} className={cn("text-center text-sm", textMuted)}>
                            Loading node metrics...
                          </TableCell>
                        </TableRow>
                      ) : nodes.length === 0 ? (
                        <TableRow>
                          <TableCell colSpan={4} className={cn("text-center text-sm", textMuted)}>
                            Chua co node metrics.
                          </TableCell>
                        </TableRow>
                      ) : (
                        nodes.map((node) => (
                          <TableRow key={node.node_name}>
                            <TableCell className={cn("font-medium", textPrimary)}>{node.node_name}</TableCell>
                            <TableCell className={textMuted}>
                              {node.cpu_milli > 0 ? formatMilliCPU(node.cpu_milli) : node.cpu_raw || "-"}
                            </TableCell>
                            <TableCell className={textMuted}>
                              {node.memory_bytes > 0 ? formatBytes(node.memory_bytes) : node.memory_raw || "-"}
                            </TableCell>
                            <TableCell className={textMuted}>{formatDate(node.timestamp)}</TableCell>
                          </TableRow>
                        ))
                      )}
                    </TableBody>
                  </Table>
                </CardContent>
              </Card>
            </div>
          </section>
        </>
      )}
    </main>
  );
}
