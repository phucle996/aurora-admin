import { Copy, KeyRound, RefreshCw, Search } from "lucide-react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import type { ModuleInstallAgent } from "@/hooks/module/use-module-install-api";
import { cn } from "@/lib/utils";

import { formatStatusLabel } from "./module-page-mapper";
import type { ModuleStatusCard } from "./module-page-types";

type ModulePageContentProps = {
  isDark: boolean;
  textPrimary: string;
  textMuted: string;
  status: string;
  error: string;
  lastFetchedAt: number;
  searchQuery: string;
  agentSearchQuery: string;
  filteredCards: ModuleStatusCard[];
  installAgents: ModuleInstallAgent[];
  installAgentsLoading: boolean;
  agentInstallScript: string;
  agentBootstrapToken: string;
  onSearchQueryChange: (query: string) => void;
  onAgentSearchQueryChange: (query: string) => void;
  onRefresh: () => void;
  onRefreshAgents: () => void;
  onRotateAgentBootstrapToken: () => void;
  onInstall: (item: ModuleStatusCard) => void;
  onReinstallCert: (item: ModuleStatusCard) => void;
  actionRunning: boolean;
};

function formatSeenAt(value: string): string {
  if (!value.trim()) {
    return "-";
  }
  const parsed = Date.parse(value);
  if (Number.isNaN(parsed)) {
    return value;
  }
  return new Date(parsed).toLocaleString();
}

