import {
  AlertTriangle,
  CheckCircle2,
  Clock3,
  Cpu,
  HardDrive,
  Layers,
  Network,
  Plus,
  RefreshCcw,
  Search,
  ShieldCheck,
  Terminal,
} from "lucide-react";
import { useMemo, useState } from "react";
import { useTheme } from "next-themes";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Progress } from "@/components/ui/progress";
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table";
import { cn } from "@/lib/utils";

type WorkloadStatus = "running" | "pending" | "failed" | "succeeded";

type WorkloadItem = {
  name: string;
  namespace: string;
  kind: "Deployment" | "StatefulSet" | "Job";
  replicas: string;
  restarts: number;
  node: string;
  status: WorkloadStatus;
  age: string;
};

type NodeItem = {
  name: string;
  role: "control-plane" | "worker";
  cpu: number;
  memory: number;
  pods: number;
  healthy: boolean;
};

const workloads: WorkloadItem[] = [
  {
    name: "api-gateway",
    namespace: "core",
    kind: "Deployment",
    replicas: "8/8",
    restarts: 2,
    node: "k8s-wk-01",
    status: "running",
    age: "12d",
  },
  {
    name: "worker-billing",
    namespace: "core",
    kind: "Deployment",
    replicas: "4/6",
    restarts: 7,
    node: "k8s-wk-03",
    status: "pending",
    age: "4d",
  },
  {
    name: "redis-main",
    namespace: "data",
    kind: "StatefulSet",
    replicas: "3/3",
    restarts: 1,
    node: "k8s-wk-02",
    status: "running",
    age: "20d",
  },
  {
    name: "daily-report-job",
    namespace: "analytics",
    kind: "Job",
    replicas: "1/1",
    restarts: 0,
    node: "k8s-wk-04",
    status: "succeeded",
    age: "9h",
  },
  {
    name: "notify-relay",
    namespace: "core",
    kind: "Deployment",
    replicas: "1/3",
    restarts: 14,
    node: "k8s-wk-03",
    status: "failed",
    age: "1d",
  },
];

const clusterNodes: NodeItem[] = [
  { name: "k8s-cp-01", role: "control-plane", cpu: 42, memory: 51, pods: 28, healthy: true },
  { name: "k8s-wk-01", role: "worker", cpu: 66, memory: 61, pods: 74, healthy: true },
  { name: "k8s-wk-02", role: "worker", cpu: 58, memory: 55, pods: 69, healthy: true },
  { name: "k8s-wk-03", role: "worker", cpu: 84, memory: 80, pods: 81, healthy: false },
  { name: "k8s-wk-04", role: "worker", cpu: 49, memory: 44, pods: 52, healthy: true },
];

const statusTabs: Array<{ value: WorkloadStatus | "all"; label: string }> = [
  { value: "all", label: "All" },
  { value: "running", label: "Running" },
  { value: "pending", label: "Pending" },
  { value: "failed", label: "Failed" },
  { value: "succeeded", label: "Succeeded" },
];

const statusColor: Record<WorkloadStatus, string> = {
  running: "text-emerald-500",
  pending: "text-amber-500",
  failed: "text-rose-500",
  succeeded: "text-sky-500",
};

