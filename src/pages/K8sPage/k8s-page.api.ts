import type { EnabledModuleItem } from "@/state/enabled-modules-context";

export type PlatformCluster = {
  id: string;
  name: string;
  api_endpoint: string;
  mode: string;
  default_namespace: string;
  region: string;
  environment: string;
  status: string;
  is_enabled: boolean;
  last_health_check_at?: string;
  last_error?: string;
  created_at?: string;
  updated_at?: string;
};

export type PlatformClusterCapability = {
  cluster_id: string;
  kubernetes_version: string;
  ingress_classes: string[];
  storage_classes: string[];
  supports_metrics_api: boolean;
  supports_ingress: boolean;
  supports_statefulset: boolean;
  supports_pvc: boolean;
  last_synced_at?: string;
};

export type PlatformNodeMetric = {
  node_name: string;
  timestamp?: string;
  cpu_raw: string;
  cpu_milli: number;
  memory_raw: string;
  memory_bytes: number;
};

export type PlatformClusterMetrics = {
  cluster_id: string;
  nodes: PlatformNodeMetric[];
};

type ApiEnvelope<T> = {
  data?: T;
};

function toRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return {};
}

function toStringValue(value: unknown): string {
  if (typeof value === "string") {
    return value.trim();
  }
  return "";
}

function toBooleanValue(value: unknown): boolean {
  if (typeof value === "boolean") {
    return value;
  }
  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase();
    return normalized === "true" || normalized === "1";
  }
  if (typeof value === "number") {
    return value > 0;
  }
  return false;
}

function toNumberValue(value: unknown): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  return 0;
}

function toStringArray(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  const out: string[] = [];
  for (const item of value) {
    if (typeof item === "string" && item.trim()) {
      out.push(item.trim());
    }
  }
  return out;
}

export function isPlatformModule(item: EnabledModuleItem): boolean {
  const text = `${item.name} ${item.endpoint}`.toLowerCase();
  return text.includes("platform-resource") || text.includes("platform");
}

export function resolvePlatformBaseURL(items: EnabledModuleItem[]): string {
  const platformItem = items.find((item) => item.installed && isPlatformModule(item));
  const endpoint = platformItem?.endpoint?.trim() || "";
  if (!endpoint) {
    return "";
  }
  if (endpoint.startsWith("https://") || endpoint.startsWith("http://")) {
    return endpoint.replace(/\/+$/, "");
  }
  return `https://${endpoint}`;
}

function normalizePlatformCluster(value: unknown): PlatformCluster | null {
  const row = toRecord(value);
  const id = toStringValue(row.id);
  const name = toStringValue(row.name);
  if (!id || !name) {
    return null;
  }
  return {
    id,
    name,
    api_endpoint: toStringValue(row.api_endpoint),
    mode: toStringValue(row.mode),
    default_namespace: toStringValue(row.default_namespace),
    region: toStringValue(row.region),
    environment: toStringValue(row.environment),
    status: toStringValue(row.status) || "unknown",
    is_enabled: toBooleanValue(row.is_enabled),
    last_health_check_at: toStringValue(row.last_health_check_at) || undefined,
    last_error: toStringValue(row.last_error) || undefined,
    created_at: toStringValue(row.created_at) || undefined,
    updated_at: toStringValue(row.updated_at) || undefined,
  };
}

function normalizePlatformCapability(value: unknown): PlatformClusterCapability | null {
  const row = toRecord(value);
  const clusterID = toStringValue(row.cluster_id);
  if (!clusterID) {
    return null;
  }
  return {
    cluster_id: clusterID,
    kubernetes_version: toStringValue(row.kubernetes_version),
    ingress_classes: toStringArray(row.ingress_classes),
    storage_classes: toStringArray(row.storage_classes),
    supports_metrics_api: toBooleanValue(row.supports_metrics_api),
    supports_ingress: toBooleanValue(row.supports_ingress),
    supports_statefulset: toBooleanValue(row.supports_statefulset),
    supports_pvc: toBooleanValue(row.supports_pvc),
    last_synced_at: toStringValue(row.last_synced_at) || undefined,
  };
}

