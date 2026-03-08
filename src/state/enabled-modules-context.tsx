import axios from "axios";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

import { hasAdminSession, resolveAdminBaseURL } from "@/lib/admin-auth";

const ENABLED_MODULES_CACHE_KEY = "admin-ui:enabled-modules-cache:v1";
const ENABLED_MODULES_CACHE_TTL_MS = 30 * 60 * 1000;

export type ModuleFeature = "kvm" | "docker" | "k8s" | "platform";

export type EnabledModuleItem = {
  name: string;
  status: string;
  endpoint: string;
  installed: boolean;
};

type EnabledModulesApiResponse = {
  data?: {
    items?: unknown;
    last_scan_at_unix?: unknown;
  };
  message?: string;
  error?: string;
};

type EnabledModulesCachePayload = {
  items: EnabledModuleItem[];
  lastFetchedAt: number;
  lastScanAtUnix: number;
};

type EnabledModulesStatus = "idle" | "loading" | "ready" | "error";

type EnabledModulesContextValue = {
  items: EnabledModuleItem[];
  features: ModuleFeature[];
  status: EnabledModulesStatus;
  error: string;
  lastFetchedAt: number;
  lastScanAtUnix: number;
  refreshModules: (opts?: { force?: boolean }) => Promise<EnabledModuleItem[]>;
  clearModules: () => void;
  isFeatureEnabled: (feature: ModuleFeature) => boolean;
};

const EnabledModulesContext = createContext<EnabledModulesContextValue | undefined>(
  undefined,
);

function normalizeModuleItems(raw: unknown): EnabledModuleItem[] {
  if (!Array.isArray(raw)) {
    return [];
  }

  const out: EnabledModuleItem[] = [];
  const seen = new Set<string>();

  for (const item of raw) {
    if (!item || typeof item !== "object") {
      continue;
    }
    const maybe = item as {
      name?: unknown;
      status?: unknown;
      endpoint?: unknown;
      installed?: unknown;
    };
    const name = typeof maybe.name === "string" ? maybe.name.trim() : "";
    const status =
      typeof maybe.status === "string" ? maybe.status.trim().toLowerCase() : "";
    const endpoint =
      typeof maybe.endpoint === "string" ? maybe.endpoint.trim() : "";
    const installedByField = typeof maybe.installed === "boolean" ? maybe.installed : false;
    const installed = installedByField || endpoint.length > 0;
    const normalizedStatus = status || (installed ? "installed" : "not_installed");
    if (!name) {
      continue;
    }
    const key = `${name}|${normalizedStatus}|${endpoint}|${installed}`;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    out.push({ name, status: normalizedStatus, endpoint, installed });
  }

  out.sort((a, b) => a.name.localeCompare(b.name));
  return out;
}

function parseUnix(raw: unknown): number {
  if (typeof raw === "number" && Number.isFinite(raw) && raw > 0) {
    return Math.floor(raw);
  }
  if (typeof raw === "string") {
    const value = Number.parseInt(raw.trim(), 10);
    if (Number.isFinite(value) && value > 0) {
      return value;
    }
  }
  return 0;
}

function buildFeatures(items: EnabledModuleItem[]): ModuleFeature[] {
  const matched = new Set<ModuleFeature>();

  for (const item of items) {
    if (!item.installed) {
      continue;
    }
    const text = `${item.name} ${item.endpoint}`.toLowerCase();

    if (
      text.includes("kvm") ||
      text.includes("vm-service") ||
      text.includes("hypervisor") ||
      text.includes("libvirt")
    ) {
      matched.add("kvm");
    }
    if (text.includes("docker")) {
      matched.add("docker");
    }
    if (text.includes("k8s") || text.includes("kubernetes")) {
      matched.add("k8s");
    }
    if (text.includes("platform-resource") || text.includes("platform")) {
      matched.add("platform");
    }
  }

  return Array.from(matched.values());
}

function loadCacheFromStorage(): EnabledModulesCachePayload | null {
  if (typeof window === "undefined") {
    return null;
  }

  const raw = localStorage.getItem(ENABLED_MODULES_CACHE_KEY);
  if (!raw) {
    return null;
  }

  try {
    const parsed = JSON.parse(raw) as Partial<EnabledModulesCachePayload>;
    const items = normalizeModuleItems(parsed.items);
    const lastFetchedAt =
      typeof parsed.lastFetchedAt === "number" && parsed.lastFetchedAt > 0
        ? parsed.lastFetchedAt
        : 0;
    const lastScanAtUnix = parseUnix(parsed.lastScanAtUnix);

    return { items, lastFetchedAt, lastScanAtUnix };
  } catch {
    return null;
  }
}

