import { Activity, CheckCircle2, LayoutGrid, ListTree } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";

import { formatStatusLabel } from "./module-page-mapper";
import type { BoardView, ModuleStatusCard } from "./module-page-types";

type ModulePageContentProps = {
  isDark: boolean;
  cardClass: string;
  textPrimary: string;
  textMuted: string;
  installedCount: number;
  pendingCount: number;
  filteredCards: ModuleStatusCard[];
  endpointCards: ModuleStatusCard[];
  pendingCards: ModuleStatusCard[];
  activityCards: ModuleStatusCard[];
  onInstall: (item: ModuleStatusCard) => void;
  onViewModeChange: (view: BoardView) => void;
};

export function ModulePageContent({
  isDark,
  cardClass,
  textPrimary,
  textMuted,
  installedCount,
  pendingCount,
  filteredCards,
  endpointCards,
  pendingCards,
  activityCards,
  onInstall,
  onViewModeChange,
}: ModulePageContentProps) {
  return (
    <div className="grid gap-4 p-4 xl:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
      <div className="grid gap-4 md:grid-cols-2">
        <Card className={cardClass}>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Current Runtime</CardTitle>
            <CardDescription>
              Tong quan module da cai va trang thai hien tai.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2 text-sm">
            <div className="flex items-center justify-between">
              <span className={textMuted}>Installed modules</span>
              <span className={textPrimary}>{installedCount}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className={textMuted}>Pending install</span>
              <span className={textPrimary}>{pendingCount}</span>
            </div>
            <div className="flex items-center justify-between">
              <span className={textMuted}>Visible items</span>
              <span className={textPrimary}>{filteredCards.length}</span>
            </div>
          </CardContent>
        </Card>

        <Card className={cardClass}>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Endpoint Registry</CardTitle>
            <CardDescription>
              Danh sach endpoint dang duoc seed trong /endpoint.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {endpointCards.length === 0 ? (
              <p className={cn("text-sm", textMuted)}>Chua co endpoint.</p>
            ) : (
              endpointCards.map((item) => (
                <div
                  key={`endpoint-${item.cardID}`}
                  className="flex items-center justify-between gap-2 rounded-md border px-2 py-1.5 text-xs"
                >
                  <span className={cn("truncate", textPrimary)}>
                    {item.sourceName}
                  </span>
                  <span className={cn("truncate", textMuted)}>
                    {item.endpoint}
                  </span>
                </div>
              ))
            )}
          </CardContent>
        </Card>

        <Card className={cn("md:col-span-2", cardClass)}>
          <CardHeader className="pb-2">
            <div className="flex items-center justify-between">
              <CardTitle className="text-base">Modules</CardTitle>
              <Badge variant="outline">{filteredCards.length} items</Badge>
            </div>
            <CardDescription>
              Data thuc tu API, khong dung mock UI.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {filteredCards.length === 0 ? (
              <p className={cn("text-sm", textMuted)}>
                Khong co module phu hop bo loc.
              </p>
            ) : (
              filteredCards.map((item) => {
                const Icon = item.icon;
                return (
                  <div
                    key={item.cardID}
                    className={cn(
                      "rounded-lg border px-3 py-2",
                      isDark ? "border-white/10" : "border-slate-200",
                    )}
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className={cn("flex items-center gap-2 text-sm font-medium", textPrimary)}>
                          <Icon className="h-4 w-4 shrink-0" />
                          <span className="truncate">{item.label}</span>
                        </p>
                        <p className={cn("mt-1 text-xs", textMuted)}>
                          {item.endpoint || "Chua co endpoint"}
                        </p>
                      </div>
                      <div className="flex items-center gap-2">
                        <Badge
                          variant="outline"
                          className={
                            item.installed
                              ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-500"
                              : "border-slate-500/40 bg-slate-500/10 text-slate-500"
                          }
                        >
                          {item.installed
                            ? formatStatusLabel(item.runtimeStatus)
                            : "Not Installed"}
                        </Badge>
                        {!item.installed ? (
                          <Button
                            size="sm"
                            onClick={() => onInstall(item)}
                          >
                            Install
                          </Button>
                        ) : null}
                      </div>
                    </div>
                  </div>
                );
              })
            )}
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4">
        <Card className={cardClass}>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Pending Tasks</CardTitle>
            <CardDescription>
              Module chua cai dat - co the install local/remote.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {pendingCards.length === 0 ? (
              <p className={cn("text-sm", textMuted)}>
                Tat ca module da cai.
              </p>
            ) : (
              pendingCards.map((item) => (
                <div
                  key={`pending-${item.cardID}`}
                  className="rounded-md border px-2.5 py-2"
                >
                  <div className="flex items-center justify-between gap-2">
                    <p className={cn("truncate text-sm font-medium", textPrimary)}>
                      {item.label}
                    </p>
                    <Button
                      size="sm"
                      variant="outline"
                      onClick={() => onInstall(item)}
                    >
                      Install
                    </Button>
                  </div>
                  <p className={cn("mt-1 text-xs", textMuted)}>
                    key: {item.sourceName}
                  </p>
                </div>
              ))
            )}
          </CardContent>
        </Card>

        <Card className={cardClass}>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Recent Activity</CardTitle>
            <CardDescription>
              Trang thai module hien tai trong endpoint registry.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {activityCards.map((item) => (
              <div key={`activity-${item.cardID}`} className="flex items-start gap-2 text-xs">
                <Activity className={cn("mt-0.5 h-3.5 w-3.5 shrink-0", textMuted)} />
                <div className="min-w-0">
                  <p className={cn("truncate", textPrimary)}>
                    {item.label} • {formatStatusLabel(item.runtimeStatus)}
                  </p>
                  <p className={cn("truncate", textMuted)}>
                    {item.endpoint || "No endpoint"}
                  </p>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>

        <Card className={cardClass}>
          <CardHeader className="pb-2">
            <CardTitle className="text-base">Board Controls</CardTitle>
          </CardHeader>
          <CardContent className="grid grid-cols-1 gap-2 text-xs">
            <Button
              variant="outline"
              className="justify-start gap-2"
              onClick={() => onViewModeChange("all")}
            >
              <LayoutGrid className="h-3.5 w-3.5" />
              View all modules
            </Button>
            <Button
              variant="outline"
              className="justify-start gap-2"
              onClick={() => onViewModeChange("installed")}
            >
              <CheckCircle2 className="h-3.5 w-3.5" />
              View installed
            </Button>
            <Button
              variant="outline"
              className="justify-start gap-2"
              onClick={() => onViewModeChange("pending")}
            >
              <ListTree className="h-3.5 w-3.5" />
              View pending
            </Button>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
