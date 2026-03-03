import api from "@/lib/api";

export type KvmHypervisorListFilter = {
  zone?: string;
  search?: string;
  limit?: number;
  offset?: number;
};

export type KvmHypervisorItem = {
  nodeId: string;
  nodeName: string;
  host: string;
  zone: string;
  cpuCoresMax: number;
  ramMbMax: number;
  diskGbMax: number;
  status: string;
  vmTotal: number;
  vmRunning: number;
  vmStopped: number;
};

export type KvmHypervisorListResult = {
  items: KvmHypervisorItem[];
  count: number;
  totalCount: number;
};

export type KvmNodeRunNowResult = {
  nodeId: string;
  ok: boolean;
  status: string;
  uri: string;
  command: string;
  output: string;
  checkedAt: string;
  error: string;
};

type KvmApiResponse = {
  data?: {
    items?: unknown[];
    count?: unknown;
    total_count?: unknown;
    totalCount?: unknown;
  };
};

type KvmRunNowApiResponse = {
  data?: unknown;
};

function toStringValue(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }
  return "";
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

function parseKvmHypervisorItem(item: unknown): KvmHypervisorItem {
  const row = item as Record<string, unknown>;
  return {
    nodeId: toStringValue(row.NodeID ?? row.node_id),
    nodeName: toStringValue(row.NodeName ?? row.node_name),
    host: toStringValue(row.Host ?? row.host),
    zone: toStringValue(row.Zone ?? row.zone),
    cpuCoresMax: toNumberValue(row.CPUCoresMax ?? row.cpu_cores_max),
    ramMbMax: toNumberValue(row.RAMMBMax ?? row.ram_mb_max),
    diskGbMax: toNumberValue(row.DiskGBMax ?? row.disk_gb_max),
    status: toStringValue(row.Status ?? row.status),
    vmTotal: toNumberValue(row.VMTotal ?? row.vm_total),
    vmRunning: toNumberValue(row.VMRunning ?? row.vm_running),
    vmStopped: toNumberValue(row.VMStopped ?? row.vm_stopped),
  };
}

function parseKvmNodeRunNowResult(item: unknown): KvmNodeRunNowResult {
  const row = item as Record<string, unknown>;
  return {
    nodeId: toStringValue(row.NodeID ?? row.node_id),
    ok: Boolean(row.OK ?? row.ok),
    status: toStringValue(row.Status ?? row.status),
    uri: toStringValue(row.URI ?? row.uri),
    command: toStringValue(row.Command ?? row.command),
    output: toStringValue(row.Output ?? row.output),
    checkedAt: toStringValue(row.CheckedAt ?? row.checked_at),
    error: toStringValue(row.Error ?? row.error),
  };
}

export async function listKvmHypervisors(
  filter: KvmHypervisorListFilter,
  signal?: AbortSignal,
): Promise<KvmHypervisorListResult> {
  const res = await api.get<KvmApiResponse>("/api/admin/hypervisors/kvm", {
    params: {
      zone: filter.zone,
      search: filter.search,
      limit: filter.limit,
      offset: filter.offset,
    },
    signal,
  });

  const payload = res.data;
  const items = Array.isArray(payload.data?.items) ? payload.data.items : [];
  const parsedItems = items.map(parseKvmHypervisorItem);
  const count = toNumberValue(payload.data?.count);
  const totalCount = toNumberValue(
    payload.data?.total_count ?? payload.data?.totalCount,
  );
  return {
    items: parsedItems,
    count: count > 0 ? count : parsedItems.length,
    totalCount: totalCount > 0 ? totalCount : parsedItems.length,
  };
}

export async function runKvmNodeNow(
  nodeId: string,
): Promise<KvmNodeRunNowResult> {
  const res = await api.post<KvmRunNowApiResponse>(
    `/api/admin/hypervisors/kvm/nodes/${nodeId}/run-now`,
  );
  return parseKvmNodeRunNowResult(res.data?.data);
}
