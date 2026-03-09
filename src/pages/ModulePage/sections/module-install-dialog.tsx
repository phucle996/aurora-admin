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
  appPort: string;
  selectedAgentID: string;
  installAgents: ModuleInstallAgent[];
  installAgentsLoading: boolean;
  onOpenChange: (open: boolean) => void;
  onAppHostChange: (value: string) => void;
  onAppPortChange: (value: string) => void;
  onSelectedAgentIDChange: (value: string) => void;
  onInstall: () => void;
};

export function ModuleInstallDialog({
  open,
  installSubmitting,
  installTarget,
  appHost,
  appPort,
  selectedAgentID,
  installAgents,
  installAgentsLoading,
  onOpenChange,
  onAppHostChange,
  onAppPortChange,
  onSelectedAgentIDChange,
  onInstall,
}: ModuleInstallDialogProps) {
  const selectedAgent = installAgents.find((item) => item.agent_id === selectedAgentID) ?? null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Install Module</DialogTitle>
          <DialogDescription>
            {installTarget
              ? `Cai module ${installTarget.label} tren remote qua agent.`
              : "Chon module de cai dat."}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-2">
            <div className="space-y-2">
              <Label htmlFor="module-app-host">App Host</Label>
              <Input
                id="module-app-host"
                value={appHost}
                onChange={(event) => onAppHostChange(event.target.value)}
                placeholder="vm.aurora.local"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="module-app-port">App Port (optional)</Label>
              <Input
                id="module-app-port"
                value={appPort}
                onChange={(event) => onAppPortChange(event.target.value)}
                placeholder="De trong de random"
                inputMode="numeric"
              />
            </div>
          </div>

          <div className="space-y-2">
            <Label htmlFor="module-agent-id">Target Agent</Label>
            <Select
              value={selectedAgentID}
              onValueChange={onSelectedAgentIDChange}
              disabled={installSubmitting || installAgentsLoading}
            >
              <SelectTrigger id="module-agent-id">
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
                    {item.agent_id} | {item.hostname || item.host || "-"} |{" "}
                    {item.status || "-"}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {selectedAgent ? (
            <div className="space-y-1 rounded-md border px-3 py-2 text-xs text-muted-foreground">
              <p>host: {selectedAgent.host || selectedAgent.ip_address || "-"}</p>
              <p>user: {selectedAgent.username || "aurora"}</p>
              <p>port: {selectedAgent.port || 22}</p>
              <p>status: {selectedAgent.status || "-"}</p>
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
