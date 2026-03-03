import { KeyRound, Loader2, Lock, Server, Waypoints } from "lucide-react";

import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";

type NewKvmNodeStepSectionProps = {
  textPrimary: string;
  textMuted: string;
  panelClass: string;
  fieldClass: string;
  nodeName: string;
  onNodeNameChange: (value: string) => void;
  host: string;
  onHostChange: (value: string) => void;
  zone: string;
  onZoneChange: (value: string) => void;
  sshPort: string;
  onSshPortChange: (value: string) => void;
  sshUsername: string;
  onSshUsernameChange: (value: string) => void;
  sshAuthMode: "password" | "key";
  onSshAuthModeChange: (value: "password" | "key") => void;
  sshPassword: string;
  onSshPasswordChange: (value: string) => void;
  sshPrivateKey: string;
  onSshPrivateKeyChange: (value: string) => void;
  nodeMetadataRaw: string;
  onNodeMetadataChange: (value: string) => void;
  checkingSSH: boolean;
  onCheckSSH: () => void;
  canProbe: boolean;
};

export function NewKvmNodeStepSection({
  textPrimary,
  textMuted,
  panelClass,
  fieldClass,
  nodeName,
  onNodeNameChange,
  host,
  onHostChange,
  zone,
  onZoneChange,
  sshPort,
  onSshPortChange,
  sshUsername,
  onSshUsernameChange,
  sshAuthMode,
  onSshAuthModeChange,
  sshPassword,
  onSshPasswordChange,
  sshPrivateKey,
  onSshPrivateKeyChange,
  nodeMetadataRaw,
  onNodeMetadataChange,
  checkingSSH,
  onCheckSSH,
  canProbe,
}: NewKvmNodeStepSectionProps) {
  return (
    <Card className={cn("shadow-lg", panelClass)}>
      <CardHeader>
        <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
          <Waypoints className="h-4 w-4 text-indigo-400" />
          Node Registration
        </CardTitle>
        <CardDescription className={textMuted}>
          Probe sẽ tự động setup kvm trên node nên hãy ưu tiên dùng node mới tạo
          để OS sạch
        </CardDescription>
      </CardHeader>
      <CardContent className="grid gap-4">
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-1.5">
            <Label className={cn("text-xs", textMuted)}>Node name</Label>
            <Input
              value={nodeName}
              onChange={(event) => onNodeNameChange(event.target.value)}
              placeholder="kvm-node-01"
              className={fieldClass}
            />
          </div>
          <div className="space-y-1.5">
            <Label className={cn("text-xs", textMuted)}>Host</Label>
            <Input
              value={host}
              onChange={(event) => onHostChange(event.target.value)}
              placeholder="10.10.10.21"
              className={fieldClass}
            />
          </div>
          <div className="space-y-1.5">
            <Label className={cn("text-xs", textMuted)}>Zone</Label>
            <Input
              value={zone}
              onChange={(event) => onZoneChange(event.target.value)}
              placeholder="ap-southeast-1a"
              className={fieldClass}
            />
          </div>
          <div className="space-y-1.5">
            <Label className={cn("text-xs", textMuted)}>SSH port</Label>
            <Input
              value={sshPort}
              onChange={(event) => onSshPortChange(event.target.value)}
              placeholder="22"
              className={fieldClass}
            />
          </div>
        </div>

        <div className="grid gap-3">
          <div className="space-y-1.5">
            <Label className={cn("text-xs", textMuted)}>SSH username</Label>
            <Input
              value={sshUsername}
              onChange={(event) => onSshUsernameChange(event.target.value)}
              placeholder="root"
              className={fieldClass}
            />
          </div>

          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant={sshAuthMode === "password" ? "default" : "outline"}
              onClick={() => onSshAuthModeChange("password")}
              className={cn(
                sshAuthMode === "password"
                  ? "bg-indigo-500 text-white hover:bg-indigo-400"
                  : "",
              )}
            >
              <Lock className="h-4 w-4" />
              User/Pass
            </Button>
            <Button
              type="button"
              variant={sshAuthMode === "key" ? "default" : "outline"}
              onClick={() => onSshAuthModeChange("key")}
              className={cn(
                sshAuthMode === "key"
                  ? "bg-indigo-500 text-white hover:bg-indigo-400"
                  : "",
              )}
            >
              <KeyRound className="h-4 w-4" />
              SSH Key
            </Button>
          </div>

          {sshAuthMode === "password" ? (
            <div className="space-y-1.5">
              <Label className={cn("text-xs", textMuted)}>SSH password</Label>
              <Input
                type="password"
                value={sshPassword}
                onChange={(event) => onSshPasswordChange(event.target.value)}
                placeholder="node ssh password"
                className={fieldClass}
              />
            </div>
          ) : (
            <div className="space-y-1.5">
              <Label className={cn("text-xs", textMuted)}>
                SSH private key
              </Label>
              <Textarea
                value={sshPrivateKey}
                onChange={(event) => onSshPrivateKeyChange(event.target.value)}
                className={cn("min-h-24 font-mono text-xs", fieldClass)}
                placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
              />
            </div>
          )}
        </div>

        <div className="space-y-1.5">
          <Label className={cn("text-xs", textMuted)}>
            Node metadata (JSON)
          </Label>
          <Textarea
            value={nodeMetadataRaw}
            onChange={(event) => onNodeMetadataChange(event.target.value)}
            className={cn("min-h-24", fieldClass)}
          />
        </div>

        <div className="flex justify-end">
          <Button
            type="button"
            onClick={onCheckSSH}
            disabled={!canProbe || checkingSSH}
            className="bg-indigo-500 text-white hover:bg-indigo-400"
          >
            {checkingSSH ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                Probing...
              </>
            ) : (
              <>
                <Server className="mr-2 h-4 w-4" />
                Setup
              </>
            )}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
