import { Server } from "lucide-react";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { KvmHypervisorDetail } from "@/pages/HypervisorPage/KvmDetailPage/sections/overview/kvm-node-detail.api";

type KvmNodeSidebarSectionProps = {
  panelClass: string;
  textPrimary: string;
  textMuted: string;
  detail: KvmHypervisorDetail | null;
  fallbackNodeID?: string;
};

export function KvmNodeSidebarSection({
  panelClass,
  textPrimary,
  textMuted,
  detail,
  fallbackNodeID = "-",
}: KvmNodeSidebarSectionProps) {
  const nodeName = detail?.nodeName || "-";
  const nodeID = detail?.nodeId || fallbackNodeID;
  const zone = detail?.zone || "-";
  const host = detail?.host || "-";
  const statusLower = (detail?.status || "").trim().toLowerCase();
  const isRunning = statusLower === "running";

  return (
    <Card className={cn("shadow-lg", panelClass)}>
      <CardHeader>
        <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
          <Server className="h-4 w-4 text-indigo-400" />
          <span
            className={cn(
              "inline-block h-2.5 w-2.5 rounded-full",
              isRunning ? "bg-emerald-500" : "bg-rose-500",
            )}
          />
          <span>{nodeName}</span>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <div>
          <p className={cn("text-xs", textMuted)}>Node ID</p>
          <p className={cn("text-sm font-medium break-all", textPrimary)}>{nodeID}</p>
        </div>
        <div>
          <p className={cn("text-xs", textMuted)}>Zone</p>
          <p className={cn("text-sm font-medium", textPrimary)}>{zone}</p>
        </div>
        <div>
          <p className={cn("text-xs", textMuted)}>Host</p>
          <p className={cn("text-sm font-medium", textPrimary)}>{host}</p>
        </div>
      </CardContent>
    </Card>
  );
}
