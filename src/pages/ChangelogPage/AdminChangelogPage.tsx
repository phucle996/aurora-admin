import { useCallback, useEffect, useState } from "react";
import {
  Clock3,
  ExternalLink,
  GitBranch,
  RefreshCcw,
  ShieldCheck,
  Sparkles,
  Wrench,
} from "lucide-react";
import { useTheme } from "next-themes";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { cn } from "@/lib/utils";

const githubRepoOwner = "phucle996";
const githubRepoName = "aurora-admin";
const githubReleasesAPI = `https://api.github.com/repos/${githubRepoOwner}/${githubRepoName}/releases`;
const githubReleasesPage = `https://github.com/${githubRepoOwner}/${githubRepoName}/releases`;

type ChangelogType = "security" | "fix" | "feature";

type ChangelogItem = {
  id: number;
  version: string;
  releaseDate: string;
  highlights: string[];
  type: ChangelogType;
  releaseURL: string;
  prerelease: boolean;
};

type GithubRelease = {
  id: number;
  tagName: string;
  name: string;
  publishedAt: string;
  body: string;
  htmlURL: string;
  draft: boolean;
  prerelease: boolean;
};

function resolveTypeIcon(type: ChangelogType) {
  if (type === "security") return ShieldCheck;
  if (type === "fix") return Wrench;
  return Sparkles;
}

function toStringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function toBooleanValue(value: unknown): boolean {
  return value === true;
}

function toNumberValue(value: unknown): number {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  return 0;
}

function parseGithubReleaseRows(raw: unknown): GithubRelease[] {
  if (!Array.isArray(raw)) {
    return [];
  }
  const out: GithubRelease[] = [];
  for (const row of raw) {
    if (!row || typeof row !== "object") {
      continue;
    }
    const item = row as Record<string, unknown>;
    const id = toNumberValue(item.id);
    const tagName = toStringValue(item.tag_name).trim();
    if (id <= 0 || tagName === "") {
      continue;
    }
    out.push({
      id,
      tagName,
      name: toStringValue(item.name).trim(),
      publishedAt: toStringValue(item.published_at).trim(),
      body: toStringValue(item.body),
      htmlURL: toStringValue(item.html_url).trim(),
      draft: toBooleanValue(item.draft),
      prerelease: toBooleanValue(item.prerelease),
    });
  }
  return out;
}

function inferType(text: string): ChangelogType {
  const lowered = text.toLowerCase();
  if (
    lowered.includes("security") ||
    lowered.includes("tls") ||
    lowered.includes("cert") ||
    lowered.includes("mfa")
  ) {
    return "security";
  }
  if (
    lowered.includes("fix") ||
    lowered.includes("bug") ||
    lowered.includes("hotfix") ||
    lowered.includes("rollback")
  ) {
    return "fix";
  }
  return "feature";
}

function parseHighlights(body: string): string[] {
  const normalized = body.replace(/\r\n/g, "\n").trim();
  if (!normalized) {
    return ["No release notes."];
  }

  const out: string[] = [];
  for (const rawLine of normalized.split("\n")) {
    const line = rawLine.trim();
    if (!line) {
      continue;
    }
    if (line.startsWith("#")) {
      continue;
    }
    const cleaned = line
      .replace(/^[-*]\s+/, "")
      .replace(/^\d+\.\s+/, "")
      .trim();
    if (!cleaned) {
      continue;
    }
    out.push(cleaned);
    if (out.length >= 8) {
      break;
    }
  }
  if (out.length === 0) {
    return ["No release notes."];
  }
  return out;
}

function formatReleaseDate(value: string): string {
  if (!value) {
    return "Unknown";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }
  return parsed.toLocaleDateString();
}

function toChangelogItems(releases: GithubRelease[]): ChangelogItem[] {
  return releases
    .filter((item) => !item.draft)
    .map((item) => {
      const title = item.name || item.tagName;
      return {
        id: item.id,
        version: item.tagName,
        releaseDate: formatReleaseDate(item.publishedAt),
        highlights: parseHighlights(item.body),
        type: inferType(`${item.tagName} ${title} ${item.body}`),
        releaseURL: item.htmlURL || githubReleasesPage,
        prerelease: item.prerelease,
      };
    });
}

