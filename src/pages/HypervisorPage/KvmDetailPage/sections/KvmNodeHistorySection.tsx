import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

export type KvmNodeHistoryRow = {
  timestamp: string;
  load1: number;
  memoryUsedBytes: number;
  diskWriteIos: number;
};

type KvmNodeHistorySectionProps = {
  panelClass: string;
  textPrimary: string;
  textMuted: string;
  rows: KvmNodeHistoryRow[];
  className?: string;
};

export function KvmNodeHistorySection({
  panelClass,
  textPrimary,
  textMuted,
  rows,
  className,
}: KvmNodeHistorySectionProps) {
  return (
    <Card className={cn("shadow-lg", panelClass, className)}>
      <CardHeader>
        <CardTitle className={cn("text-base", textPrimary)}>
          Node History
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4"></div>

        <div className="rounded-lg border border-dashed border-black/15 bg-black/[0.02] p-6 text-center dark:border-white/15 dark:bg-white/[0.02]">
          <p className={cn("text-sm", textMuted)}>Chua co du lieu history.</p>
        </div>

        <p className={cn("sr-only", textMuted)}>{rows.length}</p>
      </CardContent>
    </Card>
  );
}
