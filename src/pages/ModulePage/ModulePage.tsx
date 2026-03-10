import { useEffect, useMemo, useRef, useState } from "react";
import { useTheme } from "next-themes";
import { toast } from "sonner";

import {
  getAgentInstallBootstrapMetadata,
  installModuleStream,
  listModuleInstallAgents,
  rotateAgentBootstrapToken,
  reinstallModuleCertStream,
  type AgentInstallBootstrapMetadata,
  type ModuleInstallAgent,
  type ModuleInstallResult,
  type ModuleInstallScope,
  type ModuleReinstallCertResult,
} from "@/hooks/module/use-module-install-api";
import { useEnabledModules } from "@/state/enabled-modules-context";

import { ModuleInstallDialog } from "./sections/module-install-dialog";
import { ModuleInstallLogDialog } from "./sections/module-install-log-dialog";
import { buildModuleStatusCards } from "./sections/module-page-mapper";
import { ModulePageContent } from "./sections/module-page-content";
import type { ModuleStatusCard } from "./sections/module-page-types";

const installLogShellPrompt = "aurora@installer:~$";

function formatInstallLogLine(line: string): string {
  const timeValue = new Date().toLocaleTimeString("en-GB", { hour12: false });
  return `${timeValue} ${installLogShellPrompt} ${line}`;
}

