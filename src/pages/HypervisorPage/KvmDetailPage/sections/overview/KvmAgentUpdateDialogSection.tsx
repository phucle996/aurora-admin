import { ExternalLink } from "lucide-react";

import { Button } from "@/components/ui/button";
import type { KvmAgentUpdateCheckResult } from "@/hooks/kvm-detail/use-kvm-agent-api";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";

type KvmAgentUpdateDialogSectionProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  checking: boolean;
  updating: boolean;
  result: KvmAgentUpdateCheckResult | null;
  isDark: boolean;
  onConfirmUpdate: () => void;
};

export function KvmAgentUpdateDialogSection({
  open,
  onOpenChange,
  checking,
  updating,
  result,
  isDark,
  onConfirmUpdate,
}: KvmAgentUpdateDialogSectionProps) {
  const hasUpdate = Boolean(result?.hasUpdate);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={cn("sm:max-w-lg", isDark && "border-white/15 bg-slate-950/95")}
        showCloseButton={!checking && !updating}
        onInteractOutside={(event) => {
          if (checking || updating) {
            event.preventDefault();
          }
        }}
      >
        <DialogHeader>
          <DialogTitle>Agent update check</DialogTitle>
          <DialogDescription>
            Kiem tra version hien tai va ban release moi nhat cua agent.
          </DialogDescription>
        </DialogHeader>

        {!result ? (
          <div className="rounded-lg border border-black/10 bg-black/[0.03] p-3 text-sm dark:border-white/10 dark:bg-white/[0.03]">
            Dang tai thong tin version...
          </div>
        ) : (
          <div className="space-y-3">
            <div className="rounded-lg border border-black/10 bg-black/[0.03] p-3 dark:border-white/10 dark:bg-white/[0.03]">
              <p className="text-xs text-slate-500 dark:text-slate-400">Current version</p>
              <p className="text-sm font-semibold">{result.currentVersion || "-"}</p>
            </div>
            <div className="rounded-lg border border-black/10 bg-black/[0.03] p-3 dark:border-white/10 dark:bg-white/[0.03]">
              <p className="text-xs text-slate-500 dark:text-slate-400">Latest version</p>
              <p className="text-sm font-semibold">{result.latestVersion || "-"}</p>
            </div>
            <div
              className={cn(
                "rounded-lg border p-3 text-sm",
                hasUpdate
                  ? "border-emerald-300/60 bg-emerald-50 text-emerald-700 dark:border-emerald-500/30 dark:bg-emerald-500/10 dark:text-emerald-300"
                  : "border-black/10 bg-black/[0.03] text-slate-700 dark:border-white/10 dark:bg-white/[0.03] dark:text-slate-300",
              )}
            >
              {hasUpdate ? "Da co phien ban moi. Ban co the update ngay." : "Agent dang la phien ban moi nhat."}
            </div>
            {result.releaseURL && (
              <a
                href={result.releaseURL}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1 text-sm text-indigo-600 hover:underline dark:text-indigo-300"
              >
                Open release
                <ExternalLink className="h-4 w-4" />
              </a>
            )}
          </div>
        )}

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={checking || updating}
          >
            Close
          </Button>
          <Button
            type="button"
            onClick={onConfirmUpdate}
            disabled={!hasUpdate || checking || updating}
          >
            {updating ? "Updating..." : "Update now"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
