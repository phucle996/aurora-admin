import type { ReactNode } from "react";
import { useTheme } from "next-themes";

import { cn } from "@/lib/utils";
import ThemeSwitcher from "@/components/theme-switcher";
import LanguageSwitcher from "@/components/language-switcher";
import { authBackgrounds } from "@/pages/AuthPage/auth-layout-theme";
import {
  ADMIN_BRAND_LOGO_PATH,
  ADMIN_BRAND_NAME,
  ADMIN_BRAND_SHORT_NAME,
} from "@/lib/admin-brand";

type AuthHeroProps = {
  hero?: ReactNode;
  className?: string;
};

/**
 * Right-hand hero panel for auth pages.
 * Pair with a left content column in page components.
 */
export function AuthLayout({ hero, className }: AuthHeroProps) {
  const gridPattern =
    "linear-gradient(rgba(255,255,255,0.04) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,0.04) 1px, transparent 1px)";
  const { resolvedTheme } = useTheme();
  const isDark = resolvedTheme !== "light";
  const defaultHero = (
    <div className="relative z-10 flex flex-col items-start gap-4">
      <div className="flex items-end">
        <img
          src={ADMIN_BRAND_LOGO_PATH}
          alt={ADMIN_BRAND_NAME}
          className="h-25 w-25 object-contain"
        />
        <div>
          <p
            className={cn(
              "text-6xl font-semibold leading-none tracking-tight pb-0.5",
              isDark ? "text-white" : "text-slate-900",
            )}
          >
            {ADMIN_BRAND_SHORT_NAME}
          </p>
        </div>
      </div>
    </div>
  );

  return (
    <aside
      className={cn(
        "relative hidden items-center justify-center lg:flex",
        className,
      )}
    >
      <div
        className="absolute inset-0 z-0 transition-[background] duration-500 ease-out"
        style={{
          background: isDark ? authBackgrounds.dark : authBackgrounds.light,
        }}
      />
      <div
        className="absolute inset-0 z-10"
        style={{
          backgroundImage: gridPattern,
          backgroundSize: "64px 64px",
          backgroundPosition: "center",
          opacity: 0.35,
        }}
      />
      <div className="relative z-20 max-w-md px-10 auth-hero-anim">
        {hero ?? defaultHero}
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
    </aside>
  );
}

export default AuthLayout;
