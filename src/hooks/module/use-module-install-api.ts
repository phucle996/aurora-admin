import { resolveAdminBaseURL } from "@/lib/admin-auth";

export type ModuleInstallPayload = {
  module_name: string;
  agent_id: string;
  app_host: string;
};

export type ModuleInstallAgent = {
  agent_id: string;
  status: string;
  hostname: string;
  agent_grpc_endpoint: string;
};

export type AgentBootstrapTokenResponse = {
  token: string;
  token_hash: string;
  cluster_policy: string;
};

export type AgentInstallBootstrapMetadata = {
  admin_grpc_endpoint: string;
  admin_server_name: string;
  admin_grpc_port: number;
};

export type ModuleInstallResult = {
  operation_id?: string;
  module_name: string;
  agent_id?: string;
  version?: string;
  artifact_checksum?: string;
  service_name?: string;
  endpoint: string;
  health?: string;
  hosts_updated: string[];
  warnings: string[];
};

export type ModuleInstallOperationSummary = {
  operation_id: string;
  agent_id: string;
  module: string;
  version: string;
  service_name: string;
  artifact_checksum: string;
  app_host: string;
  endpoint: string;
  status: string;
  health: string;
  last_stage: string;
  last_message: string;
  error_text: string;
  started_at: string;
  updated_at: string;
  completed_at: string;
};

export type ModuleInstallOperationEvent = {
  operation_id: string;
  sequence: number;
  type: string;
  stage: string;
  message: string;
  observed_at: string;
};

export type ModuleReinstallCertPayload = {
  module_name: string;
};

export type ModuleReinstallCertResult = {
  module_name: string;
  endpoint: string;
  warnings: string[];
  healthcheck_passed: boolean;
};

type ModuleInstallApiResponse = {
  data?: unknown;
  message?: string;
  error?: string;
};

function toStringValue(v: unknown): string {
  return typeof v === "string" ? v : "";
}

