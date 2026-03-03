import api from "@/lib/api";

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

export type CheckKvmNodeSSHPayload = {
  node_name: string;
  host: string;
  zone?: string;
  ssh_port?: number;
  ssh_username: string;
  ssh_password?: string;
  ssh_private_key?: string;
  provider_metadata?: Record<string, unknown>;
  node_metadata?: Record<string, unknown>;
  timeout_seconds?: number;
  install_if_missing?: boolean;
};

export type KvmNodeProbeCapability = {
  cpuCores: number;
  ramMb: number;
  diskFreeGb: number;
  kvmModule: boolean;
  libvirtRunning: boolean;
  virshConnect: boolean;
  storagePools: string[];
  networks: string[];
  kvmReady: boolean;
  missingComponents: string[];
};

export type KvmAgentProfile = {
  agentEndpoint: string;
  agentPort: number;
  agentProtocol: string;
  tlsEnabled: boolean;
  tlsSkipVerify: boolean;
};

export type KvmNodeSSHCheckResult = {
  ok: boolean;
  host: string;
  sshPort: number;
  sshUsername: string;
  authMethod: string;
  latencyMs: number;
  checkedAt: string;
  error: string;
  installAttempted: boolean;
  installOutput: string;
  saved: boolean;
  savedNodeId: string;
  agent: KvmAgentProfile | null;
  capability: KvmNodeProbeCapability;
};

type KvmSSHCheckApiResponse = {
  data?: unknown;
};

type KvmRunNowApiResponse = {
  data?: unknown;
};

const vmServiceBaseURL =
  import.meta.env.VITE_VM_SERVICE_BASE_URL?.toString() ??
  import.meta.env.VITE_API_URL?.toString() ??
  "";

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

