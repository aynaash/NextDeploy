"use client";

import { useState, Suspense } from "react";
import {
  Terminal,
  Github,
  Zap,
  ArrowRight,
  Server,
  Monitor,
  Copy,
  Check,
  Play,
  Clock,
  Eye,
  Calendar,
} from "lucide-react";
import GitHubStarButton from "@/components/github-star-button";

const installOptions = [
  {
    id: "cli-unix",
    name: "Mac/Linux CLI",
    icon: Terminal,
    cmd: "curl -sSf https://nextdeploy.org/install.sh | sh",
    desc: "Install CLI tool for developers",
  },
  {
    id: "cli-win",
    name: "Windows CLI",
    icon: Monitor,
    cmd: "curl.exe -sSfO https://nextdeploy.org/install.bat && install.bat",
    desc: "Install CLI tool on Windows",
  },
  {
    id: "daemon",
    name: "Ubuntu Server",
    icon: Server,
    cmd: "curl -sSf https://nextdeploy.org/daemon.sh | sh",
    desc: "Install NextDeploy Daemon: this daemon is auto-installed on the server it is the control plane",
  },
];

export default function Hero() {
  const [copied, setCopied] = useState(false);
  const [activeTab, setActiveTab] = useState(installOptions[0]);

  const copyCmd = () => {
    navigator.clipboard.writeText(activeTab.cmd);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <section className="relative flex items-center justify-center overflow-hidden bg-surface-950 border-b border-surface-800/80 min-h-[calc(100vh-4rem)] lg:min-h-screen">
      <div className="relative z-10 w-full max-w-7xl mx-auto px-4 sm:px-6 py-12 sm:py-16 md:py-20">
        {/* Main headline */}
        <h1 className="text-3xl sm:text-4xl md:text-5xl font-bold mb-4 text-white tracking-tight leading-tight text-center">
          Next.js Deployment
          <br />
          Without the Black Box
        </h1>

        {/* Tagline */}
        <p className="text-sm md:text-base text-surface-300 mb-8 max-w-2xl mx-auto leading-relaxed text-center">
          The high-performance, developer-first deployment engine. Transform any
          VPS into a premium hosting platform with
          <strong className="text-white font-semibold"> total ownership</strong> and <strong className="text-white font-semibold">zero-downtime</strong>{" "}
          updates.
        </p>

        {/* Action Buttons */}
        <div className="flex flex-col sm:flex-row items-center justify-center gap-3 mb-10">
          <a
            href="#install"
            className="px-5 py-2.5 w-full sm:w-auto rounded-lg bg-brand-500 hover:bg-brand-400 text-surface-950 font-semibold transition-colors flex items-center justify-center gap-2 group text-sm"
          >
            Start Shipping Now
            <ArrowRight className="w-4 h-4 group-hover:translate-x-0.5 transition-transform" />
          </a>
          <a
            href="/docs"
            className="px-5 py-2.5 w-full sm:w-auto rounded-lg bg-surface-900 hover:bg-surface-800 text-surface-200 font-medium border border-surface-800 hover:border-surface-700 transition-colors flex items-center justify-center gap-2 text-sm"
          >
            Read Documentation
          </a>
        </div>

        <div className="mb-4"></div>

        {/* Install command box */}
        <div id="install" className="max-w-2xl mx-auto group scroll-mt-24">
          <div className="relative bg-surface-900/70 rounded-xl border border-surface-800 overflow-hidden text-left">
            {/* Tabs header */}
            <div className="flex border-b border-surface-800 bg-surface-950/60 overflow-x-auto">
              {installOptions.map((option) => {
                const Icon = option.icon;
                const isActive = activeTab.id === option.id;
                return (
                  <button
                    key={option.id}
                    onClick={() => setActiveTab(option)}
                    className={`flex items-center gap-2 px-4 py-2.5 text-xs font-medium transition-colors border-b-2 whitespace-nowrap ${isActive
                      ? "text-brand-400 border-brand-500 bg-surface-900/60"
                      : "text-surface-400 border-transparent hover:text-surface-200 hover:bg-surface-800/30"
                      }`}
                  >
                    <Icon className="w-3.5 h-3.5" />
                    {option.name}
                  </button>
                );
              })}
            </div>

            {/* Terminal header */}
            <div className="flex items-center justify-between px-4 sm:px-5 py-2.5 bg-surface-950/40 border-b border-surface-800/80">
              <div className="flex items-center gap-2">
                <Terminal className="w-3.5 h-3.5 text-surface-500" />
                <span className="text-xs text-surface-400 font-mono">
                  {activeTab.id === "cli-win" ? "install.bat" : "install.sh"}
                </span>
              </div>
              <button
                onClick={copyCmd}
                className="flex items-center gap-1.5 px-2.5 py-1 text-xs text-surface-400 hover:text-surface-200 hover:bg-surface-800/80 rounded transition-colors font-medium"
              >
                {copied ? (
                  <Check className="w-3.5 h-3.5 text-brand-400" />
                ) : (
                  <Copy className="w-3.5 h-3.5" />
                )}
                <span className={copied ? "text-brand-400" : ""}>{copied ? "Copied!" : "Copy"}</span>
              </button>
            </div>

            {/* Code Execution Block */}
            <div className="p-4 sm:p-6 font-mono text-xs sm:text-sm">
              <div className="flex flex-col gap-3 group">
                <div className="flex items-start gap-3">
                  <span className="text-brand-400 select-none shrink-0 font-medium">
                    $
                  </span>
                  <div className="flex-1">
                    <code className="text-surface-100 break-all font-medium leading-relaxed">
                      {activeTab.cmd}
                    </code>
                    <div className="text-surface-500 mt-3 text-xs flex items-center gap-1.5">
                      # {activeTab.desc}
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* YouTube Tutorial Section */}
        <div className="mt-12 sm:mt-16 max-w-4xl mx-auto text-center">
          <div className="mb-6">
            <h2 className="text-xl sm:text-2xl font-bold mb-2 text-white">
              Watch the Tutorial
            </h2>
            <p className="text-surface-400 text-sm max-w-xl mx-auto">
              How to deploy Next.js to your VPS with NextDeploy
            </p>
          </div>
          <div className="aspect-video w-full rounded-xl overflow-hidden border border-surface-800 bg-surface-900/50">
            <iframe
              width="100%"
              height="100%"
              src="https://www.youtube.com/embed/9C8WNZSS5o0"
              title="NextDeploy VPS Tutorial"
              allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
              allowFullScreen
              className="w-full h-full border-none"
            ></iframe>
          </div>
        </div>
      </div>
    </section>
  );
}
