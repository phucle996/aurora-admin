import { BellRing, Search } from "lucide-react";
import { useTheme } from "next-themes";
import { useMemo } from "react";
import { useLocation } from "react-router-dom";

import LanguageSwitcher from "@/components/language-switcher";
import ThemeSwitcher from "@/components/theme-switcher";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

type AdminTopDockProps = {
  className?: string;
};

function resolvePageTitle(pathname: string): string {
  if (pathname.startsWith("/dashboard")) return "Dashboard";
  if (pathname.startsWith("/module")) return "Runtime Module Status Board";
  if (pathname.startsWith("/guide")) return "User Guide";
  if (pathname.startsWith("/changelog")) return "Admin Changelog";
  if (pathname.startsWith("/settings")) return "Settings";
  if (pathname.startsWith("/hypervisor/kvm")) return "KVM Hypervisor";
  if (pathname.startsWith("/containers/docker")) return "Docker Runtime";
  if (pathname.startsWith("/orchestration/k8s")) return "Kubernetes";
  return "Aurora Admin";
}

export default function AdminTopDock({ className }: AdminTopDockProps) {
  const { resolvedTheme } = useTheme();
  const location = useLocation();

  const isDark = resolvedTheme !== "light";
  const pageTitle = useMemo(() => resolvePageTitle(location.pathname), [location.pathname]);

  return (
    <header
      className={cn(
        "sticky top-0 z-30 rounded-[18px] border px-4 py-3 shadow-sm backdrop-blur",
        isDark ? "border-white/10 bg-slate-900/92" : "border-slate-200 bg-white/92",
        className,
      )}
    >
      <div className="flex items-center gap-3">
        <div className="min-w-0">
          <p className={cn("text-base font-semibold", isDark ? "text-white" : "text-slate-900")}>
            {pageTitle}
          </p>
          <p className={cn("text-xs", isDark ? "text-slate-400" : "text-slate-500")}>
            Welcome to Aurora enterprise control plane
          </p>
        </div>

        <div
          className={cn(
            "mx-auto hidden w-full max-w-md items-center gap-2 rounded-xl border px-3 py-1.5 lg:flex",
            isDark ? "border-white/10 bg-white/5" : "border-slate-200 bg-slate-50",
          )}
        >
          <Search className={cn("h-4 w-4", isDark ? "text-slate-400" : "text-slate-500")} />
          <Input
            readOnly
            value=""
            placeholder="Search everything..."
            className={cn(
              "h-7 border-0 bg-transparent px-0 text-xs shadow-none focus-visible:ring-0",
              isDark ? "placeholder:text-slate-500" : "placeholder:text-slate-400",
            )}
          />
          <kbd
            className={cn(
              "rounded-md border px-1.5 py-0.5 text-[10px]",
              isDark ? "border-white/10 text-slate-400" : "border-slate-300 text-slate-500",
            )}
          >
            ⌘K
          </kbd>
        </div>

        <div className="ml-auto flex items-center gap-1.5">
          <Button
            type="button"
            size="icon-sm"
            variant="outline"
            className={cn(
              "rounded-lg",
              isDark ? "border-white/15 bg-white/5 text-slate-200 hover:bg-white/10" : "bg-white",
            )}
            aria-label="Notifications"
          >
            <BellRing className="h-4 w-4" />
          </Button>
          <LanguageSwitcher />
          <div
            className={cn(
              "rounded-lg border p-1.5 shadow-sm transition",
              isDark ? "border-white/10 bg-white/5 text-white" : "border-black/10 bg-black/5 text-slate-900",
            )}
          >
            <ThemeSwitcher />
          </div>
        </div>
      </div>
    </header>
  );
}
