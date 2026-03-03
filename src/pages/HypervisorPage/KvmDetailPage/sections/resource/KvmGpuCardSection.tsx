import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

type KvmGpuCardSectionProps = {
  panelClass: string;
  textPrimary: string;
  textMuted: string;
  gpuCount: number;
  gpuModel: string;
  gpuUsagePct: number;
  gpuMemoryUsedBytes: number;
  gpuMemoryTotalBytes: number;
};

export function KvmGpuCardSection({
  panelClass,
  textPrimary,
  textMuted,
  gpuCount,
  gpuModel,
  gpuUsagePct,
  gpuMemoryUsedBytes,
  gpuMemoryTotalBytes,
}: KvmGpuCardSectionProps) {
  const usedGb = gpuMemoryUsedBytes > 0 ? gpuMemoryUsedBytes / (1024 ** 3) : 0;
  const totalGb = gpuMemoryTotalBytes > 0 ? gpuMemoryTotalBytes / (1024 ** 3) : 0;

  return (
    <Card className={cn("shadow-lg", panelClass)}>
      <CardHeader>
        <CardTitle className={cn("text-base", textPrimary)}>GPU</CardTitle>
      </CardHeader>
      <CardContent className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <div className="rounded-lg border border-black/10 bg-black/[0.03] p-3 dark:border-white/10 dark:bg-white/[0.03]">
          <p className={cn("text-xs", textMuted)}>GPU count</p>
          <p className={cn("text-xl font-semibold", textPrimary)}>{gpuCount}</p>
        </div>
        <div className="rounded-lg border border-black/10 bg-black/[0.03] p-3 dark:border-white/10 dark:bg-white/[0.03] sm:col-span-2">
          <p className={cn("text-xs", textMuted)}>GPU model</p>
          <p className={cn("text-sm font-semibold", textPrimary)}>{gpuModel || "-"}</p>
        </div>
        <div className="rounded-lg border border-black/10 bg-black/[0.03] p-3 dark:border-white/10 dark:bg-white/[0.03]">
          <p className={cn("text-xs", textMuted)}>GPU usage</p>
          <p className={cn("text-xl font-semibold", textPrimary)}>
            {gpuCount > 0 ? `${gpuUsagePct.toFixed(1)}%` : "-"}
          </p>
        </div>
        <div className="rounded-lg border border-black/10 bg-black/[0.03] p-3 dark:border-white/10 dark:bg-white/[0.03] sm:col-span-2">
          <p className={cn("text-xs", textMuted)}>GPU memory</p>
          <p className={cn("text-sm font-semibold", textPrimary)}>
            {gpuCount > 0 ? `${usedGb.toFixed(1)} GB / ${totalGb.toFixed(1)} GB` : "-"}
          </p>
        </div>
        <div className="rounded-lg border border-black/10 bg-black/[0.03] p-3 dark:border-white/10 dark:bg-white/[0.03] sm:col-span-2">
          <p className={cn("text-xs", textMuted)}>Telemetry</p>
          <p className={cn("text-sm", textMuted)}>
            {gpuCount > 0 ? "GPU metric stream is active." : "No GPU detected on this node."}
          </p>
        </div>
      </CardContent>
    </Card>
  );
}

