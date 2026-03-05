import { Moon, Sun } from "lucide-react"
import { useTheme } from "next-themes"

import { Button } from "@/components/ui/button"

export function ThemeSwitcher() {
  const { setTheme, resolvedTheme } = useTheme()
  const isDark = resolvedTheme !== "light"

  return (
    <Button
      variant="ghost"
      size="icon"
      className="rounded-full bg-transparent hover:bg-transparent"
      onClick={() => setTheme(isDark ? "light" : "dark")}
      aria-label="Toggle theme"
      title={isDark ? "Switch to light mode" : "Switch to dark mode"}
    >
      {isDark ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
    </Button>
  )
}

export default ThemeSwitcher
