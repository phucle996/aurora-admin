import type { KvmHypervisorItem } from "@/pages/HypervisorPage/KvmPage/kvm-page.api";

export type NodeHealth = "healthy" | "warning";

export const healthClass: Record<NodeHealth, string> = {
  healthy: "text-emerald-500",
  warning: "text-amber-500",
};

export function ratio(numerator: number, denominator: number): number {
  if (denominator <= 0) {
    return 0;
  }
  return Math.round((numerator / denominator) * 100);
}

export function resolveHealth(node: KvmHypervisorItem): NodeHealth {
  if (node.status.toLowerCase() !== "running") {
    return "warning";
  }
  return "healthy";
}

export function formatTime(value: string): string {
  if (!value) {
    return "-";
  }
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) {
    return "-";
  }
  return d.toLocaleString();
}
