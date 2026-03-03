import api from "@/lib/api";

export type KvmHypervisorDetail = {
  nodeId: string;
  nodeName: string;
  host: string;
  zone: string;
  cpuCoresMax: number;
  ramMbMax: number;
  diskGbMax: number;
  storagePools: string[];
  networks: string[];
  sshPort: number;
  apiEndpoint: string;
  apiPort: number;
  agentStatus: string;
  tlsEnabled: boolean;
  tlsSkipVerify: boolean;
  status: string;
  lastCheckedAt: string;
  vmTotal: number;
  vmRunning: number;
  vmStopped: number;
  createdAt: string;
  updatedAt: string;
};

type KvmDetailApiResponse = {
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

function parseKvmHypervisorDetail(item: unknown): KvmHypervisorDetail {
  const row = item as Record<string, unknown>;
  return {
    nodeId: toStringValue(row.NodeID ?? row.node_id ?? row.ID ?? row.id),
    nodeName: toStringValue(row.NodeName ?? row.node_name),
    host: toStringValue(row.Host ?? row.host),
    zone: toStringValue(row.Zone ?? row.zone),
    cpuCoresMax: toNumberValue(row.CPUCoresMax ?? row.cpu_cores_max),
    ramMbMax: toNumberValue(row.RAMMBMax ?? row.ram_mb_max),
    diskGbMax: toNumberValue(row.DiskGBMax ?? row.disk_gb_max),
    storagePools: Array.isArray(row.StoragePools ?? row.storage_pools)
      ? ((row.StoragePools ?? row.storage_pools) as unknown[])
          .map((entry) => toStringValue(entry))
          .filter((entry) => entry.length > 0)
      : [],
    networks: Array.isArray(row.Networks ?? row.networks)
      ? ((row.Networks ?? row.networks) as unknown[])
          .map((entry) => toStringValue(entry))
          .filter((entry) => entry.length > 0)
      : [],
    sshPort: toNumberValue(row.SSHPort ?? row.ssh_port),
    apiEndpoint: toStringValue(row.APIEndpoint ?? row.api_endpoint),
    apiPort: toNumberValue(row.APIPort ?? row.api_port),
    agentStatus: toStringValue(row.AgentStatus ?? row.agent_status),
    tlsEnabled: Boolean(row.TLSEnabled ?? row.tls_enabled),
    tlsSkipVerify: Boolean(row.TLSSkipVerify ?? row.tls_skip_verify),
    status: toStringValue(row.Status ?? row.status),
    lastCheckedAt: toStringValue(row.LastCheckedAt ?? row.last_checked_at),
    vmTotal: toNumberValue(row.VMTotal ?? row.vm_total),
    vmRunning: toNumberValue(row.VMRunning ?? row.vm_running),
    vmStopped: toNumberValue(row.VMStopped ?? row.vm_stopped),
    createdAt: toStringValue(row.CreatedAt ?? row.created_at),
    updatedAt: toStringValue(row.UpdatedAt ?? row.updated_at),
  };
}

export async function getKvmHypervisorByNodeId(
  nodeId: string,
  signal?: AbortSignal,
): Promise<KvmHypervisorDetail> {
  const res = await api.get<KvmDetailApiResponse>(
    `/api/admin/hypervisors/kvm/${encodeURIComponent(nodeId)}`,
    { signal },
  );
  return parseKvmHypervisorDetail(res.data?.data);
}
