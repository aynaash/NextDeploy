"use client";

import React, { useEffect, useState } from "react";
import { RotateCcw, Server, Cloud, Globe } from "lucide-react";
import "./landing-showcase.css";

type TargetKey = "vps" | "cloudflare" | "aws";

interface TerminalLine {
  prefix: string;
  text: string;
  tone: "muted" | "blue" | "orange" | "green" | "violet";
}

interface TargetPreset {
  id: TargetKey;
  label: string;
  shortLabel: string;
  badge: string;
  icon: React.ComponentType<{ className?: string }>;
  liveLabel: string;
  configLines: React.ReactNode;
  shipLines: TerminalLine[];
}

const TARGET_PRESETS: Record<TargetKey, TargetPreset> = {
  vps: {
    id: "vps",
    label: "Linux VPS",
    shortLabel: "VPS",
    badge: "nextdeployd + Caddy",
    icon: Server,
    liveLabel: "https://app.example.com (1763890000-a1b2c3d)",
    configLines: (
      <>
        <span className="nd-s1">version</span>
        <span className="nd-sc">:</span> <span className="nd-s4">&quot;1.0&quot;</span>
        {"\n"}
        <span className="nd-s1">target_type</span>
        <span className="nd-sc">:</span> <span className="nd-s2">vps</span>
        {"\n\n"}
        <span className="nd-s3">app</span>
        <span className="nd-sc">:</span>
        {"\n  "}
        <span className="nd-s2">name</span>
        <span className="nd-sc">:</span> <span className="nd-s4">my-next-app</span>
        {"\n  "}
        <span className="nd-s2">domain</span>
        <span className="nd-sc">:</span> <span className="nd-s4">app.example.com</span>
        {"\n  "}
        <span className="nd-s2">health_path</span>
        <span className="nd-sc">:</span> <span className="nd-s4">/api/health</span>
        {"\n  "}
        <span className="nd-s2">gosidecar</span>
        <span className="nd-sc">:</span> <span className="nd-curlies">{"{"}</span>{" "}
        <span className="nd-s6">enabled</span>
        <span className="nd-sc">:</span> <span className="nd-op">true</span>{" "}
        <span className="nd-curlies">{"}"}</span>
        {"\n\n"}
        <span className="nd-s3">servers</span>
        <span className="nd-sc">:</span>
        {"\n  - "}
        <span className="nd-s2">name</span>
        <span className="nd-sc">:</span> <span className="nd-s4">prod</span>
        {"\n    "}
        <span className="nd-s2">host</span>
        <span className="nd-sc">:</span> <span className="nd-s5">1.2.3.4</span>
        {"\n    "}
        <span className="nd-s2">username</span>
        <span className="nd-sc">:</span> <span className="nd-s4">deploy</span>
      </>
    ),
    shipLines: [
      { prefix: "$", text: "nextdeploy ship --verify", tone: "violet" },
      { prefix: "•", text: "nextcore: 14 routes, ISR + Server Actions", tone: "muted" },
      { prefix: "•", text: "ssh: curve25519 + chacha20-poly1305 upload", tone: "blue" },
      { prefix: "•", text: "findFreePort() -> 127.0.0.1:41823 (systemd)", tone: "blue" },
      { prefix: "•", text: "waitForHealthy: TCP ok -> GET /api/health 200", tone: "orange" },
      { prefix: "•", text: "shared_static synced + caddy sandbox valid", tone: "blue" },
      { prefix: "✓", text: "os.Rename(current.tmp -> current) LIVE", tone: "green" },
    ],
  },
  cloudflare: {
    id: "cloudflare",
    label: "Cloudflare Workers",
    shortLabel: "Cloudflare",
    badge: "nextcompile + R2",
    icon: Globe,
    liveLabel: "https://my-next-app.workers.dev",
    configLines: (
      <>
        <span className="nd-s1">version</span>
        <span className="nd-sc">:</span> <span className="nd-s4">&quot;1.0&quot;</span>
        {"\n"}
        <span className="nd-s1">target_type</span>
        <span className="nd-sc">:</span> <span className="nd-s2">serverless</span>
        {"\n\n"}
        <span className="nd-s3">app</span>
        <span className="nd-sc">:</span>
        {"\n  "}
        <span className="nd-s2">name</span>
        <span className="nd-sc">:</span> <span className="nd-s4">my-next-app</span>
        {"\n  "}
        <span className="nd-s2">domain</span>
        <span className="nd-sc">:</span> <span className="nd-s4">app.example.com</span>
        {"\n\n"}
        <span className="nd-s3">serverless</span>
        <span className="nd-sc">:</span>
        {"\n  "}
        <span className="nd-s2">provider</span>
        <span className="nd-sc">:</span> <span className="nd-s4">cloudflare</span>
        {"\n  "}
        <span className="nd-s2">cloudflare</span>
        <span className="nd-sc">:</span>
        {"\n    "}
        <span className="nd-s6">bindings</span>
        <span className="nd-sc">:</span> <span className="nd-curlies">{"{"}</span>{" "}
        <span className="nd-s5">d1</span>
        <span className="nd-sc">:</span> <span className="nd-s4">app-db</span>
        <span className="nd-sc">,</span> <span className="nd-s5">kv</span>
        <span className="nd-sc">:</span> <span className="nd-s4">CACHE</span>{" "}
        <span className="nd-curlies">{"}"}</span>
      </>
    ),
    shipLines: [
      { prefix: "$", text: "nextdeploy apply --yes && nextdeploy ship", tone: "violet" },
      { prefix: "•", text: "reconcile: D1 app-db + KV CACHE + R2 assets", tone: "muted" },
      { prefix: "•", text: "nextcompile: bundle NextServer -> worker.mjs", tone: "blue" },
      { prefix: "•", text: "R2: upload immutable /_next/static/* chunks", tone: "blue" },
      { prefix: "•", text: "deploy-hash: sha256(bundle+meta+secrets)", tone: "orange" },
      { prefix: "•", text: "SmokeVerify: GET / -> HTTP 200 (19ms)", tone: "orange" },
      { prefix: "✓", text: "Worker + custom domain propagated LIVE", tone: "green" },
    ],
  },
  aws: {
    id: "aws",
    label: "AWS Lambda",
    shortLabel: "AWS",
    badge: "Lambda + CloudFront",
    icon: Cloud,
    liveLabel: "https://d1234.cloudfront.net",
    configLines: (
      <>
        <span className="nd-s1">version</span>
        <span className="nd-sc">:</span> <span className="nd-s4">&quot;1.0&quot;</span>
        {"\n"}
        <span className="nd-s1">target_type</span>
        <span className="nd-sc">:</span> <span className="nd-s2">serverless</span>
        {"\n\n"}
        <span className="nd-s3">app</span>
        <span className="nd-sc">:</span>
        {"\n  "}
        <span className="nd-s2">name</span>
        <span className="nd-sc">:</span> <span className="nd-s4">my-next-app</span>
        {"\n  "}
        <span className="nd-s2">domain</span>
        <span className="nd-sc">:</span> <span className="nd-s4">app.example.com</span>
        {"\n\n"}
        <span className="nd-s3">serverless</span>
        <span className="nd-sc">:</span>
        {"\n  "}
        <span className="nd-s2">provider</span>
        <span className="nd-sc">:</span> <span className="nd-s4">aws</span>
        {"\n  "}
        <span className="nd-s2">region</span>
        <span className="nd-sc">:</span> <span className="nd-s4">us-east-1</span>
        {"\n  "}
        <span className="nd-s2">image_optimization</span>
        <span className="nd-sc">:</span> <span className="nd-op">true</span>
      </>
    ),
    shipLines: [
      { prefix: "$", text: "nextdeploy ship --verify", tone: "violet" },
      { prefix: "•", text: "buildflow: standalone audit 42.1MB (<250MB)", tone: "muted" },
      { prefix: "•", text: "secrets: pushed to AWS Secrets Manager", tone: "blue" },
      { prefix: "•", text: "s3: uploaded assets (OAC + immutable cache)", tone: "blue" },
      { prefix: "•", text: "lambda: ARM64 + bridge + S3 manifest (keep=5)", tone: "orange" },
      { prefix: "•", text: "cloudfront: ACM TLS + distribution wired", tone: "orange" },
      { prefix: "✓", text: "SmokeVerify passed -> CloudFront LIVE", tone: "green" },
    ],
  },
};

