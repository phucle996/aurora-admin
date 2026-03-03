import type { ReactNode } from "react";
import { useEffect, useState } from "react";
import { useTheme } from "next-themes";

import { cn } from "@/lib/utils";
import ThemeSwitcher from "@/components/theme-switcher";
import LanguageSwitcher from "@/components/language-switcher";

type AuthHeroProps = {
  hero?: ReactNode;
  className?: string;
};

const lightBackground =
  "linear-gradient(150deg, #B39DDB 0%, #D1C4E9 20%, #F3E5F5 40%, #FCE4EC 60%, #FFCDD2 80%, #FFAB91 100%)";

const darkBackground = `
  radial-gradient(ellipse 70% 55% at 50% 50%, rgba(255, 20, 147, 0.15), transparent 50%),
  radial-gradient(ellipse 160% 130% at 10% 10%, rgba(0, 255, 255, 0.12), transparent 60%),
  radial-gradient(ellipse 160% 130% at 90% 90%, rgba(138, 43, 226, 0.18), transparent 65%),
  radial-gradient(ellipse 110% 50% at 80% 30%, rgba(255, 215, 0, 0.08), transparent 40%),
  #000000
`;

export const authBackgrounds = {
  light: lightBackground,
  dark: darkBackground,
} as const;

const leftDarkBackground =
  "linear-gradient(160deg, #070b1f 0%, #0a122c 45%, #05060e 100%)";
const leftLightBackground =
  "linear-gradient(150deg, #f2f4fb 0%, #f5f1f7 40%, #f6eaef 75%, #f7e6da 100%)";

export const leftBackgrounds = {
  light: leftLightBackground,
  dark: leftDarkBackground,
} as const;

/**
 * Right-hand hero panel for auth pages.
 * Pair with a left content column in page components.
 */
export function AuthLayout({ hero, className }: AuthHeroProps) {
  const gridPattern =
    "linear-gradient(rgba(255,255,255,0.04) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,0.04) 1px, transparent 1px)";
  const { resolvedTheme } = useTheme();
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const isDark = mounted ? resolvedTheme === "dark" : true;
  const defaultHero = (
    <div className="relative z-10 flex flex-col items-start gap-4">
      <div className="flex items-end">
        <img
          src="/logo.png"
          alt="Aurora"
          className="h-25 w-25 object-contain"
        />
        <div>
          <p
            className={cn(
              "text-6xl font-semibold leading-none tracking-tight pb-0.5",
              isDark ? "text-white" : "text-slate-900",
            )}
          >
            uora
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
        style={{ background: isDark ? darkBackground : lightBackground }}
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
