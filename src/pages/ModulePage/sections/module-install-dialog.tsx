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

import type { ModuleStatusCard } from "./module-page-types";

type ModuleInstallDialogProps = {
  open: boolean;
  installSubmitting: boolean;
  installTarget: ModuleStatusCard | null;
  appHost: string;
  appPort: string;
  sshHost: string;
  sshPort: string;
  sshUsername: string;
  sshPassword: string;
  sudoPassword: string;
  sshPrivateKey: string;
  sshHostKeyFingerprint: string;
  onOpenChange: (open: boolean) => void;
  onAppHostChange: (value: string) => void;
  onAppPortChange: (value: string) => void;
  onSshHostChange: (value: string) => void;
  onSshPortChange: (value: string) => void;
  onSshUsernameChange: (value: string) => void;
  onSshPasswordChange: (value: string) => void;
  onSudoPasswordChange: (value: string) => void;
  onSshPrivateKeyChange: (value: string) => void;
  onSshHostKeyFingerprintChange: (value: string) => void;
  onInstall: () => void;
};

export function ModuleInstallDialog({
  open,
  installSubmitting,
  installTarget,
  appHost,
  appPort,
  sshHost,
  sshPort,
  sshUsername,
  sshPassword,
  sudoPassword,
  sshPrivateKey,
  sshHostKeyFingerprint,
  onOpenChange,
  onAppHostChange,
  onAppPortChange,
  onSshHostChange,
  onSshPortChange,
  onSshUsernameChange,
  onSshPasswordChange,
  onSudoPasswordChange,
  onSshPrivateKeyChange,
  onSshHostKeyFingerprintChange,
  onInstall,
}: ModuleInstallDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Install Module</DialogTitle>
          <DialogDescription>
            {installTarget
              ? `Cai module ${installTarget.label} tren remote qua SSH.`
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
            <Label htmlFor="ssh-host">SSH Host / Service IP</Label>
            <Input
              id="ssh-host"
              value={sshHost}
              onChange={(event) => onSshHostChange(event.target.value)}
              placeholder="192.168.1.10"
            />
          </div>

          <div className="space-y-3">
            <p className="text-sm font-medium">SSH Remote Target</p>
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
              <Label htmlFor="sudo-password">Sudo Password (optional)</Label>
              <Input
                id="sudo-password"
                type="password"
                value={sudoPassword}
                onChange={(event) => onSudoPasswordChange(event.target.value)}
                placeholder="sudo password (khong nhap se khong auto sudo -S)"
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
            <div className="space-y-2">
              <Label htmlFor="ssh-host-key-fingerprint">
                SSH Host Key Fingerprint (SHA256)
              </Label>
              <Input
                id="ssh-host-key-fingerprint"
                value={sshHostKeyFingerprint}
                onChange={(event) =>
                  onSshHostKeyFingerprintChange(event.target.value)
                }
                placeholder="SHA256:..."
              />
            </div>
          </div>
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
