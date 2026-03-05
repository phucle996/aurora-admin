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
import { Textarea } from "@/components/ui/textarea";
import type { ModuleInstallScope } from "@/hooks/module/use-module-install-api";

import type { ModuleStatusCard } from "./module-page-types";

type ModuleInstallDialogProps = {
  open: boolean;
  installSubmitting: boolean;
  installTarget: ModuleStatusCard | null;
  installScope: ModuleInstallScope;
  appHost: string;
  endpoint: string;
  installCommand: string;
  sshHost: string;
  sshPort: string;
  sshUsername: string;
  sshPassword: string;
  sshPrivateKey: string;
  onOpenChange: (open: boolean) => void;
  onInstallScopeChange: (scope: ModuleInstallScope) => void;
  onAppHostChange: (value: string) => void;
  onEndpointChange: (value: string) => void;
  onInstallCommandChange: (value: string) => void;
  onSshHostChange: (value: string) => void;
  onSshPortChange: (value: string) => void;
  onSshUsernameChange: (value: string) => void;
  onSshPasswordChange: (value: string) => void;
  onSshPrivateKeyChange: (value: string) => void;
  onInstall: () => void;
};

export function ModuleInstallDialog({
  open,
  installSubmitting,
  installTarget,
  installScope,
  appHost,
  endpoint,
  installCommand,
  sshHost,
  sshPort,
  sshUsername,
  sshPassword,
  sshPrivateKey,
  onOpenChange,
  onInstallScopeChange,
  onAppHostChange,
  onEndpointChange,
  onInstallCommandChange,
  onSshHostChange,
  onSshPortChange,
  onSshUsernameChange,
  onSshPasswordChange,
  onSshPrivateKeyChange,
  onInstall,
}: ModuleInstallDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Install Module</DialogTitle>
          <DialogDescription>
            {installTarget
              ? `Cai module ${installTarget.label} tren local hoac remote qua SSH.`
              : "Chon module de cai dat."}
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-2">
            <Button
              type="button"
              variant={installScope === "local" ? "default" : "outline"}
              onClick={() => onInstallScopeChange("local")}
            >
              Install Local
            </Button>
            <Button
              type="button"
              variant={installScope === "remote" ? "default" : "outline"}
              onClick={() => onInstallScopeChange("remote")}
            >
              Install Remote
            </Button>
          </div>

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
            <Label htmlFor="module-endpoint">Endpoint</Label>
            <Input
              id="module-endpoint"
              value={endpoint}
              onChange={(event) => onEndpointChange(event.target.value)}
              placeholder="vm.aurora.local:3001"
            />
          </div>

          <div className="space-y-2">
            <Label htmlFor="module-command">Install Command (optional)</Label>
            <Textarea
              id="module-command"
              value={installCommand}
              onChange={(event) => onInstallCommandChange(event.target.value)}
              placeholder="bash /opt/aurora/install-vm.sh"
            />
          </div>

          {installScope === "remote" ? (
            <div className="space-y-3 rounded-md border p-3">
              <p className="text-sm font-medium">SSH Remote Target</p>
              <div className="space-y-2">
                <Label htmlFor="ssh-host">SSH Host</Label>
                <Input
                  id="ssh-host"
                  value={sshHost}
                  onChange={(event) => onSshHostChange(event.target.value)}
                  placeholder="192.168.1.10"
                />
              </div>
              <div className="grid grid-cols-2 gap-2">
                <div className="space-y-2">
                  <Label htmlFor="ssh-port">SSH Port</Label>
                  <Input
                    id="ssh-port"
                    value={sshPort}
                    onChange={(event) => onSshPortChange(event.target.value)}
                    placeholder="22"
                  />
                </div>
                <div className="space-y-2">
                  <Label htmlFor="ssh-username">SSH Username</Label>
                  <Input
                    id="ssh-username"
                    value={sshUsername}
                    onChange={(event) => onSshUsernameChange(event.target.value)}
                    placeholder="root"
                  />
                </div>
              </div>
              <div className="space-y-2">
                <Label htmlFor="ssh-password">SSH Password</Label>
                <Input
                  id="ssh-password"
                  type="password"
                  value={sshPassword}
                  onChange={(event) => onSshPasswordChange(event.target.value)}
                  placeholder="password"
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="ssh-private-key">SSH Private Key</Label>
                <Textarea
                  id="ssh-private-key"
                  value={sshPrivateKey}
                  onChange={(event) => onSshPrivateKeyChange(event.target.value)}
                  placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
                />
              </div>
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
