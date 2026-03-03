import { Activity, RefreshCw, RotateCcw } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { KvmHypervisorDetail } from "@/pages/HypervisorPage/KvmDetailPage/sections/overview/kvm-node-detail.api";

type KvmAgentSidebarSectionProps = {
  panelClass: string;
  textPrimary: string;
  textMuted: string;
  detail: KvmHypervisorDetail | null;
  loading: boolean;
  onCheckUpdate: () => void;
  onReinstall: () => void;
  onHealthcheck: () => void;
};

export function KvmAgentSidebar({
  panelClass,
  textPrimary,
  textMuted,
  detail,
  loading,
  onCheckUpdate,
  onReinstall,
  onHealthcheck,
}: KvmAgentSidebarSectionProps) {
  const endpoint = detail?.apiEndpoint || "-";
  const port = detail?.apiPort ?? 0;
  const protocol = "grpc";
  const tls = detail?.tlsEnabled ? "enabled" : "disabled";
  const verify = detail?.tlsSkipVerify ? "skip-verify" : "verify";
  const statusLower = (detail?.agentStatus || "").trim().toLowerCase();
  const isConnected =
    statusLower === "connected" ||
    statusLower === "running" ||
    statusLower === "healthy";

  return (
    <Card className={cn("shadow-lg", panelClass)}>
      <CardHeader>
        <CardTitle className={cn("flex items-center gap-2 text-base", textPrimary)}>
          <span
            className={cn(
              "inline-block h-2.5 w-2.5 rounded-full",
              isConnected ? "bg-emerald-500" : "bg-rose-500",
            )}
          />
          Agent
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="rounded-lg border border-black/10 bg-black/[0.03] p-3 dark:border-white/10 dark:bg-white/[0.03]">
          <p className={cn("text-xs", textMuted)}>Endpoint</p>
          <p className={cn("text-sm font-semibold break-all", textPrimary)}>
            {endpoint}
            {port > 0 ? `:${port}` : ""}
          </p>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div className="rounded-lg border border-black/10 bg-black/[0.03] p-3 dark:border-white/10 dark:bg-white/[0.03]">
            <p className={cn("text-xs", textMuted)}>Protocol</p>
            <p className={cn("text-sm font-semibold", textPrimary)}>
              {protocol}
            </p>
          </div>
          <div className="rounded-lg border border-black/10 bg-black/[0.03] p-3 dark:border-white/10 dark:bg-white/[0.03]">
            <p className={cn("text-xs", textMuted)}>TLS</p>
            <p className={cn("text-sm font-semibold", textPrimary)}>{tls}</p>
          </div>
        </div>

        <div className="rounded-lg border border-black/10 bg-black/[0.03] p-3 dark:border-white/10 dark:bg-white/[0.03]">
          <p className={cn("text-xs", textMuted)}>Certificate verify</p>
          <p className={cn("text-sm font-semibold", textPrimary)}>{verify}</p>
        </div>

        <div className="grid grid-cols-1 gap-2 pt-1">
          <Button
            type="button"
            variant="outline"
            onClick={onCheckUpdate}
            disabled={loading}
            className="justify-start"
          >
            <RefreshCw className="mr-2 h-4 w-4" />
            Check update
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={onReinstall}
            disabled={loading}
            className="justify-start"
          >
            <RotateCcw className="mr-2 h-4 w-4" />
            Reinstall
          </Button>
          <Button
            type="button"
            variant="outline"
            onClick={onHealthcheck}
            disabled={loading}
            className="justify-start"
          >
            <Activity className="mr-2 h-4 w-4" />
            Healthcheck
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
