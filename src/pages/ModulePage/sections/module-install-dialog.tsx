import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type { ModuleInstallAgent } from "@/hooks/module/use-module-install-api";

import type { ModuleStatusCard } from "./module-page-types";

type ModuleInstallDialogProps = {
  open: boolean;
  installSubmitting: boolean;
  installTarget: ModuleStatusCard | null;
  appHost: string;
  selectedAgentID: string;
  installAgents: ModuleInstallAgent[];
  installAgentsLoading: boolean;
  onOpenChange: (open: boolean) => void;
  onAppHostChange: (value: string) => void;
  onSelectedAgentIDChange: (value: string) => void;
  onInstall: () => void;
};

export function ModuleInstallDialog({
  open,
  installSubmitting,
  installTarget,
  appHost,
  selectedAgentID,
  installAgents,
  installAgentsLoading,
  onOpenChange,
  onAppHostChange,
  onSelectedAgentIDChange,
  onInstall,
}: ModuleInstallDialogProps) {
  const selectedAgent = installAgents.find((item) => item.agent_id === selectedAgentID) ?? null;
  const selectedAgentStatus = (selectedAgent?.status || "").toLowerCase();
  const selectedAgentConnected = selectedAgentStatus === "connected";

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>Install Module</DialogTitle>
          <DialogDescription>
            {installTarget
              ? `Cai module ${installTarget.label} tren remote qua agent.`
              : "Chon module de cai dat."}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="module-app-host">App Host</Label>
            <Input
              id="module-app-host"
              value={appHost}
              onChange={(event) => onAppHostChange(event.target.value)}
              placeholder="ums.aurora.local"
            />
            <p className="text-xs text-muted-foreground">
              App port duoc cap phat tu dong tren target host.
            </p>
          </div>

          <div className="space-y-2">
            <Label htmlFor="module-agent-id">Target Agent</Label>
            <Select
              value={selectedAgentID}
              onValueChange={onSelectedAgentIDChange}
              disabled={installSubmitting || installAgentsLoading}
            >
              <SelectTrigger id="module-agent-id" className="w-full">
                <SelectValue
                  placeholder={
                    installAgentsLoading
                      ? "Dang tai danh sach agent..."
                      : "Chon agent de install"
                  }
                />
              </SelectTrigger>
              <SelectContent>
                {installAgents.map((item) => (
                  <SelectItem key={item.agent_id} value={item.agent_id}>
                    {item.hostname || item.agent_id} | {item.agent_grpc_endpoint || "-"}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {selectedAgent ? (
            <div className="space-y-1 rounded-md border px-3 py-2 text-xs text-muted-foreground">
              <p className="inline-flex items-center gap-2">
                <span
                  className={selectedAgentConnected ? "h-2.5 w-2.5 rounded-full bg-emerald-500" : "h-2.5 w-2.5 rounded-full bg-rose-500"}
                />
                <span>status: {selectedAgent.status || "unknown"}</span>
              </p>
              <p>id: {selectedAgent.agent_id || "-"}</p>
              <p>hostname: {selectedAgent.hostname || "-"}</p>
              <p>grpc: {selectedAgent.agent_grpc_endpoint || "-"}</p>
            </div>
          ) : null}
        </div>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={installSubmitting}
          >
            Cancel
          </Button>
          <Button
            type="button"
            onClick={onInstall}
            disabled={installSubmitting}
          >
            {installSubmitting ? "Installing..." : "Install"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
