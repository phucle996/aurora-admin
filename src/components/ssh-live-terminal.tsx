import { useEffect, useRef } from "react";
import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";

import { cn } from "@/lib/utils";

type SSHLiveTerminalProps = {
  logs: string[];
  running: boolean;
  className?: string;
  emptyMessage?: string;
};

function writeLine(term: Terminal, line: string) {
  if (!line) {
    term.write("\r\n");
    return;
  }

  const normalized = line.replace(/\r?\n/g, "\r\n");
  if (normalized.endsWith("\r\n")) {
    term.write(normalized);
    return;
  }
  term.write(`${normalized}\r\n`);
}

export function SSHLiveTerminal({
  logs,
  running,
  className,
  emptyMessage = "waiting backend stream...",
}: SSHLiveTerminalProps) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const resizeObserverRef = useRef<ResizeObserver | null>(null);
  const writtenLinesRef = useRef(0);

  useEffect(() => {
    if (!hostRef.current || terminalRef.current) {
      return;
    }

    const term = new Terminal({
      convertEol: true,
      disableStdin: true,
      cursorBlink: running,
      cursorStyle: "bar",
      fontFamily:
        "JetBrains Mono, ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, Liberation Mono, monospace",
      fontSize: 12,
      lineHeight: 1.35,
      scrollback: 20000,
      theme: {
        background: "#070b14",
        foreground: "#e5e7eb",
        cursor: "#93c5fd",
        selectionBackground: "#1e293b",
      },
    });
    const fitAddon = new FitAddon();
    term.loadAddon(fitAddon);
    term.open(hostRef.current);
    fitAddon.fit();

    resizeObserverRef.current = new ResizeObserver(() => {
      fitAddon.fit();
    });
    resizeObserverRef.current.observe(hostRef.current);

    terminalRef.current = term;
    fitAddonRef.current = fitAddon;
    writtenLinesRef.current = 0;

    if (logs.length === 0) {
      term.writeln(emptyMessage);
    }

    return () => {
      resizeObserverRef.current?.disconnect();
      resizeObserverRef.current = null;
      fitAddonRef.current = null;
      terminalRef.current?.dispose();
      terminalRef.current = null;
      writtenLinesRef.current = 0;
    };
  }, [emptyMessage, logs.length, running]);

  useEffect(() => {
    const term = terminalRef.current;
    if (!term) {
      return;
    }

    term.options.cursorBlink = running;
  }, [running]);

  useEffect(() => {
    const term = terminalRef.current;
    if (!term) {
      return;
    }

    if (logs.length < writtenLinesRef.current) {
      term.reset();
      writtenLinesRef.current = 0;
      if (logs.length === 0) {
        term.writeln(emptyMessage);
      }
    }

    for (let index = writtenLinesRef.current; index < logs.length; index += 1) {
      writeLine(term, logs[index]);
    }
    writtenLinesRef.current = logs.length;
    term.scrollToBottom();
  }, [emptyMessage, logs]);

  return (
    <div
      className={cn(
        "h-[460px] overflow-hidden rounded-md border border-slate-700/80 bg-[#070b14]",
        className,
      )}
    >
      <div ref={hostRef} className="h-full w-full p-2" />
    </div>
  );
}
