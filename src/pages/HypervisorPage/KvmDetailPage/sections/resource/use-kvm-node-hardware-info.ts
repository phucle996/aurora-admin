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
  const [hardwareState, setHardwareState] = useState<{
    nodeId: string;
    info: KvmNodeHardwareInfo | null;
  }>({
    nodeId: "",
    info: null,
  });

  useEffect(() => {
    if (!nodeId) {
      return;
    }

    const controller = new AbortController();
    let active = true;

    const run = async () => {
      try {
        const info = await getKvmNodeHardwareInfo(nodeId, controller.signal);
        if (!active) {
          return;
        }
        setHardwareState({ nodeId, info });
      } catch (err) {
        if (!active || isRequestCanceled(err)) {
          return;
        }
        setHardwareState({ nodeId, info: null });
      }
    };

    void run();
    return () => {
      active = false;
      controller.abort();
    };
  }, [nodeId, reloadTick]);

  if (!nodeId) {
    return null;
  }
  if (hardwareState.nodeId !== nodeId) {
    return null;
  }
  return hardwareState.info;
}
