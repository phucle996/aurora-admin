import { useState } from "react";
import { useTheme } from "next-themes";
import { getErrorMessage } from "@/lib/api";
import {
  checkKvmNodeSSHStream,
  runKvmNodeNow,
  type KvmNodeSSHCheckResult,
} from "@/pages/HypervisorPage/NewKvmPage/kvm-page.api";
import { cn } from "@/lib/utils";

import { NewKvmHeaderSection } from "./sections/NewKvmHeaderSection";
import { NewKvmNodeStepSection } from "./sections/NewKvmNodeStepSection";
import { NewKvmOverviewSection } from "./sections/NewKvmOverviewSection";
import { NewKvmSSHCheckDialogSection } from "./sections/NewKvmSSHCheckDialogSection";

function optionalText(value: string): string | undefined {
  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : undefined;
}

function parseOptionalPort(raw: string): number | undefined {
  const trimmed = raw.trim();
  if (!trimmed) {
    return undefined;
  }
  const parsed = Number.parseInt(trimmed, 10);
  if (!Number.isInteger(parsed) || parsed < 1 || parsed > 65535) {
    throw new Error("Port must be between 1 and 65535");
  }
  return parsed;
}

function parseMetadata(
  raw: string,
  fieldLabel: string,
): Record<string, unknown> | undefined {
  const trimmed = raw.trim();
  if (!trimmed) {
    return undefined;
  }
  const parsed = JSON.parse(trimmed) as unknown;
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error(`${fieldLabel} must be a JSON object`);
  }
  return parsed as Record<string, unknown>;
}

