import { Boxes } from "lucide-react";

import { cn } from "@/lib/utils";

import { boardNavigation } from "./module-page-mapper";
import type { BoardView } from "./module-page-types";

type ModulePageSidebarProps = {
  isDark: boolean;
  textPrimary: string;
  textMuted: string;
  sidebarClass: string;
  viewMode: BoardView;
  onViewModeChange: (view: BoardView) => void;
  installedCount: number;
  totalCount: number;
  pendingCount: number;
};

export function ModulePageSidebar({
  isDark,
  textPrimary,
  textMuted,
  sidebarClass,
  viewMode,
  onViewModeChange,
  installedCount,
  totalCount,
  pendingCount,
}: ModulePageSidebarProps) {
  return (
    <aside className={cn("border-r p-4", sidebarClass)}>
      <div className="flex items-center gap-2 px-2">
        <div className="rounded-md bg-blue-500/20 p-1.5">
          <Boxes className="h-4 w-4 text-blue-500" />
        </div>
        <div>
          <p className={cn("text-sm font-semibold", textPrimary)}>Aurora</p>
          <p className={cn("text-[11px]", textMuted)}>Runtime Control</p>
        </div>
      </div>

      <div className="mt-6 space-y-1">
        {boardNavigation.map((item) => {
          const Icon = item.icon;
          const active = viewMode === item.id;
          return (
            <button
              key={item.id}
              type="button"
              onClick={() => onViewModeChange(item.id)}
              className={cn(
                "flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-sm",
                active
                  ? isDark
                    ? "bg-blue-500/20 text-blue-300"
                    : "bg-blue-50 text-blue-700"
                  : textMuted,
              )}
            >
              <Icon className="h-4 w-4" />
              <span>{item.label}</span>
            </button>
          );
        })}
      </div>

      <div className="mt-6 rounded-lg border p-3">
        <p className={cn("text-xs uppercase tracking-wide", textMuted)}>Summary</p>
        <p className={cn("mt-2 text-sm", textPrimary)}>
          {installedCount}/{totalCount} modules installed
        </p>
        <p className={cn("mt-1 text-xs", textMuted)}>
          Pending: {pendingCount}
        </p>
      </div>
    </aside>
  );
}
