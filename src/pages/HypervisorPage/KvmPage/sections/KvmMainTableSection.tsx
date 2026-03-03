import { Search } from "lucide-react";
import { Link } from "react-router-dom";

import { Input } from "@/components/ui/input";
import {
  PaginationEllipsis,
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import type { KvmHypervisorItem } from "@/pages/HypervisorPage/KvmPage/kvm-page.api";
import { cn } from "@/lib/utils";

import {
  resolveHealth,
} from "./kvm-view.helpers";

type KvmMainTableSectionProps = {
  query: string;
  onQueryChange: (value: string) => void;
  zoneFilter: string;
  onZoneFilterChange: (value: string) => void;
  zones: string[];
  nodes: KvmHypervisorItem[];
  loading: boolean;
  pageSize: number;
  onPageSizeChange: (value: number) => void;
  pageIndex: number;
  totalPages: number;
  totalCount: number;
  hasPrevPage: boolean;
  onPrevPage: () => void;
  onNextPage: () => void;
  onPageChange: (nextPage: number) => void;
  hasNextPage: boolean;
  isDark: boolean;
  textPrimary: string;
  textMuted: string;
  panelClass: string;
};

export function KvmMainTableSection({
  query,
  onQueryChange,
  zoneFilter,
  onZoneFilterChange,
  zones,
  nodes,
  loading,
  pageSize,
  onPageSizeChange,
  pageIndex,
  totalPages,
  totalCount,
  hasPrevPage,
  onPrevPage,
  onNextPage,
  onPageChange,
  hasNextPage,
  isDark,
  textPrimary,
  textMuted,
  panelClass,
}: KvmMainTableSectionProps) {
  const visiblePages = buildVisiblePages(totalPages, pageIndex);

  return (
    <section
      className={cn(
        "flex h-full flex-col rounded-xl border p-4 shadow-lg backdrop-blur-xl",
        panelClass,
      )}
    >
      <div className="mb-4 flex flex-wrap items-center gap-2">
        <div className="relative min-w-[16rem] flex-1">
          <Search
            className={cn(
              "pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2",
              textMuted,
            )}
          />
          <Input
            value={query}
            onChange={(event) => onQueryChange(event.target.value)}
            placeholder="Search by node, host, zone..."
            className={cn(
              "h-10 pl-10",
              isDark ? "border-white/10 bg-white/5" : "bg-white",
            )}
          />
        </div>
        <div className="flex gap-2">
          <select
            value={zoneFilter}
            onChange={(event) => onZoneFilterChange(event.target.value)}
            className={cn(
              "h-9 rounded-full border px-3 text-sm outline-none",
              isDark
                ? "border-white/15 bg-white/5 text-white"
                : "border-slate-200 bg-white",
            )}
          >
            {zones.map((zone) => (
              <option key={zone} value={zone}>
                {zone === "all" ? "All zones" : zone}
              </option>
            ))}
          </select>
        </div>
      </div>

      <Table>
        <TableHeader>
          <TableRow className={isDark ? "border-white/10" : "border-slate-200"}>
            <TableHead>Node</TableHead>
            <TableHead>Zone</TableHead>
            <TableHead>CPU</TableHead>
            <TableHead>RAM</TableHead>
            <TableHead>Storage</TableHead>
            <TableHead>VMs</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {nodes.map((node) => {
            const health = resolveHealth(node);

            return (
              <TableRow
                key={node.nodeId || `${node.host}-${node.nodeName}`}
                className={isDark ? "border-white/10 hover:bg-white/5" : ""}
              >
                <TableCell>
                  <div>
                    <p className={cn("flex items-center gap-2 font-semibold", textPrimary)}>
                      <span
                        className={cn(
                          "inline-block h-2.5 w-2.5 rounded-full",
                          health === "healthy" ? "bg-emerald-500" : "bg-rose-500",
                        )}
                      />
                      {node.nodeId ? (
                        <Link
                          to={`/hypervisor/kvm/${encodeURIComponent(node.nodeId.trim())}`}
                          className={cn(
                            "transition-colors hover:text-indigo-500 hover:underline",
                            isDark && "hover:text-indigo-300",
                          )}
                        >
                          {node.nodeName || "-"}
                        </Link>
                      ) : (
                        <span>{node.nodeName || "-"}</span>
                      )}
                    </p>
                    <p className={cn("text-xs", textMuted)}>
                      {node.host || "-"}
                    </p>
                  </div>
                </TableCell>
                <TableCell className={textPrimary}>
                  {node.zone || "-"}
                </TableCell>
                <TableCell className={textPrimary}>
                  {node.cpuCoresMax > 0 ? `${node.cpuCoresMax} cores` : "-"}
                </TableCell>
                <TableCell className={textPrimary}>
                  {node.ramMbMax > 0 ? `${node.ramMbMax.toLocaleString()} MB` : "-"}
                </TableCell>
                <TableCell className={textPrimary}>
                  {node.diskGbMax > 0 ? `${node.diskGbMax.toLocaleString()} GB` : "-"}
                </TableCell>
                <TableCell>
                  <span className="text-xs font-semibold text-emerald-500">
                    {node.vmRunning} running
                  </span>
                  <span className={cn("mx-1 text-xs", textMuted)}>/</span>
                  <span className="text-xs font-semibold text-rose-500">
                    {node.vmStopped} stop
                  </span>
                </TableCell>
              </TableRow>
            );
          })}

          {!loading && nodes.length === 0 && (
            <TableRow
              className={isDark ? "border-white/10" : "border-slate-200"}
            >
              <TableCell
                colSpan={6}
                className={cn("py-6 text-center text-sm", textMuted)}
              >
                No KVM hypervisor matched your filter.
              </TableCell>
            </TableRow>
          )}

          {loading && (
            <TableRow
              className={isDark ? "border-white/10" : "border-slate-200"}
            >
              <TableCell
                colSpan={6}
                className={cn("py-6 text-center text-sm", textMuted)}
              >
                Loading KVM hypervisors...
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>

      <div className="mt-auto flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <span className={cn("text-sm", textMuted)}>Items / page</span>
          <Select
            value={String(pageSize)}
            onValueChange={(value) => onPageSizeChange(Number(value))}
          >
            <SelectTrigger
              size="sm"
              className={cn(
                "w-20",
                isDark ? "border-white/15 bg-white/5 text-white" : "bg-white",
              )}
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="10">10</SelectItem>
              <SelectItem value="20">20</SelectItem>
              <SelectItem value="50">50</SelectItem>
            </SelectContent>
          </Select>
          <span className={cn("text-sm", textMuted)}>
            Page {pageIndex + 1} / {totalPages}
          </span>
          <span className={cn("text-sm", textMuted)}>
            Total {totalCount}
          </span>
        </div>

        <Pagination className="mx-0 w-auto justify-start sm:justify-end">
          <PaginationContent>
            <PaginationItem>
              <PaginationPrevious
                href="#"
                onClick={(event) => {
                  event.preventDefault();
                  if (loading || !hasPrevPage) {
                    return;
                  }
                  onPrevPage();
                }}
                className={cn(
                  (loading || !hasPrevPage) && "pointer-events-none opacity-50",
                )}
              />
            </PaginationItem>
            {visiblePages.map((page, index) => (
              <PaginationItem key={`${page}-${index}`}>
                {page === "ellipsis" ? (
                  <PaginationEllipsis />
                ) : (
                  <PaginationLink
                    href="#"
                    isActive={page === pageIndex + 1}
                    onClick={(event) => {
                      event.preventDefault();
                      if (loading) {
                        return;
                      }
                      onPageChange(page - 1);
                    }}
                    className={cn(loading && "pointer-events-none opacity-50")}
                  >
                    {page}
                  </PaginationLink>
                )}
              </PaginationItem>
            ))}
            <PaginationItem>
              <PaginationNext
                href="#"
                onClick={(event) => {
                  event.preventDefault();
                  if (loading || !hasNextPage) {
                    return;
                  }
                  onNextPage();
                }}
                className={cn(
                  (loading || !hasNextPage) && "pointer-events-none opacity-50",
                )}
              />
            </PaginationItem>
          </PaginationContent>
        </Pagination>
      </div>
    </section>
  );
}

function buildVisiblePages(
  totalPages: number,
  currentPageIndex: number,
): Array<number | "ellipsis"> {
  if (totalPages <= 7) {
    return Array.from({ length: totalPages }, (_, idx) => idx + 1);
  }

  const current = currentPageIndex + 1;
  const pages: Array<number | "ellipsis"> = [1];

  const left = Math.max(2, current - 1);
  const right = Math.min(totalPages - 1, current + 1);

  if (left > 2) {
    pages.push("ellipsis");
  }

  for (let page = left; page <= right; page += 1) {
    pages.push(page);
  }

  if (right < totalPages - 1) {
    pages.push("ellipsis");
  }

  pages.push(totalPages);
  return pages;
}
