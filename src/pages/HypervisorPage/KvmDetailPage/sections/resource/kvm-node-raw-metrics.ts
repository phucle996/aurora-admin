import type {
  KvmNodeHardwareInfo,
  KvmNodeLiteMetric,
} from "@/hooks/kvm-detail/use-kvm-node-metrics-api";
import type { KvmNodeHistoryRow } from "@/pages/HypervisorPage/KvmDetailPage/sections/KvmNodeHistorySection";

export const MAX_POINTS = 1800;

export type ChartRangePreset =
  | "5m"
  | "15m"
  | "1h"
  | "6h"
  | "24h"
  | "custom";

export type ChartDateRange = {
  from: Date | null;
  to: Date | null;
};

export const CHART_RANGE_OPTIONS: Array<{
  value: ChartRangePreset;
  label: string;
}> = [
  { value: "5m", label: "5 minutes" },
  { value: "15m", label: "15 minutes" },
  { value: "1h", label: "1 hour" },
  { value: "6h", label: "6 hours" },
  { value: "24h", label: "24 hours" },
  { value: "custom", label: "Custom" },
];

export type KvmCpuChartSample = {
  timestamp: string;
  cpuTotal: number;
  cpuUser: number;
  cpuSystem: number;
  cpuIdle: number;
  cpuIowait: number;
  load1: number;
  load5: number;
  load15: number;
  uptimeSeconds: number;
  cpuCores: number;
};

export type KvmRamChartSample = {
  timestamp: string;
  totalBytes: number;
  availableBytes: number;
  usedBytes: number;
  swapTotalBytes: number;
  swapUsedBytes: number;
};

export type KvmDiskChartSample = {
  timestamp: string;
  readBytes: number;
  writeBytes: number;
  readIos: number;
  writeIos: number;
  ioTimeMs: number;
};

export type KvmNetworkChartSample = {
  timestamp: string;
  rxBytes: number;
  txBytes: number;
  rxPackets: number;
  txPackets: number;
};

type BuildNodeRawMetricsViewArgs = {
  series: KvmNodeLiteMetric[];
  hardwareInfo: KvmNodeHardwareInfo | null;
  cpuChartRange: ChartRangePreset;
  cpuCustomRange: ChartDateRange;
  ramChartRange: ChartRangePreset;
  ramCustomRange: ChartDateRange;
  diskChartRange: ChartRangePreset;
  diskCustomRange: ChartDateRange;
  networkChartRange: ChartRangePreset;
  networkCustomRange: ChartDateRange;
};

type NodeRawMetricsView = {
  latestNodeMetric: KvmNodeLiteMetric | null;
  runtimeLatencyMs: number;
  historyRows: KvmNodeHistoryRow[];
  cpuChartSamples: KvmCpuChartSample[];
  cpuModelName: string;
  ramChartSamples: KvmRamChartSample[];
  diskChartSamples: KvmDiskChartSample[];
  diskCount: number;
  primaryDiskLabel: string;
  diskReadBytesCounter: number;
  diskWriteBytesCounter: number;
  diskReadIosCounter: number;
  diskWriteIosCounter: number;
  diskIoTimeMsCounter: number;
  networkChartSamples: KvmNetworkChartSample[];
  nicCount: number;
  primaryNicLabel: string;
  networkRxBytesCounter: number;
  networkTxBytesCounter: number;
  networkRxPacketsCounter: number;
  networkTxPacketsCounter: number;
  gpuCount: number;
  gpuModel: string;
  gpuMemoryTotalBytes: number;
};

function toSafeNumber(value: number): number {
  if (!Number.isFinite(value) || value < 0) {
    return 0;
  }
  return value;
}

function normalizeSeriesLimit(limit: number): number {
  if (!Number.isFinite(limit) || limit <= 0) {
    return MAX_POINTS;
  }
  return Math.max(10, Math.floor(limit));
}

