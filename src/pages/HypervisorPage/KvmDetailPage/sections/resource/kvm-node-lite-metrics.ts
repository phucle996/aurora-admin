import type {
  KvmNodeHardwareInfo,
  KvmNodeLiteMetric,
} from "@/hooks/kvm-detail/use-kvm-node-metrics-api";
import type { KvmNodeHistoryRow } from "@/pages/HypervisorPage/KvmDetailPage/sections/KvmNodeHistorySection";

const MEBIBYTE = 1024 * 1024;

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
  cpuUsagePct: number;
  load1: number;
  load5: number;
  load15: number;
  runQueueLength: number;
  processCount: number;
  threadCount: number;
  systemLoadPercent: number;
  cpuCores: number;
};

export type KvmRamChartSample = {
  timestamp: string;
  usagePct: number;
  usedGb: number;
  totalGb: number;
};

export type KvmDiskChartSample = {
  timestamp: string;
  throughputMBps: number;
  readMBps: number;
  writeMBps: number;
  totalIops: number;
  utilPercent: number;
};

export type KvmNetworkChartSample = {
  timestamp: string;
  throughputMBps: number;
  rxMBps: number;
  txMBps: number;
  packetRate: number;
};

type DerivedNodeMetricPoint = {
  timestamp: string;
  timestampUnix: number;
  cpuUsagePct: number;
  load1: number;
  load5: number;
  load15: number;
  runQueueLength: number;
  processCount: number;
  threadCount: number;
  systemLoadPercent: number;
  cpuCores: number;
  ramUsagePct: number;
  ramUsedGb: number;
  ramTotalGb: number;
  diskThroughputMBps: number;
  diskReadMBps: number;
  diskWriteMBps: number;
  diskTotalIops: number;
  diskUtilPercent: number;
  networkThroughputMBps: number;
  networkRxMBps: number;
  networkTxMBps: number;
  networkPacketRate: number;
};

