import { useEffect, useState } from "react";

import {
  getKvmNodeHardwareInfo,
  type KvmNodeHardwareInfo,
} from "@/hooks/kvm-detail/use-kvm-node-metrics-api";
import { isRequestCanceled } from "@/lib/api";

export function useKvmNodeHardwareInfo(
  nodeId: string,
  reloadTick: number,
): KvmNodeHardwareInfo | null {
  const [hardwareInfo, setHardwareInfo] = useState<KvmNodeHardwareInfo | null>(
    null,
  );

  useEffect(() => {
    if (!nodeId) {
      setHardwareInfo(null);
      return;
    }

    const controller = new AbortController();
    const run = async () => {
      try {
        const info = await getKvmNodeHardwareInfo(nodeId, controller.signal);
        setHardwareInfo(info);
      } catch (err) {
        if (isRequestCanceled(err)) {
          return;
        }
        setHardwareInfo(null);
      }
    };

    void run();
    return () => controller.abort();
  }, [nodeId, reloadTick]);

  return hardwareInfo;
}
