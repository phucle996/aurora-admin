import { Clock3, Cpu, MapPin, ShieldCheck } from "lucide-react";

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

type KvmMetricsSectionProps = {
  zoneCount: number;
  nodeCount: number;
  totalVMs: number;
  runningRatio: number;
  runningVMs: number;
  isDark: boolean;
  textPrimary: string;
  textMuted: string;
  panelClass: string;
};

export function KvmMetricsSection({
  zoneCount,
  nodeCount,
  totalVMs,
  runningRatio,
  runningVMs,
  isDark,
  textPrimary,
  textMuted,
  panelClass,
}: KvmMetricsSectionProps) {
  return (
    <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
      {[
        {
          icon: MapPin,
          label: "Availability Zones",
          value: `${zoneCount}`,
          hint: "active zones",
        },
        {
          icon: ShieldCheck,
          label: "Hypervisor Nodes",
          value: `${nodeCount}`,
          hint: `${nodeCount} total nodes`,
        },
        {
          icon: Cpu,
          label: "Total VMs",
          value: `${totalVMs}`,
          hint: "all assigned vm_instances",
        },
        {
          icon: Clock3,
          label: "Running Ratio",
          value: `${runningRatio}%`,
          hint: `${runningVMs} running`,
        },
      ].map((metric) => (
        <Card key={metric.label} className={cn("relative overflow-hidden shadow-lg", panelClass)}>
          <div className="pointer-events-none absolute inset-0 bg-gradient-to-br from-indigo-500/15 via-indigo-400/5 to-transparent" />
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
  );
}
