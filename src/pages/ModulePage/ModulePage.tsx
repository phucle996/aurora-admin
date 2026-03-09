import { useEffect, useMemo, useRef, useState } from "react";
import { useTheme } from "next-themes";
import { toast } from "sonner";

import type { SSHInputPrompt } from "@/components/ssh-live-terminal";
import {
  installModuleStream,
  reinstallModuleCertStream,
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

function needsTerminalSudoPassword(message: string): boolean {
  const lowered = message.toLowerCase();
  return (
    lowered.includes("provide sudo password") ||
    lowered.includes("non-interactive sudo") ||
    lowered.includes("privilege=sudo-denied")
  );
}

type InstallPromptResolver = (value: string) => void;

export default function ModulePage() {
  const { resolvedTheme } = useTheme();
  const { items, status, error, lastFetchedAt, refreshModules } =
    useEnabledModules();
  const syncedOnMountRef = useRef(false);

  const [searchQuery, setSearchQuery] = useState("");

  const [installDialogOpen, setInstallDialogOpen] = useState(false);
  const [installSubmitting, setInstallSubmitting] = useState(false);
  const [installTarget, setInstallTarget] = useState<ModuleStatusCard | null>(
    null,
  );
  const [installScope] = useState<ModuleInstallScope>("remote");
  const [appHost, setAppHost] = useState("");
  const [appPort, setAppPort] = useState("");
  const [sshHost, setSshHost] = useState("");
  const [sshPort, setSshPort] = useState("22");
  const [sshUsername, setSshUsername] = useState("aurora");
  const [sshPassword, setSshPassword] = useState("");
  const [sshPrivateKey, setSshPrivateKey] = useState("");
  const [sshHostKeyFingerprint, setSshHostKeyFingerprint] = useState("");
  const [installLogDialogOpen, setInstallLogDialogOpen] = useState(false);
  const [installLogs, setInstallLogs] = useState<string[]>([]);
  const [installRunning, setInstallRunning] = useState(false);
  const [installResult, setInstallResult] = useState<
    ModuleInstallResult | ModuleReinstallCertResult | null
  >(null);
  const [installError, setInstallError] = useState("");
  const [logDialogTitle, setLogDialogTitle] = useState(
    "Module Install SSH Logs",
  );
  const [logDialogDescription, setLogDialogDescription] = useState(
    "Log chi tiet theo tung stage de debug install flow.",
  );
  const [installInputPrompt, setInstallInputPrompt] =
    useState<SSHInputPrompt | null>(null);
  const [installPromptResolver, setInstallPromptResolver] =
    useState<InstallPromptResolver | null>(null);

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

  useEffect(() => {
    if (syncedOnMountRef.current) {
      return;
    }
    syncedOnMountRef.current = true;
    void refreshModules({ force: true }).catch(() => {
      toast.error("Khong the tai module status tu API");
    });
  }, [refreshModules]);

  const handleRefresh = () => {
    void refreshModules({ force: true })
      .then(() => {
        toast.success("Da cap nhat module status tu API");
      })
      .catch(() => {
        toast.error("Khong the cap nhat module status tu API");
      });
  };

  const openInstallDialog = (item: ModuleStatusCard) => {
    const moduleID = item.sourceName || item.moduleKey;
    const defaultHost = `${moduleID}.aurora.local`;

    setInstallTarget(item);
    setAppHost(defaultHost);
    setAppPort("");
    setSshHost("");
    setSshPort("22");
    setSshUsername("aurora");
    setSshPassword("");
    setSshPrivateKey("");
    setSshHostKeyFingerprint("");
    setInstallDialogOpen(true);
  };

  const appendInstallLog = (line: string) => {
    setInstallLogs((prev) => [...prev, line]);
  };

  const requestSudoPasswordInTerminal = (promptText: string): Promise<string> => {
    return new Promise((resolve) => {
      const promptID = `${Date.now()}-${Math.random()
        .toString(36)
        .slice(2, 8)}`;
      setInstallPromptResolver(() => resolve);
      setInstallInputPrompt({
        id: promptID,
        prompt: promptText,
        maskInput: true,
      });
    });
  };

  const handleInstallPromptSubmit = (value: string) => {
    const resolve = installPromptResolver;
    setInstallPromptResolver(null);
    setInstallInputPrompt(null);
    if (resolve) {
      resolve(value);
    }
  };

  const handleInstall = async () => {
    if (!installTarget) {
      return;
    }

    const moduleName = installTarget.sourceName || installTarget.moduleKey;
    const normalizedAppHost = appHost.trim();
    const normalizedAppPort = appPort.trim();

    if (!normalizedAppHost) {
      toast.error("app host la bat buoc");
      return;
    }
    if (normalizedAppPort) {
      const parsedPort = Number.parseInt(normalizedAppPort, 10);
      if (
        !Number.isInteger(parsedPort) ||
        parsedPort <= 0 ||
        parsedPort > 65535
      ) {
        toast.error("app port phai trong khoang 1..65535");
        return;
      }
    }

    if (!sshHost.trim() || !sshUsername.trim() || !sshHostKeyFingerprint.trim()) {
      toast.error("Remote install can ssh host, username va host key fingerprint");
      return;
    }

    setInstallSubmitting(true);
    setInstallDialogOpen(false);
    setInstallLogDialogOpen(true);
    setLogDialogTitle("Module Install SSH Logs");
    setLogDialogDescription("Log chi tiet theo tung stage de debug install flow.");
    setInstallRunning(true);
    setInstallResult(null);
    setInstallError("");
    setInstallLogs([]);
    setInstallInputPrompt(null);
    setInstallPromptResolver(null);

    try {
      const doInstall = async (sudoPassword?: string) => {
        return installModuleStream(
          {
            module_name: moduleName,
            scope: installScope,
            app_host: normalizedAppHost,
            app_port: normalizedAppPort
              ? Number.parseInt(normalizedAppPort, 10)
              : undefined,
            ssh_host: sshHost.trim() || undefined,
            ssh_port: Number.parseInt(sshPort.trim(), 10) || 22,
            ssh_username: sshUsername.trim() || undefined,
            ssh_password: sshPassword.trim() || undefined,
            sudo_password: sudoPassword,
            ssh_private_key: sshPrivateKey.trim() || undefined,
            ssh_host_key_fingerprint: sshHostKeyFingerprint.trim() || undefined,
          },
          {
            onLog: (stage, message) => {
              if (stage === "ssh") {
                appendInstallLog(message);
                return;
              }
              appendInstallLog(`[${stage}] ${message}`);
            },
          },
        );
      };

      let result: ModuleInstallResult | null = null;
      let sudoPassword: string | undefined;
      let askedSudoPassword = false;

      for (;;) {
        try {
          result = await doInstall(sudoPassword);
          break;
        } catch (err) {
          const message =
            err instanceof Error && err.message.trim()
              ? err.message
              : "Install module that bai";
          if (!askedSudoPassword && needsTerminalSudoPassword(message)) {
            askedSudoPassword = true;
            appendInstallLog(
              "[sudo] password required. Enter password in terminal then press Enter.",
            );
            const typed = await requestSudoPasswordInTerminal("[sudo] Password: ");
            if (!typed.trim()) {
              throw new Error("sudo password is empty");
            }
            sudoPassword = typed;
            appendInstallLog("[sudo] retry install with provided sudo password");
            continue;
          }
          throw err;
        }
      }

      if (!result) {
        throw new Error("Install result is empty");
      }

      setInstallResult(result);
      if (result.warnings.length > 0) {
        toast.warning(result.warnings.join(" | "));
      } else {
        toast.success(`Install ${installTarget.label} thanh cong`);
      }
      await refreshModules({ force: true });
    } catch (err) {
      const message =
        err instanceof Error && err.message.trim()
          ? err.message
          : "Install module that bai";
      setInstallError(message);
      appendInstallLog(`[error] ${message}`);
      toast.error(message);
    } finally {
      const resolvePrompt = installPromptResolver;
      if (typeof resolvePrompt === "function") {
        resolvePrompt("");
        setInstallPromptResolver(null);
      }
      setInstallInputPrompt(null);
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
    setInstallInputPrompt(null);
    setInstallPromptResolver(null);

    try {
      const result = await reinstallModuleCertStream(
        {
          module_name: moduleName,
        },
        {
          onLog: (stage, message) => {
            if (stage === "ssh") {
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

      await refreshModules({ force: true });
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
        filteredCards={filteredCards}
        onSearchQueryChange={setSearchQuery}
        onRefresh={handleRefresh}
        onInstall={openInstallDialog}
        onReinstallCert={handleReinstallCert}
        actionRunning={installRunning || installSubmitting}
      />

      <ModuleInstallDialog
        open={installDialogOpen}
        installSubmitting={installSubmitting}
        installTarget={installTarget}
        appHost={appHost}
        appPort={appPort}
        sshHost={sshHost}
        sshPort={sshPort}
        sshUsername={sshUsername}
        sshPassword={sshPassword}
        sshPrivateKey={sshPrivateKey}
        sshHostKeyFingerprint={sshHostKeyFingerprint}
        onOpenChange={setInstallDialogOpen}
        onAppHostChange={setAppHost}
        onAppPortChange={setAppPort}
        onSshHostChange={setSshHost}
        onSshPortChange={setSshPort}
        onSshUsernameChange={setSshUsername}
        onSshPasswordChange={setSshPassword}
        onSshPrivateKeyChange={setSshPrivateKey}
        onSshHostKeyFingerprintChange={setSshHostKeyFingerprint}
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
        inputPrompt={installInputPrompt}
        onInputSubmit={handleInstallPromptSubmit}
        onOpenChange={setInstallLogDialogOpen}
      />
    </main>
  );
}