function sortByTimestamp(a: KvmNodeLiteMetric, b: KvmNodeLiteMetric): number {
  const delta = a.timestampUnix - b.timestampUnix;
  if (delta !== 0) {
    return delta;
  }
  return a.timestamp.localeCompare(b.timestamp);
}

export function normalizeRawSeriesWindow(
  series: KvmNodeLiteMetric[],
  limit = MAX_POINTS,
): KvmNodeLiteMetric[] {
  if (!Array.isArray(series) || series.length === 0) {
    return [];
  }

  const sorted = series
    .filter((item) => item && item.timestampUnix > 0 && item.timestamp)
    .slice()
    .sort(sortByTimestamp);

  if (sorted.length === 0) {
    return [];
  }

  const deduped: KvmNodeLiteMetric[] = [];
  for (const sample of sorted) {
    const prev = deduped[deduped.length - 1];
    if (
      prev &&
      prev.timestampUnix === sample.timestampUnix &&
      prev.nodeId === sample.nodeId
    ) {
      deduped[deduped.length - 1] = sample;
      continue;
    }
    deduped.push(sample);
  }

  const safeLimit = normalizeSeriesLimit(limit);
  if (deduped.length <= safeLimit) {
    return deduped;
  }
  return deduped.slice(-safeLimit);
}

export function mergeRawSeries(
  previous: KvmNodeLiteMetric[],
  incoming: KvmNodeLiteMetric[],
  limit = MAX_POINTS,
): KvmNodeLiteMetric[] {
  if (!Array.isArray(incoming) || incoming.length === 0) {
    return normalizeRawSeriesWindow(previous, limit);
  }
  if (!Array.isArray(previous) || previous.length === 0) {
    return normalizeRawSeriesWindow(incoming, limit);
  }
  return normalizeRawSeriesWindow([...previous, ...incoming], limit);
}

function resolveRangeWindowMs(preset: ChartRangePreset): number {
  switch (preset) {
    case "5m":
      return 5 * 60 * 1000;
    case "15m":
      return 15 * 60 * 1000;
    case "1h":
      return 60 * 60 * 1000;
    case "6h":
      return 6 * 60 * 60 * 1000;
    case "24h":
      return 24 * 60 * 60 * 1000;
    default:
      return 0;
  }
}

function filterByChartRange(
  points: KvmNodeLiteMetric[],
  range: ChartRangePreset,
  customRange: ChartDateRange,
  anchorMs: number,
): KvmNodeLiteMetric[] {
  if (points.length === 0) {
    return [];
  }

  if (
    range === "custom" &&
    customRange.from instanceof Date &&
    customRange.to instanceof Date
  ) {
    const fromMs = customRange.from.getTime();
    const toMs = customRange.to.getTime();
    if (!Number.isFinite(fromMs) || !Number.isFinite(toMs) || fromMs > toMs) {
      return points;
    }
    return points.filter(
      (point) => point.timestampUnix >= fromMs && point.timestampUnix <= toMs,
    );
  }

  const windowMs = resolveRangeWindowMs(range);
  if (windowMs <= 0) {
    return points;
  }

  const fromMs = anchorMs - windowMs;
  return points.filter((point) => point.timestampUnix >= fromMs);
}

function toCpuSamples(
  points: KvmNodeLiteMetric[],
  hardwareInfo: KvmNodeHardwareInfo | null,
): KvmCpuChartSample[] {
  const cpuCores = Math.max(0, Math.round(hardwareInfo?.cpu?.cores ?? 0));
  return points.map((point) => ({
    timestamp: point.timestamp,
    cpuTotal: toSafeNumber(point.cpu.total),
    cpuUser: toSafeNumber(point.cpu.user),
    cpuSystem: toSafeNumber(point.cpu.system),
    cpuIdle: toSafeNumber(point.cpu.idle),
    cpuIowait: toSafeNumber(point.cpu.iowait),
    load1: Number.isFinite(point.load.load1) ? point.load.load1 : 0,
    load5: Number.isFinite(point.load.load5) ? point.load.load5 : 0,
    load15: Number.isFinite(point.load.load15) ? point.load.load15 : 0,
    uptimeSeconds: toSafeNumber(point.uptimeSeconds),
    cpuCores,
  }));
}

