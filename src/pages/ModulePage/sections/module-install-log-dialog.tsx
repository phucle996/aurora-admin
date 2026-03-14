import { CheckCircle2, Loader2, XCircle } from "lucide-react";

import { SSHLiveTerminal } from "@/components/ssh-live-terminal";
import { Button } from "@/components/ui/button";
import type { ModuleInstallOperationSummary, ModuleInstallResult } from "@/hooks/module/use-module-install-api";
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
  operationSummary?: ModuleInstallOperationSummary | null;
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
  operationSummary,
  errorMessage,
  title,
  description,
  onOpenChange,
}: ModuleInstallLogDialogProps) {
  const isSuccess = !running && !errorMessage && Boolean(result);
  const installResult = (result && typeof result === "object" ? result : null) as ModuleInstallResult | null;

  const summaryRows = [
    { label: "Operation", value: operationSummary?.operation_id || installResult?.operation_id || "" },
    { label: "Module", value: operationSummary?.module || installResult?.module_name || "" },
    { label: "Agent", value: operationSummary?.agent_id || installResult?.agent_id || "" },
    { label: "Version", value: operationSummary?.version || installResult?.version || "" },
    { label: "Service", value: operationSummary?.service_name || installResult?.service_name || "" },
    { label: "Endpoint", value: operationSummary?.endpoint || installResult?.endpoint || "" },
    { label: "Health", value: operationSummary?.health || installResult?.health || "" },
    { label: "Stage", value: operationSummary?.last_stage || "" },
    { label: "Checksum", value: operationSummary?.artifact_checksum || installResult?.artifact_checksum || "" },
  ].filter((item) => item.value);

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
            {title || "Module Install Agent Logs"}
          </DialogTitle>
          <DialogDescription>
            {description || "Log chi tiet theo tung stage de debug install flow."}
          </DialogDescription>
        </DialogHeader>

        <section className="space-y-3">
          {summaryRows.length > 0 ? (
            <div className="grid gap-2 rounded-md border p-3 text-xs sm:grid-cols-2">
              {summaryRows.map((item) => (
                <div key={item.label} className="space-y-1">
                  <p className="text-muted-foreground">{item.label}</p>
                  <p className="break-all font-medium">{item.value}</p>
                </div>
              ))}
            </div>
          ) : null}
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
