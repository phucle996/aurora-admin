import { useTheme } from "next-themes";
import { useState } from "react";
import { Outlet, useNavigate } from "react-router-dom";

import AdminSidebar from "@/components/admin-sidebar";
import AdminTopDock from "@/components/admin-top-dock";
import { setAdminSession } from "@/lib/admin-auth";
import { useEnabledModules } from "@/state/enabled-modules-context";
import { cn } from "@/lib/utils";

export default function AdminLayout() {
  const { resolvedTheme } = useTheme();
  const navigate = useNavigate();
  const { clearModules } = useEnabledModules();
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  const isDark = resolvedTheme !== "light";
  const shellBackground = isDark
    ? "linear-gradient(150deg, #070d1f 0%, #0c152c 45%, #0a1022 100%)"
    : "linear-gradient(145deg, #f2f4f8 0%, #eceff5 45%, #e8ebf1 100%)";

  const handleLogout = () => {
    setAdminSession(false);
    clearModules();
    navigate("/login");
  };

  return (
    <div className="relative min-h-screen w-full">
      <div
        className="absolute inset-0 -z-10 transition-[background] duration-500 ease-out"
        style={{ background: shellBackground }}
      />

      <div className={cn("flex min-h-screen w-full", isDark ? "bg-slate-950/35" : "bg-slate-100/70")}>
        <AdminSidebar
          onLogout={handleLogout}
          collapsed={sidebarCollapsed}
          onToggleCollapse={() => setSidebarCollapsed((prev) => !prev)}
          className={cn(
            "shrink-0 transition-[width] duration-200 ease-linear",
            sidebarCollapsed ? "w-[84px]" : "w-[276px]",
          )}
        />

        <div className="flex min-h-screen min-w-0 flex-1 flex-col gap-3 p-4 lg:p-5">
          <AdminTopDock />
          <div className="min-h-0 flex-1 overflow-auto rounded-[16px] border border-white/10 bg-white/35 p-1 dark:bg-white/[0.02]">
            <Outlet />
          </div>
        </div>
      </div>
    </div>
  );
}
