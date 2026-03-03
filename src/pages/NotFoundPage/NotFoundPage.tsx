import { Compass, Home, LogIn } from "lucide-react";
import { useTheme } from "next-themes";
import { Link } from "react-router-dom";

import { Button } from "@/components/ui/button";
import ThemeSwitcher from "@/components/theme-switcher";
import LanguageSwitcher from "@/components/language-switcher";
import { cn } from "@/lib/utils";
import { hasAdminSession } from "@/lib/admin-auth";

export default function NotFoundPage() {
  const { resolvedTheme } = useTheme();
  const isDark = resolvedTheme !== "light";
  const hasSession = hasAdminSession();

  const background = isDark
    ? "linear-gradient(160deg, #070b1f 0%, #0a122c 45%, #05060e 100%)"
    : "linear-gradient(150deg, #f2f4fb 0%, #f5f1f7 40%, #f6eaef 75%, #f7e6da 100%)";

  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";
  const accentText = isDark ? "text-indigo-200" : "text-indigo-600";

  return (
    <div className="min-h-screen w-full px-6 py-10 text-white">
      <div
        className="absolute inset-0 -z-10 transition-[background] duration-700 ease-out"
        style={{
          background,
        }}
      />
      <div className="relative mx-auto flex min-h-[calc(100vh-5rem)] max-w-6xl items-center">
        <div className="pointer-events-none absolute inset-0 opacity-35">
          <div className="absolute -right-24 top-16 h-64 w-64 rounded-full bg-indigo-500/18 blur-3xl" />
          <div className="absolute -left-24 bottom-10 h-56 w-56 rounded-full bg-purple-500/16 blur-3xl" />
        </div>

        <div className="relative grid w-full gap-10 md:grid-cols-[1.15fr_0.85fr] md:items-center">
          <div className="space-y-5">
            <p
              className={cn(
                "text-xs font-semibold uppercase tracking-[0.3em]",
                isDark ? "text-indigo-200/70" : "text-indigo-500/70",
              )}
            >
              Error Page
            </p>
            <h1
              className={cn(
                "text-4xl font-semibold tracking-tight md:text-5xl",
                textPrimary,
              )}
            >
              404 <span className={accentText}>Not Found</span>
            </h1>
            <p className={cn("max-w-md text-sm", textMuted)}>
              Trang bạn đang truy cập không tồn tại hoặc đã được di chuyển. Hãy quay lại
              đường dẫn hợp lệ để tiếp tục quản trị hệ thống.
            </p>
            <div className="flex flex-wrap items-center gap-3">
              <Button asChild className="rounded-full bg-indigo-500 px-5 text-white hover:bg-indigo-400">
                <Link to={hasSession ? "/dashboard" : "/login"} className="inline-flex items-center gap-2">
                  <Home className="h-4 w-4" />
                  {hasSession ? "Về Dashboard" : "Về Login"}
                </Link>
              </Button>
              <Button
                asChild
                variant="outline"
                className={cn(
                  "rounded-full px-5",
                  isDark ? "border-white/20 bg-white/5 text-white hover:bg-white/10" : "bg-white/70",
                )}
              >
                <Link to="/login" className="inline-flex items-center gap-2">
                  <LogIn className="h-4 w-4" />
                  Đăng nhập lại
                </Link>
              </Button>
            </div>
          </div>

          <div
            className={cn(
              "relative mx-auto flex h-[22rem] w-full max-w-md items-center justify-center rounded-3xl border backdrop-blur-xl md:h-[33rem]",
              isDark
                ? "border-white/10 bg-white/5 shadow-[0_20px_50px_rgba(2,6,23,0.45)]"
                : "border-black/10 bg-white/80 shadow-[0_20px_50px_rgba(30,41,59,0.20)]",
            )}
          >
            <div className="flex flex-col items-center gap-3 text-center">
              <Compass className={cn("h-12 w-12", accentText)} />
              <p className={cn("text-base font-medium", textPrimary)}>
                Endpoint not found
              </p>
              <p className={cn("max-w-[16rem] text-xs", textMuted)}>
                Kiểm tra lại URL hoặc quay lại dashboard để truy cập chức năng hợp lệ.
              </p>
            </div>
          </div>
        </div>
      </div>
      <div className="fixed bottom-6 right-6 z-50 flex items-center gap-2">
        <LanguageSwitcher />
        <div
          className={cn(
            "rounded-full border p-1.5 text-slate-900 shadow-lg backdrop-blur transition hover:bg-black/10",
            isDark
              ? "border-white/10 bg-white/5 text-white hover:bg-white/10"
              : "border-black/10 bg-black/5",
          )}
        >
          <ThemeSwitcher />
        </div>
      </div>
    </div>
  );
}
