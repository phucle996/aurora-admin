import api from "@/lib/api";
import {
  type KvmDetailApiResponse,
  resolveVMServiceBaseURL,
  toNumberValue,
  toStringValue,
  toTimestampValue,
} from "@/hooks/kvm-detail/kvm-detail-api.helpers";

export type KvmNodeLiteMetric = {
  nodeId: string;
  timestamp: string;
  timestampUnix: number;
  cpu: {
    total: number;
    user: number;
    system: number;
    idle: number;
    iowait: number;
  };
  memory: {
    totalBytes: number;
    availableBytes: number;
    usedBytes: number;
    swapTotalBytes: number;
    swapUsedBytes: number;
  };
  disk: {
    readBytes: number;
    writeBytes: number;
    readIos: number;
    writeIos: number;
    ioTimeMs: number;
  };
  network: {
    rxBytes: number;
    txBytes: number;
    rxPackets: number;
    txPackets: number;
  };
  load: {
    load1: number;
    load5: number;
    load15: number;
  };
  uptimeSeconds: number;
};

export type KvmNodeRealtimeMetrics = {
  nodeId: string;
  from: string;
  to: string;
  bucket: string;
  live: boolean;
  nodeSeries: KvmNodeLiteMetric[];
};

export type KvmNodeRealtimeMetricsFilter = {
  from: string;
  to: string;
  bucket?: "auto" | "3s" | "1m" | "5m";
  targetPoints?: number;
};

export type KvmNodeHardwareCpuInfo = {
  model: string;
  vendor: string;
  cores: number;
  threads: number;
  cacheL1: string;
  cacheL2: string;
  cacheL3: string;
  architecture: string;
  virtualizationSupport: string;
  baseMhz: number;
  burstMhz: number;
};

export type KvmNodeHardwareRamStick = {
  slot: string;
  model: string;
  serial: string;
  manufacturer: string;
};

export type KvmNodeHardwareDisk = {
  name: string;
  model: string;
  serial: string;
  filesystem: string;
  mountpoint: string;
};

export type KvmNodeHardwareNetIf = {
  name: string;
  macAddress: string;
  driver: string;
  pciAddress: string;
};

export type KvmNodeHardwareGpu = {
  index: number;
  vendor: string;
  model: string;
  uuid: string;
  memoryTotalBytes: number;
};

export type KvmNodeHardwareInfo = {
  nodeId: string;
  collectedAt: string;
  cpu: KvmNodeHardwareCpuInfo;
  ramSticks: KvmNodeHardwareRamStick[];
  disks: KvmNodeHardwareDisk[];
  netifs: KvmNodeHardwareNetIf[];
  gpus: KvmNodeHardwareGpu[];
};

function toRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === "object" && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return {};
}

function toArray(value: unknown): unknown[] {
  if (Array.isArray(value)) {
    return value;
  }
  return [];
}

function toUint(value: unknown): number {
  const raw = toNumberValue(value);
  if (raw <= 0) {
    return 0;
  }
  return Math.floor(raw);
}

function normalizeUnixMillis(value: unknown): number {
  const raw = toNumberValue(value);
  if (raw <= 0) {
    return 0;
  }
  if (raw >= 1_000_000_000_000_000_000) {
    return Math.floor(raw / 1_000_000);
  }
  if (raw >= 1_000_000_000_000_000) {
    return Math.floor(raw / 1_000);
  }
  if (raw >= 1_000_000_000_000) {
    return Math.floor(raw);
  }
  if (raw >= 1_000_000_000) {
    return Math.floor(raw * 1000);
  }
  return 0;
}

function timestampFromUnixMillis(value: number): string {
  if (value <= 0) {
    return "";
  }
  return new Date(value).toISOString();
}

