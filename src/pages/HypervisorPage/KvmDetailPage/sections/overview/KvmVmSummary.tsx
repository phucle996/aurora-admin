import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { KvmHypervisorDetail } from "@/pages/HypervisorPage/KvmDetailPage/sections/overview/kvm-node-detail.api";

type KvmVmRuntimeSummarySectionProps = {
  panelClass: string;
  textPrimary: string;
  textMuted: string;
  detail: KvmHypervisorDetail | null;
};

export function KvmVmRuntimeSummarySection({
  panelClass,
  textPrimary,
  textMuted,
  detail,
}: KvmVmRuntimeSummarySectionProps) {
  const vmTotal = detail?.vmTotal ?? 0;
  const vmRunning = detail?.vmRunning ?? 0;
  const vmStopped = detail?.vmStopped ?? 0;

  return (
    <Card className={cn("shadow-lg", panelClass)}>
      <CardHeader>
        <CardTitle className={cn("text-base", textPrimary)}>VM Runtime Summary</CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="rounded-lg border border-black/10 bg-black/[0.03] p-3 dark:border-white/10 dark:bg-white/[0.03]">
          <p className={cn("text-xs", textMuted)}>VM total</p>
          <p className={cn("text-2xl font-semibold", textPrimary)}>{vmTotal}</p>
        </div>
        <div className="rounded-lg border border-emerald-300/30 bg-emerald-500/5 p-3 dark:border-emerald-400/20">
          <p className={cn("text-xs", textMuted)}>Running</p>
          <p className="text-2xl font-semibold text-emerald-600 dark:text-emerald-300">
            {vmRunning}
          </p>
        </div>
        <div className="rounded-lg border border-rose-300/30 bg-rose-500/5 p-3 dark:border-rose-400/20">
          <p className={cn("text-xs", textMuted)}>Stopped</p>
          <p className="text-2xl font-semibold text-rose-600 dark:text-rose-300">
            {vmStopped}
          </p>
        </div>
      </CardContent>
    </Card>
  );
}
