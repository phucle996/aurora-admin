import {
  BarChart3,
  Box,
  DollarSign,
  LayoutDashboard,
  LogOut,
  Network,
  Server,
} from "lucide-react";
import { useTheme } from "next-themes";
import { Link, useLocation } from "react-router-dom";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type AdminSidebarProps = {
  onLogout: () => void;
  className?: string;
};

const sections = [
  {
    title: "Infrastructure",
    items: [
      {
        id: "vm",
        label: "KVM Hypervisor",
        description: "libvirt / qemu control",
        to: "/hypervisor/kvm",
        icon: Server,
        badge: "KVM",
      },
      {
        id: "docker",
        label: "Docker",
        description: "18 containers running",
        to: "/containers/docker",
        icon: Box,
        badge: "Ops",
      },
      {
        id: "k8s",
        label: "Kubernetes",
        description: "4 clusters healthy",
        to: "/orchestration/k8s",
        icon: Network,
        badge: "Scale",
      },
    ],
  },
  {
    title: "Business",
    items: [
      {
        id: "revenue",
        label: "Doanh thu",
        description: "$128,000 / month",
        to: "/dashboard?module=revenue",
        icon: DollarSign,
        badge: "Finance",
      },
      {
        id: "cost",
        label: "Chi phi",
        description: "$74,200 / month",
        to: "/dashboard?module=cost",
        icon: BarChart3,
        badge: "KPI",
      },
    ],
  },
];

export default function AdminSidebar({ onLogout, className }: AdminSidebarProps) {
  const { resolvedTheme } = useTheme();
  const location = useLocation();

  const isDark = resolvedTheme !== "light";
  const activeModule = location.pathname.startsWith("/hypervisor/kvm") ||
    location.pathname.startsWith("/vms")
    ? "vm"
    : location.pathname.startsWith("/containers/docker")
      ? "docker"
      : location.pathname.startsWith("/orchestration/k8s")
        ? "k8s"
      : new URLSearchParams(location.search).get("module") ?? "";
  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";

  return (
    <aside
      className={cn(
        "rounded-3xl border p-4 shadow-xl backdrop-blur-xl",
        isDark ? "border-white/10 bg-slate-950/65" : "border-black/10 bg-white/85",
        className,
      )}
    >
      <div className="flex h-full flex-col">
        <div className="space-y-3">
          <Badge
            variant="outline"
            className={cn(
              "w-fit rounded-full px-3 py-1 text-[10px] uppercase tracking-[0.18em]",
              isDark ? "border-indigo-300/30 bg-indigo-400/10 text-indigo-100" : "bg-indigo-50",
            )}
          >
            Admin Shell
          </Badge>
          <div className="space-y-1">
            <h2 className={cn("text-lg font-semibold", textPrimary)}>Control Modules</h2>
            <p className={cn("text-xs", textMuted)}>
              Sidebar riêng ở cấp layout cho toàn bộ trang quản trị.
            </p>
          </div>
        </div>

        <div className="mt-5 flex-1 space-y-5 overflow-y-auto pr-1">
          {sections.map((section) => (
            <div key={section.title} className="space-y-2">
              <p
                className={cn(
                  "px-1 text-[11px] font-semibold uppercase tracking-[0.14em]",
                  isDark ? "text-slate-400" : "text-slate-500",
                )}
              >
                {section.title}
              </p>
              <div className="space-y-2">
                {section.items.map((item) => {
                  const active = activeModule === item.id;
                  return (
                    <Link
                      key={item.id}
                      to={item.to}
                      className={cn(
                        "group flex items-center gap-3 rounded-2xl border px-3 py-2.5 transition-all",
                        active
                          ? isDark
                            ? "border-indigo-300/35 bg-indigo-500/15"
                            : "border-indigo-200 bg-indigo-50"
                          : isDark
                            ? "border-white/10 bg-white/5 hover:border-white/20 hover:bg-white/10"
                            : "border-slate-200 bg-white hover:border-slate-300 hover:bg-slate-50",
                      )}
                    >
                      <div
                        className={cn(
                          "grid size-9 place-items-center rounded-xl border",
                          active
                            ? isDark
                              ? "border-indigo-300/35 bg-indigo-500/20"
                              : "border-indigo-200 bg-indigo-100"
                            : isDark
                              ? "border-white/10 bg-black/20"
                              : "border-slate-200 bg-slate-100",
                        )}
                      >
                        <item.icon
                          className={cn(
                            "h-4 w-4",
                            active ? "text-indigo-300 dark:text-indigo-200" : textMuted,
                          )}
                        />
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="flex items-center justify-between gap-2">
                          <p className={cn("truncate text-sm font-semibold", textPrimary)}>
                            {item.label}
                          </p>
                          <span
                            className={cn(
                              "shrink-0 rounded-full px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wide",
                              isDark ? "bg-white/10 text-slate-200" : "bg-slate-100 text-slate-600",
                            )}
                          >
                            {item.badge}
                          </span>
                        </div>
                        <p className={cn("truncate text-xs", textMuted)}>{item.description}</p>
                      </div>
                    </Link>
                  );
                })}
              </div>
            </div>
          ))}
        </div>

        <div className="mt-4 grid gap-2">
          <Button asChild variant="outline" className={cn(isDark && "border-white/15 bg-white/5")}>
            <Link to="/dashboard" className="gap-2">
              <LayoutDashboard className="h-4 w-4" />
              Dashboard
            </Link>
          </Button>
          <Button onClick={onLogout} className="gap-2 bg-indigo-500 text-white hover:bg-indigo-400">
            <LogOut className="h-4 w-4" />
            Đăng xuất
          </Button>
        </div>
      </div>
    </aside>
  );
}
