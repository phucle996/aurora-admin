import { useEffect, useRef, useState } from "react";
import { useTheme } from "next-themes";
import { useNavigate, useParams } from "react-router-dom";
import { toast } from "sonner";

import { Card, CardContent } from "@/components/ui/card";
import {
  checkKvmNodeAgentUpdate,
  healthcheckKvmNodeAgent,
  reinstallKvmNodeAgent,
  type KvmAgentUpdateCheckResult,
  updateKvmNodeAgent,
} from "@/hooks/kvm-detail/use-kvm-agent-api";
import {
  removeKvmNode,
  runKvmNodeNow,
  stopKvmNode,
} from "@/hooks/kvm-detail/use-kvm-node-actions-api";
import { getErrorMessage } from "@/lib/api";
import { cn } from "@/lib/utils";
import { KvmDetailHeaderSection } from "@/pages/HypervisorPage/KvmDetailPage/sections/KvmDetailHeaderSection";
import { KvmNodeHistorySection } from "@/pages/HypervisorPage/KvmDetailPage/sections/KvmNodeHistorySection";
import { RemoveKvmNodeDialogSection } from "@/pages/HypervisorPage/KvmDetailPage/sections/RemoveKvmNodeDialogSection";
import { KvmAgentSidebar } from "@/pages/HypervisorPage/KvmDetailPage/sections/overview/KvmAgentSidebar";
import { KvmAgentUpdateDialogSection } from "@/pages/HypervisorPage/KvmDetailPage/sections/overview/KvmAgentUpdateDialogSection";
import { KvmNodeSidebarSection } from "@/pages/HypervisorPage/KvmDetailPage/sections/overview/KvmNodeSidebar";
import { KvmNodeOverviewResource } from "@/pages/HypervisorPage/KvmDetailPage/sections/overview/KvmOverViewResource";
import { KvmVmRuntimeSummarySection } from "@/pages/HypervisorPage/KvmDetailPage/sections/overview/KvmVmSummary";
import { useKvmNodeDetail } from "@/pages/HypervisorPage/KvmDetailPage/sections/overview/use-kvm-node-detail";
import { KvmChartRangeSelect } from "@/pages/HypervisorPage/KvmDetailPage/sections/resource/KvmChartRangeSelect";
import { KvmCpuSection } from "@/pages/HypervisorPage/KvmDetailPage/sections/resource/KvmCpuSection";
import { KvmDiskRealtimeSection } from "@/pages/HypervisorPage/KvmDetailPage/sections/resource/KvmDiskSection";
import { KvmGpuCardSection } from "@/pages/HypervisorPage/KvmDetailPage/sections/resource/KvmGpuCardSection";
import { KvmNetworkRealtimeSection } from "@/pages/HypervisorPage/KvmDetailPage/sections/resource/KvmNetworkSection";
import { KvmRamRealtimeSection } from "@/pages/HypervisorPage/KvmDetailPage/sections/resource/KvmRamSection";
import {
  CHART_RANGE_OPTIONS,
  useKvmNodeMetrics,
} from "@/pages/HypervisorPage/KvmDetailPage/sections/resource/use-kvm-node-metrics";
import { useKvmNodeHardwareInfo } from "@/pages/HypervisorPage/KvmDetailPage/sections/resource/use-kvm-node-hardware-info";