type BuildNodeLiteMetricsViewArgs = {
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

type NodeLiteMetricsView = {
  latestNodeMetric: KvmNodeLiteMetric | null;
  runtimeLatencyMs: number;
  historyRows: KvmNodeHistoryRow[];
  cpuChartSamples: KvmCpuChartSample[];
  cpuModelName: string;
  ramChartSamples: KvmRamChartSample[];
  diskChartSamples: KvmDiskChartSample[];
  diskCount: number;
  primaryDiskLabel: string;
  diskCurrentMBps: number;
  diskAverageMBps: number;
  diskPeakMBps: number;
  diskCurrentIops: number;
  diskReadMBps: number;
  diskWriteMBps: number;
  networkChartSamples: KvmNetworkChartSample[];
  nicCount: number;
  primaryNicLabel: string;
  networkCurrentMBps: number;
  networkAverageMBps: number;
  networkPeakMBps: number;
  networkRxMBps: number;
  networkTxMBps: number;
  gpuCount: number;
  gpuModel: string;
  gpuUsagePct: number;
  gpuMemoryUsedBytes: number;
  gpuMemoryTotalBytes: number;
};

function clamp(value: number, min = 0, max = 100): number {
  if (!Number.isFinite(value)) {
    return min;
  }
  if (value < min) {
    return min;
  }
  if (value > max) {
    return max;
  }
  return value;
}

function toSafeNumber(value: number): number {
  return Number.isFinite(value) ? value : 0;
}

function diffCounter(current: number, previous: number): number {
  const delta = toSafeNumber(current) - toSafeNumber(previous);
  if (!Number.isFinite(delta) || delta < 0) {
    return 0;
  }
  return delta;
}

function bytesToMBps(bytesDelta: number, durationSec: number): number {
  if (durationSec <= 0 || bytesDelta <= 0) {
    return 0;
  }
  return bytesDelta / MEBIBYTE / durationSec;
}

function normalizeSeriesLimit(limit: number): number {
  if (!Number.isFinite(limit) || limit <= 0) {
    return MAX_POINTS;
  }
  return Math.max(10, Math.floor(limit));
}

function sortByTimestamp(
  a: KvmNodeLiteMetric,
  b: KvmNodeLiteMetric,
): number {
  const delta = a.timestampUnix - b.timestampUnix;
  if (delta !== 0) {
    return delta;
  }
  return a.timestamp.localeCompare(b.timestamp);
}

export function normalizeLiteSeriesWindow(
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

export function mergeLiteSeries(
  previous: KvmNodeLiteMetric[],
  incoming: KvmNodeLiteMetric[],
  limit = MAX_POINTS,
): KvmNodeLiteMetric[] {
  if (!Array.isArray(incoming) || incoming.length === 0) {
    return normalizeLiteSeriesWindow(previous, limit);
  }
  if (!Array.isArray(previous) || previous.length === 0) {
    return normalizeLiteSeriesWindow(incoming, limit);
  }
  return normalizeLiteSeriesWindow([...previous, ...incoming], limit);
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
  points: DerivedNodeMetricPoint[],
  range: ChartRangePreset,
  customRange: ChartDateRange,
  anchorMs: number,
): DerivedNodeMetricPoint[] {
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

function buildDerivedSeries(
  series: KvmNodeLiteMetric[],
  hardwareInfo: KvmNodeHardwareInfo | null,
): DerivedNodeMetricPoint[] {
  if (series.length === 0) {
    return [];
  }

  const normalized = normalizeLiteSeriesWindow(series, Math.max(MAX_POINTS, series.length));
  const cpuCores = Math.max(0, Math.round(hardwareInfo?.cpu?.cores ?? 0));
  const cpuThreads = Math.max(0, Math.round(hardwareInfo?.cpu?.threads ?? 0));

  const points: DerivedNodeMetricPoint[] = [];
  for (let index = 0; index < normalized.length; index += 1) {
    const current = normalized[index];
    const previous = index > 0 ? normalized[index - 1] : null;

    const durationSec =
      previous && current.timestampUnix > previous.timestampUnix
        ? (current.timestampUnix - previous.timestampUnix) / 1000
        : 0;

    const deltaCpuTotal =
      previous && durationSec > 0
        ? diffCounter(current.cpu.total, previous.cpu.total)
        : 0;
    const deltaCpuIdle =
      previous && durationSec > 0
        ? diffCounter(current.cpu.idle, previous.cpu.idle)
        : 0;
    const deltaCpuIOWait =
      previous && durationSec > 0
        ? diffCounter(current.cpu.iowait, previous.cpu.iowait)
        : 0;

    const cpuNonIdle =
      deltaCpuTotal > 0 ? Math.max(0, deltaCpuTotal - deltaCpuIdle - deltaCpuIOWait) : 0;
    const cpuUsagePct =
      deltaCpuTotal > 0 ? clamp((cpuNonIdle / deltaCpuTotal) * 100, 0, 100) : 0;

    const ramTotalBytes = Math.max(0, current.memory.totalBytes);
    const ramUsedBytes =
      current.memory.usedBytes > 0
        ? current.memory.usedBytes
        : Math.max(0, ramTotalBytes - current.memory.availableBytes);
    const ramUsagePct =
      ramTotalBytes > 0 ? clamp((ramUsedBytes / ramTotalBytes) * 100, 0, 100) : 0;

    const deltaDiskReadBytes =
      previous && durationSec > 0
        ? diffCounter(current.disk.readBytes, previous.disk.readBytes)
        : 0;
    const deltaDiskWriteBytes =
      previous && durationSec > 0
        ? diffCounter(current.disk.writeBytes, previous.disk.writeBytes)
        : 0;
    const deltaDiskReadIos =
      previous && durationSec > 0
        ? diffCounter(current.disk.readIos, previous.disk.readIos)
        : 0;
    const deltaDiskWriteIos =
      previous && durationSec > 0
        ? diffCounter(current.disk.writeIos, previous.disk.writeIos)
        : 0;
    const deltaDiskIoTimeMs =
      previous && durationSec > 0
        ? diffCounter(current.disk.ioTimeMs, previous.disk.ioTimeMs)
        : 0;

    const diskReadMBps = bytesToMBps(deltaDiskReadBytes, durationSec);
    const diskWriteMBps = bytesToMBps(deltaDiskWriteBytes, durationSec);
    const diskThroughputMBps = diskReadMBps + diskWriteMBps;
    const diskTotalIops =
      durationSec > 0 ? (deltaDiskReadIos + deltaDiskWriteIos) / durationSec : 0;
    const diskUtilPercent =
      durationSec > 0 ? clamp((deltaDiskIoTimeMs / (durationSec * 1000)) * 100, 0, 100) : 0;

    const deltaNetRxBytes =
      previous && durationSec > 0
        ? diffCounter(current.network.rxBytes, previous.network.rxBytes)
        : 0;
    const deltaNetTxBytes =
      previous && durationSec > 0
        ? diffCounter(current.network.txBytes, previous.network.txBytes)
        : 0;
    const deltaNetRxPackets =
      previous && durationSec > 0
        ? diffCounter(current.network.rxPackets, previous.network.rxPackets)
        : 0;
    const deltaNetTxPackets =
      previous && durationSec > 0
        ? diffCounter(current.network.txPackets, previous.network.txPackets)
        : 0;

    const networkRxMBps = bytesToMBps(deltaNetRxBytes, durationSec);
    const networkTxMBps = bytesToMBps(deltaNetTxBytes, durationSec);
    const networkThroughputMBps = networkRxMBps + networkTxMBps;
    const networkPacketRate =
      durationSec > 0 ? (deltaNetRxPackets + deltaNetTxPackets) / durationSec : 0;

    const systemLoadPercent =
      cpuCores > 0 ? Math.max(0, (current.load.load1 / cpuCores) * 100) : 0;

    points.push({
      timestamp: current.timestamp,
      timestampUnix: current.timestampUnix,
      cpuUsagePct,
      load1: toSafeNumber(current.load.load1),
      load5: toSafeNumber(current.load.load5),
      load15: toSafeNumber(current.load.load15),
      runQueueLength: Math.max(0, Math.round(current.load.load1)),
      processCount: 0,
      threadCount: cpuThreads,
      systemLoadPercent,
      cpuCores,
      ramUsagePct,
      ramUsedGb: ramUsedBytes / 1024 ** 3,
      ramTotalGb: ramTotalBytes / 1024 ** 3,
      diskThroughputMBps,
      diskReadMBps,
      diskWriteMBps,
      diskTotalIops,
      diskUtilPercent,
      networkThroughputMBps,
      networkRxMBps,
      networkTxMBps,
      networkPacketRate,
    });
  }

  return points;
}

function toCpuSamples(points: DerivedNodeMetricPoint[]): KvmCpuChartSample[] {
  return points.map((point) => ({
    timestamp: point.timestamp,
    cpuUsagePct: point.cpuUsagePct,
    load1: point.load1,
    load5: point.load5,
    load15: point.load15,
    runQueueLength: point.runQueueLength,
    processCount: point.processCount,
    threadCount: point.threadCount,
    systemLoadPercent: point.systemLoadPercent,
    cpuCores: point.cpuCores,
  }));
}

function toRamSamples(points: DerivedNodeMetricPoint[]): KvmRamChartSample[] {
  return points.map((point) => ({
    timestamp: point.timestamp,
    usagePct: point.ramUsagePct,
    usedGb: point.ramUsedGb,
    totalGb: point.ramTotalGb,
  }));
}

function toDiskSamples(points: DerivedNodeMetricPoint[]): KvmDiskChartSample[] {
  return points.map((point) => ({
    timestamp: point.timestamp,
    throughputMBps: point.diskThroughputMBps,
    readMBps: point.diskReadMBps,
    writeMBps: point.diskWriteMBps,
    totalIops: point.diskTotalIops,
    utilPercent: point.diskUtilPercent,
  }));
}

function toNetworkSamples(
  points: DerivedNodeMetricPoint[],
): KvmNetworkChartSample[] {
  return points.map((point) => ({
    timestamp: point.timestamp,
    throughputMBps: point.networkThroughputMBps,
    rxMBps: point.networkRxMBps,
    txMBps: point.networkTxMBps,
    packetRate: point.networkPacketRate,
  }));
}

function computeAverage(values: number[]): number {
  if (values.length === 0) {
    return 0;
  }
  const sum = values.reduce((total, value) => total + value, 0);
  return sum / values.length;
}

function computePeak(values: number[]): number {
  if (values.length === 0) {
    return 0;
  }
  return values.reduce((peak, value) => (value > peak ? value : peak), 0);
}

function toHistoryRows(points: DerivedNodeMetricPoint[]): KvmNodeHistoryRow[] {
  if (points.length === 0) {
    return [];
  }
  return points.slice(-60).map((point) => ({
    timestamp: point.timestamp,
    cpuUsagePct: point.cpuUsagePct,
    ramUsagePct: point.ramUsagePct,
    iops: point.diskTotalIops,
  }));
}

function fallbackView(): NodeLiteMetricsView {
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
    diskCurrentMBps: 0,
    diskAverageMBps: 0,
    diskPeakMBps: 0,
    diskCurrentIops: 0,
    diskReadMBps: 0,
    diskWriteMBps: 0,
    networkChartSamples: [],
    nicCount: 0,
    primaryNicLabel: "",
    networkCurrentMBps: 0,
    networkAverageMBps: 0,
    networkPeakMBps: 0,
    networkRxMBps: 0,
    networkTxMBps: 0,
    gpuCount: 0,
    gpuModel: "",
    gpuUsagePct: 0,
    gpuMemoryUsedBytes: 0,
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

export function buildNodeLiteMetricsView(
  args: BuildNodeLiteMetricsViewArgs,
): NodeLiteMetricsView {
  const normalizedSeries = normalizeLiteSeriesWindow(args.series, MAX_POINTS);
  const latestNodeMetric =
    normalizedSeries.length > 0 ? normalizedSeries[normalizedSeries.length - 1] : null;

  if (!latestNodeMetric) {
    return fallbackView();
  }

  const derivedSeries = buildDerivedSeries(normalizedSeries, args.hardwareInfo);
  if (derivedSeries.length === 0) {
    return fallbackView();
  }

  const anchorMs = derivedSeries[derivedSeries.length - 1]?.timestampUnix ?? Date.now();
  const cpuPoints = filterByChartRange(
    derivedSeries,
    args.cpuChartRange,
    args.cpuCustomRange,
    anchorMs,
  );
  const ramPoints = filterByChartRange(
    derivedSeries,
    args.ramChartRange,
    args.ramCustomRange,
    anchorMs,
  );
  const diskPoints = filterByChartRange(
    derivedSeries,
    args.diskChartRange,
    args.diskCustomRange,
    anchorMs,
  );
  const networkPoints = filterByChartRange(
    derivedSeries,
    args.networkChartRange,
    args.networkCustomRange,
    anchorMs,
  );

  const cpuChartSamples = toCpuSamples(cpuPoints);
  const ramChartSamples = toRamSamples(ramPoints);
  const diskChartSamples = toDiskSamples(diskPoints);
  const networkChartSamples = toNetworkSamples(networkPoints);

  const latestDisk = diskPoints[diskPoints.length - 1];
  const latestNetwork = networkPoints[networkPoints.length - 1];
  const diskThroughputs = diskPoints
    .map((point) => point.diskThroughputMBps)
    .filter((value) => value > 0);
  const networkThroughputs = networkPoints
    .map((point) => point.networkThroughputMBps)
    .filter((value) => value > 0);

  const diskCount = args.hardwareInfo?.disks?.length ?? 0;
  const nicCount = args.hardwareInfo?.netifs?.length ?? 0;
  const gpus = args.hardwareInfo?.gpus ?? [];
  const gpuCount = gpus.length;
  const gpuMemoryTotalBytes = gpus.reduce(
    (total, gpu) => total + Math.max(0, gpu.memoryTotalBytes ?? 0),
    0,
  );
  const cpuModelName = args.hardwareInfo?.cpu?.model?.trim() ?? "";
  const runtimeLatencyMs =
    latestNodeMetric.timestampUnix > 0 ? Math.max(0, Date.now() - latestNodeMetric.timestampUnix) : 0;

  return {
    latestNodeMetric,
    runtimeLatencyMs,
    historyRows: toHistoryRows(derivedSeries),
    cpuChartSamples,
    cpuModelName,
    ramChartSamples,
    diskChartSamples,
    diskCount,
    primaryDiskLabel: resolvePrimaryDiskLabel(args.hardwareInfo),
    diskCurrentMBps: latestDisk?.diskThroughputMBps ?? 0,
    diskAverageMBps: computeAverage(diskThroughputs),
    diskPeakMBps: computePeak(diskThroughputs),
    diskCurrentIops: latestDisk?.diskTotalIops ?? 0,
    diskReadMBps: latestDisk?.diskReadMBps ?? 0,
    diskWriteMBps: latestDisk?.diskWriteMBps ?? 0,
    networkChartSamples,
    nicCount,
    primaryNicLabel: resolvePrimaryNicLabel(args.hardwareInfo),
    networkCurrentMBps: latestNetwork?.networkThroughputMBps ?? 0,
    networkAverageMBps: computeAverage(networkThroughputs),
    networkPeakMBps: computePeak(networkThroughputs),
    networkRxMBps: latestNetwork?.networkRxMBps ?? 0,
    networkTxMBps: latestNetwork?.networkTxMBps ?? 0,
    gpuCount,
    gpuModel: resolveGpuModel(args.hardwareInfo),
    gpuUsagePct: 0,
    gpuMemoryUsedBytes: 0,
    gpuMemoryTotalBytes,
  };
}