function parseNodeLiteMetric(
  value: unknown,
  fallbackNodeID = "",
  fallbackTimestampUnix = 0,
): KvmNodeLiteMetric | null {
  const row = toRecord(value);
  const nodeID =
    toStringValue(row.node_id).trim() || toStringValue(fallbackNodeID).trim();

  const timestampUnix =
    normalizeUnixMillis(row.timestamp) ||
    normalizeUnixMillis(row.timestamp_unix) ||
    (fallbackTimestampUnix > 0 ? Math.floor(fallbackTimestampUnix) : 0);

  const timestamp =
    timestampFromUnixMillis(timestampUnix) || toTimestampValue(row.timestamp);

  if (!nodeID || !timestamp) {
    return null;
  }

  const cpu = toRecord(row.cpu);
  const memory = toRecord(row.memory);
  const disk = toRecord(row.disk);
  const network = toRecord(row.network);
  const load = toRecord(row.load);

  return {
    nodeId: nodeID,
    timestamp,
    timestampUnix,
    cpu: {
      total: toUint(cpu.total),
      user: toUint(cpu.user),
      system: toUint(cpu.system),
      idle: toUint(cpu.idle),
      iowait: toUint(cpu.iowait),
    },
    memory: {
      totalBytes: toUint(memory.total_bytes),
      availableBytes: toUint(memory.available_bytes),
      usedBytes: toUint(memory.used_bytes),
      swapTotalBytes: toUint(memory.swap_total_bytes),
      swapUsedBytes: toUint(memory.swap_used_bytes),
    },
    disk: {
      readBytes: toUint(disk.read_bytes),
      writeBytes: toUint(disk.write_bytes),
      readIos: toUint(disk.read_ios),
      writeIos: toUint(disk.write_ios),
      ioTimeMs: toUint(disk.io_time_ms),
    },
    network: {
      rxBytes: toUint(network.rx_bytes),
      txBytes: toUint(network.tx_bytes),
      rxPackets: toUint(network.rx_packets),
      txPackets: toUint(network.tx_packets),
    },
    load: {
      load1: toNumberValue(load.load1),
      load5: toNumberValue(load.load5),
      load15: toNumberValue(load.load15),
    },
    uptimeSeconds: toUint(row.uptime_seconds),
  };
}

function parseKvmNodeRealtimeMetrics(item: unknown): KvmNodeRealtimeMetrics {
  const row = toRecord(item);
  const nodeID = toStringValue(row.node_id).trim();
  const series = toArray(row.series)
    .map((entry) => parseNodeLiteMetric(entry, nodeID))
    .filter((entry): entry is KvmNodeLiteMetric => entry !== null);

  return {
    nodeId: nodeID,
    from: toTimestampValue(row.from),
    to: toTimestampValue(row.to),
    bucket: toStringValue(row.bucket).trim() || "auto",
    live: Boolean(row.live),
    nodeSeries: series,
  };
}

function parseKvmNodeHardwareInfo(item: unknown): KvmNodeHardwareInfo {
  const row = toRecord(item);
  const cpuRaw = toRecord(row.cpu);

  return {
    nodeId: toStringValue(row.node_id),
    collectedAt: toTimestampValue(row.collected_at),
    cpu: {
      model: toStringValue(cpuRaw.model),
      vendor: toStringValue(cpuRaw.vendor),
      cores: toUint(cpuRaw.cores),
      threads: toUint(cpuRaw.threads),
      cacheL1: toStringValue(cpuRaw.cache_l1),
      cacheL2: toStringValue(cpuRaw.cache_l2),
      cacheL3: toStringValue(cpuRaw.cache_l3),
      architecture: toStringValue(cpuRaw.architecture),
      virtualizationSupport: toStringValue(cpuRaw.virtualization_support),
      baseMhz: toNumberValue(cpuRaw.base_mhz),
      burstMhz: toNumberValue(cpuRaw.burst_mhz),
    },
    ramSticks: toArray(row.ram_sticks).map((entry) => {
      const stick = toRecord(entry);
      return {
        slot: toStringValue(stick.slot),
        model: toStringValue(stick.model),
        serial: toStringValue(stick.serial),
        manufacturer: toStringValue(stick.manufacturer),
      };
    }),
    disks: toArray(row.disks).map((entry) => {
      const disk = toRecord(entry);
      return {
        name: toStringValue(disk.name),
        model: toStringValue(disk.model),
        serial: toStringValue(disk.serial),
        filesystem: toStringValue(disk.filesystem),
        mountpoint: toStringValue(disk.mountpoint),
      };
    }),
    netifs: toArray(row.netifs).map((entry) => {
      const netif = toRecord(entry);
      return {
        name: toStringValue(netif.name),
        macAddress: toStringValue(netif.mac_address),
        driver: toStringValue(netif.driver),
        pciAddress: toStringValue(netif.pci_address),
      };
    }),
    gpus: toArray(row.gpus).map((entry) => {
      const gpu = toRecord(entry);
      return {
        index: toUint(gpu.index),
        vendor: toStringValue(gpu.vendor),
        model: toStringValue(gpu.model),
        uuid: toStringValue(gpu.uuid),
        memoryTotalBytes: toUint(gpu.memory_total_bytes),
      };
    }),
  };
}

