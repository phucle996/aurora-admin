import { useEffect, useMemo, useRef, useState } from "react";

import {
  getKvmNodeRealtimeMetrics,
  subscribeKvmRealtimeMetrics,
  type KvmNodeHardwareInfo,
  type KvmNodeLiteMetric,
} from "@/hooks/kvm-detail/use-kvm-node-metrics-api";
import { isRequestCanceled } from "@/lib/api";
import type { KvmNodeHistoryRow } from "@/pages/HypervisorPage/KvmDetailPage/sections/KvmNodeHistorySection";
import {
  buildNodeLiteMetricsView,
  CHART_RANGE_OPTIONS,
  MAX_POINTS,
  mergeLiteSeries,
  normalizeLiteSeriesWindow,
  type ChartDateRange,
  type ChartRangePreset,
  type KvmCpuChartSample,
  type KvmDiskChartSample,
  type KvmNetworkChartSample,
  type KvmRamChartSample,
} from "@/pages/HypervisorPage/KvmDetailPage/sections/resource/kvm-node-lite-metrics";

export { CHART_RANGE_OPTIONS };
export type { ChartDateRange, ChartRangePreset };

const FLUSH_INTERVAL_MS = 500;
const LIVE_TAIL_THRESHOLD_MS = 90 * 1000;
const POINT_WIDTH_PX = 3;
const DEFAULT_TARGET_POINTS = 299;
const MIN_TARGET_POINTS = 60;
const MAX_TARGET_POINTS = 2400;

type UseKvmNodeMetricsArgs = {
  nodeId: string;
  reloadTick: number;
  hardwareInfo: KvmNodeHardwareInfo | null;
  chartWidthPx: number;
};

type UseKvmNodeMetricsResult = {
  cpuChartRange: ChartRangePreset;
  setCPUChartRange: (next: ChartRangePreset) => void;
  cpuCustomRange: ChartDateRange;
  setCPUCustomRange: (next: ChartDateRange) => void;

  ramChartRange: ChartRangePreset;
  setRAMChartRange: (next: ChartRangePreset) => void;
  ramCustomRange: ChartDateRange;
  setRAMCustomRange: (next: ChartDateRange) => void;

  diskChartRange: ChartRangePreset;
  setDiskChartRange: (next: ChartRangePreset) => void;
  diskCustomRange: ChartDateRange;
  setDiskCustomRange: (next: ChartDateRange) => void;

  networkChartRange: ChartRangePreset;
  setNetworkChartRange: (next: ChartRangePreset) => void;
  networkCustomRange: ChartDateRange;
  setNetworkCustomRange: (next: ChartDateRange) => void;

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

type QueryRangeWindow = {
  from: Date;
  to: Date;
  liveTail: boolean;
};

type ActiveQueryRange = {
  fromMs: number;
  toMs: number;
  liveTail: boolean;
};

function clampInt(value: number, min: number, max: number): number {
  if (!Number.isFinite(value)) {
    return min;
  }
  if (value < min) {
    return min;
  }
  if (value > max) {
    return max;
  }
  return Math.floor(value);
}

function presetWindowMs(preset: ChartRangePreset): number {
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
      return 15 * 60 * 1000;
  }
}

function resolveSingleRangeWindow(
  preset: ChartRangePreset,
  customRange: ChartDateRange,
  now: Date,
): QueryRangeWindow {
  if (
    preset === "custom" &&
    customRange.from instanceof Date &&
    customRange.to instanceof Date
  ) {
    const from = new Date(customRange.from.getTime());
    const to = new Date(customRange.to.getTime());
    to.setHours(23, 59, 59, 999);
    if (from.getTime() <= to.getTime()) {
      return {
        from,
        to,
        liveTail: to.getTime() >= now.getTime() - LIVE_TAIL_THRESHOLD_MS,
      };
    }
  }

  const windowMs = presetWindowMs(preset);
  return {
    from: new Date(now.getTime() - windowMs),
    to: now,
    liveTail: true,
  };
}

