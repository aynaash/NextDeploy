import React from "react";
import "./landing-showcase.css";

export function ShipSpeederBanner() {
  return (
    <div
      className="nd-ship-stage mb-6"
      role="img"
      aria-label="Animated NextDeploy ship cutover banner"
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

      <div className="relative z-20 flex flex-wrap items-center justify-between gap-2 px-4 py-3">
        <span className="inline-flex items-center gap-2 px-2.5 py-1 rounded-full bg-surface-950/80 border border-brand-500/30 text-brand-300 font-mono text-xs">
          <span className="w-2 h-2 rounded-full bg-brand-400 animate-pulse" />
          nextdeploy ship — zero-downtime cutover
        </span>
        <span className="font-mono text-[11px] text-surface-300 bg-surface-950/75 px-2.5 py-1 rounded-full border border-surface-800">
          Linux VPS · AWS · Cloudflare
        </span>
      </div>
    </div>
  );
}

export function WindowedConfigCard() {
  return (
    <div className="nd-win-card">
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
          <code>
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
          </code>
        </pre>
      </div>
    </div>
  );
}

export function TerminalWorkflowCard() {
  return (
    <div className="nd-term-card">
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
            <span className="nd-white"># Install &amp; bootstrap project</span>
            {"\n"}
            <span className="nd-red">curl</span> <span className="nd-blue">-sSf</span>{" "}
            <span className="nd-orange">https://nextdeploy.org/install.sh</span>{" "}
            <span className="nd-white">|</span> <span className="nd-violet">sh</span>
            {"\n"}
            <span className="nd-violet">nextdeploy</span>{" "}
            <span className="nd-green">init</span>
            {"\n\n"}
            <span className="nd-white"># Provision VPS &amp; ship zero-downtime</span>
            {"\n"}
            <span className="nd-violet">nextdeploy</span>{" "}
            <span className="nd-blue">prepare</span>
            {"\n"}
            <span className="nd-violet">nextdeploy</span>{" "}
            <span className="nd-green">ship</span>{" "}
            <span className="nd-orange">--verify</span>
            {"\n\n"}
            <span className="nd-white"># Rollback or inspect agent trace</span>
            {"\n"}
            <span className="nd-violet">nextdeploy</span>{" "}
            <span className="nd-red">rollback</span>{" "}
            <span className="nd-blue">--steps 1</span>
            {"\n"}
            <span className="nd-violet">nextdeploy</span>{" "}
            <span className="nd-blue">trace ui</span>{" "}
            <span className="nd-white"># http://localhost:4044</span>
          </code>
        </pre>
      </div>
    </div>
  );
}

export default function LandingCodeShowcase() {
  return (
    <div>
      <ShipSpeederBanner />
      <div className="grid lg:grid-cols-2 gap-6 items-stretch">
        <WindowedConfigCard />
        <TerminalWorkflowCard />
      </div>
    </div>
  );
}