function normalizeNodeMetric(value: unknown): PlatformNodeMetric | null {
  const row = toRecord(value);
  const nodeName = toStringValue(row.node_name);
  if (!nodeName) {
    return null;
  }

  const cpu = toRecord(row.cpu);
  const memory = toRecord(row.memory);
  return {
    node_name: nodeName,
    timestamp: toStringValue(row.timestamp) || undefined,
    cpu_raw: toStringValue(cpu.raw),
    cpu_milli: Math.max(0, Math.round(toNumberValue(cpu.milli_value))),
    memory_raw: toStringValue(memory.raw),
    memory_bytes: Math.max(0, Math.round(toNumberValue(memory.bytes_value))),
  };
}

export function formatDate(value?: string): string {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return date.toLocaleString();
}

export function statusBadgeClass(status: string): string {
  const normalized = status.trim().toLowerCase();
  if (normalized === "healthy") {
    return "border-emerald-200 bg-emerald-50 text-emerald-700";
  }
  if (normalized === "unreachable") {
    return "border-rose-200 bg-rose-50 text-rose-700";
  }
  if (normalized === "disabled") {
    return "border-slate-200 bg-slate-100 text-slate-700";
  }
  return "border-amber-200 bg-amber-50 text-amber-700";
}

export async function listPlatformClusters(baseURL: string): Promise<PlatformCluster[]> {
  const response = await fetch(`${baseURL}/platform/clusters`, {
    method: "GET",
    headers: { Accept: "application/json" },
  });
  if (!response.ok) {
    const raw = await response.text();
    throw new Error(raw || `HTTP ${response.status}`);
  }
  const payload = (await response.json()) as ApiEnvelope<unknown>;
  if (!Array.isArray(payload.data)) {
    return [];
  }
  const out = payload.data
    .map((item) => normalizePlatformCluster(item))
    .filter((item): item is PlatformCluster => item !== null);
  out.sort((a, b) => a.name.localeCompare(b.name));
  return out;
}

export async function getPlatformClusterDetail(
  baseURL: string,
  clusterID: string,
): Promise<{ cluster: PlatformCluster; capability: PlatformClusterCapability | null }> {
  const response = await fetch(`${baseURL}/platform/clusters/${encodeURIComponent(clusterID)}`, {
    method: "GET",
    headers: { Accept: "application/json" },
  });
  if (!response.ok) {
    const raw = await response.text();
    throw new Error(raw || `HTTP ${response.status}`);
  }

  const payload = (await response.json()) as ApiEnvelope<{
    cluster?: unknown;
    capability?: unknown;
  }>;
  const row = toRecord(payload.data);
  const cluster = normalizePlatformCluster(row.cluster);
  if (!cluster) {
    throw new Error("invalid cluster payload");
  }
  const capability = normalizePlatformCapability(row.capability);
  return { cluster, capability };
}

export async function getPlatformClusterMetrics(
  baseURL: string,
  clusterID: string,
): Promise<PlatformClusterMetrics> {
  const response = await fetch(
    `${baseURL}/platform/clusters/${encodeURIComponent(clusterID)}/metrics`,
    {
      method: "GET",
      headers: { Accept: "application/json" },
    },
  );
  if (!response.ok) {
    const raw = await response.text();
    throw new Error(raw || `HTTP ${response.status}`);
  }

  const payload = (await response.json()) as ApiEnvelope<unknown>;
  const row = toRecord(payload.data);
  const nodesRaw = Array.isArray(row.nodes) ? row.nodes : [];
  const nodes = nodesRaw
    .map((item) => normalizeNodeMetric(item))
    .filter((item): item is PlatformNodeMetric => item !== null)
    .sort((a, b) => a.node_name.localeCompare(b.node_name));
  return {
    cluster_id: toStringValue(row.cluster_id) || clusterID,
    nodes,
  };
}