function resolveQueryRangeWindow(args: {
  cpuChartRange: ChartRangePreset;
  cpuCustomRange: ChartDateRange;
  ramChartRange: ChartRangePreset;
  ramCustomRange: ChartDateRange;
  diskChartRange: ChartRangePreset;
  diskCustomRange: ChartDateRange;
  networkChartRange: ChartRangePreset;
  networkCustomRange: ChartDateRange;
}): QueryRangeWindow {
  const now = new Date();
  const ranges = [
    resolveSingleRangeWindow(args.cpuChartRange, args.cpuCustomRange, now),
    resolveSingleRangeWindow(args.ramChartRange, args.ramCustomRange, now),
    resolveSingleRangeWindow(args.diskChartRange, args.diskCustomRange, now),
    resolveSingleRangeWindow(
      args.networkChartRange,
      args.networkCustomRange,
      now,
    ),
  ];

  let minFrom = ranges[0].from;
  let maxTo = ranges[0].to;
  let liveTail = ranges[0].liveTail;
  for (let i = 1; i < ranges.length; i += 1) {
    if (ranges[i].from.getTime() < minFrom.getTime()) {
      minFrom = ranges[i].from;
    }
    if (ranges[i].to.getTime() > maxTo.getTime()) {
      maxTo = ranges[i].to;
    }
    liveTail = liveTail || ranges[i].liveTail;
  }

  return {
    from: minFrom,
    to: maxTo,
    liveTail,
  };
}

function estimateTargetPointsFromViewport(chartWidthPx: number): number {
  const estimatedChartWidth = Number.isFinite(chartWidthPx) && chartWidthPx > 0
    ? chartWidthPx
    : typeof window !== "undefined"
      ? Math.max(320, window.innerWidth || 0)
      : DEFAULT_TARGET_POINTS * POINT_WIDTH_PX;

  return clampInt(
    Math.floor(estimatedChartWidth / POINT_WIDTH_PX),
    MIN_TARGET_POINTS,
    MAX_TARGET_POINTS,
  );
}

