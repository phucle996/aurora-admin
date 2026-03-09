import { CheckCircle2, Loader2, XCircle } from "lucide-react";

import { SSHLiveTerminal } from "@/components/ssh-live-terminal";
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

type ModuleInstallLogDialogProps = {
  open: boolean;
  running: boolean;
  logs: string[];
  result: unknown | null;
  errorMessage: string;
  title?: string;
  description?: string;
  onOpenChange: (open: boolean) => void;
};

export function ModuleInstallLogDialog({
  open,
  running,
  logs,
  result,
  errorMessage,
  title,
  description,
  onOpenChange,
}: ModuleInstallLogDialogProps) {
  const isSuccess = !running && !errorMessage && Boolean(result);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        className={cn("w-[96vw] max-w-[96vw] sm:max-w-5xl")}
        showCloseButton={!running}
        onInteractOutside={(event) => event.preventDefault()}
        onPointerDownOutside={(event) => event.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            {running ? (
              <Loader2 className="h-4 w-4 animate-spin text-indigo-500" />
            ) : isSuccess ? (
              <CheckCircle2 className="h-4 w-4 text-emerald-500" />
            ) : (
              <XCircle className="h-4 w-4 text-red-500" />
            )}
            {title || "Module Install SSH Logs"}
          </DialogTitle>
          <DialogDescription>
            {description || "Log chi tiet theo tung stage de debug install flow."}
          </DialogDescription>
        </DialogHeader>

        <section className="space-y-3">
          <SSHLiveTerminal logs={logs} running={running} />
        </section>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={running}
          >
            Close
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
