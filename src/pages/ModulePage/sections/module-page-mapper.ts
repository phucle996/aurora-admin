import { CheckCircle2, Clock3, Layers, LayoutGrid, Server, Settings } from "lucide-react";

import type { EnabledModuleItem } from "@/state/enabled-modules-context";

import type { BoardView, ModuleCatalogItem, ModuleStatusCard } from "./module-page-types";

export const DEFAULT_MODULE_PORT: Record<string, number> = {
  vm: 3001,
  docker: 3010,
  k8s: 3011,
  ums: 3005,
  mail: 8080,
  gateway: 443,
  monitoring: 8428,
};

const moduleCatalog: ModuleCatalogItem[] = [
  {
    id: "vm",
    label: "Virtual Machine",
    description: "Quan ly hypervisor KVM, VM lifecycle va node metrics.",
    icon: Server,
    aliases: ["vm", "vm-service", "kvm", "hypervisor", "libvirt"],
  },
  {
    id: "docker",
    label: "Docker Runtime",
    description: "Quan ly container runtime va workload Docker.",
    icon: Layers,
    aliases: ["docker"],
  },
  {
    id: "k8s",
    label: "Kubernetes",
    description: "Orchestration cluster va workload tren Kubernetes.",
    icon: Layers,
    aliases: ["k8s", "kubernetes"],
  },
  {
    id: "ums",
    label: "User Management",
    description: "Xac thuc, phan quyen va quan ly user/service account.",
    icon: Settings,
    aliases: ["ums", "user", "usermanagment", "user-management"],
  },
  {
    id: "mail",
    label: "Mail Service",
    description: "Gui email, template va luong notification.",
    icon: Settings,
    aliases: ["mail", "smtp"],
  },
  {
    id: "gateway",
    label: "Gateway",
    description: "Entry point route va TLS termination cho he thong.",
    icon: Layers,
    aliases: ["gateway", "nginx", "proxy"],
  },
  {
    id: "monitoring",
    label: "Monitoring",
    description: "Thu thap va luu tru metrics/telemetry toan cum.",
    icon: Layers,
    aliases: ["monitor", "metrics", "victoria", "prometheus"],
  },
];

export const boardNavigation: Array<{
  id: BoardView;
  label: string;
  icon: typeof LayoutGrid;
}> = [
  { id: "all", label: "All Modules", icon: LayoutGrid },
  { id: "installed", label: "Installed", icon: CheckCircle2 },
  { id: "pending", label: "Pending Install", icon: Clock3 },
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

export function formatStatusLabel(status: string): string {
  return status
    .replace(/[_-]+/g, " ")
    .replace(/\s+/g, " ")
    .trim()
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function isAdminLikeModule(value: string): boolean {
  return normalizeText(value).includes("admin");
}

function resolveModuleCatalog(item: EnabledModuleItem): ModuleCatalogItem | null {
  const text = normalizeText(`${item.name} ${item.endpoint}`);
  return (
    moduleCatalog.find((catalogItem) =>
      catalogItem.aliases.some((alias) =>
        text.includes(normalizeText(alias)),
      ),
    ) ?? null
  );
}

function normalizeModuleKey(value: string): string {
  const key = value
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return key || "module";
}

export function buildModuleStatusCards(items: EnabledModuleItem[]): ModuleStatusCard[] {
  const cleaned = items.filter((item) => {
    const text = `${item.name} ${item.endpoint}`;
    return !isAdminLikeModule(text);
  });

  return cleaned.map<ModuleStatusCard>((item, index) => {
    const catalog = resolveModuleCatalog(item);
    const installed = Boolean(item.installed || item.endpoint);

    if (catalog) {
      return {
        ...catalog,
        cardID: `${catalog.id}-${index}`,
        moduleKey: catalog.id,
        installed,
        runtimeStatus: item.status || (installed ? "installed" : "not_installed"),
        endpoint: item.endpoint || "",
        sourceName: item.name || "",
      };
    }

    const fallbackKey = normalizeModuleKey(item.name);
    return {
      id: fallbackKey,
      cardID: `detected-${fallbackKey}-${index}`,
      moduleKey: fallbackKey,
      label: prettifyName(item.name),
      description: "Module phat hien tu endpoint registry.",
      icon: Layers,
      aliases: [],
      installed,
      runtimeStatus: item.status || (installed ? "installed" : "not_installed"),
      endpoint: item.endpoint,
      sourceName: item.name,
    };
  });
}
