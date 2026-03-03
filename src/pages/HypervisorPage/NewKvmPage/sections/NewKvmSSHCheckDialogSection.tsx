import { CheckCircle2, Loader2, XCircle } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { cn } from "@/lib/utils";
import { KvmTerminalStreamSection } from "@/components/terminal-stream";
import type { KvmNodeSSHCheckResult } from "@/pages/HypervisorPage/NewKvmPage/kvm-page.api";

type NewKvmSSHCheckDialogSectionProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  checking: boolean;
  logs: string[];
  result: KvmNodeSSHCheckResult | null;
  textPrimary: string;
  textMuted: string;
  panelClass: string;
};

export function NewKvmSSHCheckDialogSection({
  open,
  onOpenChange,
  checking,
  logs,
  result,
  textPrimary,
  textMuted,
  panelClass,
}: NewKvmSSHCheckDialogSectionProps) {
  const isSuccess = Boolean(result?.ok);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={cn("w-[96vw] max-w-[96vw] sm:max-w-5xl", panelClass)}
        showCloseButton={!checking}
        onInteractOutside={(event) => event.preventDefault()}
        onPointerDownOutside={(event) => event.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle className={cn("flex items-center gap-2", textPrimary)}>
            {checking ? (
              <Loader2 className="h-4 w-4 animate-spin text-indigo-400" />
            ) : isSuccess ? (
              <CheckCircle2 className="h-4 w-4 text-emerald-500" />
            ) : (
              <XCircle className="h-4 w-4 text-red-500" />
            )}
            SSH Connectivity Check
          </DialogTitle>
          <DialogDescription className={textMuted}>
            Terminal log chi tiet theo tung stage cua qua trinh probe node.
          </DialogDescription>
        </DialogHeader>

        <section className="space-y-3">
          <KvmTerminalStreamSection
            logs={logs}
            checking={checking}
            terminalLabel="ubuntu@kvm-probe:~"
            shellPrompt="phucle@kvm-node:~$"
            shellName="bash"
            emptyMessage="initializing probe session..."
          />
        </section>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={checking}
          >
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
