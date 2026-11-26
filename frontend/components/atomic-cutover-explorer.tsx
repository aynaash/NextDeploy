"use client";

import React, { useState } from "react";
import Link from "next/link";
import { ArrowRight, Check, ChevronLeft, ChevronRight, Copy } from "lucide-react";

interface CutoverStage {
  id: string;
  step: string;
  label: string;
  summary: string;
  sourceRef: string;
  caddyState: string;
  oldNodeState: string;
  newNodeState: string;
  symlinkState: string;
  staticPoolState: string;
  highlightLines: number[];
  code: string;
}

const CUTOVER_STAGES: CutoverStage[] = [
  {
    id: "lock",
    step: "01",
    label: "Lock & Ephemeral Port",
    summary:
      "appLocker acquires a fail-fast per-app mutex so concurrent ships cannot race, and findFreePort() holds a loopback listener open until right before systemd starts the candidate release.",
    sourceRef: "daemon/internal/daemon/command_handler.go",
    caddyState: "Serving :3001 (100% traffic)",
    oldNodeState: "LIVE on 127.0.0.1:3001",
    newNodeState: "Reserved 127.0.0.1:41823 -> Starting unit",
    symlinkState: "current -> releases/1763880000-9f8e7d6",
    staticPoolState: "Retaining R1 chunks",
    highlightLines: [1, 7, 10],
    code: `release, ok := ch.deployLocks.tryAcquire(appName)
if !ok {
    return types.Response{Success: false, Message: "deploy already in progress"}
}
defer release()

p, closePort, err := findFreePort() // keeps 127.0.0.1:<p> reserved
// ... render .env.nextdeploy (0600) & systemd unit ...
_ = closePort()
ch.processManager.StartService(serviceName)`,
  },
  {
    id: "probe",
    step: "02",
    label: "Dual-Stage TCP + HTTP Probe",
    summary:
      "A Next.js server can accept TCP while returning 500s on every route. waitForHealthy requires both a TCP dial and an HTTP GET returning status < 500 before any traffic shifts.",
    sourceRef: "daemon/internal/daemon/command_handler.go",
    caddyState: "Serving :3001 (unchanged)",
    oldNodeState: "LIVE on 127.0.0.1:3001",
    newNodeState: "PROBING TCP + HTTP /api/health -> 200 OK",
    symlinkState: "current -> releases/1763880000-9f8e7d6",
    staticPoolState: "Retaining R1 chunks",
    highlightLines: [1, 4, 7],
    code: `conn, err := net.DialTimeout("tcp", addr, 1*time.Second) // Gate 1: TCP
if err == nil {
    _ = conn.Close()
    resp, herr := client.Get(url) // Gate 2: HTTP < 500
    if herr == nil {
        _ = resp.Body.Close()
        if resp.StatusCode < 500 {
            return nil
        }
    }
}`,
  },
  {
    id: "pool",
    step: "03",
    label: "Cumulative shared_static Sync",
    summary:
      "Before Caddy reloads, new content-hashed assets from .next/static are copied additively into shared_static/ so active browser tabs on older HTML never hit ChunkLoadError 404s.",
    sourceRef: "daemon/internal/daemon/command_handler.go",
    caddyState: "Serving /_next/static/* from shared_static",
    oldNodeState: "LIVE on 127.0.0.1:3001",
    newNodeState: "HEALTHY on 127.0.0.1:41823",
    symlinkState: "current -> releases/1763880000-9f8e7d6",
    staticPoolState: "R1 + R2 chunks pooled (immutable)",
    highlightLines: [1, 4],
    code: `sharedStaticDir := filepath.Join(appsDir, ctx.AppName, "shared_static")
sourceStaticDir := filepath.Join(ctx.ReleaseDir, ctx.DistDir, "static")
if _, err := os.Stat(sourceStaticDir); err == nil {
    cmd := exec.Command(resolveTool("cp"), "-R", sourceStaticDir+"/.", sharedStaticDir)
    if out, err := cmd.CombinedOutput(); err != nil {
        return types.Response{Success: false, Message: "sync shared static failed"}
    }
}`,
  },
  {
    id: "swap",
    step: "04",
    label: "Caddy Sandbox & POSIX Rename",
    summary:
      "commitFragmentSafely validates the new Caddy fragment in an isolated temp sandbox, reloads Caddy onto :41823 (and optional Go sidecar), then atomically renames current.tmp to current.",
    sourceRef: "daemon/internal/daemon/caddy_manager.go & command_handler.go",
    caddyState: "VALIDATED -> Reloaded to :41823",
    oldNodeState: "Draining remaining connections",
    newNodeState: "LIVE (100% traffic)",
    symlinkState: "os.Rename(current.tmp -> current)",
    staticPoolState: "R1 + R2 chunks active",
    highlightLines: [1, 2, 6],
    code: `if err := ch.caddyManager.Validate(); err != nil { /* rollback fragment */ }
if err := ch.caddyManager.Reload(); err != nil   { /* rollback fragment */ }

tmpSymlink := currentSymlink + ".tmp"
_ = os.Symlink(ctx.ReleaseDir, tmpSymlink)
if err := os.Rename(tmpSymlink, currentSymlink); err != nil {
    /* restore fragment & bail */
}
activationCommitted = true`,
  },
  {
    id: "drain",
    step: "05",
    label: "Drain & Mark-Sweep Cleanup",
    summary:
      "Older systemd units stop gracefully (SIGTERM, 10s TimeoutStopSec), release history prunes to 5 retained builds, and pruneSharedStaticAt removes only chunks no retained release needs.",
    sourceRef: "daemon/internal/daemon/command_handler.go & process_manager.go",
    caddyState: "Serving :41823 + Coraza WAF",
    oldNodeState: "STOPPED & unit removed",
    newNodeState: "LIVE on 127.0.0.1:41823",
    symlinkState: "current -> releases/1763890000-a1b2c3d",
    staticPoolState: "Pruned chunks unreferenced by last 5",
    highlightLines: [3, 6, 7],
    code: `for _, s := range services {
    if s != serviceName {
        _ = ch.processManager.RemoveService(s) // SIGTERM -> 10s -> SIGKILL
    }
}
if err := pruneReleases(ctx.AppName, 5); err == nil {
    _ = pruneSharedStatic(ctx.AppName, 5) // mark-and-sweep across last 5
}`,
  },
];