function toRamSamples(points: KvmNodeLiteMetric[]): KvmRamChartSample[] {
  return points.map((point) => ({
    timestamp: point.timestamp,
    totalBytes: toSafeNumber(point.memory.totalBytes),
    availableBytes: toSafeNumber(point.memory.availableBytes),
    usedBytes: toSafeNumber(point.memory.usedBytes),
    swapTotalBytes: toSafeNumber(point.memory.swapTotalBytes),
    swapUsedBytes: toSafeNumber(point.memory.swapUsedBytes),
  }));
}

function toDiskSamples(points: KvmNodeLiteMetric[]): KvmDiskChartSample[] {
  return points.map((point) => ({
    timestamp: point.timestamp,
    readBytes: toSafeNumber(point.disk.readBytes),
    writeBytes: toSafeNumber(point.disk.writeBytes),
    readIos: toSafeNumber(point.disk.readIos),
    writeIos: toSafeNumber(point.disk.writeIos),
    ioTimeMs: toSafeNumber(point.disk.ioTimeMs),
  }));
}

function toNetworkSamples(points: KvmNodeLiteMetric[]): KvmNetworkChartSample[] {
  return points.map((point) => ({
    timestamp: point.timestamp,
    rxBytes: toSafeNumber(point.network.rxBytes),
    txBytes: toSafeNumber(point.network.txBytes),
    rxPackets: toSafeNumber(point.network.rxPackets),
    txPackets: toSafeNumber(point.network.txPackets),
  }));
}

function toHistoryRows(points: KvmNodeLiteMetric[]): KvmNodeHistoryRow[] {
  if (points.length === 0) {
    return [];
  }
  return points.slice(-60).map((point) => ({
    timestamp: point.timestamp,
    load1: Number.isFinite(point.load.load1) ? point.load.load1 : 0,
    memoryUsedBytes: toSafeNumber(point.memory.usedBytes),
    diskWriteIos: toSafeNumber(point.disk.writeIos),
  }));
}

function fallbackView(): NodeRawMetricsView {
  return {
    latestNodeMetric: null,
    runtimeLatencyMs: 0,
    historyRows: [],
    cpuChartSamples: [],
    cpuModelName: "",
    ramChartSamples: [],
    diskChartSamples: [],
    diskCount: 0,
    primaryDiskLabel: "",
    diskReadBytesCounter: 0,
    diskWriteBytesCounter: 0,
    diskReadIosCounter: 0,
    diskWriteIosCounter: 0,
    diskIoTimeMsCounter: 0,
    networkChartSamples: [],
    nicCount: 0,
    primaryNicLabel: "",
    networkRxBytesCounter: 0,
    networkTxBytesCounter: 0,
    networkRxPacketsCounter: 0,
    networkTxPacketsCounter: 0,
    gpuCount: 0,
    gpuModel: "",
    gpuMemoryTotalBytes: 0,
  };
}

function resolvePrimaryDiskLabel(hardwareInfo: KvmNodeHardwareInfo | null): string {
  const firstDisk = hardwareInfo?.disks?.[0];
  if (!firstDisk) {
    return "";
  }
  return firstDisk.mountpoint || firstDisk.name || firstDisk.model || "";
}

function resolvePrimaryNicLabel(hardwareInfo: KvmNodeHardwareInfo | null): string {
  const firstNic = hardwareInfo?.netifs?.[0];
  if (!firstNic) {
    return "";
  }
  return firstNic.name || firstNic.driver || "";
}

