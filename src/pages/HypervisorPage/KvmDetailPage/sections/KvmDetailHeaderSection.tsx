import { ArrowLeft, Play, Square, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type KvmDetailHeaderSectionProps = {
  isDark: boolean;
  textPrimary: string;
  textMuted: string;
  canRemove: boolean;
  canRun: boolean;
  canStop: boolean;
  loading: boolean;
  actionLoading: boolean;
  removeLoading: boolean;
  onBack: () => void;
  onOpenRemoveDialog: () => void;
  onRunNow: () => void;
  onStop: () => void;
};

export function KvmDetailHeaderSection({
  isDark,
  textPrimary,
  textMuted,
  canRemove,
  canRun,
  canStop,
  loading,
  actionLoading,
  removeLoading,
  onBack,
  onOpenRemoveDialog,
  onRunNow,
  onStop,
}: KvmDetailHeaderSectionProps) {
  return (
    <section className="space-y-3">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex items-center gap-3">
          <Button
            type="button"
            variant="outline"
            onClick={onBack}
            className={cn("mt-0.5", isDark && "border-white/15 bg-white/5")}
          >
            <ArrowLeft className="mr-2 h-4 w-4 " />
            Back to KVM
          </Button>

          <div className="space-y-1">
            <p
              className={cn(
                "inline-flex rounded-full border px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.14em]",
                isDark
                  ? "border-indigo-300/30 bg-indigo-500/10 text-indigo-100"
                  : "border-indigo-200 bg-indigo-50 text-indigo-700",
              )}
            >
              KVM Detail
            </p>
            <h1
              className={cn(
                "text-3xl font-semibold tracking-tight",
                textPrimary,
              )}
            >
              KVM Node Detail
            </h1>
            <p className={cn("text-sm", textMuted)}>
              Chi tiet thong tin node va runtime cho hypervisor da chon.
            </p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            onClick={onOpenRemoveDialog}
            disabled={!canRemove || loading || actionLoading || removeLoading}
            className={cn(
              "border-rose-300 text-rose-700 hover:bg-rose-50 hover:text-rose-700",
              isDark &&
                "border-rose-400/35 bg-rose-500/10 text-rose-200 hover:bg-rose-500/20",
            )}
          >
            <Trash2 className="mr-2 h-4 w-4" />
            {removeLoading ? "Removing..." : "Remove node"}
          </Button>
          {canRun && (
            <Button
              type="button"
              onClick={onRunNow}
              disabled={actionLoading || loading || removeLoading}
              className="bg-emerald-600 text-white hover:bg-emerald-500"
            >
              <Play className="mr-2 h-4 w-4" />
              {actionLoading ? "Running..." : "Run now"}
            </Button>
          )}
          {canStop && (
            <Button
              type="button"
              onClick={onStop}
              disabled={actionLoading || loading || removeLoading}
              className="bg-rose-600 text-white hover:bg-rose-500"
            >
              <Square className="mr-2 h-4 w-4" />
              {actionLoading ? "Stopping..." : "Stop"}
            </Button>
          )}
        </div>
      </div>
    </section>
  );
}
