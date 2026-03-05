import { useEffect, useMemo, useRef } from "react";
import {
  CheckCircle2,
  Layers,
  Server,
  Settings,
  XCircle,
  type LucideIcon,
} from "lucide-react";
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
import {
  useEnabledModules,
  type EnabledModuleItem,
} from "@/state/enabled-modules-context";

type ModuleCatalogItem = {
  id: string;
  label: string;
  description: string;
  icon: LucideIcon;
  aliases: string[];
};

type ModuleStatusCard = ModuleCatalogItem & {
  state: "installed" | "not_installed";
  runtimeStatus: string;
  endpoint: string;
  sourceName: string;
};

const moduleCatalog: ModuleCatalogItem[] = [
  {
    id: "vm",
    label: "Virtual Machine",
    description: "Quản lý hypervisor KVM, VM lifecycle và node metrics.",
    icon: Server,
    aliases: ["vm", "vm-service", "kvm", "hypervisor", "libvirt"],
  },
  {
    id: "docker",
    label: "Docker Runtime",
    description: "Quản lý container runtime và workload Docker.",
    icon: Layers,
    aliases: ["docker"],
  },
  {
    id: "k8s",
    label: "Kubernetes",
    description: "Orchestration cluster và workload trên Kubernetes.",
    icon: Layers,
    aliases: ["k8s", "kubernetes"],
  },
  {
    id: "ums",
    label: "User Management",
    description: "Xác thực, phân quyền và quản lý user/service account.",
    icon: Settings,
    aliases: ["ums", "user", "usermanagment", "user-management"],
  },
  {
    id: "mail",
    label: "Mail Service",
    description: "Gửi email, template và luồng notification.",
    icon: Settings,
    aliases: ["mail", "smtp"],
  },
  {
    id: "admin",
    label: "Admin Service",
    description: "Quản trị API key, token secret và service bootstrap.",
    icon: Settings,
    aliases: ["admin", "admin-service"],
  },
  {
    id: "gateway",
    label: "Gateway",
    description: "Entry point route và TLS termination cho toàn hệ thống.",
    icon: Layers,
    aliases: ["gateway", "nginx", "proxy"],
  },
  {
    id: "monitoring",
    label: "Monitoring",
    description: "Thu thập và lưu trữ metrics/telemetry toàn cụm.",
    icon: Layers,
    aliases: ["monitor", "metrics", "victoria", "prometheus"],
  },
];

function normalizeText(value: string): string {
  return value.trim().toLowerCase();
}