export function useKvmNodeMetrics({
  nodeId,
  reloadTick,
  hardwareInfo,
  chartWidthPx,
}: UseKvmNodeMetricsArgs): UseKvmNodeMetricsResult {
  const [series, setSeries] = useState<KvmNodeLiteMetric[]>([]);
  const wsBufferRef = useRef<KvmNodeLiteMetric[]>([]);
  const activeRangeRef = useRef<ActiveQueryRange>({
    fromMs: 0,
    toMs: 0,
    liveTail: true,
  });
  const targetPoints = useMemo(
    () => estimateTargetPointsFromViewport(chartWidthPx),
    [chartWidthPx],
  );

  const [cpuChartRange, setCPUChartRange] = useState<ChartRangePreset>("15m");
  const [cpuCustomRange, setCPUCustomRange] = useState<ChartDateRange>({
    from: null,
    to: null,
  });

  const [ramChartRange, setRAMChartRange] = useState<ChartRangePreset>("15m");
  const [ramCustomRange, setRAMCustomRange] = useState<ChartDateRange>({
    from: null,
    to: null,
  });

  const [diskChartRange, setDiskChartRange] = useState<ChartRangePreset>("15m");
  const [diskCustomRange, setDiskCustomRange] = useState<ChartDateRange>({
    from: null,
    to: null,
  });

  const [networkChartRange, setNetworkChartRange] =
    useState<ChartRangePreset>("15m");
  const [networkCustomRange, setNetworkCustomRange] = useState<ChartDateRange>({
    from: null,
    to: null,
  });

  useEffect(() => {
    if (!nodeId) {
      wsBufferRef.current = [];
      activeRangeRef.current = {
        fromMs: 0,
        toMs: 0,
        liveTail: true,
      };
      return;
    }

    let active = true;
    const controller = new AbortController();

    const run = async () => {
      const queryRange = resolveQueryRangeWindow({
        cpuChartRange,
        cpuCustomRange,
        ramChartRange,
        ramCustomRange,
        diskChartRange,
        diskCustomRange,
        networkChartRange,
        networkCustomRange,
      });
      const fromMs = queryRange.from.getTime();
      const toMs = queryRange.to.getTime();

      activeRangeRef.current = {
        fromMs,
        toMs,
        liveTail: queryRange.liveTail,
      };

      try {
        const payload = await getKvmNodeRealtimeMetrics(
          nodeId,
          {
            from: queryRange.from.toISOString(),
            to: queryRange.to.toISOString(),
            bucket: "auto",
            targetPoints,
          },
          controller.signal,
        );

        if (!active) {
          return;
        }

        setSeries(
          normalizeLiteSeriesWindow(
            payload.nodeSeries ?? [],
            Math.max(MAX_POINTS, targetPoints * 2),
          ),
        );
        wsBufferRef.current = [];
      } catch (err) {
        if (!active || isRequestCanceled(err)) {
          return;
        }
      }
    };

    void run();

    return () => {
      active = false;
      controller.abort();
    };
  }, [
    nodeId,
    reloadTick,
    cpuChartRange,
    cpuCustomRange,
    ramChartRange,
    ramCustomRange,
    diskChartRange,
    diskCustomRange,
    networkChartRange,
    networkCustomRange,
    targetPoints,
  ]);

  useEffect(() => {
    if (!nodeId) {
      return;
    }

    return subscribeKvmRealtimeMetrics(nodeId, {
      onMessage: (event) => {
        const ts = event.timestampUnix;
        const activeRange = activeRangeRef.current;
        if (ts <= 0 || ts < activeRange.fromMs) {
          return;
        }
        if (!activeRange.liveTail && ts > activeRange.toMs) {
          return;
        }
        wsBufferRef.current.push(event);
      },
    });
  }, [nodeId]);

  useEffect(() => {
    if (!nodeId) {
      return;
    }

    const intervalID = window.setInterval(() => {
      const buffered = wsBufferRef.current;
      if (buffered.length === 0) {
        return;
      }

      wsBufferRef.current = [];
      setSeries((prev) =>
        mergeLiteSeries(prev, buffered, Math.max(MAX_POINTS, targetPoints * 2)),
      );
    }, FLUSH_INTERVAL_MS);

    return () => {
      window.clearInterval(intervalID);
    };
  }, [nodeId, targetPoints]);

  return useMemo(() => {
    const view = buildNodeLiteMetricsView({
      series,
      hardwareInfo,
      cpuChartRange,
      cpuCustomRange,
      ramChartRange,
      ramCustomRange,
      diskChartRange,
      diskCustomRange,
      networkChartRange,
      networkCustomRange,
    });

    return {
      cpuChartRange,
      setCPUChartRange,
      cpuCustomRange,
      setCPUCustomRange,

      ramChartRange,
      setRAMChartRange,
      ramCustomRange,
      setRAMCustomRange,

      diskChartRange,
      setDiskChartRange,
      diskCustomRange,
      setDiskCustomRange,

      networkChartRange,
      setNetworkChartRange,
      networkCustomRange,
      setNetworkCustomRange,

      latestNodeMetric: view.latestNodeMetric,
      runtimeLatencyMs: view.runtimeLatencyMs,

      historyRows: view.historyRows,

      cpuChartSamples: view.cpuChartSamples,
      cpuModelName: view.cpuModelName,

      ramChartSamples: view.ramChartSamples,

      diskChartSamples: view.diskChartSamples,
      diskCount: view.diskCount,
      primaryDiskLabel: view.primaryDiskLabel,
      diskCurrentMBps: view.diskCurrentMBps,
      diskAverageMBps: view.diskAverageMBps,
      diskPeakMBps: view.diskPeakMBps,
      diskCurrentIops: view.diskCurrentIops,
      diskReadMBps: view.diskReadMBps,
      diskWriteMBps: view.diskWriteMBps,

      networkChartSamples: view.networkChartSamples,
      nicCount: view.nicCount,
      primaryNicLabel: view.primaryNicLabel,
      networkCurrentMBps: view.networkCurrentMBps,
      networkAverageMBps: view.networkAverageMBps,
      networkPeakMBps: view.networkPeakMBps,
      networkRxMBps: view.networkRxMBps,
      networkTxMBps: view.networkTxMBps,

      gpuCount: view.gpuCount,
      gpuModel: view.gpuModel,
      gpuUsagePct: view.gpuUsagePct,
      gpuMemoryUsedBytes: view.gpuMemoryUsedBytes,
      gpuMemoryTotalBytes: view.gpuMemoryTotalBytes,
    };
  }, [
    series,
    hardwareInfo,
    cpuChartRange,
    cpuCustomRange,
    ramChartRange,
    ramCustomRange,
    diskChartRange,
    diskCustomRange,
    networkChartRange,
    networkCustomRange,
  ]);
}
