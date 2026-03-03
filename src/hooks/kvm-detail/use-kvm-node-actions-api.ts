import api from "@/lib/api";
import {
  type KvmDetailApiResponse,
  toBooleanValue,
} from "@/hooks/kvm-detail/kvm-detail-api.helpers";

export type KvmNodeActionResult = {
  ok: boolean;
};

export async function runKvmNodeNow(
  nodeId: string,
): Promise<KvmNodeActionResult> {
  const res = await api.post<KvmDetailApiResponse>(
    `/api/admin/hypervisors/kvm/nodes/${encodeURIComponent(nodeId)}/run-now`,
  );
  const row = (res.data?.data ?? {}) as Record<string, unknown>;
  return {
    ok: toBooleanValue(row.OK ?? row.ok),
  };
}

export async function stopKvmNode(
  nodeId: string,
): Promise<KvmNodeActionResult> {
  const res = await api.post<KvmDetailApiResponse>(
    `/api/admin/hypervisors/kvm/nodes/${encodeURIComponent(nodeId)}/stop`,
  );
  const row = (res.data?.data ?? {}) as Record<string, unknown>;
  return {
    ok: toBooleanValue(row.OK ?? row.ok),
  };
}

export async function removeKvmNode(
  nodeId: string,
): Promise<KvmNodeActionResult> {
  const res = await api.delete<KvmDetailApiResponse>(
    `/api/admin/hypervisors/kvm/nodes/${encodeURIComponent(nodeId)}`,
  );
  const row = (res.data?.data ?? {}) as Record<string, unknown>;
  return {
    ok: toBooleanValue(row.OK ?? row.ok),
  };
}
