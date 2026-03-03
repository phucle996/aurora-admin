import { useTheme } from "next-themes";
import { Outlet, useNavigate } from "react-router-dom";

import AdminTopDock from "@/components/admin-top-dock";
import { leftBackgrounds } from "@/pages/AuthPage/Authlayout";
import { setAdminSession } from "@/lib/admin-auth";

export default function AdminLayout() {
  const { resolvedTheme } = useTheme();
  const navigate = useNavigate();

  const isDark = resolvedTheme !== "light";

  const handleLogout = () => {
    setAdminSession(false);
    navigate("/login");
  };

  return (
    <div className="min-h-screen w-full">
      <div
        className="absolute inset-0 -z-10 transition-[background] duration-500 ease-out"
        style={{ background: isDark ? leftBackgrounds.dark : leftBackgrounds.light }}
      />

      <div className="w-full px-3 py-5 sm:px-4 md:px-5 lg:px-6">
        <AdminTopDock onLogout={handleLogout} />
        <div className="mt-4 min-w-0">
          <Outlet />
        </div>
      </div>
    </div>
  );
}
