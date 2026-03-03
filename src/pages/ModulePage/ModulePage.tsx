import { useMemo, useState } from "react";
import { Copy, Download, Layers, PackageCheck } from "lucide-react";
import { useTheme } from "next-themes";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { cn } from "@/lib/utils";

type ModuleItem = {
  id: string;
  name: string;
  type: "service" | "agent" | "frontend";
  runtime: string;
  description: string;
  dependencies: string[];
  installCommand: string;
};

const moduleCatalog: ModuleItem[] = [
  {
    id: "ums",
    name: "UserManagementSystem",
    type: "service",
    runtime: "Go + PostgreSQL + Redis + ETCD",
    description: "Auth/RBAC trung tâm cho toàn bộ platform.",
    dependencies: ["PostgreSQL", "Redis", "ETCD"],
    installCommand: "cd UserManagmentSystem && go run cmd/server/main.go",
  },
  {
    id: "vm-service",
    name: "VM Service",
    type: "service",
    runtime: "Go + PostgreSQL + Redis + VictoriaMetrics",
    description: "Quản lý hypervisor, node, VM instances và metrics.",
    dependencies: ["PostgreSQL", "Redis", "VictoriaMetrics"],
    installCommand: "cd vm-service && go run cmd/server/main.go",
  },
  {
    id: "vm-agent",
    name: "VM Agent",
    type: "agent",
    runtime: "Go daemon",
    description: "Agent chạy trên node để đồng bộ trạng thái hạ tầng.",
    dependencies: ["Node access", "Network"],
    installCommand: "cd vm-agent && go run cmd/agent/main.go",
  },
  {
    id: "mail-service",
    name: "Mail Service",
    type: "service",
    runtime: "Go + PostgreSQL + Redis + ETCD",
    description: "Quản lý SMTP profile, template, consumer, email history.",
    dependencies: ["PostgreSQL", "Redis", "ETCD"],
    installCommand: "cd mail-service && go run cmd/server/main.go",
  },
  {
    id: "admin-ui",
    name: "Admin UI",
    type: "frontend",
    runtime: "React + Vite",
    description: "Giao diện điều phối và quản trị hệ sinh thái dịch vụ.",
    dependencies: ["Admin API"],
    installCommand: "cd Admin && npm install && npm run dev",
  },
];

const typeLabel: Record<ModuleItem["type"], string> = {
  service: "Service",
  agent: "Agent",
  frontend: "Frontend",
};

export default function ModulePage() {
  const { resolvedTheme } = useTheme();
  const [installing, setInstalling] = useState<Record<string, boolean>>({});
  const [installed, setInstalled] = useState<Record<string, boolean>>({});

  const isDark = resolvedTheme !== "light";
  const panelClass = isDark
    ? "border-white/10 bg-slate-950/60"
    : "border-black/10 bg-white/85";
  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";

  const installedCount = useMemo(
    () => Object.values(installed).filter(Boolean).length,
    [installed],
  );

  const copyCommand = async (command: string) => {
    try {
      await navigator.clipboard.writeText(command);
      toast.success("Đã copy install command");
    } catch {
      toast.error("Không thể copy command");
    }
  };

  const installModule = async (mod: ModuleItem) => {
    setInstalling((prev) => ({ ...prev, [mod.id]: true }));
    try {
      await new Promise((resolve) => setTimeout(resolve, 900));
      setInstalled((prev) => ({ ...prev, [mod.id]: true }));
      toast.success(`Đã đánh dấu install: ${mod.name}`);
    } finally {
      setInstalling((prev) => ({ ...prev, [mod.id]: false }));
    }
  };

  return (
    <main className="space-y-4 py-3 lg:py-1">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="space-y-2">
          <Badge
            variant="outline"
            className={cn(
              "rounded-full px-3 py-1 text-xs uppercase tracking-[0.12em]",
              isDark
                ? "border-white/20 bg-white/5 text-slate-200"
                : "bg-white/70",
            )}
          >
            Module Catalog
          </Badge>
          <h1
            className={cn("text-3xl font-semibold tracking-tight", textPrimary)}
          >
            Service Modules & Install
          </h1>
          <p className={cn("text-sm", textMuted)}>
            Danh mục module vận hành và command cài đặt nhanh cho từng service.
          </p>
        </div>
      </header>

      <section className="grid gap-4 md:grid-cols-2 2xl:grid-cols-3">
        {moduleCatalog.map((mod) => {
          const isInstalling = installing[mod.id] === true;
          const isInstalled = installed[mod.id] === true;

          return (
            <Card
              key={mod.id}
              className={cn("shadow-lg backdrop-blur-xl", panelClass)}
            >
              <CardHeader className="space-y-2">
                <div className="flex items-center justify-between gap-2">
                  <CardTitle className={cn("text-lg", textPrimary)}>
                    {mod.name}
                  </CardTitle>
                  <Badge
                    variant="outline"
                    className={cn(
                      isDark
                        ? "border-white/20 text-slate-200"
                        : "text-slate-700",
                    )}
                  >
                    {typeLabel[mod.type]}
                  </Badge>
                </div>
                <CardDescription className={textMuted}>
                  {mod.description}
                </CardDescription>
              </CardHeader>

              <CardContent className="space-y-4">
                <div className="space-y-1">
                  <p
                    className={cn(
                      "text-xs uppercase tracking-[0.1em]",
                      textMuted,
                    )}
                  >
                    Runtime
                  </p>
                  <p className={cn("text-sm", textPrimary)}>{mod.runtime}</p>
                </div>

                <div className="space-y-2">
                  <p
                    className={cn(
                      "text-xs uppercase tracking-[0.1em]",
                      textMuted,
                    )}
                  >
                    Dependencies
                  </p>
                  <div className="flex flex-wrap gap-1.5">
                    {mod.dependencies.map((dep) => (
                      <Badge
                        key={dep}
                        variant="secondary"
                        className={cn(
                          "rounded-full",
                          isDark
                            ? "bg-white/10 text-slate-100"
                            : "bg-slate-100 text-slate-700",
                        )}
                      >
                        {dep}
                      </Badge>
                    ))}
                  </div>
                </div>

                <div
                  className={cn(
                    "rounded-xl border p-2 text-xs",
                    isDark
                      ? "border-white/10 bg-black/20 text-slate-200"
                      : "border-slate-200 bg-slate-50 text-slate-700",
                  )}
                >
                  <code className="break-all">{mod.installCommand}</code>
                </div>

                <div className="flex items-center gap-2">
                  <Button
                    type="button"
                    variant={isInstalled ? "secondary" : "default"}
                    className={cn(
                      "flex-1 gap-2",
                      isInstalled && "text-emerald-700 dark:text-emerald-200",
                    )}
                    onClick={() => installModule(mod)}
                    disabled={isInstalling || isInstalled}
                  >
                    {isInstalled ? (
                      <PackageCheck className="h-4 w-4" />
                    ) : (
                      <Download className="h-4 w-4" />
                    )}
                    {isInstalling
                      ? "Installing..."
                      : isInstalled
                        ? "Installed"
                        : "Install"}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    size="icon"
                    onClick={() => copyCommand(mod.installCommand)}
                    title="Copy install command"
                  >
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
              </CardContent>
            </Card>
          );
        })}
      </section>
    </main>
  );
}
