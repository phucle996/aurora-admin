export type KvmDetailApiResponse = {
  data?: unknown;
};

export function toStringValue(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }
  return "";
}

export function toBooleanValue(value: unknown): boolean {
  if (typeof value === "boolean") {
    return value;
  }
  if (typeof value === "string") {
    const normalized = value.trim().toLowerCase();
    return normalized === "true" || normalized === "1" || normalized === "yes";
  }
  if (typeof value === "number") {
    return value !== 0;
  }
  return false;
}

export function toNumberValue(value: unknown): number {
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

export function toTimestampValue(value: unknown): string {
  if (typeof value === "string") {
    return value;
  }
  if (typeof value === "number" && Number.isFinite(value)) {
    return new Date(value).toISOString();
  }
  return "";
}

export function resolveVMServiceBaseURL(): string {
  const fromEnv =
    import.meta.env.VITE_VM_SERVICE_BASE_URL?.toString() ??
    import.meta.env.VITE_API_URL?.toString() ??
    "";
  if (fromEnv.trim().length > 0) {
    return fromEnv.trim();
  }
  if (typeof window !== "undefined") {
    return window.location.origin;
  }
  return "";
}