function saveCacheToStorage(payload: EnabledModulesCachePayload): void {
  if (typeof window === "undefined") {
    return;
  }
  localStorage.setItem(ENABLED_MODULES_CACHE_KEY, JSON.stringify(payload));
}

function clearCacheFromStorage(): void {
  if (typeof window === "undefined") {
    return;
  }
  localStorage.removeItem(ENABLED_MODULES_CACHE_KEY);
}

function resolveErrorMessage(error: unknown): string {
  if (axios.isAxiosError(error)) {
    const payload = error.response?.data as
      | { message?: string; error?: string }
      | undefined;
    return payload?.error || payload?.message || error.message || "Request failed";
  }
  if (error instanceof Error) {
    return error.message || "Request failed";
  }
  return "Request failed";
}

async function requestEnabledModules(): Promise<{
  items: EnabledModuleItem[];
  lastScanAtUnix: number;
}> {
  const response = await axios.get<EnabledModulesApiResponse>(
    `${resolveAdminBaseURL()}/api/v1/modules/status`,
    {
      withCredentials: true,
      timeout: 20000,
    },
  );

  const items = normalizeModuleItems(response.data?.data?.items);
  const lastScanAtUnix = parseUnix(response.data?.data?.last_scan_at_unix);
  return { items, lastScanAtUnix };
}

export function EnabledModulesProvider({ children }: { children: ReactNode }) {
  const cached = useMemo(() => loadCacheFromStorage(), []);

  const [items, setItems] = useState<EnabledModuleItem[]>(cached?.items ?? []);
  const [lastFetchedAt, setLastFetchedAt] = useState<number>(
    cached?.lastFetchedAt ?? 0,
  );
  const [lastScanAtUnix, setLastScanAtUnix] = useState<number>(
    cached?.lastScanAtUnix ?? 0,
  );
  const [status, setStatus] = useState<EnabledModulesStatus>(
    cached ? "ready" : "idle",
  );
  const [error, setError] = useState<string>("");

  const clearModules = useCallback(() => {
    setItems([]);
    setLastFetchedAt(0);
    setLastScanAtUnix(0);
    setStatus("idle");
    setError("");
    clearCacheFromStorage();
  }, []);

  const refreshModules = useCallback(
    async (opts?: { force?: boolean }) => {
      if (!hasAdminSession()) {
        clearModules();
        return [];
      }

      const now = Date.now();
      const force = opts?.force === true;
      if (!force && lastFetchedAt > 0 && now-lastFetchedAt < ENABLED_MODULES_CACHE_TTL_MS) {
        return items;
      }

      setStatus((prev) => (prev === "ready" ? "ready" : "loading"));
      setError("");

      try {
        const result = await requestEnabledModules();
        setItems(result.items);
        setLastFetchedAt(now);
        setLastScanAtUnix(result.lastScanAtUnix);
        setStatus("ready");
        setError("");
        saveCacheToStorage({
          items: result.items,
          lastFetchedAt: now,
          lastScanAtUnix: result.lastScanAtUnix,
        });
        return result.items;
      } catch (err) {
        setStatus("error");
        setError(resolveErrorMessage(err));
        throw err;
      }
    },
    [clearModules, items, lastFetchedAt],
  );

  useEffect(() => {
    if (!hasAdminSession()) {
      return;
    }
    const timer = window.setTimeout(() => {
      void refreshModules();
    }, 0);
    return () => window.clearTimeout(timer);
  }, [refreshModules]);

  useEffect(() => {
    if (!hasAdminSession()) {
      return;
    }
    const id = window.setInterval(() => {
      void refreshModules({ force: true });
    }, ENABLED_MODULES_CACHE_TTL_MS);
    return () => window.clearInterval(id);
  }, [refreshModules]);

  const features = useMemo(() => buildFeatures(items), [items]);
  const featureSet = useMemo(() => new Set(features), [features]);

  const value = useMemo<EnabledModulesContextValue>(
    () => ({
      items,
      features,
      status,
      error,
      lastFetchedAt,
      lastScanAtUnix,
      refreshModules,
      clearModules,
      isFeatureEnabled: (feature: ModuleFeature) => featureSet.has(feature),
    }),
    [
      clearModules,
      error,
      featureSet,
      features,
      items,
      lastFetchedAt,
      lastScanAtUnix,
      refreshModules,
      status,
    ],
  );

  return (
    <EnabledModulesContext.Provider value={value}>
      {children}
    </EnabledModulesContext.Provider>
  );
}

// eslint-disable-next-line react-refresh/only-export-components
export function useEnabledModules() {
  const ctx = useContext(EnabledModulesContext);
  if (!ctx) {
    throw new Error("useEnabledModules must be used within EnabledModulesProvider");
  }
  return ctx;
}