export default function K8sPage() {
  const { resolvedTheme } = useTheme();
  const [query, setQuery] = useState("");
  const [namespaceFilter, setNamespaceFilter] = useState("all");
  const [statusFilter, setStatusFilter] = useState<WorkloadStatus | "all">("all");

  const isDark = resolvedTheme !== "light";
  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";
  const panelClass = isDark
    ? "border-white/10 bg-slate-950/60"
    : "border-black/10 bg-white/85";

  const namespaces = useMemo(() => {
    return ["all", ...Array.from(new Set(workloads.map((workload) => workload.namespace)))];
  }, []);

  const filtered = useMemo(() => {
    return workloads.filter((workload) => {
      const q = query.trim().toLowerCase();
      const matchesQuery =
        q.length === 0 ||
        workload.name.toLowerCase().includes(q) ||
        workload.namespace.toLowerCase().includes(q) ||
        workload.node.toLowerCase().includes(q);
      const matchesNamespace = namespaceFilter === "all" ? true : workload.namespace === namespaceFilter;
      const matchesStatus = statusFilter === "all" ? true : workload.status === statusFilter;
      return matchesQuery && matchesNamespace && matchesStatus;
    });
  }, [namespaceFilter, query, statusFilter]);

  const runningCount = workloads.filter((workload) => workload.status === "running").length;
  const failedCount = workloads.filter((workload) => workload.status === "failed").length;
  const totalPods = clusterNodes.reduce((acc, node) => acc + node.pods, 0);
  const avgCpu = Math.round(clusterNodes.reduce((acc, node) => acc + node.cpu, 0) / clusterNodes.length);

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
            Orchestration
          </Badge>
          <h1 className={cn("text-3xl font-semibold tracking-tight", textPrimary)}>
            Kubernetes Control Plane
          </h1>
          <p className={cn("text-sm", textMuted)}>
            Quan ly namespace, workload va node health tren cum Kubernetes theo thoi gian thuc.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" className={cn(isDark && "border-white/20 bg-white/5")}>
            <RefreshCcw className="h-4 w-4" />
            Sync Cluster
          </Button>
          <Button className="bg-indigo-500 text-white hover:bg-indigo-400">
            <Plus className="h-4 w-4" />
            Deploy Workload
          </Button>
        </div>
      </header>

      <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {[
          { icon: Network, label: "Clusters", value: "4", hint: "multi-region ready" },
          { icon: ShieldCheck, label: "Running Workloads", value: `${runningCount}`, hint: "stable services" },
          { icon: Layers, label: "Total Pods", value: `${totalPods}`, hint: "scheduled pods" },
          { icon: Cpu, label: "Avg Node CPU", value: `${avgCpu}%`, hint: "cluster pressure" },
        ].map((metric) => (
          <Card key={metric.label} className={cn("relative overflow-hidden shadow-lg", panelClass)}>
            <div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-sky-500/15 via-sky-400/5 to-transparent" />
            <CardHeader className="pb-1">
              <div className="flex items-center justify-between">
                <CardDescription className={textMuted}>{metric.label}</CardDescription>
                <metric.icon className={cn("h-4 w-4", isDark ? "text-slate-300" : "text-slate-500")} />
              </div>
              <CardTitle className={cn("text-3xl", textPrimary)}>{metric.value}</CardTitle>
            </CardHeader>
            <CardContent className={cn("text-xs", textMuted)}>{metric.hint}</CardContent>
          </Card>
        ))}
      </section>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_20rem]">
        <section className={cn("rounded-xl border p-4 shadow-lg backdrop-blur-xl", panelClass)}>
          <div className="mb-4 flex flex-wrap items-center gap-2">
            <div className="relative min-w-[16rem] flex-1">
              <Search
                className={cn(
                  "pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2",
                  textMuted,
                )}
              />
              <Input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="Search workload by name, namespace, node..."
                className={cn("h-10 pl-10", isDark ? "border-white/10 bg-white/5" : "bg-white")}
              />
            </div>
            <div className="flex gap-2">
              <select
                value={namespaceFilter}
                onChange={(event) => setNamespaceFilter(event.target.value)}
                className={cn(
                  "h-9 rounded-full border px-3 text-sm outline-none",
                  isDark ? "border-white/15 bg-white/5 text-white" : "border-slate-200 bg-white",
                )}
              >
                {namespaces.map((namespace) => (
                  <option key={namespace} value={namespace}>
                    {namespace === "all" ? "All namespaces" : namespace}
                  </option>
                ))}
              </select>
            </div>
            {statusTabs.map((tab) => (
              <Button
                key={tab.value}
                variant={statusFilter === tab.value ? "default" : "outline"}
                onClick={() => setStatusFilter(tab.value)}
                className={cn(
                  "h-9 rounded-full px-3",
                  statusFilter === tab.value
                    ? "bg-indigo-500 text-white hover:bg-indigo-400"
                    : isDark
                      ? "border-white/15 bg-white/5"
                      : "bg-white",
                )}
              >
                {tab.label}
              </Button>
            ))}
          </div>

          <Table>
            <TableHeader>
              <TableRow className={isDark ? "border-white/10" : "border-slate-200"}>
                <TableHead>Workload</TableHead>
                <TableHead>Namespace</TableHead>
                <TableHead>Kind</TableHead>
                <TableHead>Replicas</TableHead>
                <TableHead>Restarts</TableHead>
                <TableHead>Node</TableHead>
                <TableHead>Status</TableHead>
                <TableHead className="text-right">Age</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((workload) => (
                <TableRow key={`${workload.namespace}-${workload.name}`} className={isDark ? "border-white/10 hover:bg-white/5" : ""}>
                  <TableCell className={cn("font-semibold", textPrimary)}>{workload.name}</TableCell>
                  <TableCell className={textPrimary}>{workload.namespace}</TableCell>
                  <TableCell className={textPrimary}>{workload.kind}</TableCell>
                  <TableCell className={textPrimary}>{workload.replicas}</TableCell>
                  <TableCell className={textPrimary}>{workload.restarts}</TableCell>
                  <TableCell className={textPrimary}>{workload.node}</TableCell>
                  <TableCell>
                    <span className={cn("text-xs font-medium capitalize", statusColor[workload.status])}>
                      {workload.status}
                    </span>
                  </TableCell>
                  <TableCell className={cn("text-right", textPrimary)}>{workload.age}</TableCell>
                </TableRow>
              ))}
              {filtered.length === 0 && (
                <TableRow className={isDark ? "border-white/10" : "border-slate-200"}>
                  <TableCell colSpan={8} className={cn("py-6 text-center text-sm", textMuted)}>
                    No workload matched your filter.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </section>

        <aside className="space-y-4">
          <Card className={cn("shadow-lg", panelClass)}>
            <CardHeader>
              <CardTitle className={textPrimary}>Node Health</CardTitle>
              <CardDescription className={textMuted}>Control-plane and worker capacity</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {clusterNodes.map((node) => (
                <div
                  key={node.name}
                  className={cn(
                    "rounded-lg border px-3 py-2",
                    isDark ? "border-white/10 bg-white/5" : "border-slate-200 bg-slate-50",
                  )}
                >
                  <div className="mb-1 flex items-center justify-between">
                    <p className={cn("text-xs font-medium", textPrimary)}>
                      {node.name} ({node.role})
                    </p>
                    {node.healthy ? (
                      <CheckCircle2 className="h-3.5 w-3.5 text-emerald-500" />
                    ) : (
                      <AlertTriangle className="h-3.5 w-3.5 text-amber-500" />
                    )}
                  </div>
                  <div className={cn("space-y-1 text-[11px]", textMuted)}>
                    <div className="flex items-center justify-between">
                      <span>CPU</span>
                      <span>{node.cpu}%</span>
                    </div>
                    <Progress value={node.cpu} className="h-1.5" />
                    <div className="flex items-center justify-between">
                      <span>Memory</span>
                      <span>{node.memory}%</span>
                    </div>
                    <Progress value={node.memory} className="h-1.5" />
                  </div>
                  <p className={cn("mt-1 text-[11px]", textMuted)}>{node.pods} pods allocated</p>
                </div>
              ))}
            </CardContent>
          </Card>

          <Card className={cn("shadow-lg", panelClass)}>
            <CardHeader>
              <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
                <HardDrive className="h-4 w-4 text-indigo-400" />
                StorageClass Usage
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {[
                { name: "fast-ssd", used: 72, cap: "1.9 / 2.6 TB" },
                { name: "standard-hdd", used: 65, cap: "5.3 / 8.1 TB" },
                { name: "archive", used: 39, cap: "3.1 / 7.9 TB" },
              ].map((storage) => (
                <div key={storage.name} className="space-y-1">
                  <div className={cn("flex items-center justify-between text-xs", textMuted)}>
                    <span>{storage.name}</span>
                    <span>{storage.cap}</span>
                  </div>
                  <Progress value={storage.used} className="h-1.5" />
                </div>
              ))}
            </CardContent>
          </Card>

          <Card className={cn("shadow-lg", panelClass)}>
            <CardHeader>
              <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
                <Clock3 className="h-4 w-4 text-indigo-400" />
                Control Plane Events
              </CardTitle>
              <CardDescription className={textMuted}>
                {failedCount} critical workload currently failed.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              {[
                "api-server latency spike detected on k8s-cp-01",
                "new node k8s-wk-04 joined cluster successfully",
                "network policy sync completed for namespace core",
              ].map((event, index) => (
                <div
                  key={index}
                  className={cn(
                    "rounded-lg border px-3 py-2 text-xs",
                    isDark ? "border-white/10 bg-white/5 text-slate-200" : "border-slate-200 bg-slate-50",
                  )}
                >
                  {event}
                </div>
              ))}
              <Button variant="outline" className={cn("mt-1 w-full gap-2", isDark && "border-white/15 bg-white/5")}>
                <Terminal className="h-4 w-4" />
                Open kubectl Console
              </Button>
            </CardContent>
          </Card>
        </aside>
      </div>
    </main>
  );
}
