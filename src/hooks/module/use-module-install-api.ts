import axios from "axios";

import { resolveAdminBaseURL } from "@/lib/admin-auth";

export type ModuleInstallScope = "local" | "remote";

export type ModuleInstallPayload = {
  module_name: string;
  scope: ModuleInstallScope;
  app_host: string;
  endpoint: string;
  install_command?: string;
  ssh_host?: string;
  ssh_port?: number;
  ssh_username?: string;
  ssh_password?: string;
  ssh_private_key?: string;
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
