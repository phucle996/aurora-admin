import { useTheme } from "next-themes";
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
    <div className="min-h-screen w-full px-4 py-5 md:px-6 md:py-6">
      <div
        className="absolute inset-0 -z-10 transition-[background] duration-500 ease-out"
        style={{ background: shellBackground }}
      />

      <div
        className={cn(
          "mx-auto grid min-h-[calc(100vh-3rem)] w-full max-w-[1600px] grid-cols-1 gap-4 rounded-[28px] border p-3 shadow-2xl backdrop-blur-xl lg:grid-cols-[240px_1fr] lg:p-4",
          isDark ? "border-white/10 bg-slate-950/55" : "border-slate-200 bg-white/70",
        )}
      >
        <AdminSidebar onLogout={handleLogout} className="min-h-0" />

        <div className="flex min-h-0 flex-col gap-3">
          <AdminTopDock onLogout={handleLogout} />
          <div className="min-h-0 flex-1 overflow-auto rounded-[18px] border border-transparent p-1">
            <Outlet />
          </div>
        </div>
      </div>
    </div>
  );
}
