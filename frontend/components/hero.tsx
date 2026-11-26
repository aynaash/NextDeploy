"use client";

import { useState } from "react";
import Link from "next/link";
import { ArrowRight, BookOpen, Check, Copy, Github } from "lucide-react";
import HeroShipConsole from "@/components/hero-ship-console";
import "./landing-showcase.css";

const INSTALL_CMDS = {
  unix: {
    label: "macOS / Linux",
    cmd: "curl -sSf https://nextdeploy.org/install.sh | sh",
  },
  win: {
    label: "Windows",
    cmd: "curl.exe -sSfO https://nextdeploy.org/install.bat && install.bat",
  },
} as const;

const PROOF_METRICS = [
  { k: "Cutover", v: "current.tmp -> current" },
  { k: "Rollback", v: "5-release ring buffer" },
  { k: "Targets", v: "VPS · AWS · Cloudflare" },
  { k: "Trace UI", v: "localhost:4044 + MCP" },
];

export default function Hero() {
  const [os, setOs] = useState<"unix" | "win">("unix");
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(INSTALL_CMDS[os].cmd);
      setCopied(true);
      setTimeout(() => setCopied(false), 1800);
    } catch (err) {
      console.error("Failed to copy install command:", err);
    }
  };

  return (
    <section
      aria-labelledby="hero-heading"
      className="relative overflow-hidden bg-surface-950 border-b border-surface-800/80 nd-blueprint-grid"
    >
      <div className="relative z-10 w-full max-w-7xl mx-auto px-4 sm:px-6 py-8 sm:py-16 lg:py-20">
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-8 items-center">
          {/* Left column: concrete engineering pitch + instant install */}
          <div className="lg:col-span-5 text-left min-w-0">
            <h1
              id="hero-heading"
              className="text-[1.75rem] sm:text-5xl font-bold mb-4 sm:mb-5 text-white tracking-tight leading-[1.12] sm:leading-[1.08]"
            >
              Ship Next.js to your own metal, AWS, or Cloudflare.{" "}
              <span className="text-brand-400">Zero platform lock-in.</span>
            </h1>

            <p className="text-sm sm:text-base text-surface-300 mb-6 leading-relaxed">
              One <code className="text-brand-300 font-mono text-xs sm:text-sm">nextdeploy.yml</code>{" "}
              and one static CLI. NextDeploy inspects your compiled manifests into{" "}
              <code className="text-brand-300 font-mono text-xs sm:text-sm">metadata.json</code>,
              gates every cutover on dual-stage TCP + HTTP health checks, pools{" "}
              <code className="text-brand-300 font-mono text-xs sm:text-sm">shared_static</code>{" "}
              chunks, and hardens your host with Coraza WAF and systemd isolation.
            </p>

            {/* Primary CTAs */}
            <div className="grid grid-cols-2 sm:flex sm:flex-wrap items-center gap-2.5 sm:gap-3 mb-5 sm:mb-6">
              <Link
                href="/docs/getting-started/installation"
                className="col-span-2 sm:col-span-1 px-5 py-2.5 rounded-lg bg-brand-500 hover:bg-brand-400 text-surface-950 font-semibold transition-colors inline-flex items-center justify-center gap-2 group text-sm"
              >
                Get Started
                <ArrowRight
                  className="w-4 h-4 group-hover:translate-x-0.5 transition-transform"
                  aria-hidden="true"
                />
              </Link>
              <Link
                href="/docs"
                className="px-3 sm:px-4 py-2.5 rounded-lg bg-surface-900 hover:bg-surface-800 text-surface-100 font-medium border border-surface-700 transition-colors inline-flex items-center justify-center gap-2 text-sm"
              >
                <BookOpen className="w-4 h-4 text-brand-400 shrink-0" aria-hidden="true" />
                Read Documentation
              </Link>
              <a
                href="https://github.com/aynaash/NextDeploy"
                target="_blank"
                rel="noopener noreferrer"
                className="px-3 sm:px-3.5 py-2.5 rounded-lg bg-transparent hover:bg-surface-900 text-surface-300 hover:text-white border border-surface-800 transition-colors inline-flex items-center justify-center gap-2 text-sm"
              >
                <Github className="w-4 h-4 shrink-0" aria-hidden="true" />
                GitHub
              </a>
            </div>

            {/* Compact inline install strip */}
            <div className="min-w-0 max-w-full bg-surface-900/90 border border-surface-800 rounded-xl p-3 mb-6">
              <div className="flex items-center justify-between gap-2 mb-2">
                <div className="inline-flex items-center gap-1 text-[11px] font-mono">
                  {(["unix", "win"] as const).map((key) => (
                    <button
                      key={key}
                      type="button"
                      onClick={() => setOs(key)}
                      className={`px-2.5 py-1 rounded transition-colors ${
                        os === key
                          ? "bg-brand-500/15 text-brand-300 border border-brand-500/30"
                          : "text-surface-400 hover:text-surface-200 border border-transparent"
                      }`}
                    >
                      {INSTALL_CMDS[key].label}
                    </button>
                  ))}
                </div>
                <button
                  type="button"
                  onClick={handleCopy}
                  className="inline-flex items-center gap-1 px-2 py-1 text-[11px] font-mono text-surface-400 hover:text-white transition-colors shrink-0"
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
              <div className="flex items-center gap-2 font-mono text-xs text-surface-100 min-w-0 overflow-x-auto pb-1">
                <span className="text-brand-400 select-none shrink-0">$</span>
                <code className="whitespace-nowrap">{INSTALL_CMDS[os].cmd}</code>
              </div>
            </div>

            {/* Monospace proof strip */}
            <dl className="grid grid-cols-2 gap-2 sm:gap-2.5">
              {PROOF_METRICS.map((m) => (
                <div
                  key={m.k}
                  className="min-w-0 px-3 py-2 rounded-lg bg-surface-900/50 border border-surface-800/80"
                >
                  <dt className="text-[10px] font-mono uppercase tracking-wider text-surface-500">
                    {m.k}
                  </dt>
                  <dd className="text-[11px] sm:text-xs font-mono text-surface-200 mt-0.5 truncate">
                    {m.v}
                  </dd>
                </div>
              ))}
            </dl>
          </div>

          {/* Right column: Interactive Ship Console */}
          <div className="lg:col-span-7 min-w-0">
            <HeroShipConsole />
          </div>
        </div>
      </div>
    </section>
  );
}
