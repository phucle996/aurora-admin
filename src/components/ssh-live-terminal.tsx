import { useEffect, useRef } from "react";
import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";

import { cn } from "@/lib/utils";

export type SSHInputPrompt = {
  id: string;
  prompt: string;
  maskInput?: boolean;
};

type SSHLiveTerminalProps = {
  logs: string[];
  running: boolean;
  inputPrompt?: SSHInputPrompt | null;
  onInputSubmit?: (value: string) => void;
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
  inputPrompt,
  onInputSubmit,
  className,
  emptyMessage = "waiting backend stream...",
}: SSHLiveTerminalProps) {
  const hostRef = useRef<HTMLDivElement | null>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const fitAddonRef = useRef<FitAddon | null>(null);
  const resizeObserverRef = useRef<ResizeObserver | null>(null);
  const writtenLinesRef = useRef(0);
  const dataDisposableRef = useRef<{ dispose: () => void } | null>(null);
  const promptInputBufferRef = useRef("");
  const activePromptRef = useRef<SSHInputPrompt | null>(null);
  const onInputSubmitRef = useRef(onInputSubmit);

  useEffect(() => {
    onInputSubmitRef.current = onInputSubmit;
  }, [onInputSubmit]);

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
    promptInputBufferRef.current = "";
    activePromptRef.current = null;

    if (logs.length === 0) {
      term.writeln(emptyMessage);
    }

    return () => {
      resizeObserverRef.current?.disconnect();
      resizeObserverRef.current = null;
      dataDisposableRef.current?.dispose();
      dataDisposableRef.current = null;
      fitAddonRef.current = null;
      terminalRef.current?.dispose();
      terminalRef.current = null;
      writtenLinesRef.current = 0;
      promptInputBufferRef.current = "";
      activePromptRef.current = null;
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

    term.options.disableStdin = !inputPrompt;
    if (!inputPrompt) {
      promptInputBufferRef.current = "";
      activePromptRef.current = null;
      return;
    }

    if (activePromptRef.current?.id === inputPrompt.id) {
      return;
    }

    activePromptRef.current = inputPrompt;
    promptInputBufferRef.current = "";
    term.write(inputPrompt.prompt);
    term.focus();
  }, [inputPrompt]);

  useEffect(() => {
    const term = terminalRef.current;
    if (!term) {
      return;
    }

    dataDisposableRef.current?.dispose();
    dataDisposableRef.current = term.onData((data) => {
      const prompt = activePromptRef.current;
      if (!prompt) {
        return;
      }

      if (data === "\r") {
        const value = promptInputBufferRef.current;
        promptInputBufferRef.current = "";
        activePromptRef.current = null;
        term.options.disableStdin = true;
        term.write("\r\n");
        onInputSubmitRef.current?.(value);
        return;
      }

      if (data === "\u007f") {
        if (promptInputBufferRef.current.length === 0) {
          return;
        }
        promptInputBufferRef.current = promptInputBufferRef.current.slice(0, -1);
        if (!prompt.maskInput) {
          term.write("\b \b");
        }
        return;
      }

      if (data === "\u0003") {
        // Ctrl+C: submit empty to unblock caller.
        promptInputBufferRef.current = "";
        activePromptRef.current = null;
        term.options.disableStdin = true;
        term.write("^C\r\n");
        onInputSubmitRef.current?.("");
        return;
      }

      for (const char of data) {
        if (char < " " || char > "~") {
          continue;
        }
        promptInputBufferRef.current += char;
        if (!prompt.maskInput) {
          term.write(char);
        }
      }
    });

    return () => {
      dataDisposableRef.current?.dispose();
      dataDisposableRef.current = null;
    };
  }, []);

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
