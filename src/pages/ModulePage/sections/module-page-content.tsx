import { Search } from "lucide-react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
  filteredCards: ModuleStatusCard[];
  onSearchQueryChange: (query: string) => void;
  onRefresh: () => void;
  onInstall: (item: ModuleStatusCard) => void;
  onReinstallCert: (item: ModuleStatusCard) => void;
  actionRunning: boolean;
};

export function ModulePageContent({
  isDark,
  textPrimary,
  textMuted,
  status,
  error,
  lastFetchedAt,
  searchQuery,
  filteredCards,
  onSearchQueryChange,
  onRefresh,
  onInstall,
  onReinstallCert,
  actionRunning,
}: ModulePageContentProps) {
  const lastSyncText = lastFetchedAt > 0
    ? new Date(lastFetchedAt).toLocaleTimeString()
    : "n/a";

  return (
    <div>
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

        <div className="flex items-center gap-2">
          <Button type="button" variant="outline" size="sm" onClick={onRefresh}>
            Refresh
          </Button>
          <div className="relative w-[260px] max-w-full">
            <Search className="absolute left-2.5 top-2.5 h-4 w-4 text-slate-400" />
            <Input
              value={searchQuery}
              onChange={(event) => onSearchQueryChange(event.target.value)}
              placeholder="Search module, endpoint..."
              className="pl-8"
            />
          </div>
          <Badge variant="outline">{filteredCards.length} items</Badge>
        </div>
      </header>

      <div className="space-y-4 py-3">
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
      </div>
    </div>
  );
}
