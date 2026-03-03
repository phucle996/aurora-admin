import { ArrowLeft, Server, ShieldCheck } from "lucide-react";

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
import { Switch } from "@/components/ui/switch";
import { Textarea } from "@/components/ui/textarea";
import { cn } from "@/lib/utils";

export type TLSMode = "system_ca" | "custom_ca" | "mutual_tls";

type NewKvmFormMainSectionProps = {
  textPrimary: string;
  textMuted: string;
  panelClass: string;
  fieldClass: string;
  apiEndpoint: string;
  onApiEndpointChange: (value: string) => void;
  apiPort: string;
  onApiPortChange: (value: string) => void;
  providerMetadataRaw: string;
  onProviderMetadataChange: (value: string) => void;
  tlsEnabled: boolean;
  onTlsEnabledChange: (value: boolean) => void;
  tlsSkipVerify: boolean;
  onTlsSkipVerifyChange: (value: boolean) => void;
  tlsMode: TLSMode;
  onTlsModeChange: (value: TLSMode) => void;
  includeCA: boolean;
  includeMutualTLS: boolean;
  caCertPem: string;
  onCaCertPemChange: (value: string) => void;
  clientCertPem: string;
  onClientCertPemChange: (value: string) => void;
  clientKeyPem: string;
  onClientKeyPemChange: (value: string) => void;
  onBackToNode: () => void;
};

export function NewKvmFormMainSection({
  textPrimary,
  textMuted,
  panelClass,
  fieldClass,
  apiEndpoint,
  onApiEndpointChange,
  apiPort,
  onApiPortChange,
  providerMetadataRaw,
  onProviderMetadataChange,
  tlsEnabled,
  onTlsEnabledChange,
  tlsSkipVerify,
  onTlsSkipVerifyChange,
  tlsMode,
  onTlsModeChange,
  includeCA,
  includeMutualTLS,
  caCertPem,
  onCaCertPemChange,
  clientCertPem,
  onClientCertPemChange,
  clientKeyPem,
  onClientKeyPemChange,
  onBackToNode,
}: NewKvmFormMainSectionProps) {
  return (
    <section className="space-y-4">
      <Card className={cn("shadow-lg", panelClass)}>
        <CardHeader>
          <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
            <Server className="h-4 w-4 text-indigo-400" />
            Step 2: Provider Profile
          </CardTitle>
          <CardDescription className={textMuted}>
            Endpoint va TLS profile se duoc bootstrap tu Step 1 probe.
          </CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 sm:grid-cols-2">
          <div className="space-y-1.5">
            <Label className={cn("text-xs", textMuted)}>Libvirt endpoint</Label>
            <Input
              value={apiEndpoint}
              onChange={(event) => onApiEndpointChange(event.target.value)}
              placeholder="https://kvm-api.internal"
              className={fieldClass}
            />
          </div>
          <div className="space-y-1.5">
            <Label className={cn("text-xs", textMuted)}>Libvirt port</Label>
            <Input
              value={apiPort}
              onChange={(event) => onApiPortChange(event.target.value)}
              placeholder="16514"
              className={fieldClass}
            />
          </div>
          <div className="space-y-1.5 sm:col-span-2">
            <Label className={cn("text-xs", textMuted)}>
              Provider metadata (JSON)
            </Label>
            <Textarea
              value={providerMetadataRaw}
              onChange={(event) => onProviderMetadataChange(event.target.value)}
              className={cn("min-h-24", fieldClass)}
            />
          </div>
        </CardContent>
      </Card>

      <Card className={cn("shadow-lg", panelClass)}>
        <CardHeader>
          <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
            <ShieldCheck className="h-4 w-4 text-indigo-400" />
            SSL/TLS Settings
          </CardTitle>
          <CardDescription className={textMuted}>
            Tuy chon TLS cho ket noi provider.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="flex items-center justify-between rounded-lg border px-3 py-2">
              <Label className={cn("text-xs", textMuted)}>TLS enabled</Label>
              <Switch
                checked={tlsEnabled}
                onCheckedChange={onTlsEnabledChange}
                disabled
              />
            </div>
            <div className="flex items-center justify-between rounded-lg border px-3 py-2">
              <Label className={cn("text-xs", textMuted)}>Skip verify</Label>
              <Switch
                checked={tlsSkipVerify}
                onCheckedChange={onTlsSkipVerifyChange}
                disabled={!tlsEnabled}
              />
            </div>
          </div>

          <div className="space-y-1.5">
            <Label className={cn("text-xs", textMuted)}>TLS mode</Label>
            <select
              value={tlsMode}
              onChange={(event) =>
                onTlsModeChange(event.target.value as TLSMode)
              }
              className={cn(
                "h-10 w-full rounded-md border px-3 text-sm outline-none",
                fieldClass,
              )}
              disabled={!tlsEnabled}
            >
              <option value="system_ca">System CA (server verify)</option>
              <option value="custom_ca">Custom CA bundle</option>
              <option value="mutual_tls">Mutual TLS (client cert/key)</option>
            </select>
          </div>

          {includeCA && (
            <div className="space-y-1.5">
              <Label className={cn("text-xs", textMuted)}>CA cert PEM</Label>
              <Textarea
                value={caCertPem}
                onChange={(event) => onCaCertPemChange(event.target.value)}
                className={cn("min-h-28 font-mono text-xs", fieldClass)}
                placeholder="-----BEGIN CERTIFICATE-----"
              />
            </div>
          )}

          {includeMutualTLS && (
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1.5">
                <Label className={cn("text-xs", textMuted)}>
                  Client cert PEM
                </Label>
                <Textarea
                  value={clientCertPem}
                  onChange={(event) =>
                    onClientCertPemChange(event.target.value)
                  }
                  className={cn("min-h-28 font-mono text-xs", fieldClass)}
                  placeholder="-----BEGIN CERTIFICATE-----"
                />
              </div>
              <div className="space-y-1.5">
                <Label className={cn("text-xs", textMuted)}>
                  Client key PEM
                </Label>
                <Textarea
                  value={clientKeyPem}
                  onChange={(event) => onClientKeyPemChange(event.target.value)}
                  className={cn("min-h-28 font-mono text-xs", fieldClass)}
                  placeholder="-----BEGIN PRIVATE KEY-----"
                />
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      <div className="flex justify-start">
        <Button
          type="button"
          variant="outline"
          onClick={onBackToNode}
          className="gap-2"
        >
          <ArrowLeft className="h-4 w-4" />
          Back to Step 1
        </Button>
      </div>
    </section>
  );
}
