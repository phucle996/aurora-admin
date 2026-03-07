import {
  AlertTriangle,
  ArrowRight,
  CheckCircle2,
  RefreshCcw,
  Search,
  Server,
  XCircle,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useTheme } from "next-themes";
import { useNavigate } from "react-router-dom";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";
import { useEnabledModules } from "@/state/enabled-modules-context";
import {
  formatDate,
  isPlatformModule,
  listPlatformClusters,
  resolvePlatformBaseURL,
  statusBadgeClass,
  type PlatformCluster,
} from "@/pages/K8sPage/k8s-page.api";

export default function K8sPage() {
  const { resolvedTheme } = useTheme();
  const navigate = useNavigate();
  const { items } = useEnabledModules();
  const [query, setQuery] = useState("");
  const [clusters, setClusters] = useState<PlatformCluster[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const platformBaseURL = useMemo(() => resolvePlatformBaseURL(items), [items]);
  const hasPlatformModule = useMemo(
    () => items.some((item) => item.installed && isPlatformModule(item)),
    [items],
  );

  const loadClusters = useCallback(async () => {
    if (!platformBaseURL) {
      setClusters([]);
      setError("");
      return;
    }

    setLoading(true);
    setError("");
    try {
      setClusters(await listPlatformClusters(platformBaseURL));
    } catch (err) {
      const message = err instanceof Error ? err.message : "Load cluster failed";
      setError(message);
      setClusters([]);
    } finally {
      setLoading(false);
    }
  }, [platformBaseURL]);

  useEffect(() => {
    if (!platformBaseURL) {
      return;
    }
    void loadClusters();
  }, [platformBaseURL, loadClusters]);

  const isDark = resolvedTheme !== "light";
  const panelClass = isDark ? "border-white/10 bg-slate-950/60" : "border-black/10 bg-white/85";
  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) {
      return clusters;
    }
    return clusters.filter((cluster) => {
      const text =
        `${cluster.name} ${cluster.region} ${cluster.environment} ${cluster.api_endpoint} ${cluster.status}`.toLowerCase();
      return text.includes(q);
    });
  }, [clusters, query]);

  const healthyCount = clusters.filter((cluster) => cluster.status.toLowerCase() === "healthy").length;
  const unreachableCount = clusters.filter((cluster) => cluster.status.toLowerCase() === "unreachable").length;
  const enabledCount = clusters.filter((cluster) => cluster.is_enabled).length;

  return (
    <main className="space-y-4 py-3 lg:py-1">
      <header className="mb-2 flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-2">
          <Badge
            variant="outline"
            className={cn(
              "rounded-full px-3 py-1 text-xs uppercase tracking-[0.12em]",
              isDark ? "border-white/20 bg-white/5 text-slate-200" : "bg-white/70",
            )}
          >
            K8s - Platform Resource
          </Badge>
          <h1 className={cn("text-3xl font-semibold tracking-tight", textPrimary)}>
            Kubernetes Cluster Registry
          </h1>
          <p className={cn("text-sm", textMuted)}>
            Chon cluster de xem detail thong tin va metric runtime.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            className={cn(isDark && "border-white/20 bg-white/5")}
            onClick={() => void loadClusters()}
            disabled={!platformBaseURL || loading}
          >
            <RefreshCcw className={cn("h-4 w-4", loading && "animate-spin")} />
            Refresh
          </Button>
        </div>
      </header>

      <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {[
          { label: "Total Clusters", value: clusters.length, icon: Server },
          { label: "Healthy", value: healthyCount, icon: CheckCircle2 },
          { label: "Unreachable", value: unreachableCount, icon: XCircle },
          { label: "Enabled", value: enabledCount, icon: AlertTriangle },
        ].map((metric) => (
          <Card key={metric.label} className={cn("shadow-lg", panelClass)}>
            <CardHeader className="pb-1">
              <div className="flex items-center justify-between">
                <CardDescription className={textMuted}>{metric.label}</CardDescription>
                <metric.icon className={cn("h-4 w-4", isDark ? "text-slate-300" : "text-slate-500")} />
              </div>
              <CardTitle className={cn("text-3xl", textPrimary)}>{metric.value}</CardTitle>
            </CardHeader>
          </Card>
        ))}
      </section>

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
        <Card className={cn("shadow-lg", panelClass)}>
          <CardHeader className="space-y-3">
            <div className="flex flex-wrap items-center justify-between gap-3">
              <div>
                <CardTitle className={textPrimary}>Registered Clusters</CardTitle>
                <CardDescription className={textMuted}>
                  Source: <code>{platformBaseURL}</code>
                </CardDescription>
              </div>
              <div className="relative w-full min-w-[16rem] max-w-[24rem]">
                <Search className={cn("pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2", textMuted)} />
                <Input
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  placeholder="Search by name/region/env/status..."
                  className={cn("h-10 pl-10", isDark ? "border-white/10 bg-white/5" : "bg-white")}
                />
              </div>
            </div>
            {error ? <p className="text-sm text-rose-500">{error}</p> : null}
          </CardHeader>
          <CardContent>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>API Endpoint</TableHead>
                  <TableHead>Region</TableHead>
                  <TableHead>Environment</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Enabled</TableHead>
                  <TableHead>Last Health Check</TableHead>
                  <TableHead className="text-right">Action</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {loading ? (
                  <TableRow>
                    <TableCell colSpan={8} className={cn("text-center text-sm", textMuted)}>
                      Loading clusters...
                    </TableCell>
                  </TableRow>
                ) : filtered.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={8} className={cn("text-center text-sm", textMuted)}>
                      Khong co cluster nao.
                    </TableCell>
                  </TableRow>
                ) : (
                  filtered.map((cluster) => (
                    <TableRow key={cluster.id}>
                      <TableCell className={cn("font-medium", textPrimary)}>{cluster.name}</TableCell>
                      <TableCell className={textMuted}>{cluster.api_endpoint || "-"}</TableCell>
                      <TableCell className={textMuted}>{cluster.region || "-"}</TableCell>
                      <TableCell className={textMuted}>{cluster.environment || "-"}</TableCell>
                      <TableCell>
                        <Badge variant="outline" className={cn("rounded-full", statusBadgeClass(cluster.status))}>
                          {cluster.status || "unknown"}
                        </Badge>
                      </TableCell>
                      <TableCell className={textMuted}>{cluster.is_enabled ? "true" : "false"}</TableCell>
                      <TableCell className={textMuted}>{formatDate(cluster.last_health_check_at)}</TableCell>
                      <TableCell className="text-right">
                        <Button
                          size="sm"
                          variant="outline"
                          className={cn(isDark && "border-white/20 bg-white/5")}
                          onClick={() => navigate(`/orchestration/k8s/${cluster.id}`)}
                        >
                          Detail
                          <ArrowRight className="ml-1 h-4 w-4" />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}
    </main>
  );
}
