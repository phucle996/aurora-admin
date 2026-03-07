import {
  BookOpenText,
  Box,
  Layers,
  LayoutDashboard,
  LogOut,
  Network,
  PanelLeftClose,
  PanelLeftOpen,
  ScrollText,
  Server,
  Settings,
} from "lucide-react";
import { useTheme } from "next-themes";
import { Link, useLocation } from "react-router-dom";

import { Button } from "@/components/ui/button";
import { useEnabledModules, type ModuleFeature } from "@/state/enabled-modules-context";
import { cn } from "@/lib/utils";

type AdminSidebarProps = {
  onLogout: () => void;
  collapsed: boolean;
  onToggleCollapse: () => void;
  className?: string;
};

const navItems: Array<{
  id: string;
  label: string;
  to: string;
  icon: typeof LayoutDashboard;
  feature?: ModuleFeature;
}> = [
  { id: "dashboard", label: "Dashboard", to: "/dashboard", icon: LayoutDashboard },
  { id: "module", label: "Runtime Modules", to: "/module", icon: Layers },
  { id: "kvm", label: "KVM Hypervisor", to: "/hypervisor/kvm", icon: Server, feature: "kvm" },
  { id: "docker", label: "Docker Runtime", to: "/containers/docker", icon: Box, feature: "docker" },
  { id: "k8s", label: "Kubernetes", to: "/orchestration/k8s", icon: Network, feature: "platform" },
  { id: "settings", label: "Settings", to: "/settings", icon: Settings },
];

const footerNavItems: Array<{
  id: string;
  label: string;
  to: string;
  icon: typeof LayoutDashboard;
}> = [
  { id: "guide", label: "User Guide", to: "/guide", icon: BookOpenText },
  { id: "changelog", label: "Changelog", to: "/changelog", icon: ScrollText },
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
  if (itemID === "guide") {
    return pathname.startsWith("/guide");
  }
  if (itemID === "changelog") {
    return pathname.startsWith("/changelog");
  }
  return false;
}

export default function AdminSidebar({
  onLogout,
  collapsed,
  onToggleCollapse,
  className,
}: AdminSidebarProps) {
  const { resolvedTheme } = useTheme();
  const location = useLocation();
  const { status, isFeatureEnabled } = useEnabledModules();

  const isDark = resolvedTheme !== "light";
  const navVisibleItems = navItems.filter((item) => {
    if (!item.feature) {
      return true;
    }
    if (status !== "ready") {
      return false;
    }
    return isFeatureEnabled(item.feature);
  });

  return (
    <aside
      className={cn(
        "flex h-screen min-h-screen flex-col border-r px-3 py-4",
        isDark ? "border-white/10 bg-[#0f172a]/92" : "border-slate-200 bg-white/90",
        className,
      )}
    >
      <div className={cn("flex items-center pb-4", collapsed ? "justify-between px-1" : "gap-2 px-2")}>
        <div
          className={cn(
            "grid h-8 w-8 place-items-center rounded-lg border text-xs font-semibold",
            isDark
              ? "border-sky-300/30 bg-sky-500/10 text-sky-100"
              : "border-sky-200 bg-sky-50 text-sky-700",
          )}
        >
          A
        </div>
        {!collapsed ? (
          <div className="min-w-0">
            <p className={cn("truncate text-sm font-semibold", isDark ? "text-white" : "text-slate-900")}>
              Aurora Admin
            </p>
            <p className={cn("text-[11px]", isDark ? "text-slate-400" : "text-slate-500")}>
              Enterprise Console
            </p>
          </div>
        ) : null}
        {!collapsed ? (
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            onClick={onToggleCollapse}
            className={cn("ml-auto rounded-lg", isDark ? "text-slate-300 hover:bg-white/10" : "text-slate-600 hover:bg-slate-100")}
            aria-label="Collapse sidebar"
          >
            <PanelLeftClose className="h-4 w-4" />
          </Button>
        ) : (
          <Button
            type="button"
            size="icon-sm"
            variant="ghost"
            onClick={onToggleCollapse}
            className={cn("rounded-lg", isDark ? "text-slate-300 hover:bg-white/10" : "text-slate-600 hover:bg-slate-100")}
            aria-label="Expand sidebar"
          >
            <PanelLeftOpen className="h-4 w-4" />
          </Button>
        )}
      </div>

      <div className="border-t pt-4">
        {!collapsed ? (
          <p className={cn("px-2 text-[10px] font-semibold tracking-[0.16em]", isDark ? "text-slate-500" : "text-slate-400")}>
            MAIN
          </p>
        ) : null}
        <nav className="mt-2 space-y-1">
          {navVisibleItems.map((item) => {
            const active = isActivePath(item.id, location.pathname);
            return (
              <Link
                key={item.id}
                to={item.to}
                className={cn(
                  "flex items-center gap-2 rounded-lg border px-2.5 py-2 text-sm transition-colors",
                  collapsed && "justify-center px-2",
                  active
                    ? isDark
                      ? "border-sky-300/25 bg-sky-500/15 text-sky-100"
                      : "border-sky-200 bg-sky-50 text-sky-700"
                    : isDark
                      ? "border-transparent text-slate-300 hover:border-white/10 hover:bg-white/5"
                      : "border-transparent text-slate-600 hover:border-slate-200 hover:bg-slate-50",
                )}
              >
                <item.icon className="h-4 w-4" />
                {!collapsed ? <span className="truncate">{item.label}</span> : null}
              </Link>
            );
          })}
        </nav>
      </div>

      <div className="mt-auto pt-4">
        {!collapsed ? (
          <p className={cn("px-2 text-[10px] font-semibold tracking-[0.16em]", isDark ? "text-slate-500" : "text-slate-400")}>
            RESOURCES
          </p>
        ) : null}
        <nav className="mt-2 space-y-1">
          {footerNavItems.map((item) => {
            const active = isActivePath(item.id, location.pathname);
            return (
              <Link
                key={item.id}
                to={item.to}
                className={cn(
                  "flex items-center gap-2 rounded-lg border px-2.5 py-2 text-sm transition-colors",
                  collapsed && "justify-center px-2",
                  active
                    ? isDark
                      ? "border-sky-300/25 bg-sky-500/15 text-sky-100"
                      : "border-sky-200 bg-sky-50 text-sky-700"
                    : isDark
                      ? "border-transparent text-slate-300 hover:border-white/10 hover:bg-white/5"
                      : "border-transparent text-slate-600 hover:border-slate-200 hover:bg-slate-50",
                )}
              >
                <item.icon className="h-4 w-4" />
                {!collapsed ? <span className="truncate">{item.label}</span> : null}
              </Link>
            );
          })}
        </nav>

        {!collapsed ? (
          <div
            className={cn(
              "mt-3 rounded-xl border px-3 py-2 text-xs",
              isDark ? "border-white/10 bg-white/5 text-slate-300" : "border-slate-200 bg-slate-50 text-slate-600",
            )}
          >
            Session: Admin authenticated
          </div>
        ) : null}
        <Button
          type="button"
          onClick={onLogout}
          variant="outline"
          className={cn(
            "mt-2 w-full justify-start gap-2 rounded-lg",
            collapsed && "justify-center px-2",
            isDark ? "border-white/15 bg-white/5 text-white hover:bg-white/10" : "bg-white",
          )}
        >
          <LogOut className="h-4 w-4" />
          {!collapsed ? "Logout" : null}
        </Button>
      </div>
    </aside>
  );
}