const TONE_CLASS: Record<TerminalLine["tone"], string> = {
  muted: "nd-white",
  blue: "nd-blue",
  orange: "nd-orange",
  green: "nd-green",
  violet: "nd-violet",
};

export default function HeroShipConsole() {
  const [target, setTarget] = useState<TargetKey>("vps");
  const [runId, setRunId] = useState(0);
  const preset = TARGET_PRESETS[target];
  const [visibleCount, setVisibleCount] = useState(preset.shipLines.length);

  useEffect(() => {
    if (runId === 0) {
      setVisibleCount(preset.shipLines.length);
      return;
    }

    setVisibleCount(1);
    let current = 1;
    const timer = setInterval(() => {
      current += 1;
      setVisibleCount(current);
      if (current >= preset.shipLines.length) {
        clearInterval(timer);
      }
    }, 340);

    return () => clearInterval(timer);
  }, [runId, preset.shipLines.length]);

  const isReplaying = runId > 0 && visibleCount < preset.shipLines.length;
  const activeStep = preset.shipLines[Math.min(visibleCount - 1, preset.shipLines.length - 1)];

  const handleSelectTarget = (next: TargetKey) => {
    setTarget(next);
    setRunId((n) => n + 1);
  };

  return (
    <div className="min-w-0 max-w-full overflow-hidden bg-surface-900/55 border border-surface-800 rounded-2xl p-3 sm:p-4 shadow-2xl shadow-black/40">
      {/* Control bar */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 mb-3">
        <div
          className="grid grid-cols-3 sm:inline-flex items-center gap-1 p-1 rounded-lg bg-surface-950/90 border border-surface-800 w-full sm:w-auto"
          role="tablist"
          aria-label="Deployment target preview"
        >
          {(Object.keys(TARGET_PRESETS) as TargetKey[]).map((key) => {
            const item = TARGET_PRESETS[key];
            const Icon = item.icon;
            const active = target === key;
            return (
              <button
                key={key}
                type="button"
                role="tab"
                aria-selected={active}
                onClick={() => handleSelectTarget(key)}
                className={`inline-flex items-center justify-center gap-1.5 px-2 sm:px-2.5 py-1.5 rounded-md text-xs font-medium transition-colors ${
                  active
                    ? "bg-brand-500/15 text-brand-300 border border-brand-500/30"
                    : "text-surface-400 hover:text-surface-200 border border-transparent"
                }`}
              >
                <Icon className="w-3.5 h-3.5 shrink-0" aria-hidden="true" />
                <span className="sm:hidden">{item.shortLabel}</span>
                <span className="hidden sm:inline">{item.label}</span>
              </button>
            );
          })}
        </div>

        <button
          type="button"
          onClick={() => setRunId((n) => n + 1)}
          className="w-full sm:w-auto inline-flex items-center justify-center gap-1.5 px-2.5 py-1.5 rounded-md bg-surface-950 hover:bg-surface-800 border border-surface-700 text-xs font-mono text-brand-300 transition-colors"
        >
          <RotateCcw
            className={`w-3.5 h-3.5 ${isReplaying ? "animate-spin" : ""}`}
            aria-hidden="true"
          />
          {isReplaying ? "Shipping..." : "Replay ship"}
        </button>
      </div>

      {/* Animated ship stage */}
      <div
        className={`nd-ship-stage nd-ship-stage--compact mb-3 ${
          isReplaying ? "nd-ship-stage--boosting" : ""
        }`}
        role="img"
        aria-label={`Animated NextDeploy ship progress for ${preset.label}`}
      >
        <div className="nd-clouds" aria-hidden="true">
          <div className="nd-cloud nd-cloud1" />
          <div className="nd-cloud nd-cloud2" />
          <div className="nd-cloud nd-cloud3" />
          <div className="nd-cloud nd-cloud4" />
          <div className="nd-cloud nd-cloud5" />
        </div>

        <div className="nd-ship-loader" aria-hidden="true">
          <span>
            <span />
            <span />
            <span />
            <span />
          </span>
          <div className="nd-ship-base">
            <span />
            <div className="nd-ship-face" />
          </div>
        </div>

        <div className="nd-longfazers" aria-hidden="true">
          <span />
          <span />
          <span />
          <span />
        </div>

        <div className="relative z-20 flex flex-wrap items-center justify-between gap-2 px-3 py-2.5 min-w-0">
          <span className="inline-flex items-center gap-2 px-2.5 py-1 rounded-full bg-surface-950/85 border border-brand-500/30 text-brand-300 font-mono text-[11px] truncate max-w-full min-w-0">
            <span className="w-1.5 h-1.5 rounded-full bg-brand-400 animate-pulse shrink-0" />
            <span className="truncate">
              {isReplaying ? activeStep.text : preset.liveLabel}
            </span>
          </span>
          <span className="hidden sm:inline-block font-mono text-[10px] text-surface-300 bg-surface-950/80 px-2 py-0.5 rounded-full border border-surface-800 shrink-0">
            {preset.badge}
          </span>
        </div>
      </div>

      {/* Dual visual cards: Windows Config Card + 3-Dot Terminal Card */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-3 items-stretch min-w-0">
        <div className="nd-win-card nd-win-card--compact">
          <div className="nd-win-titlebar">
            <span className="nd-win-title">nextdeploy.yml</span>
            <span className="nd-win-buttons" aria-hidden="true">
              <button type="button" tabIndex={-1} className="nd-win-btn nd-win-minimize">
                <svg x="0px" y="0px" viewBox="0 0 10.2 1" width={10} height={1}>
                  <rect x={0} y="50%" width="10.2" height={1} />
                </svg>
              </button>
              <button type="button" tabIndex={-1} className="nd-win-btn nd-win-maximize">
                <svg viewBox="0 0 10 10" width={10} height={10}>
                  <path d="M0,0v10h10V0H0z M9,9H1V1h8V9z" />
                </svg>
              </button>
              <button type="button" tabIndex={-1} className="nd-win-btn nd-win-close">
                <svg viewBox="0 0 10 10" width={10} height={10}>
                  <polygon points="10.2,0.7 9.5,0 5.1,4.4 0.7,0 0,0.7 4.4,5.1 0,9.5 0.7,10.2 5.1,5.8 9.5,10.2 10.2,9.5 5.8,5.1" />
                </svg>
              </button>
            </span>
          </div>
          <div className="nd-win-body">
            <pre className="nd-win-pre">
              <code>{preset.configLines}</code>
            </pre>
          </div>
        </div>

        <div className="nd-term-card nd-term-card--compact">
          <div className="nd-term-top">
            <div className="nd-term-dots" aria-hidden="true">
              <div className="nd-term-dot nd-term-dot-1" />
              <div className="nd-term-dot nd-term-dot-2" />
              <div className="nd-term-dot nd-term-dot-3" />
            </div>
            <p className="nd-term-prompt">_&gt;</p>
          </div>
          <div className="nd-term-bottom">
            <pre className="nd-term-code">
              <code>
                {preset.shipLines.slice(0, visibleCount).map((line, i) => (
                  <React.Fragment key={`${target}-${i}`}>
                    <span className="nd-white">{line.prefix} </span>
                    <span className={TONE_CLASS[line.tone]}>{line.text}</span>
                    {i < visibleCount - 1 ? "\n" : ""}
                  </React.Fragment>
                ))}
              </code>
            </pre>
          </div>
        </div>
      </div>
    </div>
  );
}
