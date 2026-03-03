import { CheckCircle2, Database, Play, Server } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { KvmNodeSSHCheckResult } from "@/pages/HypervisorPage/NewKvmPage/kvm-page.api";

type NewKvmOverviewSectionProps = {
  textPrimary: string;
  textMuted: string;
  panelClass: string;
  nodeName: string;
  host: string;
  zone: string;
  result: KvmNodeSSHCheckResult | null;
  onRunNow: () => void;
  runNowLoading: boolean;
  runNowMessage: string | null;
  isRunning: boolean;
};

function boolLabel(value: boolean): string {
  return value ? "yes" : "no";
}

export function NewKvmOverviewSection({
  textPrimary,
  textMuted,
  panelClass,
  nodeName,
  host,
  zone,
  result,
  onRunNow,
  runNowLoading,
  runNowMessage,
  isRunning,
}: NewKvmOverviewSectionProps) {
  if (!result?.ok || !result.saved || !result.savedNodeId) {
    return null;
  }

  return (
    <section className="space-y-4">
      <Card className={cn("shadow-lg", panelClass)}>
        <CardHeader>
          <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
            <CheckCircle2 className="h-4 w-4 text-emerald-500" />
            KVM Node Overview
            <Badge
              className={cn(
                "ml-2 text-white",
                isRunning
                  ? "bg-emerald-500/90 hover:bg-emerald-500"
                  : "bg-slate-500/90 hover:bg-slate-500",
              )}
            >
              {isRunning ? "running" : "stop"}
            </Badge>
            {!isRunning && (
              <Button
                type="button"
                size="sm"
                onClick={onRunNow}
                disabled={runNowLoading}
                className="ml-2 bg-indigo-500 text-white hover:bg-indigo-400"
              >
                <Play className="mr-1 h-3.5 w-3.5" />
                {runNowLoading ? "Running..." : "Run now"}
              </Button>
            )}
          </CardTitle>
        </CardHeader>
        <CardContent className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <p className={cn("text-xs", textMuted)}>Node ID</p>
            <p className={cn("text-sm font-medium break-all", textPrimary)}>
              {result.savedNodeId}
            </p>
          </div>
          <div>
            <p className={cn("text-xs", textMuted)}>Node name</p>
            <p className={cn("text-sm font-medium", textPrimary)}>
              {nodeName || "-"}
            </p>
          </div>
          <div>
            <p className={cn("text-xs", textMuted)}>Host / Zone</p>
            <p className={cn("text-sm font-medium", textPrimary)}>
              {host || result.host || "-"} / {zone || "-"}
            </p>
          </div>
          <div>
            <p className={cn("text-xs", textMuted)}>SSH</p>
            <p className={cn("text-sm font-medium", textPrimary)}>
              {result.sshUsername}@{result.host}:{result.sshPort} (
              {result.authMethod})
            </p>
          </div>
          <div>
            <p className={cn("text-xs", textMuted)}>Checked at</p>
            <p className={cn("text-sm font-medium", textPrimary)}>
              {new Date(result.checkedAt).toLocaleString()}
            </p>
          </div>
          <div>
            <p className={cn("text-xs", textMuted)}>Probe latency</p>
            <p className={cn("text-sm font-medium", textPrimary)}>
              {result.latencyMs} ms
            </p>
          </div>
        </CardContent>
      </Card>
      {runNowMessage && (
        <p className={cn("text-sm", textMuted)}>{runNowMessage}</p>
      )}

      <div className="grid gap-4 lg:grid-cols-2">
        <Card className={cn("shadow-lg", panelClass)}>
          <CardHeader>
            <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
              <Server className="h-4 w-4 text-indigo-400" />
              Capability
            </CardTitle>
          </CardHeader>
          <CardContent className="grid gap-3 sm:grid-cols-2">
            <div>
              <p className={cn("text-xs", textMuted)}>CPU cores</p>
              <p className={cn("text-sm font-medium", textPrimary)}>
                {result.capability.cpuCores}
              </p>
            </div>
            <div>
              <p className={cn("text-xs", textMuted)}>RAM</p>
              <p className={cn("text-sm font-medium", textPrimary)}>
                {result.capability.ramMb} MB
              </p>
            </div>
            <div>
              <p className={cn("text-xs", textMuted)}>Disk free</p>
              <p className={cn("text-sm font-medium", textPrimary)}>
                {result.capability.diskFreeGb} GB
              </p>
            </div>
            <div>
              <p className={cn("text-xs", textMuted)}>KVM module</p>
              <p className={cn("text-sm font-medium", textPrimary)}>
                {boolLabel(result.capability.kvmModule)}
              </p>
            </div>
            <div>
              <p className={cn("text-xs", textMuted)}>Storage pools</p>
              <p className={cn("text-sm font-medium", textPrimary)}>
                {result.capability.storagePools.join(", ") || "-"}
              </p>
            </div>
            <div>
              <p className={cn("text-xs", textMuted)}>virsh connect</p>
              <p className={cn("text-sm font-medium", textPrimary)}>
                {boolLabel(result.capability.virshConnect)}
              </p>
            </div>
            <div>
              <p className={cn("text-xs", textMuted)}>Networks</p>
              <p className={cn("text-sm font-medium", textPrimary)}>
                {result.capability.networks.join(", ") || "-"}
              </p>
            </div>
            <div>
              <p className={cn("text-xs", textMuted)}>libvirt running</p>
              <p className={cn("text-sm font-medium", textPrimary)}>
                {boolLabel(result.capability.libvirtRunning)}
              </p>
            </div>
          </CardContent>
        </Card>

        <Card className={cn("shadow-lg", panelClass)}>
          <CardHeader>
            <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
              <Server className="h-4 w-4 text-emerald-500" />
              Agent Connection
            </CardTitle>
          </CardHeader>
          <CardContent className="grid gap-3 sm:grid-cols-2">
            <div>
              <p className={cn("text-xs", textMuted)}>Endpoint</p>
              <p className={cn("text-sm font-medium", textPrimary)}>
                {result.agent?.agentEndpoint || "-"}
              </p>
            </div>
            <div>
              <p className={cn("text-xs", textMuted)}>Port</p>
              <p className={cn("text-sm font-medium", textPrimary)}>
                {result.agent?.agentPort || 0}
              </p>
            </div>
            <div>
              <p className={cn("text-xs", textMuted)}>Protocol</p>
              <p className={cn("text-sm font-medium", textPrimary)}>
                {result.agent?.agentProtocol || "-"}
              </p>
            </div>
            <div>
              <p className={cn("text-xs", textMuted)}>Secure transport</p>
              <p className={cn("text-sm font-medium", textPrimary)}>
                {boolLabel(Boolean(result.agent?.tlsEnabled))}
              </p>
            </div>
            {result.capability.missingComponents.length > 0 && (
              <div className="sm:col-span-2">
                <p className={cn("text-xs", textMuted)}>Missing components</p>
                <p className={cn("text-sm font-medium", textPrimary)}>
                  {result.capability.missingComponents.join(", ")}
                </p>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      <Card className={cn("shadow-lg", panelClass)}>
        <CardHeader>
          <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
            <Database className="h-4 w-4 text-indigo-400" />
            Raw Node Metadata
          </CardTitle>
        </CardHeader>
        <CardContent>
          <pre
            className={cn(
              "max-h-60 overflow-auto rounded-md border p-3 text-xs",
              textPrimary,
            )}
          >
            {JSON.stringify(
              {
                saved: result.saved,
                saved_node_id: result.savedNodeId,
                node_name: nodeName || "-",
                host: host || result.host || "-",
                zone: zone || "-",
              },
              null,
              2,
            )}
          </pre>
        </CardContent>
      </Card>
    </section>
  );
}