function parseKvmNodeSSHCheckResult(item: unknown): KvmNodeSSHCheckResult {
  const row = item as Record<string, unknown>;
  const capability = (row.Capability ?? row.capability) as
    | Record<string, unknown>
    | undefined;
  const agent = (row.Agent ?? row.agent) as Record<string, unknown> | undefined;
  return {
    ok: Boolean(row.OK ?? row.ok),
    host: toStringValue(row.Host ?? row.host),
    sshPort: toNumberValue(row.SSHPort ?? row.ssh_port),
    sshUsername: toStringValue(row.SSHUsername ?? row.ssh_username),
    authMethod: toStringValue(row.AuthMethod ?? row.auth_method),
    latencyMs: toNumberValue(row.LatencyMS ?? row.latency_ms),
    checkedAt: toStringValue(row.CheckedAt ?? row.checked_at),
    error: toStringValue(row.Error ?? row.error),
    installAttempted: Boolean(row.InstallAttempted ?? row.install_attempted),
    installOutput: toStringValue(row.InstallOutput ?? row.install_output),
    saved: Boolean(row.Saved ?? row.saved),
    savedNodeId: toStringValue(row.SavedNodeID ?? row.saved_node_id),
    agent: agent
      ? {
          agentEndpoint: toStringValue(
            agent.AgentEndpoint ?? agent.agent_endpoint,
          ),
          agentPort: toNumberValue(agent.AgentPort ?? agent.agent_port),
          agentProtocol: toStringValue(
            agent.AgentProtocol ?? agent.agent_protocol,
          ),
          tlsEnabled: Boolean(agent.TLSEnabled ?? agent.tls_enabled),
          tlsSkipVerify: Boolean(agent.TLSSkipVerify ?? agent.tls_skip_verify),
        }
      : null,
    capability: {
      cpuCores: toNumberValue(capability?.CPUCores ?? capability?.cpu_cores),
      ramMb: toNumberValue(capability?.RAMMB ?? capability?.ram_mb),
      diskFreeGb: toNumberValue(
        capability?.DiskFreeGB ?? capability?.disk_free_gb,
      ),
      kvmModule: Boolean(capability?.KVMModule ?? capability?.kvm_module),
      libvirtRunning: Boolean(
        capability?.LibvirtRunning ?? capability?.libvirt_running,
      ),
      virshConnect: Boolean(
        capability?.VirshConnect ?? capability?.virsh_connect,
      ),
      storagePools: Array.isArray(
        capability?.StoragePools ?? capability?.storage_pools,
      )
        ? (
            (capability?.StoragePools ?? capability?.storage_pools) as unknown[]
          ).map(toStringValue)
        : [],
      networks: Array.isArray(capability?.Networks ?? capability?.networks)
        ? ((capability?.Networks ?? capability?.networks) as unknown[]).map(
            toStringValue,
          )
        : [],
      kvmReady: Boolean(capability?.KVMReady ?? capability?.kvm_ready),
      missingComponents: Array.isArray(
        capability?.MissingComponents ?? capability?.missing_components,
      )
        ? (
            (capability?.MissingComponents ??
              capability?.missing_components) as unknown[]
          ).map(toStringValue)
        : [],
    },
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

export async function checkKvmNodeSSH(
  payload: CheckKvmNodeSSHPayload,
  signal?: AbortSignal,
): Promise<KvmNodeSSHCheckResult> {
  const res = await api.post<KvmSSHCheckApiResponse>(
    "/api/admin/hypervisors/kvm/nodes/probe",
    payload,
    { signal },
  );
  return parseKvmNodeSSHCheckResult(res.data?.data);
}

type CheckKvmNodeSSHStreamOptions = {
  signal?: AbortSignal;
  onLog?: (stage: string, message: string) => void;
};

function extractErrorMessage(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  if (typeof error === "string") {
    return error;
  }
  return "unknown error";
}

function toSingleLine(value: string): string {
  return value.replace(/\s+/g, " ").trim();
}

export async function checkKvmNodeSSHStream(
  payload: CheckKvmNodeSSHPayload,
  options?: CheckKvmNodeSSHStreamOptions,
): Promise<KvmNodeSSHCheckResult> {
  const streamPath = "/api/admin/hypervisors/kvm/nodes/probe/stream";
  const url = vmServiceBaseURL
    ? new URL(streamPath, vmServiceBaseURL).toString()
    : streamPath;

  const res = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "text/event-stream",
    },
    body: JSON.stringify(payload),
    signal: options?.signal,
  });

  if (!res.ok) {
    const text = await res.text();
    let detail = toSingleLine(text);
    try {
      const parsed = JSON.parse(text) as Record<string, unknown>;
      detail = toStringValue(parsed.message ?? parsed.error) || detail;
    } catch {
      // ignore parse error, keep raw text
    }
    throw new Error(
      `Probe stream failed (HTTP ${res.status} ${res.statusText}): ${detail || "empty response"}`,
    );
  }
  if (!res.body) {
    throw new Error("Probe stream is empty");
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let result: KvmNodeSSHCheckResult | null = null;
  let lastStage = "";
  let lastMessage = "";
  let eventCount = 0;

  const parseEventBlock = (block: string) => {
    const dataLines = block
      .split("\n")
      .filter((line) => line.startsWith("data:"))
      .map((line) => line.slice(5).trimStart());
    if (dataLines.length === 0) {
      return;
    }
    const payloadRaw = dataLines.join("\n");
    const evt = JSON.parse(payloadRaw) as {
      type?: string;
      stage?: string;
      message?: string;
      data?: unknown;
    };

    if (evt.type === "log") {
      lastStage = evt.stage ?? "log";
      lastMessage = evt.message ?? "";
      eventCount += 1;
      options?.onLog?.(lastStage, lastMessage);
      return;
    }
    if (evt.type === "error") {
      const stage = evt.stage ?? "service";
      const message = evt.message || "Probe stream failed";
      lastStage = stage;
      lastMessage = message;
      eventCount += 1;
      options?.onLog?.(stage, `[error] ${message}`);
      throw new Error(`backend stream error (stage=${stage}): ${message}`);
    }
    if (evt.type === "result") {
      lastStage = evt.stage ?? "service";
      lastMessage = evt.message ?? "probe completed";
      eventCount += 1;
      result = parseKvmNodeSSHCheckResult(evt.data);
    }
  };

  while (true) {
    let value: Uint8Array | undefined;
    let done = false;
    try {
      const readResult = await reader.read();
      value = readResult.value;
      done = readResult.done;
    } catch (error) {
      const reason = extractErrorMessage(error);
      throw new Error(
        `probe stream read failed after ${eventCount} events (last_stage=${lastStage || "none"}, last_log=${toSingleLine(lastMessage) || "none"}): ${reason}`,
      );
    }
    buffer += decoder.decode(value ?? new Uint8Array(), { stream: !done });

    let splitIndex = buffer.indexOf("\n\n");
    while (splitIndex >= 0) {
      const block = buffer.slice(0, splitIndex);
      buffer = buffer.slice(splitIndex + 2);
      parseEventBlock(block);
      splitIndex = buffer.indexOf("\n\n");
    }

    if (done) {
      break;
    }
  }

  if (buffer.trim().length > 0) {
    parseEventBlock(buffer);
  }
  if (!result) {
    throw new Error(
      `probe stream ended without result (events=${eventCount}, last_stage=${lastStage || "none"}, last_log=${toSingleLine(lastMessage) || "none"})`,
    );
  }
  return result;
}

export async function runKvmNodeNow(
  nodeId: string,
): Promise<KvmNodeRunNowResult> {
  const res = await api.post<KvmRunNowApiResponse>(
    `/api/admin/hypervisors/kvm/nodes/${nodeId}/run-now`,
  );
  return parseKvmNodeRunNowResult(res.data?.data);
}
