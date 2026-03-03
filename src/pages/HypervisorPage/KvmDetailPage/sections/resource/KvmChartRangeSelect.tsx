import { CalendarDays, X } from "lucide-react";
import { format } from "date-fns";

import { Button } from "@/components/ui/button";
import { Calendar } from "@/components/ui/calendar";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import type {
  ChartDateRange,
  ChartRangePreset,
} from "@/pages/HypervisorPage/KvmDetailPage/sections/resource/use-kvm-node-metrics";

type KvmChartRangeSelectProps = {
  value: ChartRangePreset;
  onValueChange: (next: ChartRangePreset) => void;
  options: Array<{ value: ChartRangePreset; label: string }>;
  dateRange: ChartDateRange;
  onDateRangeChange: (next: ChartDateRange) => void;
};

export function KvmChartRangeSelect({
  value,
  onValueChange,
  options,
  dateRange,
  onDateRangeChange,
}: KvmChartRangeSelectProps) {
  const hasRange = Boolean(dateRange.from && dateRange.to);
  const dateRangeLabel = hasRange
    ? `${format(dateRange.from as Date, "dd/MM/yyyy")} - ${format(
        dateRange.to as Date,
        "dd/MM/yyyy",
      )}`
    : "Chon from/to";

  return (
    <div className="flex flex-wrap items-center gap-2">
      <Select
        value={value}
        onValueChange={(next) => onValueChange(next as ChartRangePreset)}
      >
        <SelectTrigger className="h-10 w-[170px] text-sm font-medium">
          <SelectValue placeholder="Range" />
        </SelectTrigger>
        <SelectContent>
          {options.map((option) => (
            <SelectItem key={option.value} value={option.value}>
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Popover>
        <PopoverTrigger asChild>
          <Button
            variant="outline"
            size="sm"
            className="h-10 min-w-[230px] justify-start gap-2 text-sm font-medium"
          >
            <CalendarDays className="h-4 w-4" />
            {dateRangeLabel}
          </Button>
        </PopoverTrigger>
        <PopoverContent align="end" className="w-auto p-0">
          <div className="border-b px-3 py-2">
            <p className="text-sm font-medium">Chon khoang thoi gian</p>
            <p className="text-xs text-muted-foreground">
              Chon from va to de loc du lieu theo lich.
            </p>
          </div>
          <Calendar
            mode="range"
            numberOfMonths={2}
            selected={{
              from: dateRange.from ?? undefined,
              to: dateRange.to ?? undefined,
            }}
            onSelect={(nextRange) =>
              onDateRangeChange({
                from: nextRange?.from ?? null,
                to: nextRange?.to ?? null,
              })
            }
            className="rounded-md"
          />
          <div className="flex items-center justify-end border-t px-3 py-2">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-8 text-xs"
              onClick={() => onDateRangeChange({ from: null, to: null })}
            >
              <X className="h-3.5 w-3.5" />
              Clear
            </Button>
          </div>
        </PopoverContent>
      </Popover>
    </div>
  );
}