function resolveGpuModel(hardwareInfo: KvmNodeHardwareInfo | null): string {
  const firstGpu = hardwareInfo?.gpus?.[0];
  if (!firstGpu) {
    return "";
  }
  if (firstGpu.model && firstGpu.vendor) {
    return `${firstGpu.vendor} ${firstGpu.model}`.trim();
  }
  return firstGpu.model || firstGpu.vendor || "";
}

export function buildNodeRawMetricsView(
  args: BuildNodeRawMetricsViewArgs,
): NodeRawMetricsView {
  const normalizedSeries = normalizeRawSeriesWindow(args.series, MAX_POINTS);
  const latestNodeMetric =
    normalizedSeries.length > 0 ? normalizedSeries[normalizedSeries.length - 1] : null;

  if (!latestNodeMetric) {
    return fallbackView();
  }

  const anchorMs = latestNodeMetric.timestampUnix || Date.now();

  const cpuPoints = filterByChartRange(
    normalizedSeries,
    args.cpuChartRange,
    args.cpuCustomRange,
    anchorMs,
  );
  const ramPoints = filterByChartRange(
    normalizedSeries,
    args.ramChartRange,
    args.ramCustomRange,
    anchorMs,
  );
  const diskPoints = filterByChartRange(
    normalizedSeries,
    args.diskChartRange,
    args.diskCustomRange,
    anchorMs,
  );
  const networkPoints = filterByChartRange(
    normalizedSeries,
    args.networkChartRange,
    args.networkCustomRange,
    anchorMs,
  );

  const diskLatest = diskPoints.length > 0 ? diskPoints[diskPoints.length - 1] : latestNodeMetric;
  const networkLatest =
    networkPoints.length > 0 ? networkPoints[networkPoints.length - 1] : latestNodeMetric;

  const gpus = args.hardwareInfo?.gpus ?? [];
  const gpuMemoryTotalBytes = gpus.reduce(
    (total, gpu) => total + Math.max(0, gpu.memoryTotalBytes ?? 0),
    0,
  );

  return {
    latestNodeMetric,
    runtimeLatencyMs:
      latestNodeMetric.timestampUnix > 0
        ? Math.max(0, Date.now() - latestNodeMetric.timestampUnix)
        : 0,
    historyRows: toHistoryRows(normalizedSeries),
    cpuChartSamples: toCpuSamples(cpuPoints, args.hardwareInfo),
    cpuModelName: args.hardwareInfo?.cpu?.model?.trim() ?? "",
    ramChartSamples: toRamSamples(ramPoints),
    diskChartSamples: toDiskSamples(diskPoints),
    diskCount: args.hardwareInfo?.disks?.length ?? 0,
    primaryDiskLabel: resolvePrimaryDiskLabel(args.hardwareInfo),
    diskReadBytesCounter: toSafeNumber(diskLatest.disk.readBytes),
    diskWriteBytesCounter: toSafeNumber(diskLatest.disk.writeBytes),
    diskReadIosCounter: toSafeNumber(diskLatest.disk.readIos),
    diskWriteIosCounter: toSafeNumber(diskLatest.disk.writeIos),
    diskIoTimeMsCounter: toSafeNumber(diskLatest.disk.ioTimeMs),
    networkChartSamples: toNetworkSamples(networkPoints),
    nicCount: args.hardwareInfo?.netifs?.length ?? 0,
    primaryNicLabel: resolvePrimaryNicLabel(args.hardwareInfo),
    networkRxBytesCounter: toSafeNumber(networkLatest.network.rxBytes),
    networkTxBytesCounter: toSafeNumber(networkLatest.network.txBytes),
    networkRxPacketsCounter: toSafeNumber(networkLatest.network.rxPackets),
    networkTxPacketsCounter: toSafeNumber(networkLatest.network.txPackets),
    gpuCount: gpus.length,
    gpuModel: resolveGpuModel(args.hardwareInfo),
    gpuMemoryTotalBytes,
  };
}