export default function AdminChangelogPage() {
  const { resolvedTheme } = useTheme();
  const isDark = resolvedTheme !== "light";
  const panelClass = isDark ? "border-white/10 bg-slate-950/60" : "border-black/10 bg-white/85";
  const textPrimary = isDark ? "text-white" : "text-slate-900";
  const textMuted = isDark ? "text-slate-300" : "text-slate-600";
  const [status, setStatus] = useState<"idle" | "loading" | "ready" | "error">("idle");
  const [error, setError] = useState("");
  const [changelogItems, setChangelogItems] = useState<ChangelogItem[]>([]);
  const [lastUpdatedAt, setLastUpdatedAt] = useState("");

  const loadChangelog = useCallback(async (signal?: AbortSignal) => {
    setStatus("loading");
    setError("");

    try {
      const response = await fetch(githubReleasesAPI, {
        method: "GET",
        headers: {
          Accept: "application/vnd.github+json",
        },
        signal,
      });
      if (!response.ok) {
        throw new Error(`Cannot load changelog (HTTP ${response.status})`);
      }

      const payload = (await response.json()) as unknown;
      const items = toChangelogItems(parseGithubReleaseRows(payload));
      setChangelogItems(items);
      setLastUpdatedAt(new Date().toLocaleTimeString());
      setStatus("ready");
    } catch (err) {
      if (err instanceof Error && err.name === "AbortError") {
        return;
      }
      setStatus("error");
      if (err instanceof Error && err.message.trim()) {
        setError(err.message);
      } else {
        setError("Cannot load changelog from GitHub");
      }
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      void loadChangelog(controller.signal);
    }, 0);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [loadChangelog]);

  return (
    <main className="space-y-4 py-3 lg:py-1">
      <header className="space-y-2">
        <Badge
          variant="outline"
          className={cn(
            "rounded-full px-3 py-1 text-xs uppercase tracking-[0.12em]",
            isDark ? "border-white/20 bg-white/5 text-slate-200" : "bg-white/70",
          )}
        >
          Release Notes
        </Badge>
        <h1 className={cn("text-3xl font-semibold tracking-tight", textPrimary)}>
          Admin Changelog
        </h1>
        <p className={cn("text-sm", textMuted)}>
          Theo dõi lịch sử release và các thay đổi quan trọng của Aurora Admin.
        </p>
      </header>

      <Card className={cn("shadow-lg backdrop-blur-xl", panelClass)}>
        <CardHeader>
          <div className="flex flex-wrap items-center justify-between gap-2">
            <CardTitle className={cn("flex items-center gap-2", textPrimary)}>
              <GitBranch className="h-4 w-4 text-indigo-400" />
              Version Timeline
            </CardTitle>
            <div className="flex items-center gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => {
                  void loadChangelog();
                }}
                disabled={status === "loading"}
              >
                <RefreshCcw className={cn("mr-1 h-3.5 w-3.5", status === "loading" && "animate-spin")} />
                Refresh
              </Button>
              <Button asChild type="button" variant="outline" size="sm">
                <a href={githubReleasesPage} target="_blank" rel="noreferrer">
                  GitHub
                  <ExternalLink className="ml-1 h-3.5 w-3.5" />
                </a>
              </Button>
            </div>
          </div>
          <CardDescription className={textMuted}>
            Log thay đổi theo từng release từ GitHub repository.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          {status === "loading" && changelogItems.length === 0 ? (
            <p className={cn("text-sm", textMuted)}>Loading changelog from GitHub...</p>
          ) : null}
          {status === "error" ? (
            <p className="text-sm text-red-500">{error || "Cannot load changelog"}</p>
          ) : null}
          {status === "ready" && changelogItems.length === 0 ? (
            <p className={cn("text-sm", textMuted)}>No releases found in repository.</p>
          ) : null}

          {changelogItems.map((item) => {
            const TypeIcon = resolveTypeIcon(item.type);
            return (
              <div
                key={item.version}
                className={cn(
                  "rounded-xl border p-3",
                  isDark ? "border-white/10 bg-white/5" : "border-slate-200 bg-slate-50/70",
                )}
              >
                <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                  <div className="flex items-center gap-2">
                    <TypeIcon className="h-4 w-4 text-indigo-400" />
                    <span className={cn("text-base font-semibold", textPrimary)}>{item.version}</span>
                    {item.prerelease ? (
                      <Badge variant="outline" className="h-5 rounded-full px-2 text-[10px] uppercase">
                        prerelease
                      </Badge>
                    ) : null}
                  </div>
                  <span className={cn("inline-flex items-center gap-1 text-xs", textMuted)}>
                    <Clock3 className="h-3.5 w-3.5" />
                    {item.releaseDate}
                  </span>
                </div>

                <div className="space-y-1">
                  {item.highlights.map((line) => (
                    <p key={line} className={cn("text-sm", textMuted)}>
                      - {line}
                    </p>
                  ))}
                </div>
                <div className="mt-2">
                  <a
                    href={item.releaseURL}
                    target="_blank"
                    rel="noreferrer"
                    className={cn("inline-flex items-center gap-1 text-xs text-indigo-400 hover:underline")}
                  >
                    Open release
                    <ExternalLink className="h-3.5 w-3.5" />
                  </a>
                </div>
              </div>
            );
          })}
          {lastUpdatedAt ? (
            <p className={cn("pt-1 text-xs", textMuted)}>Last updated: {lastUpdatedAt}</p>
          ) : null}
        </CardContent>
      </Card>
    </main>
  );
}
