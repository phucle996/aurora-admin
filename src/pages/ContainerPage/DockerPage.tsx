import {
  Activity,
  Box,
  HardDrive,
  Layers,
  Package,
  Plus,
  RefreshCcw,
  Search,
  Server,
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

type ContainerState = "running" | "restarting" | "exited" | "paused";

type ContainerItem = {
  id: string;
  name: string;
  image: string;
  host: string;
  cpu: number;
  memoryMb: number;
  ports: string;
  uptime: string;
  state: ContainerState;
};

const containers: ContainerItem[] = [
  {
    id: "c-8fa2a1",
    name: "api-gateway",
    image: "aurora/api-gateway:2.3.1",
    host: "docker-node-01",
    cpu: 31,
    memoryMb: 620,
    ports: "8080:8080",
    uptime: "9d 12h",
    state: "running",
  },
  {
    id: "c-90f11b",
    name: "worker-queue",
    image: "aurora/worker:1.14.8",
    host: "docker-node-02",
    cpu: 52,
    memoryMb: 780,
    ports: "-",
    uptime: "2d 06h",
    state: "running",
  },
  {
    id: "c-7ddc34",
    name: "notification-service",
    image: "aurora/noti:1.3.0",
    host: "docker-node-01",
    cpu: 17,
    memoryMb: 410,
    ports: "9091:9090",
    uptime: "0d 18h",
    state: "restarting",
  },
  {
    id: "c-2af552",
    name: "legacy-reporter",
    image: "aurora/reporter:0.9.4",
    host: "docker-node-03",
    cpu: 0,
    memoryMb: 0,
    ports: "-",
    uptime: "0d",
    state: "exited",
  },
  {
    id: "c-b12ec9",
    name: "cache-sidecar",
    image: "redis:7-alpine",
    host: "docker-node-02",
    cpu: 6,
    memoryMb: 180,
    ports: "6379:6379",
    uptime: "6d 02h",
    state: "paused",
  },
];

const stateTabs: Array<{ value: ContainerState | "all"; label: string }> = [
  { value: "all", label: "All" },
  { value: "running", label: "Running" },
  { value: "restarting", label: "Restarting" },
  { value: "paused", label: "Paused" },
  { value: "exited", label: "Exited" },
];

const stateColor: Record<ContainerState, string> = {
  running: "text-emerald-500",
  restarting: "text-amber-500",
  paused: "text-sky-500",
  exited: "text-slate-500",
};

export default function DockerPage() {
  const { resolvedTheme } = useTheme();
  const [query, setQuery] = useState("");
  const [stateFilter, setStateFilter] = useState<ContainerState | "all">("all");

  const isDark = resolvedTheme !== "light";
  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";
  const panelClass = isDark
    ? "border-white/10 bg-slate-950/60"
    : "border-black/10 bg-white/85";

  const filtered = useMemo(() => {
    return containers.filter((container) => {
      const matchesState = stateFilter === "all" ? true : container.state === stateFilter;
      const q = query.trim().toLowerCase();
      const matchesQuery =
        q.length === 0 ||
        container.name.toLowerCase().includes(q) ||
        container.image.toLowerCase().includes(q) ||
        container.id.toLowerCase().includes(q) ||
        container.host.toLowerCase().includes(q);
      return matchesState && matchesQuery;
    });
  }, [query, stateFilter]);

  const runningCount = containers.filter((container) => container.state === "running").length;
  const restartingCount = containers.filter((container) => container.state === "restarting").length;
  const avgCpu = Math.round(
    containers.filter((container) => container.state !== "exited").reduce((acc, item) => acc + item.cpu, 0) /
      (containers.filter((container) => container.state !== "exited").length || 1),
  );
  const totalMemoryMb = containers.reduce((acc, container) => acc + container.memoryMb, 0);

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
            Container Runtime
          </Badge>
          <h1 className={cn("text-3xl font-semibold tracking-tight", textPrimary)}>Docker Operations</h1>
          <p className={cn("text-sm", textMuted)}>
            Quan ly container, image, service health va resource allocation tren cac docker node.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="outline" className={cn(isDark && "border-white/20 bg-white/5")}>
            <RefreshCcw className="h-4 w-4" />
            Sync Engine
          </Button>
          <Button className="bg-indigo-500 text-white hover:bg-indigo-400">
            <Plus className="h-4 w-4" />
            Deploy Service
          </Button>
        </div>
      </header>

      <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {[
          { icon: Box, label: "Containers", value: `${containers.length}`, hint: "across 3 nodes" },
          { icon: ShieldCheck, label: "Running", value: `${runningCount}`, hint: "healthy workload" },
          { icon: Activity, label: "Avg CPU", value: `${avgCpu}%`, hint: "active containers" },
          {
            icon: HardDrive,
            label: "Total Memory",
            value: `${(totalMemoryMb / 1024).toFixed(1)} GB`,
            hint: "allocated RAM",
          },
        ].map((metric) => (
          <Card key={metric.label} className={cn("relative overflow-hidden shadow-lg", panelClass)}>
            <div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-cyan-500/15 via-cyan-400/5 to-transparent" />
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
                placeholder="Search container by name, image, host..."
                className={cn("h-10 pl-10", isDark ? "border-white/10 bg-white/5" : "bg-white")}
              />
            </div>
            {stateTabs.map((tab) => (
              <Button
                key={tab.value}
                variant={stateFilter === tab.value ? "default" : "outline"}
                onClick={() => setStateFilter(tab.value)}
                className={cn(
                  "h-9 rounded-full px-3",
                  stateFilter === tab.value
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
                <TableHead>Container</TableHead>
                <TableHead>Image</TableHead>
                <TableHead>Host</TableHead>
                <TableHead>CPU</TableHead>
                <TableHead>Memory</TableHead>
                <TableHead>Ports</TableHead>
                <TableHead>State</TableHead>
                <TableHead className="text-right">Uptime</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((container) => (
                <TableRow key={container.id} className={isDark ? "border-white/10 hover:bg-white/5" : ""}>
                  <TableCell>
                    <div>
                      <p className={cn("font-semibold", textPrimary)}>{container.name}</p>
                      <p className={cn("text-xs", textMuted)}>{container.id}</p>
                    </div>
                  </TableCell>
                  <TableCell className={cn("max-w-[14rem] truncate", textPrimary)}>{container.image}</TableCell>
                  <TableCell className={textPrimary}>{container.host}</TableCell>
                  <TableCell>
                    <div className="w-24 space-y-1">
                      <Progress value={container.cpu} className="h-1.5" />
                      <p className={cn("text-[11px]", textMuted)}>{container.cpu}%</p>
                    </div>
                  </TableCell>
                  <TableCell className={textPrimary}>{container.memoryMb} MB</TableCell>
                  <TableCell className={textPrimary}>{container.ports}</TableCell>
                  <TableCell>
                    <span className={cn("text-xs font-medium capitalize", stateColor[container.state])}>
                      {container.state}
                    </span>
                  </TableCell>
                  <TableCell className={cn("text-right", textPrimary)}>{container.uptime}</TableCell>
                </TableRow>
              ))}
              {filtered.length === 0 && (
                <TableRow className={isDark ? "border-white/10" : "border-slate-200"}>
                  <TableCell colSpan={8} className={cn("py-6 text-center text-sm", textMuted)}>
                    No container matched your filter.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </section>

        <aside className="space-y-4">
          <Card className={cn("shadow-lg", panelClass)}>
            <CardHeader>
              <CardTitle className={textPrimary}>Cluster Health</CardTitle>
              <CardDescription className={textMuted}>Node level utilization</CardDescription>
            </CardHeader>
            <CardContent className="space-y-3">
              {[
                { node: "docker-node-01", cpu: 64, ram: 58 },
                { node: "docker-node-02", cpu: 71, ram: 66 },
                { node: "docker-node-03", cpu: 48, ram: 39 },
              ].map((node) => (
                <div
                  key={node.node}
                  className={cn(
                    "rounded-lg border px-3 py-2",
                    isDark ? "border-white/10 bg-white/5" : "border-slate-200 bg-slate-50",
                  )}
                >
                  <p className={cn("mb-1 text-xs font-medium", textPrimary)}>{node.node}</p>
                  <div className={cn("space-y-1 text-[11px]", textMuted)}>
                    <div className="flex items-center justify-between">
                      <span>CPU</span>
                      <span>{node.cpu}%</span>
                    </div>
                    <Progress value={node.cpu} className="h-1.5" />
                    <div className="flex items-center justify-between">
                      <span>RAM</span>
                      <span>{node.ram}%</span>
                    </div>
                    <Progress value={node.ram} className="h-1.5" />
                  </div>
                </div>
              ))}
            </CardContent>
          </Card>

          <Card className={cn("shadow-lg", panelClass)}>
            <CardHeader>
              <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
                <Package className="h-4 w-4 text-indigo-400" />
                Image Registry
              </CardTitle>
              <CardDescription className={textMuted}>pull/scan/push pipeline status</CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              {[
                { name: "aurora/api-gateway", tag: "2.3.1", size: "398 MB" },
                { name: "aurora/worker", tag: "1.14.8", size: "512 MB" },
                { name: "aurora/noti", tag: "1.3.0", size: "241 MB" },
              ].map((image) => (
                <div
                  key={image.name}
                  className={cn(
                    "rounded-lg border px-3 py-2 text-xs",
                    isDark ? "border-white/10 bg-white/5 text-slate-200" : "border-slate-200 bg-slate-50",
                  )}
                >
                  <p className="font-medium">{image.name}</p>
                  <p className={textMuted}>
                    tag {image.tag} • {image.size}
                  </p>
                </div>
              ))}
            </CardContent>
          </Card>

          <Card className={cn("shadow-lg", panelClass)}>
            <CardHeader>
              <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
                <Layers className="h-4 w-4 text-indigo-400" />
                Runtime Actions
              </CardTitle>
              <CardDescription className={textMuted}>
                Restart policy audit: {restartingCount} service need attention.
              </CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              <Button variant="outline" className={cn("w-full gap-2", isDark && "border-white/15 bg-white/5")}>
                <Terminal className="h-4 w-4" />
                Open Docker Console
              </Button>
              <Button variant="outline" className={cn("w-full gap-2", isDark && "border-white/15 bg-white/5")}>
                <Server className="h-4 w-4" />
                Node Drain Assistant
              </Button>
            </CardContent>
          </Card>
        </aside>
      </div>
    </main>
  );
}
