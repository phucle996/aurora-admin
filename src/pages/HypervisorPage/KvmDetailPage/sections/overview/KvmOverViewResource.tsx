import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";
import type { KvmHypervisorDetail } from "@/pages/HypervisorPage/KvmDetailPage/sections/overview/kvm-node-detail.api";

type KvmNodeOverviewResourceProps = {
  panelClass: string;
  textPrimary: string;
  textMuted: string;
  detail: KvmHypervisorDetail | null;
  gpuCount: number;
  gpuMemoryTotalBytes: number;
};

export function KvmNodeOverviewResource({
  panelClass,
  textPrimary,
  textMuted,
  detail,
  gpuCount,
  gpuMemoryTotalBytes,
}: KvmNodeOverviewResourceProps) {
  const cpuCoresMax = detail?.cpuCoresMax ?? 0;
  const ramMbMax = detail?.ramMbMax ?? 0;
  const diskGbMax = detail?.diskGbMax ?? 0;
  const ramGbMax = ramMbMax > 0 ? ramMbMax / 1024 : 0;
  const gpuVramTotalGb =
    gpuMemoryTotalBytes > 0 ? gpuMemoryTotalBytes / 1024 ** 3 : 0;

  const stats = [
    {
      label: "CPU",
      value: cpuCoresMax > 0 ? `${cpuCoresMax} cores` : "-",
      note: "Compute capacity",
    },
    {
      label: "RAM",
      value: ramGbMax > 0 ? `${ramGbMax.toFixed(1)} GB` : "-",
      note:
        ramMbMax > 0
          ? `${ramMbMax.toLocaleString()} MB total`
          : "Memory capacity",
    },
    {
      label: "Storage",
      value: diskGbMax > 0 ? `${diskGbMax.toLocaleString()} GB` : "-",
      note: "Disk capacity",
    },
    {
      label: "GPU VRAM Total",
      value: gpuVramTotalGb > 0 ? `${gpuVramTotalGb.toFixed(1)} GB` : "-",
      note: gpuCount > 0 ? `${gpuCount} GPU detected` : "No GPU detected",
    },
  ];

  return (
    <Card className={cn("shadow-lg", panelClass)}>
      <CardHeader>
        <CardTitle className={cn("text-base", textPrimary)}>
          Resource Overview
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-2">
          {stats.map((item) => (
            <div
              key={item.label}
              className="rounded-xl border border-black/10 bg-black/[0.03] p-4 dark:border-white/10 dark:bg-white/[0.03]"
            >
              <p
                className={cn(
                  "text-[11px] font-medium uppercase tracking-wide",
                  textMuted,
                )}
              >
                {item.label}
              </p>
              <p
                className={cn(
                  "mt-1 text-2xl font-semibold leading-tight",
                  textPrimary,
                )}
              >
                {item.value}
              </p>
              <p className={cn("mt-1 text-xs", textMuted)}>{item.note}</p>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