const GO_KEYWORDS = new Set([
  "if",
  "else",
  "return",
  "defer",
  "for",
  "range",
  "func",
  "var",
  "const",
  "type",
  "struct",
  "package",
  "import",
]);

const GO_LITERALS = new Set(["true", "false", "nil"]);

function highlightGoLine(line: string): React.ReactNode[] {
  if (!line) {
    return ["\u00A0"];
  }

  const tokenRe =
    /(\/\/.*$|\/\*[\s\S]*?\*\/|"(?:\\.|[^"\\])*"|\b\d+\b|:=|!=|==|=|<|>|\+|-|\*|!|[{}()[\],.;:]|\b[A-Za-z_][A-Za-z0-9_]*\b)/g;

  const nodes: React.ReactNode[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  let keyIdx = 0;

  while ((match = tokenRe.exec(line)) !== null) {
    if (match.index > lastIndex) {
      nodes.push(line.slice(lastIndex, match.index));
    }

    const token = match[0];
    const start = match.index;
    const end = start + token.length;
    const after = line.slice(end);
    const before = line.slice(0, start);
    let cls = "text-slate-100";

    if (token.startsWith("//") || token.startsWith("/*")) {
      cls = "text-slate-500 italic";
    } else if (token.startsWith('"')) {
      cls = "text-emerald-300";
    } else if (/^\d+$/.test(token)) {
      cls = "text-amber-300";
    } else if (
      token === ":=" ||
      token === "!=" ||
      token === "==" ||
      token === "=" ||
      token === "<" ||
      token === ">" ||
      token === "+" ||
      token === "-" ||
      token === "*" ||
      token === "!"
    ) {
      cls = "text-sky-300/80";
    } else if (/^[{}()[\],.;:]$/.test(token)) {
      cls = "text-slate-400";
    } else if (GO_KEYWORDS.has(token)) {
      cls = "text-fuchsia-400 font-medium";
    } else if (GO_LITERALS.has(token)) {
      cls = "text-amber-300";
    } else if (token === "_") {
      cls = "text-slate-500";
    } else if (/^\s*\(/.test(after)) {
      cls = "text-sky-300";
    } else if (/^\s*:(?!=)/.test(after)) {
      cls = "text-rose-300";
    } else if (/^\s*\{/.test(after)) {
      cls = "text-teal-300";
    } else if (after.startsWith(".") && !before.endsWith(".")) {
      cls = "text-teal-300/90";
    } else if (before.endsWith(".")) {
      cls = /^[A-Z]/.test(token) ? "text-cyan-200" : "text-indigo-200";
    }

    nodes.push(
      <span key={keyIdx++} className={cls}>
        {token}
      </span>
    );
    lastIndex = end;
  }

  if (lastIndex < line.length) {
    nodes.push(line.slice(lastIndex));
  }

  return nodes;
}

export default function AtomicCutoverExplorer() {
  const [activeIdx, setActiveIdx] = useState(0);
  const [copied, setCopied] = useState(false);
  const stage = CUTOVER_STAGES[activeIdx];
  const codeLines = stage.code.split("\n");

  const selectStage = (idx: number) => {
    setActiveIdx(idx);
    setCopied(false);
  };

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(stage.code);
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    } catch (err) {
      console.error("Failed to copy Go snippet:", err);
    }
  };

  return (
    <div className="min-w-0 max-w-full overflow-hidden bg-surface-900/45 border border-surface-800 rounded-2xl p-3.5 sm:p-6">
      {/* Stage selector tabs */}
      <div
        className="flex flex-nowrap sm:flex-wrap gap-2 mb-5 sm:mb-6 overflow-x-auto pb-2 sm:pb-0 -mx-1 px-1 sm:mx-0 sm:px-0"
        role="tablist"
        aria-label="Atomic zero-downtime cutover stages"
      >
        {CUTOVER_STAGES.map((item, idx) => {
          const active = idx === activeIdx;
          return (
            <button
              key={item.id}
              type="button"
              role="tab"
              aria-selected={active}
              onClick={() => selectStage(idx)}
              className={`shrink-0 whitespace-nowrap inline-flex items-center gap-2 px-3 py-2 rounded-lg text-xs sm:text-sm font-medium transition-colors border ${
                active
                  ? "bg-brand-500/15 text-brand-300 border-brand-500/40"
                  : "bg-surface-950/70 text-surface-400 border-surface-800 hover:text-surface-200 hover:border-surface-700"
              }`}
            >
              <span className="font-mono text-xs text-brand-400">{item.step}</span>
              <span>{item.label}</span>
            </button>
          );
        })}
      </div>

      {/* Content grid */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4 sm:gap-5 items-stretch">
        {/* Left: Live runtime state panel */}
        <div className="lg:col-span-5 min-w-0 flex flex-col justify-between bg-surface-950/70 border border-surface-800 rounded-xl p-4 sm:p-5">
          <div>
            <p className="text-xs font-mono uppercase tracking-wider text-brand-400 mb-2">
              Stage {stage.step} — Host State
            </p>
            <p className="text-sm text-surface-300 leading-relaxed mb-5">
              {stage.summary}
            </p>

            <dl className="space-y-2.5 text-xs font-mono">
              <div className="min-w-0 flex flex-col sm:flex-row sm:justify-between gap-1 p-2.5 rounded-lg bg-surface-900/80 border border-surface-800/80">
                <dt className="text-surface-400 shrink-0">Caddy Edge</dt>
                <dd className="text-code-300 break-words sm:text-right">{stage.caddyState}</dd>
              </div>
              <div className="min-w-0 flex flex-col sm:flex-row sm:justify-between gap-1 p-2.5 rounded-lg bg-surface-900/80 border border-surface-800/80">
                <dt className="text-surface-400 shrink-0">Old Release</dt>
                <dd className="text-surface-200 break-words sm:text-right">{stage.oldNodeState}</dd>
              </div>
              <div className="min-w-0 flex flex-col sm:flex-row sm:justify-between gap-1 p-2.5 rounded-lg bg-surface-900/80 border border-brand-500/20">
                <dt className="text-surface-400 shrink-0">Candidate</dt>
                <dd className="text-brand-300 break-words sm:text-right">{stage.newNodeState}</dd>
              </div>
              <div className="min-w-0 flex flex-col sm:flex-row sm:justify-between gap-1 p-2.5 rounded-lg bg-surface-900/80 border border-surface-800/80">
                <dt className="text-surface-400 shrink-0">Symlink</dt>
                <dd className="text-info-300 break-words sm:text-right">{stage.symlinkState}</dd>
              </div>
              <div className="min-w-0 flex flex-col sm:flex-row sm:justify-between gap-1 p-2.5 rounded-lg bg-surface-900/80 border border-surface-800/80">
                <dt className="text-surface-400 shrink-0">shared_static</dt>
                <dd className="text-warning-300 break-words sm:text-right">{stage.staticPoolState}</dd>
              </div>
            </dl>
          </div>

          <div className="mt-5 pt-4 border-t border-surface-800/80">
            <Link
              href="/docs/vps/atomic-cutover"
              className="inline-flex items-center gap-1.5 text-xs font-medium text-brand-400 hover:text-brand-300 transition-colors"
            >
              Read atomic cutover documentation
              <ArrowRight className="w-3.5 h-3.5" aria-hidden="true" />
            </Link>
          </div>
        </div>

        {/* Right: Verbatim Go source */}
        <div className="lg:col-span-7 min-w-0 flex flex-col rounded-xl overflow-hidden border border-surface-800 bg-[#090d16] shadow-[0_20px_50px_-24px_rgba(16,185,129,0.28)] relative">
          <div
            className="h-px w-full bg-gradient-to-r from-brand-500/60 via-cyan-400/40 to-transparent"
            aria-hidden="true"
          />

          <div className="flex items-center justify-between gap-2 px-3.5 sm:px-4 py-2.5 bg-surface-900/90 border-b border-surface-800/90">
            <div className="flex items-center gap-2.5 min-w-0">
              <span className="flex items-center gap-1.5 shrink-0" aria-hidden="true">
                <span className="w-2.5 h-2.5 rounded-full bg-rose-500/75" />
                <span className="w-2.5 h-2.5 rounded-full bg-amber-400/75" />
                <span className="w-2.5 h-2.5 rounded-full bg-emerald-400/75" />
              </span>
              <span className="font-mono text-[11px] sm:text-xs text-slate-300 truncate min-w-0">
                {stage.sourceRef}
              </span>
            </div>

            <div className="flex items-center gap-2 shrink-0">
              <span className="font-mono text-[10px] uppercase tracking-wider px-2 py-0.5 rounded-md bg-cyan-500/10 text-cyan-300 border border-cyan-500/25">
                Go
              </span>
              <button
                type="button"
                onClick={handleCopy}
                aria-label="Copy Go snippet"
                className="inline-flex items-center gap-1 px-2 py-1 rounded-md text-[11px] font-mono text-slate-400 hover:text-white hover:bg-surface-800/80 border border-transparent hover:border-surface-700 transition-colors"
              >
                {copied ? (
                  <Check className="w-3.5 h-3.5 text-brand-400" aria-hidden="true" />
                ) : (
                  <Copy className="w-3.5 h-3.5" aria-hidden="true" />
                )}
                <span className={copied ? "text-brand-400" : ""}>
                  {copied ? "Copied" : "Copy"}
                </span>
              </button>
            </div>
          </div>

          <div className="flex-1 overflow-x-auto py-3.5 sm:py-4 font-mono text-[11px] sm:text-[13px] leading-[1.7] [scrollbar-width:thin]">
            <div className="min-w-max">
              {codeLines.map((line, i) => {
                const lineNo = i + 1;
                const isHighlighted = stage.highlightLines.includes(lineNo);
                return (
                  <div
                    key={`${stage.id}-${lineNo}`}
                    className={
                      isHighlighted
                        ? "flex items-baseline px-3 sm:px-4 border-l-2 border-brand-400 bg-brand-500/[0.08]"
                        : "flex items-baseline px-3 sm:px-4 border-l-2 border-transparent hover:bg-white/[0.025] transition-colors"
                    }
                  >
                    <span
                      aria-hidden="true"
                      className={`w-6 sm:w-7 shrink-0 select-none text-right pr-3 text-[10px] sm:text-[11px] ${
                        isHighlighted ? "text-brand-400/90" : "text-slate-600"
                      }`}
                    >
                      {lineNo}
                    </span>
                    <span className="whitespace-pre">{highlightGoLine(line)}</span>
                  </div>
                );
              })}
            </div>
          </div>

          <div className="flex items-center justify-between gap-2 px-3.5 sm:px-4 py-2 bg-surface-900/70 border-t border-surface-800/90 text-[11px] font-mono text-slate-400">
            <div className="flex items-center gap-2 min-w-0">
              <span>Stage {stage.step} / 05</span>
              <span className="hidden sm:inline text-slate-500">
                · highlighted = critical cutover ops
              </span>
            </div>

            <div className="flex items-center gap-1.5 shrink-0">
              <button
                type="button"
                disabled={activeIdx === 0}
                onClick={() => selectStage(activeIdx - 1)}
                className="inline-flex items-center gap-1 px-2 py-1 rounded border border-surface-800 bg-surface-950/80 text-slate-300 hover:text-white hover:border-surface-700 disabled:opacity-35 disabled:pointer-events-none transition-colors"
              >
                <ChevronLeft className="w-3.5 h-3.5" aria-hidden="true" />
                Prev
              </button>
              <button
                type="button"
                disabled={activeIdx === CUTOVER_STAGES.length - 1}
                onClick={() => selectStage(activeIdx + 1)}
                className="inline-flex items-center gap-1 px-2 py-1 rounded border border-surface-800 bg-surface-950/80 text-slate-300 hover:text-white hover:border-surface-700 disabled:opacity-35 disabled:pointer-events-none transition-colors"
              >
                Next
                <ChevronRight className="w-3.5 h-3.5" aria-hidden="true" />
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