function toNumberValue(v: unknown): number {
  if (typeof v === "number" && Number.isFinite(v)) {
    return v;
  }
  if (typeof v === "string") {
    const parsed = Number(v);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  return 0;
}

function toStringList(v: unknown): string[] {
  if (!Array.isArray(v)) {
    return [];
  }
  return v
    .map((item) => toStringValue(item).trim())
    .filter((item) => item.length > 0);
}

function parseModuleInstallAgent(raw: unknown): ModuleInstallAgent {
  const row = (raw ?? {}) as Record<string, unknown>;
  return {
    agent_id: toStringValue(row.agent_id),
    status: toStringValue(row.status),
    hostname: toStringValue(row.hostname),
    agent_grpc_endpoint: toStringValue(row.agent_grpc_endpoint),
  };
}

export async function listModuleInstallAgents(): Promise<ModuleInstallAgent[]> {
  const baseURL = resolveAdminBaseURL();
  const path = "/api/v1/modules/install/agents";
  const url = baseURL ? new URL(path, baseURL).toString() : path;

  const res = await fetch(url, {
    method: "GET",
    headers: {
      Accept: "application/json",
    },
    credentials: "include",
  });

  const text = await res.text();
  let parsed: ModuleInstallApiResponse | null = null;
  try {
    parsed = JSON.parse(text) as ModuleInstallApiResponse;
  } catch {
    parsed = null;
  }

  if (!res.ok) {
    const detail = toStringValue(parsed?.message ?? parsed?.error) || toSingleLine(text);
    throw new Error(`Load install agents failed (HTTP ${res.status} ${res.statusText}): ${detail || "empty response"}`);
  }

  const body = (parsed?.data ?? {}) as Record<string, unknown>;
  const rows = Array.isArray(body.items) ? body.items : [];
  return rows.map((item) => parseModuleInstallAgent(item));
}

export async function rotateAgentBootstrapToken(): Promise<AgentBootstrapTokenResponse> {
  const baseURL = resolveAdminBaseURL();
  const path = "/api/v1/modules/install/agent-bootstrap-token";
  const url = baseURL ? new URL(path, baseURL).toString() : path;

  const res = await fetch(url, {
    method: "POST",
    headers: {
      Accept: "application/json",
    },
    credentials: "include",
  });

  const text = await res.text();
  let parsed: ModuleInstallApiResponse | null = null;
  try {
    parsed = JSON.parse(text) as ModuleInstallApiResponse;
  } catch {
    parsed = null;
  }

  if (!res.ok) {
    const detail = toStringValue(parsed?.message ?? parsed?.error) || toSingleLine(text);
    throw new Error(`Rotate bootstrap token failed (HTTP ${res.status} ${res.statusText}): ${detail || "empty response"}`);
  }

  const body = (parsed?.data ?? {}) as Record<string, unknown>;
  return {
    token: toStringValue(body.token),
    token_hash: toStringValue(body.token_hash),
    cluster_policy: toStringValue(body.cluster_policy),
  };
}

export async function getAgentInstallBootstrapMetadata(): Promise<AgentInstallBootstrapMetadata> {
  const baseURL = resolveAdminBaseURL();
  const path = "/api/v1/modules/install/agent-bootstrap-metadata";
  const url = baseURL ? new URL(path, baseURL).toString() : path;

  const res = await fetch(url, {
    method: "GET",
    headers: {
      Accept: "application/json",
    },
    credentials: "include",
  });

  const text = await res.text();
  let parsed: ModuleInstallApiResponse | null = null;
  try {
    parsed = JSON.parse(text) as ModuleInstallApiResponse;
  } catch {
    parsed = null;
  }

  if (!res.ok) {
    const detail = toStringValue(parsed?.message ?? parsed?.error) || toSingleLine(text);
    throw new Error(`Load agent bootstrap metadata failed (HTTP ${res.status} ${res.statusText}): ${detail || "empty response"}`);
  }

  const body = (parsed?.data ?? {}) as Record<string, unknown>;
  return {
    admin_grpc_endpoint: toStringValue(body.admin_grpc_endpoint),
    admin_server_name: toStringValue(body.admin_server_name),
    admin_grpc_port: toNumberValue(body.admin_grpc_port),
  };
}

function parseModuleInstallResult(raw: unknown): ModuleInstallResult {
  const row = (raw ?? {}) as Record<string, unknown>;
  return {
    operation_id: toStringValue(row.operation_id) || undefined,
    module_name: toStringValue(row.module_name),
    agent_id: toStringValue(row.agent_id) || undefined,
    version: toStringValue(row.version) || undefined,
    artifact_checksum: toStringValue(row.artifact_checksum) || undefined,
    service_name: toStringValue(row.service_name) || undefined,
    endpoint: toStringValue(row.endpoint),
    health: toStringValue(row.health) || undefined,
    hosts_updated: toStringList(row.hosts_updated),
    warnings: toStringList(row.warnings),
  };
}

function parseModuleReinstallCertResult(raw: unknown): ModuleReinstallCertResult {
  const row = (raw ?? {}) as Record<string, unknown>;
  return {
    module_name: toStringValue(row.module_name),
    endpoint: toStringValue(row.endpoint),
    warnings: toStringList(row.warnings),
    healthcheck_passed: Boolean(row.healthcheck_passed),
  };
}

function parseModuleInstallOperationSummary(raw: unknown): ModuleInstallOperationSummary {
  const row = (raw ?? {}) as Record<string, unknown>;
  return {
    operation_id: toStringValue(row.operation_id),
    agent_id: toStringValue(row.agent_id),
    module: toStringValue(row.module),
    version: toStringValue(row.version),
    service_name: toStringValue(row.service_name),
    artifact_checksum: toStringValue(row.artifact_checksum),
    app_host: toStringValue(row.app_host),
    endpoint: toStringValue(row.endpoint),
    status: toStringValue(row.status),
    health: toStringValue(row.health),
    last_stage: toStringValue(row.last_stage),
    last_message: toStringValue(row.last_message),
    error_text: toStringValue(row.error_text),
    started_at: toStringValue(row.started_at),
    updated_at: toStringValue(row.updated_at),
    completed_at: toStringValue(row.completed_at),
  };
}

function parseModuleInstallOperationEvent(raw: unknown): ModuleInstallOperationEvent {
  const row = (raw ?? {}) as Record<string, unknown>;
  return {
    operation_id: toStringValue(row.operation_id),
    sequence: toNumberValue(row.sequence),
    type: toStringValue(row.type),
    stage: toStringValue(row.stage),
    message: toStringValue(row.message),
    observed_at: toStringValue(row.observed_at),
  };
}

export async function getModuleInstallOperation(operationID: string): Promise<{
  summary: ModuleInstallOperationSummary;
  events: ModuleInstallOperationEvent[];
}> {
  const baseURL = resolveAdminBaseURL();
  const path = `/api/v1/modules/install/operations/${encodeURIComponent(operationID)}`;
  const url = baseURL ? new URL(path, baseURL).toString() : path;

  const res = await fetch(url, {
    method: "GET",
    headers: {
      Accept: "application/json",
    },
    credentials: "include",
  });

  const text = await res.text();
  let parsed: ModuleInstallApiResponse | null = null;
  try {
    parsed = JSON.parse(text) as ModuleInstallApiResponse;
  } catch {
    parsed = null;
  }

  if (!res.ok) {
    const detail = toStringValue(parsed?.message ?? parsed?.error) || toSingleLine(text);
    throw new Error(
      `Load install operation failed (HTTP ${res.status} ${res.statusText}): ${detail || "empty response"}`,
    );
  }

  const body = (parsed?.data ?? {}) as Record<string, unknown>;
  const rows = Array.isArray(body.events) ? body.events : [];
  return {
    summary: parseModuleInstallOperationSummary(body.summary),
    events: rows.map((item) => parseModuleInstallOperationEvent(item)),
  };
}

type ModuleInstallStreamOptions = {
  signal?: AbortSignal;
  onLog?: (stage: string, message: string) => void;
};

function toSingleLine(value: string): string {
  return value.replace(/\s+/g, " ").trim();
}

function extractErrorMessage(error: unknown): string {
  if (error instanceof Error && error.message.trim()) {
    return error.message;
  }
  if (typeof error === "string" && error.trim()) {
    return error;
  }
  return "unknown error";
}

export async function installModuleStream(
  payload: ModuleInstallPayload,
  options?: ModuleInstallStreamOptions,
): Promise<ModuleInstallResult> {
  const baseURL = resolveAdminBaseURL();
  const streamPath = "/api/v1/modules/install/stream";
  const url = baseURL ? new URL(streamPath, baseURL).toString() : streamPath;

  const res = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "text/event-stream",
    },
    credentials: "include",
    body: JSON.stringify(payload),
    signal: options?.signal,
  });

  if (!res.ok) {
    const text = await res.text();
    let detail = toSingleLine(text);
    try {
      const parsed = JSON.parse(text) as ModuleInstallApiResponse;
      detail = toStringValue(parsed.message ?? parsed.error) || detail;
    } catch {
      // keep raw body detail
    }
    throw new Error(
      `Install stream failed (HTTP ${res.status} ${res.statusText}): ${detail || "empty response"}`,
    );
  }
  if (!res.body) {
    throw new Error("Install stream is empty");
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let result: ModuleInstallResult | null = null;
  let eventCount = 0;
  let lastStage = "";
  let lastMessage = "";

  const parseEventBlock = (block: string) => {
    const dataLines = block
      .split("\n")
      .filter((line) => line.startsWith("data:"))
      .map((line) => line.slice(5).trimStart());
    if (dataLines.length === 0) {
      return;
    }

    const rawPayload = dataLines.join("\n");
    const evt = JSON.parse(rawPayload) as {
      type?: string;
      stage?: string;
      message?: string;
      data?: unknown;
    };

    if (evt.type === "log") {
      lastStage = toStringValue(evt.stage) || "log";
      lastMessage = toStringValue(evt.message);
      eventCount += 1;
      options?.onLog?.(lastStage, lastMessage);
      return;
    }

    if (evt.type === "error") {
      const stage = toStringValue(evt.stage) || "service";
      const message = toStringValue(evt.message) || "module install failed";
      lastStage = stage;
      lastMessage = message;
      eventCount += 1;
      options?.onLog?.(stage, `[error] ${message}`);
      throw new Error(`backend stream error (stage=${stage}): ${message}`);
    }

    if (evt.type === "result") {
      lastStage = toStringValue(evt.stage) || "service";
      lastMessage = toStringValue(evt.message) || "module install completed";
      eventCount += 1;
      result = parseModuleInstallResult(evt.data);
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
        `install stream read failed after ${eventCount} events (last_stage=${lastStage || "none"}, last_log=${toSingleLine(lastMessage) || "none"}): ${reason}`,
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
      `install stream ended without result (events=${eventCount}, last_stage=${lastStage || "none"}, last_log=${toSingleLine(lastMessage) || "none"})`,
    );
  }
  return result;
}

