"use client";

import { useState } from "react";
import { Terminal, Server, Monitor, Copy, Check } from "lucide-react";

const installOptions = [
  {
    id: "cli-unix",
    name: "Mac/Linux CLI",
    icon: Terminal,
    cmd: "curl -sSf https://nextdeploy.org/install.sh | sh",
    desc: "Install NextDeploy CLI for macOS and Linux",
  },
  {
    id: "cli-win",
    name: "Windows CLI",
    icon: Monitor,
    cmd: "curl.exe -sSfO https://nextdeploy.org/install.bat && install.bat",
    desc: "Install NextDeploy CLI on Windows",
  },
  {
    id: "daemon",
    name: "Ubuntu Server",
    icon: Server,
    cmd: "curl -sSf https://nextdeploy.org/daemon.sh | sh",
    desc: "Install NextDeploy Daemon (auto-installed during nextdeploy prepare)",
  },
];

export default function InstallTabs() {
  const [copied, setCopied] = useState(false);
  const [activeTab, setActiveTab] = useState(installOptions[0]);

  const copyCmd = async () => {
    try {
      await navigator.clipboard.writeText(activeTab.cmd);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error("Failed to copy install command:", err);
    }
  };

  return (
    <div id="install" className="max-w-2xl mx-auto group scroll-mt-24 min-w-0">
      <div className="relative bg-surface-900/70 rounded-xl border border-surface-800 overflow-hidden text-left min-w-0">
        {/* Tabs header */}
        <div className="flex border-b border-surface-800 bg-surface-950/60 overflow-x-auto">
          {installOptions.map((option) => {
            const Icon = option.icon;
            const isActive = activeTab.id === option.id;
            return (
              <button
                key={option.id}
                type="button"
                onClick={() => setActiveTab(option)}
                className={`flex items-center gap-2 px-3 sm:px-4 py-2.5 text-xs font-medium transition-colors border-b-2 whitespace-nowrap shrink-0 ${
                  isActive
                    ? "text-brand-400 border-brand-500 bg-surface-900/60"
                    : "text-surface-400 border-transparent hover:text-surface-200 hover:bg-surface-800/30"
                }`}
              >
                <Icon className="w-3.5 h-3.5" aria-hidden="true" />
                {option.name}
              </button>
            );
          })}
        </div>

        {/* Terminal header */}
        <div className="flex items-center justify-between px-4 sm:px-5 py-2.5 bg-surface-950/40 border-b border-surface-800/80">
          <div className="flex items-center gap-2">
            <Terminal className="w-3.5 h-3.5 text-surface-500" aria-hidden="true" />
            <span className="text-xs text-surface-400 font-mono">
              {activeTab.id === "cli-win" ? "install.bat" : "install.sh"}
            </span>
          </div>
          <button
            type="button"
            onClick={copyCmd}
            aria-label={`Copy ${activeTab.name} install command`}
            className="flex items-center gap-1.5 px-2.5 py-1 text-xs text-surface-400 hover:text-surface-200 hover:bg-surface-800/80 rounded transition-colors font-medium"
          >
            {copied ? (
              <Check className="w-3.5 h-3.5 text-brand-400" aria-hidden="true" />
            ) : (
              <Copy className="w-3.5 h-3.5" aria-hidden="true" />
            )}
            <span className={copied ? "text-brand-400" : ""}>
              {copied ? "Copied!" : "Copy"}
            </span>
          </button>
        </div>

        {/* Code Execution Block */}
        <div className="p-4 sm:p-6 font-mono text-xs sm:text-sm">
          <div className="flex flex-col gap-3">
            <div className="flex items-start gap-3">
              <span className="text-brand-400 select-none shrink-0 font-medium">
                $
              </span>
              <div className="flex-1 min-w-0">
                <code className="text-surface-100 break-all font-medium leading-relaxed">
                  {activeTab.cmd}
                </code>
                <div className="text-surface-500 mt-3 text-xs">
                  # {activeTab.desc}
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