export default function NewKvmPage() {
  const { resolvedTheme } = useTheme();

  const [nodeName, setNodeName] = useState("");
  const [host, setHost] = useState("");
  const [zone, setZone] = useState("");
  const [sshPort, setSshPort] = useState("22");
  const nodeSSHEnabled = true;
  const [nodeSSHUsername, setNodeSSHUsername] = useState("root");
  const [nodeSSHAuthMode, setNodeSSHAuthMode] = useState<"password" | "key">(
    "password",
  );
  const [nodeSSHPassword, setNodeSSHPassword] = useState("");
  const [nodeSSHPrivateKey, setNodeSSHPrivateKey] = useState("");
  const [nodeMetadataRaw, setNodeMetadataRaw] = useState("");

  const [checkingSSH, setCheckingSSH] = useState(false);
  const [sshDialogOpen, setSshDialogOpen] = useState(false);
  const [sshCheckLogs, setSshCheckLogs] = useState<string[]>([]);
  const [sshCheckResult, setSshCheckResult] =
    useState<KvmNodeSSHCheckResult | null>(null);
  const [runNowLoading, setRunNowLoading] = useState(false);
  const [runNowMessage, setRunNowMessage] = useState<string | null>(null);
  const [isNodeRunning, setIsNodeRunning] = useState(false);
  const [showNodeStep, setShowNodeStep] = useState(true);

  const isDark = resolvedTheme !== "light";
  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";
  const panelClass = isDark
    ? "border-white/10 bg-slate-950/60"
    : "border-black/10 bg-white/85";
  const fieldClass = cn(isDark && "border-white/10 bg-white/5");

  const activeNodeCredential =
    nodeSSHAuthMode === "password"
      ? nodeSSHPassword.trim()
      : nodeSSHPrivateKey.trim();
  const isNodeBaseValid = nodeName.trim().length > 0 && host.trim().length > 0;

  const handleCheckSSH = async () => {
    const appendLog = (line: string, timestamp?: string) => {
      const displayTime = timestamp
        ? new Date(timestamp).toLocaleTimeString()
        : new Date().toLocaleTimeString();
      setSshCheckLogs((prev) => [...prev, `[${displayTime}] ${line}`]);
    };

    if (!nodeName.trim() || !host.trim()) {
      setSshDialogOpen(true);
      setSshCheckResult(null);
      setSshCheckLogs([
        `[${new Date().toLocaleTimeString()}] Node name and host are required.`,
      ]);
      return;
    }
    if (!nodeSSHUsername.trim()) {
      setSshDialogOpen(true);
      setSshCheckResult(null);
      setSshCheckLogs([
        `[${new Date().toLocaleTimeString()}] SSH username is required.`,
      ]);
      return;
    }
    if (!activeNodeCredential) {
      setSshDialogOpen(true);
      setSshCheckResult(null);
      setSshCheckLogs([
        `[${new Date().toLocaleTimeString()}] Missing SSH credential for selected auth mode.`,
      ]);
      return;
    }

    setCheckingSSH(true);
    setSshCheckResult(null);
    setRunNowMessage(null);
    setIsNodeRunning(false);
    setShowNodeStep(true);
    setSshDialogOpen(true);
    const authSummary =
      nodeSSHAuthMode === "password"
        ? `password(length=${nodeSSHPassword.length})`
        : `private_key(length=${nodeSSHPrivateKey.length})`;
    setSshCheckLogs([
      `[${new Date().toLocaleTimeString()}] [ui] start kvm node probe`,
      `[${new Date().toLocaleTimeString()}] [ui] profile host=${host.trim()} port=${sshPort.trim() || "22"} user=${nodeSSHUsername.trim()} auth=${authSummary}`,
      `[${new Date().toLocaleTimeString()}] [ui] waiting for backend probe response...`,
    ]);

    try {
      const sshPortValue = parseOptionalPort(sshPort) ?? 22;
      appendLog("[probe] sending probe request to backend");
      const result = await checkKvmNodeSSHStream(
        {
          node_name: nodeName.trim(),
          host: host.trim(),
          zone: optionalText(zone),
          ssh_port: sshPortValue,
          ssh_username: nodeSSHUsername.trim(),
          ssh_password:
            nodeSSHAuthMode === "password"
              ? optionalText(nodeSSHPassword)
              : undefined,
          ssh_private_key:
            nodeSSHAuthMode === "key"
              ? optionalText(nodeSSHPrivateKey)
              : undefined,
          node_metadata: {
            ...(parseMetadata(nodeMetadataRaw, "Node metadata") ?? {}),
            ssh_username: nodeSSHUsername.trim(),
            ssh_enabled: nodeSSHEnabled,
            ssh_auth_mode: nodeSSHAuthMode,
          },
          timeout_seconds: 8,
          install_if_missing: true,
        },
        {
          onLog: (stage, message) => {
            appendLog(`[${stage}] ${message}`);
          },
        },
      );

      setSshCheckResult(result);
      appendLog(
        `[probe] cpu_cores=${result.capability.cpuCores} ram_mb=${result.capability.ramMb} disk_free_gb=${result.capability.diskFreeGb}`,
      );
      appendLog(
        `[probe] kvm_module=${result.capability.kvmModule} libvirt_running=${result.capability.libvirtRunning} virsh_connect=${result.capability.virshConnect}`,
      );
      appendLog(
        `[probe] storage_pools=${result.capability.storagePools.join(",") || "-"} networks=${result.capability.networks.join(",") || "-"}`,
      );
      if (result.installAttempted) {
        appendLog(
          "[probe] install script was triggered because kvm was incomplete",
        );
      }

      if (result.ok) {
        if (!result.saved || !result.savedNodeId) {
          throw new Error("Probe succeeded but save node failed.");
        }

        if (result.agent) {
          appendLog(
            `[probe] agent profile endpoint=${result.agent.agentEndpoint}:${result.agent.agentPort} protocol=${result.agent.agentProtocol}`,
          );
        }
        appendLog(
          `[db] node saved node_id=${result.savedNodeId}`,
        );

        appendLog(`Probe success (${result.latencyMs} ms).`);
        return;
      } else {
        const message = result.error || "Probe failed.";
        appendLog(`[probe][error] ${message.split(/\r?\n/)[0]}`);
      }
    } catch (error) {
      const message =
        error instanceof Error
          ? error.message
          : typeof error === "string"
            ? error
            : "Cannot check SSH connection";
      appendLog(`[ui][error] ssh check request error: ${message}`);
    } finally {
      setCheckingSSH(false);
    }
  };

  const handleRunNow = async () => {
    const nodeId = sshCheckResult?.savedNodeId;
    if (!nodeId) {
      setRunNowMessage("Node is not saved yet.");
      return;
    }

    setRunNowLoading(true);
    setRunNowMessage(null);
    try {
      const result = await runKvmNodeNow(nodeId);
      if (result.ok) {
        setIsNodeRunning(true);
        setRunNowMessage("Runtime connected successfully.");
      } else {
        setIsNodeRunning(false);
        setRunNowMessage(result.error || "Run now failed.");
      }

    } catch (error) {
      setIsNodeRunning(false);
      setRunNowMessage(
        getErrorMessage(error, "Cannot run runtime connect now"),
      );
    } finally {
      setRunNowLoading(false);
    }
  };

  return (
    <main className="space-y-4 py-3 lg:py-1">
      <NewKvmHeaderSection
        isDark={isDark}
        textPrimary={textPrimary}
        textMuted={textMuted}
      />

      <div className="grid gap-4">
        {showNodeStep && (
          <NewKvmNodeStepSection
            textPrimary={textPrimary}
            textMuted={textMuted}
            panelClass={panelClass}
            fieldClass={fieldClass}
            nodeName={nodeName}
            onNodeNameChange={setNodeName}
            host={host}
            onHostChange={setHost}
            zone={zone}
            onZoneChange={setZone}
            sshPort={sshPort}
            onSshPortChange={setSshPort}
            sshUsername={nodeSSHUsername}
            onSshUsernameChange={setNodeSSHUsername}
            sshAuthMode={nodeSSHAuthMode}
            onSshAuthModeChange={setNodeSSHAuthMode}
            sshPassword={nodeSSHPassword}
            onSshPasswordChange={setNodeSSHPassword}
            sshPrivateKey={nodeSSHPrivateKey}
            onSshPrivateKeyChange={setNodeSSHPrivateKey}
            nodeMetadataRaw={nodeMetadataRaw}
            onNodeMetadataChange={setNodeMetadataRaw}
            checkingSSH={checkingSSH}
            onCheckSSH={handleCheckSSH}
            canProbe={isNodeBaseValid}
          />
        )}
        <NewKvmOverviewSection
          textPrimary={textPrimary}
          textMuted={textMuted}
          panelClass={panelClass}
          nodeName={nodeName.trim()}
          host={host.trim()}
          zone={zone.trim()}
          result={sshCheckResult}
          onRunNow={handleRunNow}
          runNowLoading={runNowLoading}
          runNowMessage={runNowMessage}
          isRunning={isNodeRunning}
        />
      </div>

      <NewKvmSSHCheckDialogSection
        open={sshDialogOpen}
        onOpenChange={(open) => {
          setSshDialogOpen(open);
          if (
            !open &&
            sshCheckResult?.ok &&
            sshCheckResult.saved &&
            Boolean(sshCheckResult.savedNodeId)
          ) {
            setShowNodeStep(false);
          }
        }}
        checking={checkingSSH}
        logs={sshCheckLogs}
        result={sshCheckResult}
        textPrimary={textPrimary}
        textMuted={textMuted}
        panelClass={panelClass}
      />
    </main>
  );
}
