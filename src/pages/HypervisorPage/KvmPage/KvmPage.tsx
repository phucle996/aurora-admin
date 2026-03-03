import { useEffect, useMemo, useState } from "react";
import { useTheme } from "next-themes";
import { useLocation, useNavigate } from "react-router-dom";
import { Play } from "lucide-react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { getErrorMessage, isRequestCanceled } from "@/lib/api";
import {
  type KvmHypervisorItem,
  listKvmHypervisors,
  runKvmNodeNow,
} from "@/pages/HypervisorPage/KvmPage/kvm-page.api";

import { KvmFeedbackSection } from "./sections/KvmFeedbackSection";
import { KvmHeaderSection } from "./sections/KvmHeaderSection";
import { KvmMainTableSection } from "./sections/KvmMainTableSection";
import { KvmMetricsSection } from "./sections/KvmMetricsSection";
import { ratio } from "./sections/kvm-view.helpers";

export default function KvmPage() {
  const { resolvedTheme } = useTheme();
  const location = useLocation();
  const navigate = useNavigate();

  const [query, setQuery] = useState("");
  const [zoneFilter, setZoneFilter] = useState("all");
  const [nodes, setNodes] = useState<KvmHypervisorItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [reloadTick, setReloadTick] = useState(0);
  const [runNowLoading, setRunNowLoading] = useState(false);
  const [runNowMessage, setRunNowMessage] = useState<string | null>(null);
  const [pageSize, setPageSize] = useState(10);
  const [pageIndex, setPageIndex] = useState(0);
  const [totalCount, setTotalCount] = useState(0);

  const isDark = resolvedTheme !== "light";
  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";
  const panelClass = isDark
    ? "border-white/10 bg-slate-950/60"
    : "border-black/10 bg-white/85";

  const createdState =
    (location.state as {
      createdNodeName?: string;
      createdNode?: {
        nodeId: string;
        nodeName: string;
        host: string;
        zone: string;
        sshPort: number;
        status: string;
        apiEndpoint: string;
        apiPort: number;
        tlsSkipVerify: boolean;
      };
    } | null) ?? null;
  const createdNodeOverview = createdState?.createdNode ?? null;
  const createdMessage = useMemo(() => {
    if (createdNodeOverview) {
      return `Node ${createdNodeOverview.nodeName} saved with status ${createdNodeOverview.status}.`;
    }
    if (!createdState?.createdNodeName) {
      return null;
    }
    return `Node ${createdState.createdNodeName} created successfully.`;
  }, [createdNodeOverview, createdState]);

  const handleRunNow = async () => {
    if (!createdNodeOverview?.nodeId) {
      return;
    }
    setRunNowLoading(true);
    setRunNowMessage(null);
    try {
      const result = await runKvmNodeNow(createdNodeOverview.nodeId);
      if (result.ok) {
        setRunNowMessage(
          `Runtime connected successfully. Status=${result.status}`,
        );
      } else {
        setRunNowMessage(result.error || "Run now failed.");
      }
      setReloadTick((prev) => prev + 1);
      navigate(location.pathname, {
        replace: true,
        state: {
          ...createdState,
          createdNode: {
            ...createdNodeOverview,
            status: result.status || createdNodeOverview.status,
          },
        },
      });
    } catch (err) {
      setRunNowMessage(getErrorMessage(err, "Cannot run runtime connect now"));
    } finally {
      setRunNowLoading(false);
    }
  };

  const zones = useMemo(() => {
    const zoneList = Array.from(
      new Set(nodes.map((node) => node.zone).filter((zone) => zone.length > 0)),
    );
    return ["all", ...zoneList];
  }, [nodes]);

  useEffect(() => {
    setPageIndex(0);
  }, [query, zoneFilter, pageSize]);

  useEffect(() => {
    const controller = new AbortController();

    const run = async () => {
      setLoading(true);

      try {
        const result = await listKvmHypervisors(
          {
            search: query.trim() || undefined,
            zone: zoneFilter !== "all" ? zoneFilter : undefined,
            limit: pageSize,
            offset: pageIndex * pageSize,
          },
          controller.signal,
        );
        setNodes(result.items);
        setTotalCount(result.totalCount);
      } catch (err) {
        if (isRequestCanceled(err)) {
          return;
        }
        toast.error("Network Error");
        setNodes([]);
        setTotalCount(0);
      } finally {
        setLoading(false);
      }
    };

    void run();

    return () => {
      controller.abort();
    };
  }, [query, zoneFilter, reloadTick, pageSize, pageIndex]);

  const zoneCount = zones.length - 1;
  const nodeCount = nodes.length;
  const totalPages = Math.max(1, Math.ceil(totalCount / pageSize));
  const hasPrevPage = pageIndex > 0;
  const hasNextPage = pageIndex + 1 < totalPages;
  const totalVMs = nodes.reduce((acc, node) => acc + node.vmTotal, 0);
  const runningVMs = nodes.reduce((acc, node) => acc + node.vmRunning, 0);
  const runningRatio = ratio(runningVMs, totalVMs || 1);

  useEffect(() => {
    if (pageIndex > totalPages - 1) {
      setPageIndex(Math.max(0, totalPages - 1));
    }
  }, [pageIndex, totalPages]);

  return (
    <main className="space-y-4 py-3 lg:py-1">
      <KvmHeaderSection
        isDark={isDark}
        textPrimary={textPrimary}
        textMuted={textMuted}
        loading={loading}
        onRefresh={() => setReloadTick((prev) => prev + 1)}
      />

      <KvmMetricsSection
        zoneCount={zoneCount}
        nodeCount={nodeCount}
        totalVMs={totalVMs}
        runningRatio={runningRatio}
        runningVMs={runningVMs}
        isDark={isDark}
        textPrimary={textPrimary}
        textMuted={textMuted}
        panelClass={panelClass}
      />

      <KvmFeedbackSection
        createdMessage={createdMessage}
        textPrimary={textPrimary}
        panelClass={panelClass}
        onCloseCreated={() => navigate(location.pathname, { replace: true })}
      />

      {createdNodeOverview && (
        <Card className={panelClass}>
          <CardHeader>
            <CardTitle className={textPrimary}>New KVM Overview</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3 text-sm">
            <div className={textPrimary}>
              Node: {createdNodeOverview.nodeName}
            </div>
            <div className={textMuted}>Host: {createdNodeOverview.host}</div>
            <div className={textMuted}>
              Zone: {createdNodeOverview.zone || "-"}
            </div>
            <div className={textMuted}>
              SSH Port: {createdNodeOverview.sshPort}
            </div>
            <div className={textMuted}>
              Libvirt Endpoint: {createdNodeOverview.apiEndpoint}:
              {createdNodeOverview.apiPort}
            </div>
            <div className={textMuted}>
              TLS Skip Verify: {String(createdNodeOverview.tlsSkipVerify)}
            </div>
            <div className={textPrimary}>
              Current Status: {createdNodeOverview.status}
            </div>
            <div className="flex items-center gap-3">
              <Button
                type="button"
                onClick={handleRunNow}
                disabled={runNowLoading}
                className="bg-indigo-500 text-white hover:bg-indigo-400"
              >
                <Play className="mr-2 h-4 w-4" />
                {runNowLoading ? "Running..." : "Run now"}
              </Button>
              {runNowMessage && (
                <span className={textMuted}>{runNowMessage}</span>
              )}
            </div>
          </CardContent>
        </Card>
      )}

      <KvmMainTableSection
        query={query}
        onQueryChange={setQuery}
        zoneFilter={zoneFilter}
        onZoneFilterChange={setZoneFilter}
        zones={zones}
        nodes={nodes}
        loading={loading}
        isDark={isDark}
        textPrimary={textPrimary}
        textMuted={textMuted}
        panelClass={panelClass}
        pageSize={pageSize}
        onPageSizeChange={setPageSize}
        pageIndex={pageIndex}
        totalPages={totalPages}
        totalCount={totalCount}
        hasPrevPage={hasPrevPage}
        onPrevPage={() => setPageIndex((prev) => Math.max(0, prev - 1))}
        onNextPage={() => {
          if (!hasNextPage) {
            return;
          }
          setPageIndex((prev) => prev + 1);
        }}
        onPageChange={(nextPage) => {
          const clamped = Math.min(Math.max(nextPage, 0), totalPages - 1);
          setPageIndex(clamped);
        }}
        hasNextPage={hasNextPage}
      />
    </main>
  );
}