type ModuleReinstallCertStreamOptions = {
  signal?: AbortSignal;
  onLog?: (stage: string, message: string) => void;
};

export async function reinstallModuleCertStream(
  payload: ModuleReinstallCertPayload,
  options?: ModuleReinstallCertStreamOptions,
): Promise<ModuleReinstallCertResult> {
  const baseURL = resolveAdminBaseURL();
  const streamPath = "/api/v1/modules/reinstall-cert/stream";
  const url = baseURL ? new URL(streamPath, baseURL).toString() : streamPath;

  const res = await fetch(url, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Accept: "text/event-stream",
    },
    credentials: "include",
    body: JSON.stringify(payload),
    signal: options?.signal,
  });

  if (!res.ok) {
    const text = await res.text();
    let detail = toSingleLine(text);
    try {
      const parsed = JSON.parse(text) as ModuleInstallApiResponse;
      detail = toStringValue(parsed.message ?? parsed.error) || detail;
    } catch {
      // keep raw body detail
    }
    throw new Error(
      `Reinstall cert stream failed (HTTP ${res.status} ${res.statusText}): ${detail || "empty response"}`,
    );
  }
  if (!res.body) {
    throw new Error("Reinstall cert stream is empty");
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  let result: ModuleReinstallCertResult | null = null;
  let eventCount = 0;
  let lastStage = "";
  let lastMessage = "";

  const parseEventBlock = (block: string) => {
    const dataLines = block
      .split("\n")
      .filter((line) => line.startsWith("data:"))
      .map((line) => line.slice(5).trimStart());
    if (dataLines.length === 0) {
      return;
    }

    const rawPayload = dataLines.join("\n");
    const evt = JSON.parse(rawPayload) as {
      type?: string;
      stage?: string;
      message?: string;
      data?: unknown;
    };

    if (evt.type === "log") {
      lastStage = toStringValue(evt.stage) || "log";
      lastMessage = toStringValue(evt.message);
      eventCount += 1;
      options?.onLog?.(lastStage, lastMessage);
      return;
    }

    if (evt.type === "error") {
      const stage = toStringValue(evt.stage) || "service";
      const message = toStringValue(evt.message) || "module reinstall cert failed";
      lastStage = stage;
      lastMessage = message;
      eventCount += 1;
      options?.onLog?.(stage, `[error] ${message}`);
      throw new Error(`backend stream error (stage=${stage}): ${message}`);
    }

    if (evt.type === "result") {
      lastStage = toStringValue(evt.stage) || "service";
      lastMessage = toStringValue(evt.message) || "module cert reinstalled";
      eventCount += 1;
      result = parseModuleReinstallCertResult(evt.data);
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
        `reinstall cert stream read failed after ${eventCount} events (last_stage=${lastStage || "none"}, last_log=${toSingleLine(lastMessage) || "none"}): ${reason}`,
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
      `reinstall cert stream ended without result (events=${eventCount}, last_stage=${lastStage || "none"}, last_log=${toSingleLine(lastMessage) || "none"})`,
    );
  }
  return result;
}