export async function getKvmNodeRealtimeMetrics(
  nodeId: string,
  filter: KvmNodeRealtimeMetricsFilter,
  signal?: AbortSignal,
): Promise<KvmNodeRealtimeMetrics> {
  const res = await api.get<KvmDetailApiResponse>(
    `/api/metrics/node/${encodeURIComponent(nodeId)}`,
    {
      params: {
        from: filter.from,
        to: filter.to,
        bucket: filter.bucket ?? "auto",
        target_points:
          Number.isFinite(filter.targetPoints) && (filter.targetPoints ?? 0) > 0
            ? Math.floor(filter.targetPoints as number)
            : undefined,
      },
      signal,
    },
  );

  return parseKvmNodeRealtimeMetrics(res.data?.data);
}

export async function getKvmNodeHardwareInfo(
  nodeId: string,
  signal?: AbortSignal,
): Promise<KvmNodeHardwareInfo> {
  const res = await api.get<KvmDetailApiResponse>(
    `/api/admin/hypervisors/kvm/${encodeURIComponent(nodeId)}/hardware-info`,
    { signal },
  );

  return parseKvmNodeHardwareInfo(res.data?.data);
}

function buildKvmMetricsWSURL(nodeId: string): string {
  const base = resolveVMServiceBaseURL();
  const url = /^https?:\/\//i.test(base)
    ? new URL(base)
    : new URL(base || "/", window.location.origin);

  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";

  const basePath = url.pathname.replace(/\/+$/, "");
  url.pathname = `${basePath === "" || basePath === "/" ? "" : basePath}/ws/${encodeURIComponent(
    nodeId,
  )}/lite`;
  url.search = "";
  return url.toString();
}

type RealtimeHandlers = {
  onMessage?: (event: KvmNodeLiteMetric) => void;
  onOpen?: () => void;
  onClose?: () => void;
  onError?: () => void;
};

function parseRealtimeMetric(
  frame: unknown,
  fallbackNodeID: string,
): KvmNodeLiteMetric | null {
  const row = toRecord(frame);
  const metrics = toRecord(row.metrics);

  const raw = Object.keys(metrics).length > 0 ? metrics : row;
  return parseNodeLiteMetric(
    raw,
    toStringValue(row.node_id) || fallbackNodeID,
    normalizeUnixMillis(row.timestamp) || normalizeUnixMillis(row.timestamp_unix),
  );
}

function normalizeRealtimeEvents(
  payload: unknown,
  fallbackNodeID: string,
): KvmNodeLiteMetric[] {
  if (Array.isArray(payload)) {
    return payload
      .map((item) => parseRealtimeMetric(item, fallbackNodeID))
      .filter((item): item is KvmNodeLiteMetric => item !== null);
  }

  const parsed = parseRealtimeMetric(payload, fallbackNodeID);
  return parsed ? [parsed] : [];
}

export function subscribeKvmRealtimeMetrics(
  nodeId: string,
  handlers: RealtimeHandlers = {},
): () => void {
  const normalizedNodeID = nodeId.trim();
  if (!normalizedNodeID || typeof window === "undefined") {
    return () => {};
  }

  const ws = new WebSocket(buildKvmMetricsWSURL(normalizedNodeID));

  ws.onopen = () => {
    handlers.onOpen?.();
  };

  ws.onmessage = (rawEvent) => {
    try {
      const payload = JSON.parse(rawEvent.data) as unknown;
      const events = normalizeRealtimeEvents(payload, normalizedNodeID);
      for (const event of events) {
        handlers.onMessage?.(event);
      }
    } catch {
      // Ignore malformed realtime frames.
    }
  };

  ws.onerror = () => {
    handlers.onError?.();
  };

  ws.onclose = () => {
    handlers.onClose?.();
  };

  return () => {
    if (
      ws.readyState === WebSocket.OPEN ||
      ws.readyState === WebSocket.CONNECTING
    ) {
      ws.close();
    }
  };
}
