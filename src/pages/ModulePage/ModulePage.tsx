import { useEffect, useMemo, useRef, useState } from "react";
import { useTheme } from "next-themes";
import { toast } from "sonner";

import {
  installModule,
  type ModuleInstallScope,
} from "@/hooks/module/use-module-install-api";
import { useEnabledModules } from "@/state/enabled-modules-context";

import { ModuleInstallDialog } from "./sections/module-install-dialog";
import {
  buildModuleStatusCards,
  DEFAULT_MODULE_PORT,
} from "./sections/module-page-mapper";
import { ModulePageContent } from "./sections/module-page-content";
import { ModulePageHeader } from "./sections/module-page-header";
import type { BoardView, ModuleStatusCard } from "./sections/module-page-types";

export default function ModulePage() {
  const { resolvedTheme } = useTheme();
  const { items, status, error, lastFetchedAt, refreshModules } = useEnabledModules();
  const syncedOnMountRef = useRef(false);

  const [viewMode, setViewMode] = useState<BoardView>("all");
  const [searchQuery, setSearchQuery] = useState("");

  const [installDialogOpen, setInstallDialogOpen] = useState(false);
  const [installSubmitting, setInstallSubmitting] = useState(false);
  const [installTarget, setInstallTarget] = useState<ModuleStatusCard | null>(null);
  const [installScope, setInstallScope] = useState<ModuleInstallScope>("local");
  const [appHost, setAppHost] = useState("");
  const [endpoint, setEndpoint] = useState("");
  const [installCommand, setInstallCommand] = useState("");
  const [sshHost, setSshHost] = useState("");
  const [sshPort, setSshPort] = useState("22");
  const [sshUsername, setSshUsername] = useState("aurora");
  const [sshPassword, setSshPassword] = useState("");
  const [sshPrivateKey, setSshPrivateKey] = useState("");

  const isDark = resolvedTheme !== "light";
  const cardClass = isDark ? "border-white/10 bg-white/[0.02]" : "border-slate-200 bg-white";
  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";

  const cards = useMemo(() => buildModuleStatusCards(items), [items]);
  const installedCount = useMemo(
    () => cards.filter((item) => item.installed).length,
    [cards],
  );
  const pendingCount = cards.length - installedCount;

  const filteredCards = useMemo(() => {
    return cards.filter((item) => {
      if (viewMode === "installed" && !item.installed) {
        return false;
      }
      if (viewMode === "pending" && item.installed) {
        return false;
      }

      const q = searchQuery.trim().toLowerCase();
      if (!q) {
        return true;
      }
      const text = `${item.label} ${item.sourceName} ${item.endpoint} ${item.runtimeStatus}`.toLowerCase();
      return text.includes(q);
    });
  }, [cards, searchQuery, viewMode]);

  const endpointCards = useMemo(
    () => cards.filter((item) => item.endpoint).slice(0, 8),
    [cards],
  );
  const pendingCards = useMemo(
    () => cards.filter((item) => !item.installed).slice(0, 8),
    [cards],
  );
  const activityCards = useMemo(() => cards.slice(0, 10), [cards]);

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
    const defaultPort = DEFAULT_MODULE_PORT[item.moduleKey] ?? 3000;

    setInstallTarget(item);
    setInstallScope("local");
    setAppHost(defaultHost);
    setEndpoint(item.endpoint || `${defaultHost}:${defaultPort}`);
    setInstallCommand("");
    setSshHost("");
    setSshPort("22");
    setSshUsername("aurora");
    setSshPassword("");
    setSshPrivateKey("");
    setInstallDialogOpen(true);
  };

  const handleInstall = async () => {
    if (!installTarget) {
      return;
    }

    const moduleName = installTarget.sourceName || installTarget.moduleKey;
    const normalizedAppHost = appHost.trim();
    const normalizedEndpoint = endpoint.trim();

    if (!normalizedAppHost || !normalizedEndpoint) {
      toast.error("app host va endpoint la bat buoc");
      return;
    }

    if (
      installScope === "remote" &&
      (!sshHost.trim() || !sshUsername.trim() || (!sshPassword.trim() && !sshPrivateKey.trim()))
    ) {
      toast.error("Remote install can ssh host, username va password/private key");
      return;
    }

    setInstallSubmitting(true);
    try {
      const result = await installModule({
        module_name: moduleName,
        scope: installScope,
        app_host: normalizedAppHost,
        endpoint: normalizedEndpoint,
        install_command: installCommand.trim() || undefined,
        ssh_host: sshHost.trim() || undefined,
        ssh_port: Number.parseInt(sshPort.trim(), 10) || 22,
        ssh_username: sshUsername.trim() || undefined,
        ssh_password: sshPassword.trim() || undefined,
        ssh_private_key: sshPrivateKey.trim() || undefined,
      });

      if (result.warnings.length > 0) {
        toast.warning(result.warnings.join(" | "));
      } else {
        toast.success(`Install ${installTarget.label} thanh cong`);
      }

      await refreshModules({ force: true });
      setInstallDialogOpen(false);
    } catch (err) {
      if (err instanceof Error && err.message.trim()) {
        toast.error(err.message);
      } else {
        toast.error("Install module that bai");
      }
    } finally {
      setInstallSubmitting(false);
    }
  };

  return (
    <main className="py-2">
      <section className="overflow-hidden rounded-[22px] border border-white/10 bg-white/35 dark:bg-white/[0.02]">
        <ModulePageHeader
          textPrimary={textPrimary}
          textMuted={textMuted}
          status={status}
          error={error}
          lastFetchedAt={lastFetchedAt}
          searchQuery={searchQuery}
          onSearchQueryChange={setSearchQuery}
          onRefresh={handleRefresh}
        />

        <ModulePageContent
          isDark={isDark}
          cardClass={cardClass}
          textPrimary={textPrimary}
          textMuted={textMuted}
          installedCount={installedCount}
          pendingCount={pendingCount}
          filteredCards={filteredCards}
          endpointCards={endpointCards}
          pendingCards={pendingCards}
          activityCards={activityCards}
          onInstall={openInstallDialog}
          onViewModeChange={setViewMode}
        />
      </section>

      <ModuleInstallDialog
        open={installDialogOpen}
        installSubmitting={installSubmitting}
        installTarget={installTarget}
        installScope={installScope}
        appHost={appHost}
        endpoint={endpoint}
        installCommand={installCommand}
        sshHost={sshHost}
        sshPort={sshPort}
        sshUsername={sshUsername}
        sshPassword={sshPassword}
        sshPrivateKey={sshPrivateKey}
        onOpenChange={setInstallDialogOpen}
        onInstallScopeChange={setInstallScope}
        onAppHostChange={setAppHost}
        onEndpointChange={setEndpoint}
        onInstallCommandChange={setInstallCommand}
        onSshHostChange={setSshHost}
        onSshPortChange={setSshPort}
        onSshUsernameChange={setSshUsername}
        onSshPasswordChange={setSshPassword}
        onSshPrivateKeyChange={setSshPrivateKey}
        onInstall={handleInstall}
      />
    </main>
  );
}
