import { Box, LayoutDashboard, Layers, LogOut, Network, Server, Settings } from "lucide-react";
import { useTheme } from "next-themes";
import { Link, useLocation } from "react-router-dom";

import LanguageSwitcher from "@/components/language-switcher";
import ThemeSwitcher from "@/components/theme-switcher";
import { Button } from "@/components/ui/button";
import { useEnabledModules, type ModuleFeature } from "@/state/enabled-modules-context";
import { cn } from "@/lib/utils";

type AdminTopDockProps = {
  onLogout: () => void;
  className?: string;
};

const dockItems = [
  { id: "dashboard", label: "Dashboard", to: "/dashboard", icon: LayoutDashboard },
  { id: "module", label: "Modules", to: "/module", icon: Layers },
  { id: "settings", label: "Settings", to: "/settings", icon: Settings },
  {
    id: "kvm",
    label: "KVM Hypervisor",
    to: "/hypervisor/kvm",
    icon: Server,
    feature: "kvm" as ModuleFeature,
  },
  {
    id: "docker",
    label: "Docker",
    to: "/containers/docker",
    icon: Box,
    feature: "docker" as ModuleFeature,
  },
  {
    id: "k8s",
    label: "Kubernetes",
    to: "/orchestration/k8s",
    icon: Network,
    feature: "k8s" as ModuleFeature,
  },
];

function isActivePath(itemID: string, pathname: string) {
  if (itemID === "dashboard") {
    return pathname.startsWith("/dashboard");
  }
  if (itemID === "kvm") {
    return pathname.startsWith("/hypervisor/kvm") || pathname.startsWith("/vms");
  }
  if (itemID === "module") {
    return pathname.startsWith("/module");
  }
  if (itemID === "settings") {
    return pathname.startsWith("/settings");
  }
  if (itemID === "docker") {
    return pathname.startsWith("/containers/docker");
  }
  if (itemID === "k8s") {
    return pathname.startsWith("/orchestration/k8s");
  }
  return false;
}

export default function AdminTopDock({ onLogout, className }: AdminTopDockProps) {
  const { resolvedTheme } = useTheme();
  const location = useLocation();
  const { status, isFeatureEnabled } = useEnabledModules();

  const isDark = resolvedTheme !== "light";
  const visibleItems = dockItems.filter((item) => {
    if (!item.feature) {
      return true;
    }
    if (status !== "ready") {
      return true;
    }
    return isFeatureEnabled(item.feature);
  });

  return (
    <header
      className={cn(
        "sticky top-4 z-40 rounded-3xl border px-3 py-2 shadow-xl backdrop-blur-xl",
        isDark ? "border-white/10 bg-slate-950/70" : "border-black/10 bg-white/80",
        className,
      )}
    >
      <div className="flex flex-wrap items-center gap-3">
        <div
          className={cn(
            "rounded-full border px-3 py-1 text-[10px] font-semibold tracking-[0.2em]",
            isDark ? "border-indigo-300/30 bg-indigo-400/10 text-indigo-100" : "bg-indigo-50",
          )}
        >
          ADMIN SHELL
        </div>

        <nav
          className={cn(
            "mx-auto inline-flex items-center gap-2 rounded-2xl border p-1.5",
            isDark ? "border-white/10 bg-black/20" : "border-slate-200 bg-white/90",
          )}
          aria-label="Main navigation"
        >
          {visibleItems.map((item) => {
            const active = isActivePath(item.id, location.pathname);
            return (
              <Link
                key={item.id}
                to={item.to}
                title={item.label}
                aria-label={item.label}
                className={cn(
                  "grid h-10 w-10 place-items-center rounded-xl border transition-all duration-200",
                  active
                    ? isDark
                      ? "border-indigo-300/35 bg-indigo-500/20 text-indigo-100 shadow-lg shadow-indigo-500/20"
                      : "border-indigo-200 bg-indigo-100 text-indigo-700 shadow-md shadow-indigo-200/70"
                    : isDark
                      ? "border-white/10 bg-white/5 text-slate-300 hover:border-white/20 hover:bg-white/10 hover:text-white"
                      : "border-slate-200 bg-slate-50 text-slate-600 hover:border-slate-300 hover:bg-slate-100 hover:text-slate-900",
                )}
              >
                <item.icon className="h-4 w-4" />
              </Link>
            );
          })}
        </nav>

        <div className="ml-auto flex items-center gap-2">
          <LanguageSwitcher />
          <div
            className={cn(
              "rounded-full border p-1.5 shadow-lg backdrop-blur transition",
              isDark ? "border-white/10 bg-white/5 text-white" : "border-black/10 bg-black/5 text-slate-900",
            )}
          >
            <ThemeSwitcher />
          </div>
          <Button
            type="button"
            variant="outline"
            size="icon"
            className={cn(
              "rounded-xl",
              isDark ? "border-white/15 bg-white/5 text-white hover:bg-white/10" : "bg-white/90",
            )}
            onClick={onLogout}
            title="Đăng xuất"
            aria-label="Đăng xuất"
          >
            <LogOut className="h-4 w-4" />
          </Button>
        </div>
      </div>
    </header>
  );
}