function prettifyName(value: string): string {
  return value
    .replace(/[-_]+/g, " ")
    .replace(/\s+/g, " ")
    .trim()
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function formatStatusLabel(status: string): string {
  return status
    .replace(/[_-]+/g, " ")
    .replace(/\s+/g, " ")
    .trim()
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function buildModuleStatusCards(items: EnabledModuleItem[]): ModuleStatusCard[] {
  const usedIndexes = new Set<number>();

  const mapped = moduleCatalog.map<ModuleStatusCard>((catalogItem) => {
    let matchedItem: EnabledModuleItem | null = null;

    for (let idx = 0; idx < items.length; idx += 1) {
      if (usedIndexes.has(idx)) continue;

      const candidate = items[idx];
      const text = normalizeText(`${candidate.name} ${candidate.endpoint}`);

      const matched = catalogItem.aliases.some((alias) =>
        text.includes(normalizeText(alias)),
      );

      if (!matched) continue;

      matchedItem = candidate;
      usedIndexes.add(idx);
      break;
    }

    const installed = Boolean(matchedItem?.installed || matchedItem?.endpoint);
    return {
      ...catalogItem,
      state: installed ? "installed" : "not_installed",
      runtimeStatus: matchedItem?.status || (installed ? "installed" : "not_installed"),
      endpoint: matchedItem?.endpoint || "",
      sourceName: matchedItem?.name || "",
    };
  });

  const discovered = items.flatMap<ModuleStatusCard>((item, idx) => {
    if (usedIndexes.has(idx)) return [];

    return [
      {
        id: `detected-${item.name}-${idx}`,
        label: prettifyName(item.name),
        description: "Module phát hiện từ endpoint registry.",
        icon: Layers,
        aliases: [],
        state: item.installed ? "installed" : "not_installed",
        runtimeStatus: item.status || (item.installed ? "installed" : "not_installed"),
        endpoint: item.endpoint,
        sourceName: item.name,
      },
    ];
  });

  return [...mapped, ...discovered];
}

export default function ModulePage() {
  const { resolvedTheme } = useTheme();
  const { items, status, error, lastFetchedAt, refreshModules } = useEnabledModules();
  const syncedOnMountRef = useRef(false);

  const isDark = resolvedTheme !== "light";

  const panelClass = isDark
    ? "border-white/10 bg-slate-950/60"
    : "border-black/10 bg-white/85";

  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";

  const cards = useMemo(() => buildModuleStatusCards(items), [items]);

  useEffect(() => {
    if (syncedOnMountRef.current) {
      return;
    }
    syncedOnMountRef.current = true;
    void refreshModules({ force: true }).catch(() => {
      toast.error("Không thể tải module status từ API");
    });
  }, [refreshModules]);

  const handleMockAction = (action: string, moduleLabel: string) => {
    toast.info(`${action} cho module "${moduleLabel}" đang ở chế độ mock UI.`);
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
            Module Status
          </Badge>

          <h1
            className={cn(
              "text-3xl font-semibold tracking-tight",
              textPrimary,
            )}
          >
            Runtime Module Status Board
          </h1>

          <p className={cn("text-sm", textMuted)}>
            Danh sách module lấy từ endpoint registry và hiển thị trạng thái
            cài đặt.
          </p>

          <p className={cn("text-xs", textMuted)}>
            API status: {status}
            {lastFetchedAt > 0
              ? ` • cập nhật lúc ${new Date(lastFetchedAt).toLocaleString()}`
              : ""}
          </p>

          {error ? (
            <p className="text-xs text-rose-400">{error}</p>
          ) : null}
        </div>

      </header>

      <section className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {cards.map((item) => {
          const Icon = item.icon;
          const installed = item.state === "installed";

          return (
            <Card
              key={item.id}
              className={cn(
                "shadow-lg",
                panelClass,
                installed
                  ? isDark
                    ? "border-emerald-400/20"
                    : "border-emerald-500/30"
                  : isDark
                    ? "border-slate-500/30"
                    : "border-slate-300",
              )}
            >
              <CardHeader className="pb-3">
                <div className="flex items-start justify-between gap-2">
                  <div className="flex min-w-0 items-center gap-2">
                    <div
                      className={cn(
                        "rounded-lg border p-2",
                        isDark
                          ? "border-white/20 bg-white/5"
                          : "border-slate-300 bg-slate-50",
                      )}
                    >
                      <Icon className={cn("h-5 w-5", textPrimary)} />
                    </div>

                    <div className="min-w-0">
                      <CardTitle
                        className={cn("truncate text-base", textPrimary)}
                      >
                        {item.label}
                      </CardTitle>
                    </div>
                  </div>

                  <Badge
                    variant="outline"
                    className={cn(
                      installed
                        ? "border-emerald-500/30 bg-emerald-500/10 text-emerald-400"
                        : "border-slate-500/30 bg-slate-500/10 text-slate-400",
                    )}
                  >
                    {installed ? "Đã cài" : "Chưa cài"}
                  </Badge>
                </div>

                <CardDescription className={cn("pt-1 text-sm", textMuted)}>
                  {item.description}
                </CardDescription>
              </CardHeader>

              <CardContent className="space-y-3">
                <div
                  className={cn(
                    "rounded-lg border px-3 py-2",
                    isDark
                      ? "border-white/10 bg-white/5"
                      : "border-slate-200 bg-slate-50/70",
                  )}
                >
                  <p className={cn("text-[11px] uppercase", textMuted)}>Endpoint</p>
                  <p className={cn("truncate text-sm", textPrimary)}>
                    {item.endpoint || "Chưa có endpoint"}
                  </p>
                </div>

                {installed ? (
                  <>
                    <div className="inline-flex items-center gap-1.5 rounded-full bg-emerald-500/15 px-2.5 py-1 text-xs font-medium text-emerald-400">
                      <CheckCircle2 className="h-3.5 w-3.5" />
                      {formatStatusLabel(item.runtimeStatus)}
                    </div>

                    <div className="grid grid-cols-4 gap-2 pt-1">
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() =>
                          handleMockAction("Healthcheck", item.label)
                        }
                      >
                        Health
                      </Button>

                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => handleMockAction("Stop", item.label)}
                      >
                        Stop
                      </Button>

                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => handleMockAction("Update", item.label)}
                      >
                        Update
                      </Button>

                      <Button
                        size="sm"
                        variant="destructive"
                        onClick={() =>
                          handleMockAction("Uninstall", item.label)
                        }
                      >
                        Remove
                      </Button>
                    </div>
                  </>
                ) : (
                  <>
                    <div className="inline-flex items-center gap-1.5 rounded-full bg-slate-500/15 px-2.5 py-1 text-xs font-medium text-slate-400">
                      <XCircle className="h-3.5 w-3.5" />
                      Chưa cài
                    </div>

                    <Button
                      className="w-full"
                      size="sm"
                      onClick={() => handleMockAction("Install", item.label)}
                    >
                      Install
                    </Button>
                  </>
                )}
              </CardContent>
            </Card>
          );
        })}
      </section>
    </main>
  );
}
