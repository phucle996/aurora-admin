import axios from "axios";

import { resolveAdminBaseURL } from "@/lib/admin-auth";

export type ModuleInstallScope = "remote";

export type ModuleInstallPayload = {
  module_name: string;
  scope: ModuleInstallScope;
  app_host: string;
  app_port?: number;
  endpoint?: string;
  install_command?: string;
  ssh_host?: string;
  ssh_port?: number;
  ssh_username?: string;
  ssh_password?: string;
  ssh_private_key?: string;
  ssh_host_key_fingerprint?: string;
};

export type ModuleInstallResult = {
  module_name: string;
  scope: string;
  endpoint: string;
  endpoint_value: string;
  install_executed: boolean;
  install_output: string;
  install_exit_code: number;
  hosts_updated: string[];
  warnings: string[];
  schema_key: string;
  schema_name: string;
  migration_files: string[];
  migration_source: string;
  healthcheck_passed: boolean;
  healthcheck_output: string;
};

export type ModuleReinstallCertPayload = {
  module_name: string;
};

export type ModuleReinstallCertResult = {
  module_name: string;
  scope: string;
  endpoint: string;
  target_host: string;
  cert_path: string;
  key_path: string;
  ca_path: string;
  warnings: string[];
  healthcheck_passed: boolean;
  healthcheck_output: string;
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

function parseModuleInstallResult(raw: unknown): ModuleInstallResult {
  const row = (raw ?? {}) as Record<string, unknown>;
  return {
    module_name: toStringValue(row.module_name),
    scope: toStringValue(row.scope),
    endpoint: toStringValue(row.endpoint),
    endpoint_value: toStringValue(row.endpoint_value),
    install_executed: Boolean(row.install_executed),
    install_output: toStringValue(row.install_output),
    install_exit_code: toNumberValue(row.install_exit_code),
    hosts_updated: toStringList(row.hosts_updated),
    warnings: toStringList(row.warnings),
    schema_key: toStringValue(row.schema_key),
    schema_name: toStringValue(row.schema_name),
    migration_files: toStringList(row.migration_files),
    migration_source: toStringValue(row.migration_source),
    healthcheck_passed: Boolean(row.healthcheck_passed),
    healthcheck_output: toStringValue(row.healthcheck_output),
  };
}

function parseModuleReinstallCertResult(raw: unknown): ModuleReinstallCertResult {
  const row = (raw ?? {}) as Record<string, unknown>;
  return {
    module_name: toStringValue(row.module_name),
    scope: toStringValue(row.scope),
    endpoint: toStringValue(row.endpoint),
    target_host: toStringValue(row.target_host),
    cert_path: toStringValue(row.cert_path),
    key_path: toStringValue(row.key_path),
    ca_path: toStringValue(row.ca_path),
    warnings: toStringList(row.warnings),
    healthcheck_passed: Boolean(row.healthcheck_passed),
    healthcheck_output: toStringValue(row.healthcheck_output),
  };
}

export async function installModule(
  payload: ModuleInstallPayload,
): Promise<ModuleInstallResult> {
  const response = await axios.post<ModuleInstallApiResponse>(
    `${resolveAdminBaseURL()}/api/v1/modules/install`,
    payload,
    {
      withCredentials: true,
      timeout: 45000,
      headers: {
        "Content-Type": "application/json",
      },
    },
  );
  return parseModuleInstallResult(response.data?.data);
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
