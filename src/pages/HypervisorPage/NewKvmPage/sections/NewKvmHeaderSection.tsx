import { ArrowLeft } from "lucide-react";
import { Link } from "react-router-dom";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type NewKvmHeaderSectionProps = {
  isDark: boolean;
  textPrimary: string;
  textMuted: string;
};

export function NewKvmHeaderSection({
  isDark,
  textPrimary,
  textMuted,
}: NewKvmHeaderSectionProps) {
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
          KVM Onboarding
        </Badge>
        <h1 className={cn("text-3xl font-semibold tracking-tight", textPrimary)}>
          Add KVM Node
        </h1>
        <p className={cn("text-sm", textMuted)}>
          Step 1/1: Probe node, save profile, then open overview.
        </p>
      </div>
      <div className="flex items-center gap-2">
        <Button asChild variant="outline" className={cn(isDark && "border-white/20 bg-white/5")}>
          <Link to="/hypervisor/kvm" className="gap-2">
            <ArrowLeft className="h-4 w-4" />
            Back to KVM
          </Link>
        </Button>
      </div>
    </header>
  );
}