export default function ModulePage() {
  const { resolvedTheme } = useTheme();
  const { items, status, error, lastFetchedAt, refreshModules } =
    useEnabledModules();
  const syncedOnMountRef = useRef(false);

  const [searchQuery, setSearchQuery] = useState("");
  const [agentSearchQuery, setAgentSearchQuery] = useState("");
  const [agentBootstrapToken, setAgentBootstrapToken] = useState("");
  const [agentBootstrapMeta, setAgentBootstrapMeta] = useState<AgentInstallBootstrapMetadata | null>(null);

  const [installDialogOpen, setInstallDialogOpen] = useState(false);
  const [installSubmitting, setInstallSubmitting] = useState(false);
  const [installTarget, setInstallTarget] = useState<ModuleStatusCard | null>(
    null,
  );
  const [installScope] = useState<ModuleInstallScope>("remote");
  const [appHost, setAppHost] = useState("");
  const [selectedAgentID, setSelectedAgentID] = useState("");
  const [installAgents, setInstallAgents] = useState<ModuleInstallAgent[]>([]);
  const [installAgentsLoading, setInstallAgentsLoading] = useState(false);
  const [installLogDialogOpen, setInstallLogDialogOpen] = useState(false);
  const [installLogs, setInstallLogs] = useState<string[]>([]);
  const [installRunning, setInstallRunning] = useState(false);
  const [installResult, setInstallResult] = useState<
    ModuleInstallResult | ModuleReinstallCertResult | null
  >(null);
  const [installError, setInstallError] = useState("");
  const [logDialogTitle, setLogDialogTitle] = useState(
    "Module Install Logs",
  );
  const [logDialogDescription, setLogDialogDescription] = useState(
    "Log chi tiet theo tung stage de debug install flow.",
  );

  const isDark = resolvedTheme !== "light";
  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";

  const cards = useMemo(() => buildModuleStatusCards(items), [items]);

  const filteredCards = useMemo(() => {
    return cards.filter((item) => {
      const q = searchQuery.trim().toLowerCase();
      if (!q) {
        return true;
      }
      const text =
        `${item.label} ${item.sourceName} ${item.endpoint} ${item.runtimeStatus}`.toLowerCase();
      return text.includes(q);
    });
  }, [cards, searchQuery]);

  const agentInstallScript = useMemo(() => {
    const grpcEndpoint = agentBootstrapMeta?.admin_grpc_endpoint.trim() || "";
    const serverName = agentBootstrapMeta?.admin_server_name.trim() || "";
    const tokenSegment = agentBootstrapToken.trim()
      ? ` --bootstrap-token '${agentBootstrapToken.trim()}'`
      : "";
    const endpointSegment = grpcEndpoint
      ? ` --admin-grpc-endpoint ${grpcEndpoint}`
      : "";
    const serverNameSegment = serverName
      ? ` --admin-server-name ${serverName}`
      : "";
    if (!endpointSegment) {
      return [
        "curl -fsSL https://raw.githubusercontent.com/phucle996/aurora-agent/main/scripts/install.sh -o install.sh",
        "chmod +x install.sh",
        "# missing admin grpc direct endpoint metadata; refresh Runtime Module page",
      ].join("\n");
    }
    return [
      "curl -fsSL https://raw.githubusercontent.com/phucle996/aurora-agent/main/scripts/install.sh -o install.sh",
      "chmod +x install.sh",
      `sudo ./install.sh${endpointSegment}${serverNameSegment}${tokenSegment}`,
    ].join("\n");
  }, [agentBootstrapMeta, agentBootstrapToken]);

  const refreshAgentBootstrapMetadata = (silent = false) => {
    return getAgentInstallBootstrapMetadata()
      .then((meta) => {
        setAgentBootstrapMeta(meta);
      })
      .catch((err) => {
        const message =
          err instanceof Error && err.message.trim()
            ? err.message
            : "Khong the tai bootstrap metadata";
        if (!silent) {
          toast.error(message);
        }
      });
  };

  const refreshInstallAgents = (silent = false) => {
    setInstallAgentsLoading(true);
    return listModuleInstallAgents()
      .then((items) => {
        setInstallAgents(items);
        if (!selectedAgentID.trim()) {
          setSelectedAgentID(items[0]?.agent_id || "");
          return;
        }
        const stillExists = items.some((agent) => agent.agent_id === selectedAgentID);
        if (!stillExists) {
          setSelectedAgentID(items[0]?.agent_id || "");
        }
      })
      .catch((err) => {
        const message =
          err instanceof Error && err.message.trim()
            ? err.message
            : "Khong the tai danh sach agent";
        if (!silent) {
          toast.error(message);
        }
      })
      .finally(() => {
        setInstallAgentsLoading(false);
      });
  };

  useEffect(() => {
    if (syncedOnMountRef.current) {
      return;
    }
    syncedOnMountRef.current = true;
    void Promise.all([
      refreshModules({ force: true }),
      refreshInstallAgents(true),
      refreshAgentBootstrapMetadata(true),
    ]).catch(() => {
      toast.error("Khong the tai du lieu runtime");
    });
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [refreshModules]);

  const handleRefresh = () => {
    void Promise.all([
      refreshModules({ force: true }),
      refreshInstallAgents(true),
      refreshAgentBootstrapMetadata(true),
    ])
      .then(() => {
        toast.success("Da cap nhat runtime + agent");
      })
      .catch(() => {
        toast.error("Khong the cap nhat du lieu runtime");
      });
  };

  const handleRotateAgentBootstrapToken = () => {
    return rotateAgentBootstrapToken()
      .then((result) => {
        const token = result.token.trim();
        if (!token) {
          throw new Error("bootstrap token response is empty");
        }
        setAgentBootstrapToken(token);
        if (!agentBootstrapMeta?.admin_grpc_endpoint.trim()) {
          void refreshAgentBootstrapMetadata(true);
        }
        toast.success("Da tao bootstrap token moi");
      })
      .catch((err) => {
        const message =
          err instanceof Error && err.message.trim()
            ? err.message
            : "Khong the tao bootstrap token";
        toast.error(message);
        throw err;
      });
  };

  const openInstallDialog = (item: ModuleStatusCard) => {
    const moduleID = item.sourceName || item.moduleKey;
    const defaultHost = `${moduleID}.aurora.local`;

    setInstallTarget(item);
    setAppHost(defaultHost);
    if (!selectedAgentID.trim() && installAgents.length > 0) {
      setSelectedAgentID(installAgents[0].agent_id);
    }
    setInstallDialogOpen(true);
    if (installAgents.length === 0 && !installAgentsLoading) {
      void refreshInstallAgents(false);
    }
  };

  const appendInstallLog = (line: string) => {
    setInstallLogs((prev) => [...prev, formatInstallLogLine(line)]);
  };

  const handleInstall = async () => {
    if (!installTarget) {
      return;
    }

    const moduleName = installTarget.sourceName || installTarget.moduleKey;
    const normalizedAppHost = appHost.trim();

    if (!normalizedAppHost) {
      toast.error("app host la bat buoc");
      return;
    }

    if (!selectedAgentID.trim()) {
      toast.error("Hay chon agent de install");
      return;
    }

    setInstallSubmitting(true);
    setInstallDialogOpen(false);
    setInstallLogDialogOpen(true);
    setLogDialogTitle("Module Install Logs");
    setLogDialogDescription("Log chi tiet theo tung stage de debug install flow.");
    setInstallRunning(true);
    setInstallResult(null);
    setInstallError("");
    setInstallLogs([]);

    try {
      const result = await installModuleStream(
        {
          module_name: moduleName,
          scope: installScope,
          install_runtime: "linux",
          agent_id: selectedAgentID.trim(),
          app_host: normalizedAppHost,
        },
        {
          onLog: (stage, message) => {
            if (stage === "agent") {
              appendInstallLog(message);
              return;
            }
            appendInstallLog(`[${stage}] ${message}`);
          },
        },
      );
      setInstallResult(result);
      if (result.warnings.length > 0) {
        toast.warning(result.warnings.join(" | "));
      } else {
        toast.success(`Install ${installTarget.label} thanh cong`);
      }
      await Promise.all([
        refreshModules({ force: true }),
        refreshInstallAgents(true),
      ]);
    } catch (err) {
      const message =
        err instanceof Error && err.message.trim()
          ? err.message
          : "Install module that bai";
      setInstallError(message);
      appendInstallLog(`[error] ${message}`);
      toast.error(message);
    } finally {
      setInstallSubmitting(false);
      setInstallRunning(false);
    }
  };

  const handleReinstallCert = async (item: ModuleStatusCard) => {
    const moduleName = item.sourceName || item.moduleKey;
    if (!moduleName.trim()) {
      toast.error("module name khong hop le");
      return;
    }

    setInstallDialogOpen(false);
    setInstallSubmitting(false);
    setInstallLogDialogOpen(true);
    setLogDialogTitle("Module Reinstall Cert Logs");
    setLogDialogDescription(
      "Reinstall CA/private/public key cho service va healthcheck sau khi cap nhat.",
    );
    setInstallRunning(true);
    setInstallResult(null);
    setInstallError("");
    setInstallLogs([]);

    try {
      const result = await reinstallModuleCertStream(
        {
          module_name: moduleName,
        },
        {
          onLog: (stage, message) => {
            if (stage === "agent") {
              appendInstallLog(message);
              return;
            }
            appendInstallLog(`[${stage}] ${message}`);
          },
        },
      );

      setInstallResult(result);
      if (result.warnings.length > 0) {
        toast.warning(result.warnings.join(" | "));
      } else {
        toast.success(`Reinstall cert ${item.label} thanh cong`);
      }

      await Promise.all([
        refreshModules({ force: true }),
        refreshInstallAgents(true),
      ]);
    } catch (err) {
      const message =
        err instanceof Error && err.message.trim()
          ? err.message
          : "Reinstall cert that bai";
      setInstallError(message);
      appendInstallLog(`[error] ${message}`);
      toast.error(message);
    } finally {
      setInstallRunning(false);
    }
  };

  return (
    <main className="py-2">
      <ModulePageContent
        isDark={isDark}
        textPrimary={textPrimary}
        textMuted={textMuted}
        status={status}
        error={error}
        lastFetchedAt={lastFetchedAt}
        searchQuery={searchQuery}
        agentSearchQuery={agentSearchQuery}
        filteredCards={filteredCards}
        installAgents={installAgents}
        installAgentsLoading={installAgentsLoading}
        agentInstallScript={agentInstallScript}
        agentBootstrapToken={agentBootstrapToken}
        onSearchQueryChange={setSearchQuery}
        onAgentSearchQueryChange={setAgentSearchQuery}
        onRefresh={handleRefresh}
        onRefreshAgents={() => {
          void refreshInstallAgents(false);
        }}
        onRotateAgentBootstrapToken={() => {
          void handleRotateAgentBootstrapToken();
        }}
        onInstall={openInstallDialog}
        onReinstallCert={handleReinstallCert}
        actionRunning={installRunning || installSubmitting}
      />

      <ModuleInstallDialog
        open={installDialogOpen}
        installSubmitting={installSubmitting}
        installTarget={installTarget}
        appHost={appHost}
        selectedAgentID={selectedAgentID}
        installAgents={installAgents}
        installAgentsLoading={installAgentsLoading}
        onOpenChange={setInstallDialogOpen}
        onAppHostChange={setAppHost}
        onSelectedAgentIDChange={setSelectedAgentID}
        onInstall={handleInstall}
      />

      <ModuleInstallLogDialog
        open={installLogDialogOpen}
        running={installRunning}
        logs={installLogs}
        result={installResult}
        errorMessage={installError}
        title={logDialogTitle}
        description={logDialogDescription}
        onOpenChange={setInstallLogDialogOpen}
      />
    </main>
  );
}