export default function KvmDetailPage() {
  const { resolvedTheme } = useTheme();
  const navigate = useNavigate();
  const { nodeId = "" } = useParams();

  const [actionLoading, setActionLoading] = useState(false);
  const [removeLoading, setRemoveLoading] = useState(false);
  const [removeDialogOpen, setRemoveDialogOpen] = useState(false);
  const [removeConfirmText, setRemoveConfirmText] = useState("");
  const [removeLogs, setRemoveLogs] = useState<string[]>([]);
  const [agentActionLoading, setAgentActionLoading] = useState(false);
  const [agentCheckingUpdate, setAgentCheckingUpdate] = useState(false);
  const [agentUpdating, setAgentUpdating] = useState(false);
  const [agentUpdateDialogOpen, setAgentUpdateDialogOpen] = useState(false);
  const [agentUpdateCheck, setAgentUpdateCheck] =
    useState<KvmAgentUpdateCheckResult | null>(null);
  const [reloadTick, setReloadTick] = useState(0);
  const chartWidthProbeRef = useRef<HTMLDivElement | null>(null);
  const [chartWidthPx, setChartWidthPx] = useState(0);

  useEffect(() => {
    if (typeof ResizeObserver === "undefined") {
      return;
    }

    const target = chartWidthProbeRef.current;
    if (!target) {
      return;
    }

    const observer = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width ?? 0;
      if (!Number.isFinite(width) || width <= 0) {
        return;
      }
      setChartWidthPx((prev) => {
        const next = Math.floor(width);
        return prev === next ? prev : next;
      });
    });

    observer.observe(target);
    return () => {
      observer.disconnect();
    };
  }, []);

  const { detail, loading, error } = useKvmNodeDetail(nodeId, reloadTick);
  const hardwareInfo = useKvmNodeHardwareInfo(nodeId, reloadTick);
  const {
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
    historyRows,
    cpuChartSamples,
    cpuModelName,
    ramChartSamples,
    diskChartSamples,
    diskCount,
    primaryDiskLabel,
    diskReadBytesCounter,
    diskWriteBytesCounter,
    diskReadIosCounter,
    diskWriteIosCounter,
    diskIoTimeMsCounter,
    networkChartSamples,
    nicCount,
    primaryNicLabel,
    networkRxBytesCounter,
    networkTxBytesCounter,
    networkRxPacketsCounter,
    networkTxPacketsCounter,
    gpuCount,
    gpuModel,
    gpuMemoryTotalBytes,
  } = useKvmNodeMetrics({
    nodeId,
    reloadTick,
    hardwareInfo,
    chartWidthPx,
  });

  const isDark = resolvedTheme !== "light";
  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";
  const panelClass = isDark
    ? "border-white/10 bg-slate-950/60"
    : "border-black/10 bg-white/85";

  const statusLower = (detail?.status || "").trim().toLowerCase();
  const canRun = statusLower === "stop" || statusLower === "stopped";
  const canStop = statusLower === "running";
  const canRemove = Boolean(detail?.nodeId || nodeId);
  const isAgentBusy = agentActionLoading || agentCheckingUpdate || agentUpdating;

  const handleRunNow = async () => {
    const targetNodeID = detail?.nodeId || nodeId;
    if (!targetNodeID || actionLoading) {
      return;
    }
    setActionLoading(true);
    try {
      const res = await runKvmNodeNow(targetNodeID);
      if (!res.ok) {
        toast.error("Run now failed.");
        return;
      }
      toast.success("KVM node is running.");
      setReloadTick((prev) => prev + 1);
    } catch (err) {
      toast.error(getErrorMessage(err, "Cannot run node"));
    } finally {
      setActionLoading(false);
    }
  };

  const handleStop = async () => {
    const targetNodeID = detail?.nodeId || nodeId;
    if (!targetNodeID || actionLoading) {
      return;
    }
    setActionLoading(true);
    try {
      const res = await stopKvmNode(targetNodeID);
      if (!res.ok) {
        toast.error("Stop failed.");
        return;
      }
      toast.success("KVM node stopped.");
      setReloadTick((prev) => prev + 1);
    } catch (err) {
      toast.error(getErrorMessage(err, "Cannot stop node"));
    } finally {
      setActionLoading(false);
    }
  };

  const handleOpenRemoveDialog = () => {
    if (!canRemove || loading || actionLoading || removeLoading) {
      return;
    }
    setRemoveConfirmText("");
    setRemoveLogs([]);
    setRemoveDialogOpen(true);
  };

  const appendRemoveLog = (line: string, timestamp?: string) => {
    const displayTime = timestamp
      ? new Date(timestamp).toLocaleTimeString()
      : new Date().toLocaleTimeString();
    setRemoveLogs((prev) => [...prev, `[${displayTime}] ${line}`]);
  };

  const handleRemoveNode = async () => {
    const targetNodeID = detail?.nodeId || nodeId;
    const targetNodeName = (detail?.nodeName || "").trim();
    if (!targetNodeID || removeLoading) {
      return;
    }
    if (targetNodeName && removeConfirmText.trim() !== targetNodeName) {
      appendRemoveLog("[ui][error] node name does not match");
      toast.error("Node name does not match.");
      return;
    }

    appendRemoveLog("[ui] remove node session started");
    appendRemoveLog(
      `[ui] target node_id=${targetNodeID} node_name=${targetNodeName || "-"}`,
    );
    appendRemoveLog("[remove] sending remove request to backend");

    setRemoveLoading(true);
    try {
      const res = await removeKvmNode(targetNodeID);
      if (!res.ok) {
        appendRemoveLog("[service][error] remove node returned ok=false");
        toast.error("Remove node failed.");
        return;
      }
      appendRemoveLog("[service] remove node completed");
      toast.success("KVM node removed.");
      window.setTimeout(() => {
        setRemoveDialogOpen(false);
        navigate("/hypervisor/kvm");
      }, 500);
    } catch (err) {
      appendRemoveLog(
        `[service][error] ${getErrorMessage(err, "Cannot remove node")}`,
      );
      toast.error(getErrorMessage(err, "Cannot remove node"));
    } finally {
      setRemoveLoading(false);
    }
  };

  const handleCheckAgentUpdate = async () => {
    const targetNodeID = detail?.nodeId || nodeId;
    if (!targetNodeID || agentCheckingUpdate || agentUpdating || agentActionLoading) {
      return;
    }
    setAgentUpdateDialogOpen(true);
    setAgentUpdateCheck(null);
    setAgentCheckingUpdate(true);
    try {
      const result = await checkKvmNodeAgentUpdate(targetNodeID);
      setAgentUpdateCheck(result);
      if (result.hasUpdate) {
        toast.info(`New version available: ${result.latestVersion || "-"}`);
      } else {
        toast.success("Agent is already up to date.");
      }
      setReloadTick((prev) => prev + 1);
    } catch (err) {
      setAgentUpdateDialogOpen(false);
      toast.error(getErrorMessage(err, "Cannot check agent update"));
    } finally {
      setAgentCheckingUpdate(false);
    }
  };

  const handleUpdateAgent = async () => {
    const targetNodeID = detail?.nodeId || nodeId;
    if (!targetNodeID || !agentUpdateCheck || agentUpdating || agentCheckingUpdate) {
      return;
    }
    setAgentUpdating(true);
    try {
      const result = await updateKvmNodeAgent(
        targetNodeID,
        agentUpdateCheck.latestVersion || undefined,
      );
      if (!result.ok) {
        toast.error("Agent update failed.");
        return;
      }
      toast.success(`Agent updated to ${result.updatedVersion || "latest"}.`);
      setAgentUpdateDialogOpen(false);
      setReloadTick((prev) => prev + 1);
    } catch (err) {
      toast.error(getErrorMessage(err, "Cannot update agent"));
    } finally {
      setAgentUpdating(false);
    }
  };

  const handleReinstallAgent = async () => {
    const targetNodeID = detail?.nodeId || nodeId;
    if (!targetNodeID || agentActionLoading || agentUpdating || agentCheckingUpdate) {
      return;
    }
    setAgentActionLoading(true);
    try {
      const result = await reinstallKvmNodeAgent(targetNodeID);
      if (!result.ok) {
        toast.error("Agent reinstall failed.");
        return;
      }
      toast.success("Agent reinstalled successfully.");
      setReloadTick((prev) => prev + 1);
    } catch (err) {
      toast.error(getErrorMessage(err, "Cannot reinstall agent"));
    } finally {
      setAgentActionLoading(false);
    }
  };

  const handleHealthcheckAgent = async () => {
    const targetNodeID = detail?.nodeId || nodeId;
    if (!targetNodeID || agentActionLoading || agentUpdating || agentCheckingUpdate) {
      return;
    }
    setAgentActionLoading(true);
    try {
      const result = await healthcheckKvmNodeAgent(targetNodeID);
      if (result.reachable) {
        toast.success(`Agent healthy (${result.currentVersion || "-"})`);
      } else {
        toast.error(result.error || "Agent is unreachable");
      }
      setReloadTick((prev) => prev + 1);
    } catch (err) {
      toast.error(getErrorMessage(err, "Cannot healthcheck agent"));
    } finally {
      setAgentActionLoading(false);
    }
  };

  const displayNodeID = detail?.nodeId || nodeId || "-";

  return (
    <main className="space-y-4 py-3 lg:py-1">
      <KvmDetailHeaderSection
        isDark={isDark}
        textPrimary={textPrimary}
        textMuted={textMuted}
        canRemove={canRemove}
        canRun={canRun}
        canStop={canStop}
        loading={loading}
        actionLoading={actionLoading}
        removeLoading={removeLoading}
        onBack={() => navigate("/hypervisor/kvm")}
        onOpenRemoveDialog={handleOpenRemoveDialog}
        onRunNow={handleRunNow}
        onStop={handleStop}
      />

      <RemoveKvmNodeDialogSection
        open={removeDialogOpen}
        onOpenChange={setRemoveDialogOpen}
        nodeName={detail?.nodeName || ""}
        confirmValue={removeConfirmText}
        onConfirmValueChange={setRemoveConfirmText}
        loading={removeLoading}
        logs={removeLogs}
        onConfirm={handleRemoveNode}
        isDark={isDark}
      />
      <KvmAgentUpdateDialogSection
        open={agentUpdateDialogOpen}
        onOpenChange={setAgentUpdateDialogOpen}
        checking={agentCheckingUpdate}
        updating={agentUpdating}
        result={agentUpdateCheck}
        isDark={isDark}
        onConfirmUpdate={handleUpdateAgent}
      />

      {error && (
        <Card className={cn("shadow-lg", panelClass)}>
          <CardContent className="pt-6">
            <p className="text-sm text-red-500">{error}</p>
          </CardContent>
        </Card>
      )}

      <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_360px]">
        <div className="space-y-4">
          <div className="grid gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
            <KvmNodeOverviewResource
              panelClass={panelClass}
              textPrimary={textPrimary}
              textMuted={textMuted}
              detail={detail}
              gpuCount={gpuCount}
              gpuMemoryTotalBytes={gpuMemoryTotalBytes}
            />

            <KvmVmRuntimeSummarySection
              panelClass={panelClass}
              textPrimary={textPrimary}
              textMuted={textMuted}
              detail={detail}
            />
          </div>

          <KvmNodeHistorySection
            panelClass={panelClass}
            textPrimary={textPrimary}
            textMuted={textMuted}
            rows={historyRows}
            className="xl:min-h-[280px]"
          />
        </div>

        <aside className="space-y-4">
          <KvmNodeSidebarSection
            panelClass={panelClass}
            textPrimary={textPrimary}
            textMuted={textMuted}
            detail={detail}
            fallbackNodeID={displayNodeID}
          />

          <KvmAgentSidebar
            panelClass={panelClass}
            textPrimary={textPrimary}
            textMuted={textMuted}
            detail={detail}
            loading={isAgentBusy || loading || removeLoading || actionLoading}
            onCheckUpdate={handleCheckAgentUpdate}
            onReinstall={handleReinstallAgent}
            onHealthcheck={handleHealthcheckAgent}
          />
        </aside>
      </section>

      <div ref={chartWidthProbeRef} className="space-y-4">
        <section className="space-y-4">
          <KvmCpuSection
            panelClass={panelClass}
            textPrimary={textPrimary}
            textMuted={textMuted}
            rangeControl={
              <KvmChartRangeSelect
                value={cpuChartRange}
                onValueChange={setCPUChartRange}
                options={CHART_RANGE_OPTIONS}
                dateRange={cpuCustomRange}
                onDateRangeChange={setCPUCustomRange}
              />
            }
            cpuModel={cpuModelName}
            samples={cpuChartSamples}
          />
        </section>

        <section className="space-y-4">
          <KvmRamRealtimeSection
            panelClass={panelClass}
            textPrimary={textPrimary}
            textMuted={textMuted}
            rangeControl={
              <KvmChartRangeSelect
                value={ramChartRange}
                onValueChange={setRAMChartRange}
                options={CHART_RANGE_OPTIONS}
                dateRange={ramCustomRange}
                onDateRangeChange={setRAMCustomRange}
              />
            }
            samples={ramChartSamples}
          />

          <KvmDiskRealtimeSection
            panelClass={panelClass}
            textPrimary={textPrimary}
            textMuted={textMuted}
            rangeControl={
              <KvmChartRangeSelect
                value={diskChartRange}
                onValueChange={setDiskChartRange}
                options={CHART_RANGE_OPTIONS}
                dateRange={diskCustomRange}
                onDateRangeChange={setDiskCustomRange}
              />
            }
            samples={diskChartSamples}
            diskCount={diskCount}
            primaryDiskLabel={primaryDiskLabel}
            readBytesCounter={diskReadBytesCounter}
            writeBytesCounter={diskWriteBytesCounter}
            readIosCounter={diskReadIosCounter}
            writeIosCounter={diskWriteIosCounter}
            ioTimeMsCounter={diskIoTimeMsCounter}
          />

          <KvmNetworkRealtimeSection
            panelClass={panelClass}
            textPrimary={textPrimary}
            textMuted={textMuted}
            rangeControl={
              <KvmChartRangeSelect
                value={networkChartRange}
                onValueChange={setNetworkChartRange}
                options={CHART_RANGE_OPTIONS}
                dateRange={networkCustomRange}
                onDateRangeChange={setNetworkCustomRange}
              />
            }
            samples={networkChartSamples}
            nicCount={nicCount}
            primaryNicLabel={primaryNicLabel}
            rxBytesCounter={networkRxBytesCounter}
            txBytesCounter={networkTxBytesCounter}
            rxPacketsCounter={networkRxPacketsCounter}
            txPacketsCounter={networkTxPacketsCounter}
          />

          <KvmGpuCardSection
            panelClass={panelClass}
            textPrimary={textPrimary}
            textMuted={textMuted}
            gpuCount={gpuCount}
            gpuModel={gpuModel}
            gpuMemoryTotalBytes={gpuMemoryTotalBytes}
          />
        </section>
      </div>
    </main>
  );
}
