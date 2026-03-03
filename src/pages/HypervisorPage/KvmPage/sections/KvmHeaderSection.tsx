import { Plus, RefreshCcw } from "lucide-react";
import { Link } from "react-router-dom";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type KvmHeaderSectionProps = {
  isDark: boolean;
  textPrimary: string;
  textMuted: string;
  loading: boolean;
  onRefresh: () => void;
};

export function KvmHeaderSection({
  isDark,
  textPrimary,
  textMuted,
  loading,
  onRefresh,
}: KvmHeaderSectionProps) {
  return (
    <header className="mb-2 flex flex-wrap items-start justify-between gap-4">
      <div className="space-y-2">
        <Badge
          variant="outline"
          className={cn(
            "rounded-full px-3 py-1 text-xs uppercase tracking-[0.12em]",
            isDark ? "border-white/20 bg-white/5 text-slate-200" : "bg-white/70",
          )}
        >
          Hypervisor
        </Badge>
        <h1 className={cn("text-3xl font-semibold tracking-tight", textPrimary)}>
          KVM Control Plane
        </h1>
        <p className={cn("text-sm", textMuted)}>
          Du lieu thuc tu VM service, tong hop tu provider + node + vm_instances.
        </p>
      </div>
      <div className="flex items-center gap-2">
        <Button asChild className="bg-indigo-500 text-white hover:bg-indigo-400">
          <Link to="/hypervisor/kvm/new" className="gap-2">
            <Plus className="h-4 w-4" />
            Add node
          </Link>
        </Button>
        <Button
          variant="outline"
          className={cn(isDark && "border-white/20 bg-white/5")}
          onClick={onRefresh}
          disabled={loading}
        >
          <RefreshCcw className={cn("h-4 w-4", loading && "animate-spin")} />
          Refresh
        </Button>
      </div>
    </header>
  );
}
