import { TerminalSquare } from "lucide-react";
import { useMemo } from "react";

import { cn } from "@/lib/utils";

type ParsedTerminalLine = {
  timestamp: string;
  stage: string;
  level: "info" | "warn" | "error" | "done";
  message: string;
};

type KvmTerminalStreamSectionProps = {
  logs: string[];
  checking: boolean;
  terminalLabel?: string;
  shellPrompt?: string;
  shellName?: string;
  emptyMessage?: string;
  className?: string;
};

function parseTerminalLine(line: string): ParsedTerminalLine {
  const timeMatch = line.match(/^\[(.+?)\]\s*(.*)$/);
  const timestamp = timeMatch?.[1] ?? "--:--:--";
  const content = timeMatch?.[2] ?? line;

  const stageMatch = content.match(/^\[([a-zA-Z0-9_/-]+)\]\s*/);
  const stage = stageMatch?.[1] ?? "log";
  const message = content.replace(/^\[[a-zA-Z0-9_/-]+\]\s*/, "");

  let level: ParsedTerminalLine["level"] = "info";
  if (content.includes("[error]") || content.includes(" failed")) {
    level = "error";
  } else if (content.includes("[warn]")) {
    level = "warn";
  } else if (content.includes("[done]") || content.includes(" success")) {
    level = "done";
  }

  return {
    timestamp,
    stage,
    level,
    message,
  };
}

function levelClassName(level: ParsedTerminalLine["level"]): string {
  if (level === "error") {
    return "text-red-400";
  }
  if (level === "warn") {
    return "text-amber-400";
  }
  if (level === "done") {
    return "text-emerald-400";
  }
  return "text-slate-100";
}

function stageClassName(stage: string): string {
  if (stage === "probe" || stage === "remove") {
    return "text-cyan-400";
  }
  if (stage === "ui") {
    return "text-indigo-300";
  }
  if (stage === "service") {
    return "text-violet-300";
  }
  if (stage === "network" || stage === "ssh") {
    return "text-sky-300";
  }
  return "text-emerald-300";
}

export function KvmTerminalStreamSection({
  logs,
  checking,
  terminalLabel = "ubuntu@kvm-probe:~",
  shellPrompt = "phucle@kvm-node:~$",
  shellName = "bash",
  emptyMessage = "initializing session...",
  className,
}: KvmTerminalStreamSectionProps) {
  const parsedLogs = useMemo(() => logs.map(parseTerminalLine), [logs]);

  return (
    <div
      className={cn(
        "overflow-hidden rounded-lg border border-[#3a2c18] bg-[#300a24]",
        className,
      )}
    >
      <div className="flex items-center justify-between border-b border-[#49311f] bg-[#2a091f] px-3 py-2">
        <div className="flex items-center gap-2 text-xs text-orange-100">
          <span className="inline-flex items-center gap-1">
            <span className="h-2 w-2 rounded-full bg-red-400/90" />
            <span className="h-2 w-2 rounded-full bg-yellow-400/90" />
            <span className="h-2 w-2 rounded-full bg-green-400/90" />
          </span>
          <TerminalSquare className="h-3.5 w-3.5" />
          {terminalLabel}
        </div>
        <div className="text-[11px] text-orange-200/70">{shellName}</div>
      </div>

      <div className="h-[420px] overflow-x-hidden overflow-y-auto space-y-1 p-3 font-mono text-[12px] leading-5 [scrollbar-width:none] [-ms-overflow-style:none] [&::-webkit-scrollbar]:h-0 [&::-webkit-scrollbar]:w-0">
        {parsedLogs.length === 0 ? (
          <p className="text-orange-100/70">{emptyMessage}</p>
        ) : (
          parsedLogs.map((line, index) => (
            <div
              key={`${line.timestamp}-${line.stage}-${index}`}
              className="flex min-w-0 flex-wrap items-baseline gap-2"
            >
              <span className="text-orange-200/70">{line.timestamp}</span>
              <span className="text-emerald-300">{shellPrompt}</span>
              <span className={cn("uppercase tracking-wide", stageClassName(line.stage))}>
                [{line.stage}]
              </span>
              <span
                className={cn(
                  "min-w-0 flex-1 break-all whitespace-pre-wrap",
                  levelClassName(line.level),
                )}
              >
                {line.message}
              </span>
            </div>
          ))
        )}
        {checking && (
          <div className="flex items-center gap-2 text-slate-100">
            <span className="text-orange-200/70">{new Date().toLocaleTimeString()}</span>
            <span className="text-emerald-300">{shellPrompt}</span>
            <span className="inline-block h-4 w-2 animate-pulse bg-slate-200/90" />
          </div>
        )}
      </div>
    </div>
  );
}
