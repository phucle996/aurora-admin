import { BellRing, Search, UserCircle2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

type ModulePageHeaderProps = {
  textPrimary: string;
  textMuted: string;
  status: string;
  error: string;
  lastFetchedAt: number;
  searchQuery: string;
  onSearchQueryChange: (query: string) => void;
  onRefresh: () => void;
};

export function ModulePageHeader({
  textPrimary,
  textMuted,
  status,
  error,
  lastFetchedAt,
  searchQuery,
  onSearchQueryChange,
  onRefresh,
}: ModulePageHeaderProps) {
  return (
    <header className="flex flex-wrap items-center justify-between gap-3 border-b px-4 py-3">
      <div>
        <h1 className={cn("text-lg font-semibold", textPrimary)}>
          Runtime Module Status Board
        </h1>
        <p className={cn("text-xs", textMuted)}>
          API status: {status}
          {lastFetchedAt > 0 ? ` • ${new Date(lastFetchedAt).toLocaleString()}` : ""}
        </p>
        {error ? <p className="text-xs text-rose-500">{error}</p> : null}
      </div>

      <div className="flex items-center gap-2">
        <div className="relative w-[260px] max-w-full">
          <Search className="absolute top-2.5 left-2.5 h-4 w-4 text-slate-400" />
          <Input
            value={searchQuery}
            onChange={(event) => onSearchQueryChange(event.target.value)}
            placeholder="Search module, endpoint..."
            className="pl-8"
          />
        </div>

        <Button size="icon" variant="outline" onClick={onRefresh}>
          <BellRing className="h-4 w-4" />
        </Button>
        <Button variant="outline" className="gap-2">
          <UserCircle2 className="h-4 w-4" />
          Admin
        </Button>
      </div>
    </header>
  );
}