export function ModulePageContent({
  isDark,
  textPrimary,
  textMuted,
  status,
  error,
  lastFetchedAt,
  searchQuery,
  agentSearchQuery,
  filteredCards,
  installAgents,
  installAgentsLoading,
  agentInstallScript,
  agentBootstrapToken,
  onSearchQueryChange,
  onAgentSearchQueryChange,
  onRefresh,
  onRefreshAgents,
  onRotateAgentBootstrapToken,
  onInstall,
  onReinstallCert,
  actionRunning,
}: ModulePageContentProps) {
  const lastSyncText = lastFetchedAt > 0
    ? new Date(lastFetchedAt).toLocaleTimeString()
    : "n/a";

  const normalizedAgentQuery = agentSearchQuery.trim().toLowerCase();
  const filteredAgents = installAgents.filter((item) => {
    if (!normalizedAgentQuery) {
      return true;
    }
    const text = `${item.agent_id} ${item.hostname} ${item.host} ${item.ip_address} ${item.status} ${item.agent_grpc_endpoint}`.toLowerCase();
    return text.includes(normalizedAgentQuery);
  });

  const handleCopyScript = () => {
    void navigator.clipboard.writeText(agentInstallScript)
      .then(() => toast.success("Da copy install script"))
      .catch(() => toast.error("Khong the copy install script"));
  };

  const handleRotateToken = () => {
    try {
      onRotateAgentBootstrapToken();
    } catch {
      // handled in caller
    }
  };

  return (
    <div className="space-y-3">
      <header className="flex flex-wrap items-center justify-between gap-3 px-1 py-2">
        <div>
          <h1 className={cn("text-lg font-semibold", textPrimary)}>
            Runtime Module
          </h1>
          <p className={cn("text-xs", textMuted)}>
            status: {status || "unknown"} | last sync: {lastSyncText}
          </p>
          {error ? <p className="text-xs text-rose-500">{error}</p> : null}
        </div>
      </header>

      <Tabs defaultValue="runtime" className="w-full">
        <TabsList variant="line">
          <TabsTrigger value="runtime">Runtime Module</TabsTrigger>
          <TabsTrigger value="agent">Agent</TabsTrigger>
        </TabsList>

        <TabsContent value="runtime" className="space-y-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="relative w-[320px] max-w-full">
              <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-slate-400" />
              <Input
                value={searchQuery}
                onChange={(event) => onSearchQueryChange(event.target.value)}
                placeholder="Search module, endpoint..."
                className="pl-8"
              />
            </div>
            <div className="flex items-center gap-2">
              <Button type="button" variant="outline" size="sm" onClick={onRefresh}>
                Refresh
              </Button>
              <Badge variant="outline">{filteredCards.length} items</Badge>
            </div>
          </div>

          <section className="space-y-3">
            {filteredCards.length === 0 ? (
              <p className={cn("text-sm", textMuted)}>Khong co module phu hop bo loc.</p>
            ) : (
              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                {filteredCards.map((item) => {
                  const Icon = item.icon;
                  return (
                    <div
                      key={item.cardID}
                      className={cn(
                        "flex min-h-[220px] flex-col rounded-2xl border p-3",
                        isDark
                          ? "border-white/10 bg-slate-950/35 shadow-[0_10px_30px_-20px_rgba(2,6,23,0.9)]"
                          : "border-slate-200/90 bg-white/85 shadow-[0_10px_30px_-20px_rgba(15,23,42,0.22)]",
                      )}
                    >
                      <div className="flex items-center gap-2">
                        <Icon className="h-4 w-4" />
                        <p className={cn("truncate text-sm font-semibold", textPrimary)}>{item.label}</p>
                      </div>

                      <p className={cn("mt-3 line-clamp-2 text-xs", textMuted)}>{item.description}</p>
                      <p className={cn("mt-2 truncate text-xs", textMuted)}>
                        endpoint: {item.endpoint || "not set"}
                      </p>
                      <p className={cn("mt-1 truncate text-xs", textMuted)}>
                        key: {item.sourceName}
                      </p>

                      <div className="mt-auto space-y-2 pt-3">
                        <Badge
                          variant="outline"
                          className={
                            item.installed
                              ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-500"
                              : "border-slate-500/40 bg-slate-500/10 text-slate-500"
                          }
                        >
                          {item.installed ? formatStatusLabel(item.runtimeStatus) : "Not Installed"}
                        </Badge>

                        {!item.installed ? (
                          <Button size="sm" className="w-full" onClick={() => onInstall(item)} disabled={actionRunning}>
                            Install
                          </Button>
                        ) : (
                          <Button size="sm" variant="outline" className="w-full" onClick={() => onReinstallCert(item)} disabled={actionRunning}>
                            Reinstall Cert
                          </Button>
                        )}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </section>
        </TabsContent>

        <TabsContent value="agent" className="space-y-4">
          <section
            className={cn(
              "space-y-3 rounded-xl border p-3",
              isDark ? "border-white/10 bg-slate-950/30" : "border-slate-200 bg-slate-50/70",
            )}
          >
            <div className="flex flex-wrap items-center justify-between gap-2">
              <p className={cn("text-sm font-semibold", textPrimary)}>Agent Install Script</p>
              <div className="flex items-center gap-2">
                <Button type="button" size="sm" variant="outline" onClick={handleCopyScript}>
                  <Copy className="mr-1 h-3.5 w-3.5" />
                  Copy
                </Button>
              </div>
            </div>
            <pre className="overflow-x-auto rounded-md border border-slate-700/40 bg-[#070b14] p-3 text-xs text-slate-200">
              <code>{agentInstallScript}</code>
            </pre>
            <div className="flex flex-wrap items-center gap-2">
              <Button type="button" size="sm" onClick={handleRotateToken}>
                <KeyRound className="mr-1 h-3.5 w-3.5" />
                Get Bootstrap Token
              </Button>
              <p className={cn("text-xs", textMuted)}>
                {agentBootstrapToken.trim() ? "Token da cap nhat vao script install." : "Chua co bootstrap token. Bam nut de tao token moi."}
              </p>
            </div>
          </section>

          <section className="space-y-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="relative w-[320px] max-w-full">
                <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-slate-400" />
                <Input
                  value={agentSearchQuery}
                  onChange={(event) => onAgentSearchQueryChange(event.target.value)}
                  placeholder="Search agent, host, endpoint..."
                  className="pl-8"
                />
              </div>
              <div className="flex items-center gap-2">
                <Button type="button" variant="outline" size="sm" onClick={onRefreshAgents} disabled={installAgentsLoading}>
                  <RefreshCw className={cn("mr-1 h-3.5 w-3.5", installAgentsLoading && "animate-spin")} />
                  Refresh Agent
                </Button>
                <Badge variant="outline">{filteredAgents.length} agents</Badge>
              </div>
            </div>

            {installAgentsLoading ? (
              <p className={cn("text-sm", textMuted)}>Dang tai danh sach agent...</p>
            ) : filteredAgents.length === 0 ? (
              <p className={cn("text-sm", textMuted)}>Khong co agent phu hop bo loc.</p>
            ) : (
              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                {filteredAgents.map((agent) => {
                  const connected = agent.status.trim().toLowerCase() === "connected";
                  return (
                    <div
                      key={agent.agent_id}
                      className={cn(
                        "rounded-xl border p-3",
                        isDark ? "border-white/10 bg-slate-950/35" : "border-slate-200 bg-white/80",
                      )}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <p className={cn("truncate text-sm font-semibold", textPrimary)}>
                          {agent.agent_id}
                        </p>
                        <Badge
                          variant="outline"
                          className={connected
                            ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-500"
                            : "border-amber-500/40 bg-amber-500/10 text-amber-500"}
                        >
                          {agent.status || "unknown"}
                        </Badge>
                      </div>
                      <div className={cn("mt-2 space-y-1 text-xs", textMuted)}>
                        <p>hostname: {agent.hostname || "-"}</p>
                        <p>host: {agent.host || agent.ip_address || "-"}</p>
                        <p>grpc: {agent.agent_grpc_endpoint || "-"}</p>
                        <p>ssh user: {agent.username || "-"}</p>
                        <p>ssh port: {agent.port || 22}</p>
                        <p>last seen: {formatSeenAt(agent.last_seen_at)}</p>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </section>
        </TabsContent>
      </Tabs>
    </div>
  );
}
