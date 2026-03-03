import api from "@/lib/api";
import {
  type KvmDetailApiResponse,
  toBooleanValue,
  toNumberValue,
  toStringValue,
  toTimestampValue,
} from "@/hooks/kvm-detail/kvm-detail-api.helpers";

export type KvmAgentUpdateCheckResult = {
  nodeId: string;
  agentStatus: string;
  currentVersion: string;
  latestVersion: string;
  hasUpdate: boolean;
  releaseURL: string;
  releaseTag: string;
  checkedAt: string;
  agentStreamMode: string;
  probeListenAddr: string;
};

export type KvmAgentUpdateResult = {
  nodeId: string;
  ok: boolean;
  agentStatus: string;
  previousVersion: string;
  targetVersion: string;
  updatedVersion: string;
  releaseURL: string;
  releaseTag: string;
  agentStreamMode: string;
  probeListenAddr: string;
  remoteExitCode: number;
  remoteOutput: string;
  checkedAt: string;
};

export type KvmAgentHealthcheckResult = {
  nodeId: string;
  agentStatus: string;
  reachable: boolean;
  currentVersion: string;
  checkedAt: string;
  agentStreamMode: string;
  probeListenAddr: string;
  error: string;
};

export async function checkKvmNodeAgentUpdate(
  nodeId: string,
): Promise<KvmAgentUpdateCheckResult> {
  const res = await api.get<KvmDetailApiResponse>(
    `/api/admin/hypervisors/kvm/nodes/${encodeURIComponent(nodeId)}/agent/update-check`,
  );
  const row = (res.data?.data ?? {}) as Record<string, unknown>;
  return {
    nodeId: toStringValue(row.NodeID ?? row.node_id),
    agentStatus: toStringValue(row.AgentStatus ?? row.agent_status),
    currentVersion: toStringValue(row.CurrentVersion ?? row.current_version),
    latestVersion: toStringValue(row.LatestVersion ?? row.latest_version),
    hasUpdate: toBooleanValue(row.HasUpdate ?? row.has_update),
    releaseURL: toStringValue(row.ReleaseURL ?? row.release_url),
    releaseTag: toStringValue(row.ReleaseTag ?? row.release_tag),
    checkedAt: toTimestampValue(row.CheckedAt ?? row.checked_at),
    agentStreamMode: toStringValue(
      row.AgentStreamMode ?? row.agent_stream_mode,
    ),
    probeListenAddr: toStringValue(
      row.ProbeListenAddr ?? row.probe_listen_addr,
    ),
  };
}

export async function updateKvmNodeAgent(
  nodeId: string,
  targetVersion?: string,
): Promise<KvmAgentUpdateResult> {
  const payload =
    typeof targetVersion === "string" && targetVersion.trim().length > 0
      ? { target_version: targetVersion.trim() }
      : {};

  const res = await api.post<KvmDetailApiResponse>(
    `/api/admin/hypervisors/kvm/nodes/${encodeURIComponent(nodeId)}/agent/update`,
    payload,
  );
  const row = (res.data?.data ?? {}) as Record<string, unknown>;
  return {
    nodeId: toStringValue(row.NodeID ?? row.node_id),
    ok: toBooleanValue(row.OK ?? row.ok),
    agentStatus: toStringValue(row.AgentStatus ?? row.agent_status),
    previousVersion: toStringValue(row.PreviousVersion ?? row.previous_version),
    targetVersion: toStringValue(row.TargetVersion ?? row.target_version),
    updatedVersion: toStringValue(row.UpdatedVersion ?? row.updated_version),
    releaseURL: toStringValue(row.ReleaseURL ?? row.release_url),
    releaseTag: toStringValue(row.ReleaseTag ?? row.release_tag),
    agentStreamMode: toStringValue(
      row.AgentStreamMode ?? row.agent_stream_mode,
    ),
    probeListenAddr: toStringValue(
      row.ProbeListenAddr ?? row.probe_listen_addr,
    ),
    remoteExitCode: toNumberValue(row.RemoteExitCode ?? row.remote_exit_code),
    remoteOutput: toStringValue(row.RemoteOutput ?? row.remote_output),
    checkedAt: toTimestampValue(row.CheckedAt ?? row.checked_at),
  };
}

export async function reinstallKvmNodeAgent(
  nodeId: string,
): Promise<KvmAgentUpdateResult> {
  const res = await api.post<KvmDetailApiResponse>(
    `/api/admin/hypervisors/kvm/nodes/${encodeURIComponent(nodeId)}/agent/reinstall`,
  );
  const row = (res.data?.data ?? {}) as Record<string, unknown>;
  return {
    nodeId: toStringValue(row.NodeID ?? row.node_id),
    ok: toBooleanValue(row.OK ?? row.ok),
    agentStatus: toStringValue(row.AgentStatus ?? row.agent_status),
    previousVersion: toStringValue(row.PreviousVersion ?? row.previous_version),
    targetVersion: toStringValue(row.TargetVersion ?? row.target_version),
    updatedVersion: toStringValue(row.UpdatedVersion ?? row.updated_version),
    releaseURL: toStringValue(row.ReleaseURL ?? row.release_url),
    releaseTag: toStringValue(row.ReleaseTag ?? row.release_tag),
    agentStreamMode: toStringValue(
      row.AgentStreamMode ?? row.agent_stream_mode,
    ),
    probeListenAddr: toStringValue(
      row.ProbeListenAddr ?? row.probe_listen_addr,
    ),
    remoteExitCode: toNumberValue(row.RemoteExitCode ?? row.remote_exit_code),
    remoteOutput: toStringValue(row.RemoteOutput ?? row.remote_output),
    checkedAt: toTimestampValue(row.CheckedAt ?? row.checked_at),
  };
}

export async function healthcheckKvmNodeAgent(
  nodeId: string,
): Promise<KvmAgentHealthcheckResult> {
  const res = await api.get<KvmDetailApiResponse>(
    `/api/admin/hypervisors/kvm/nodes/${encodeURIComponent(nodeId)}/agent/healthcheck`,
  );
  const row = (res.data?.data ?? {}) as Record<string, unknown>;
  return {
    nodeId: toStringValue(row.NodeID ?? row.node_id),
    agentStatus: toStringValue(row.AgentStatus ?? row.agent_status),
    reachable: toBooleanValue(row.Reachable ?? row.reachable),
    currentVersion: toStringValue(row.CurrentVersion ?? row.current_version),
    checkedAt: toTimestampValue(row.CheckedAt ?? row.checked_at),
    agentStreamMode: toStringValue(
      row.AgentStreamMode ?? row.agent_stream_mode,
    ),
    probeListenAddr: toStringValue(
      row.ProbeListenAddr ?? row.probe_listen_addr,
    ),
    error: toStringValue(row.Error ?? row.error),
  };
}
