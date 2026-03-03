import { useEffect, useState } from "react";

import { getErrorMessage, isRequestCanceled } from "@/lib/api";
import {
  getKvmHypervisorByNodeId,
  type KvmHypervisorDetail,
} from "@/pages/HypervisorPage/KvmDetailPage/sections/overview/kvm-node-detail.api";

type UseKvmNodeDetailResult = {
  detail: KvmHypervisorDetail | null;
  loading: boolean;
  error: string | null;
};

export function useKvmNodeDetail(
  nodeId: string,
  reloadTick: number,
): UseKvmNodeDetailResult {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [detail, setDetail] = useState<KvmHypervisorDetail | null>(null);

  useEffect(() => {
    if (!nodeId) {
      setError("Missing node id");
      setDetail(null);
      return;
    }

    const controller = new AbortController();
    const run = async () => {
      setLoading(true);
      setError(null);
      try {
        const item = await getKvmHypervisorByNodeId(nodeId, controller.signal);
        setDetail(item);
      } catch (err) {
        if (isRequestCanceled(err)) {
          return;
        }
        setDetail(null);
        setError(getErrorMessage(err, "Cannot load KVM node detail"));
      } finally {
        setLoading(false);
      }
    };

    void run();
    return () => controller.abort();
  }, [nodeId, reloadTick]);

  return {
    detail,
    loading,
    error,
  };
}

