import Link from "next/link";
import {
  ArrowRight,
  Bot,
  Check,
  Cpu,
  FileCode2,
  GitBranch,
  Github,
  Layers,
  Minus,
  Shield,
  Star,
} from "lucide-react";
import AtomicCutoverExplorer from "@/components/atomic-cutover-explorer";
import InstallTabs from "@/components/install-tabs";

const METADATA_SAMPLE = `{
  "app_name": "my-next-app",
  "output_mode": "standalone",
  "git_commit": "a1b2c3d4e5f6...",
  "git_dirty": false,
  "route_info": {
    "static_routes": ["/", "/pricing"],
    "ssr_routes": ["/dashboard"],
    "isr_routes": ["/blog/[slug]"]
  },
  "detected_features": { "has_stripe": true, "has_server_actions": true }
}`;

const ARCHITECTURE_GUIDES = [
  { n: "01", title: "Atomic Zero-Downtime Cutover", href: "/docs/vps/atomic-cutover" },
  { n: "02", title: "Instant Rollback", href: "/docs/vps/rollback" },
  { n: "03", title: "Security Model & Coraza WAF", href: "/docs/concepts/security" },
  { n: "04", title: "Automated Provisioning", href: "/docs/advanced/auto-provisioning" },
  { n: "05", title: "Deployment Audit Logging", href: "/docs/operations/audit-log" },
  { n: "06", title: "Build Metadata & Intelligence", href: "/docs/concepts/metadata" },
  { n: "07", title: "Static Chunk Pooling (shared_static)", href: "/docs/vps/chunk-pooling" },
  { n: "08", title: "Agent Trace & MCP Server", href: "/docs/agent-trace/architecture" },
  { n: "09", title: "Zero-Touch CI/CD Pipelines", href: "/docs/operations/ci-cd" },
];

const COMPARISON_ROWS = [
  {
    feature: "Infrastructure Ownership & Cost",
    nextdeploy: "Your VPS, AWS, or Cloudflare account ($0 platform fee)",
    paas: "Per-seat + bandwidth & build markups",
    diy: "Your infra, custom glue scripts to maintain",
  },
  {
    feature: "Zero-Downtime + ChunkLoadError Guard",
    nextdeploy: "TCP/HTTP probe + current.tmp rename + shared_static pool",
    paas: "Managed by vendor CDN/runtime",
    diy: "Manual Nginx/Caddy + broken static chunks on deploy",
  },
  {
    feature: "Target Portability",
    nextdeploy: "Switch VPS <-> AWS <-> Cloudflare in nextdeploy.yml",
    paas: "Locked to vendor primitives",
    diy: "Rewrite Terraform/Docker per provider",
  },
  {
    feature: "AI Coding Agent Continuity",
    nextdeploy: "Native .nextdeploy/trace/ (:4044 UI) + stdio MCP server",
    paas: "None built in",
    diy: "None built in",
  },
  {
    feature: "VPS Edge & Process Hardening",
    nextdeploy: "Coraza WAF (OWASP CRS) + Fail2Ban + systemd + HMAC-SHA256",
    paas: "Vendor-managed edge",
    diy: "Hand-configured firewall, WAF, and systemd units",
  },
];

export default function Features() {
  return (
    <div className="bg-surface-950">
      {/* ─── 1. INTERACTIVE ATOMIC CUTOVER EXPLORER ─── */}
      <section
        aria-labelledby="cutover-heading"
        className="py-12 sm:py-20 lg:py-24 border-b border-surface-800/80"
      >
        <div className="max-w-7xl mx-auto px-4 sm:px-6">
          <div className="max-w-3xl mb-8 sm:mb-10">
            <p className="text-xs font-mono font-semibold tracking-wider uppercase text-brand-400 mb-2">
              Inside nextdeployd
            </p>
            <h2
              id="cutover-heading"
              className="text-2xl sm:text-4xl font-bold text-white tracking-tight mb-3"
            >
              See the cutover, step by step.
            </h2>
            <p className="text-surface-400 text-sm sm:text-base leading-relaxed">
              Every VPS deploy and rollback executes this exact five-stage state machine in Go—keeping
              your previous release online until the candidate passes both TCP and HTTP probes.
            </p>
          </div>

          <AtomicCutoverExplorer />
        </div>
      </section>

      {/* ─── 2. ASYMMETRIC SYSTEMS BENTO GRID ─── */}
      <section
        aria-labelledby="systems-heading"
        className="py-12 sm:py-20 lg:py-24 border-b border-surface-800/80"
      >
        <div className="max-w-7xl mx-auto px-4 sm:px-6">
          <div className="max-w-3xl mb-8 sm:mb-12">
            <p className="text-xs font-mono font-semibold tracking-wider uppercase text-brand-400 mb-2">
              Systems Architecture
            </p>
            <h2
              id="systems-heading"
              className="text-2xl sm:text-4xl font-bold text-white tracking-tight"
            >
              Built from verifiable primitives, not platform magic.
            </h2>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-12 gap-4 sm:gap-5">
            {/* Build Intelligence (7 cols) */}
            <div className="lg:col-span-7 min-w-0 p-4 sm:p-6 bg-surface-900/40 border border-surface-800 rounded-2xl flex flex-col justify-between">
              <div className="min-w-0">
                <div className="flex flex-wrap items-center justify-between gap-2 mb-3">
                  <div className="inline-flex items-center gap-2 text-brand-400 font-mono text-xs uppercase tracking-wider">
                    <FileCode2 className="w-4 h-4 shrink-0" aria-hidden="true" />
                    Build Intelligence (nextcore)
                  </div>
                  <span className="font-mono text-[11px] text-surface-400">
                    .nextdeploy/metadata.json
                  </span>
                </div>
                <h3 className="text-xl font-bold text-white mb-2">
                  Manifest introspection + incremental build.lock
                </h3>
                <p className="text-sm text-surface-300 leading-relaxed mb-4">
                  <code className="text-brand-300 font-mono text-xs">nextdeploy build</code> classifies
                  every static, SSR, ISR, and middleware route, detects integrations like Stripe, and
                  hashes build-affecting config fields in{" "}
                  <code className="text-brand-300 font-mono text-xs">ConfigFingerprint</code> to skip
                  unnecessary <code className="text-brand-300 font-mono text-xs">next build</code> runs.
                </p>
                <pre className="p-3 sm:p-4 rounded-xl bg-surface-950 border border-surface-800 text-[11px] sm:text-xs font-mono text-surface-200 overflow-x-auto max-w-full mb-4">
                  <code>{METADATA_SAMPLE}</code>
                </pre>
              </div>
              <Link
                href="/docs/concepts/metadata"
                className="inline-flex items-center gap-1.5 text-xs font-medium text-brand-400 hover:text-brand-300"
              >
                Explore Build Metadata &amp; metadata.json
                <ArrowRight className="w-3.5 h-3.5" aria-hidden="true" />
              </Link>
            </div>

            {/* Defense-in-Depth Security (5 cols) */}
            <div className="lg:col-span-5 min-w-0 p-4 sm:p-6 bg-surface-900/40 border border-surface-800 rounded-2xl flex flex-col justify-between">
              <div className="min-w-0">
                <div className="inline-flex items-center gap-2 text-danger-400 font-mono text-xs uppercase tracking-wider mb-3">
                  <Shield className="w-4 h-4 shrink-0" aria-hidden="true" />
                  Defense-in-Depth Security
                </div>
                <h3 className="text-xl font-bold text-white mb-3">
                  Edge WAF to kernel cgroups
                </h3>
                <ul className="space-y-2.5 text-xs sm:text-sm text-surface-300 list-none p-0 m-0 mb-5">
                  <li className="p-2.5 rounded-lg bg-surface-950/70 border border-surface-800/80 min-w-0">
                    <span className="font-mono text-brand-300 block text-xs break-words">
                      Coraza WAF (OWASP CRS) + Fail2Ban
                    </span>
                    <span className="text-surface-400 text-xs break-words">
                      Blocks SQLi/XSS at Caddy and auto-bans IPs via{" "}
                      <code className="break-all">/var/log/caddy/audit.log</code>.
                    </span>
                  </li>
                  <li className="p-2.5 rounded-lg bg-surface-950/70 border border-surface-800/80 min-w-0">
                    <span className="font-mono text-info-300 block text-xs break-words">
                      curve25519 + chacha20-poly1305 &amp; HMAC-SHA256
                    </span>
                    <span className="text-surface-400 text-xs break-words">
                      Pinned SSH cipher suite plus 16-byte nonce &amp; 5m ReplayGuard on daemon RPC.
                    </span>
                  </li>
                  <li className="p-2.5 rounded-lg bg-surface-950/70 border border-surface-800/80 min-w-0">
                    <span className="font-mono text-warning-300 block text-xs break-words">
                      ProtectSystem=strict · PrivateTmp=yes · NoNewPrivileges
                    </span>
                    <span className="text-surface-400 text-xs break-words">
                      Unprivileged <code>User=nextdeploy</code> unit + <code>0600</code>{" "}
                      <code>.env.nextdeploy</code> &amp; redacted audit logs.
                    </span>
                  </li>
                </ul>
              </div>
              <Link
                href="/docs/concepts/security"
                className="inline-flex items-center gap-1.5 text-xs font-medium text-brand-400 hover:text-brand-300"
              >
                Read Security Model docs
                <ArrowRight className="w-3.5 h-3.5" aria-hidden="true" />
              </Link>
            </div>

            {/* Agent Trace & MCP (5 cols) */}
            <div className="lg:col-span-5 min-w-0 p-4 sm:p-6 bg-surface-900/40 border border-surface-800 rounded-2xl flex flex-col justify-between">
              <div className="min-w-0">
                <div className="inline-flex items-center gap-2 text-accent-400 font-mono text-xs uppercase tracking-wider mb-3">
                  <Bot className="w-4 h-4 shrink-0" aria-hidden="true" />
                  Agent Trace &amp; MCP Server
                </div>
                <h3 className="text-xl font-bold text-white mb-2">
                  Built for Cursor, Claude Code &amp; Antigravity
                </h3>
                <p className="text-sm text-surface-300 leading-relaxed mb-4">
                  Snapshots AI agent mental models, prompts, and dirty files into{" "}
                  <code className="text-accent-300 font-mono text-xs">.nextdeploy/trace/</code>.
                  Inspect diffs on <code className="text-accent-300 font-mono text-xs">localhost:4044</code>,
                  run <code className="text-accent-300 font-mono text-xs">nextdeploy trace takeover -o</code>,
                  or expose 7 JSON-RPC 2.0 tools over stdio.
                </p>
                <div className="p-3.5 rounded-xl bg-surface-950 border border-surface-800 font-mono text-xs text-surface-200 space-y-1.5 mb-5 overflow-x-auto">
                  <div className="whitespace-nowrap">
                    <span className="text-brand-400">$</span> nextdeploy mcp init --client all
                  </div>
                  <div className="whitespace-nowrap">
                    <span className="text-brand-400">$</span> nextdeploy trace ui{" "}
                    <span className="text-surface-500"># http://localhost:4044</span>
                  </div>
                  <div className="whitespace-nowrap">
                    <span className="text-brand-400">$</span> nextdeploy trace takeover -o
                  </div>
                </div>
              </div>
              <Link
                href="/docs/agent-trace/architecture"
                className="inline-flex items-center gap-1.5 text-xs font-medium text-brand-400 hover:text-brand-300"
              >
                Explore Agent Trace &amp; MCP docs
                <ArrowRight className="w-3.5 h-3.5" aria-hidden="true" />
              </Link>
            </div>

            {/* Go Sidecar + shared_static + Rollback (7 cols) */}
            <div className="lg:col-span-7 min-w-0 p-4 sm:p-6 bg-surface-900/40 border border-surface-800 rounded-2xl flex flex-col justify-between">
              <div className="min-w-0">
                <div className="inline-flex flex-wrap items-center gap-2 text-code-400 font-mono text-xs uppercase tracking-wider mb-3">
                  <Cpu className="w-4 h-4 shrink-0" aria-hidden="true" />
                  <span>Optional Go Sidecar · shared_static · Instant Rollback</span>
                </div>
                <h3 className="text-xl font-bold text-white mb-2">
                  Sub-millisecond HTML/RSC caching &amp; zero ChunkLoadError 404s
                </h3>
                <p className="text-sm text-surface-300 leading-relaxed mb-4">
                  Enable <code className="text-code-300 font-mono text-xs">gosidecar</code> on VPS to
                  serve prerendered HTML/RSC payloads in memory and transcode{" "}
                  <code className="text-code-300 font-mono text-xs">/_next/image</code> in Go with
                  automatic Caddy <code className="text-code-300 font-mono text-xs">lb_policy first</code>{" "}
                  failover to Node.js. Combined with cumulative{" "}
                  <code className="text-code-300 font-mono text-xs">shared_static</code> pooling and
                  5-release ring buffers, rollbacks via{" "}
                  <code className="text-code-300 font-mono text-xs">nextdeploy rollback --to &lt;sha&gt;</code>{" "}
                  reuse the same zero-downtime health probe.
                </p>
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-2.5 sm:gap-3 mb-5">
                  <div className="min-w-0 p-3 rounded-lg bg-surface-950/70 border border-surface-800">
                    <div className="text-xs font-mono text-accent-300 font-semibold">Go Sidecar</div>
                    <div className="text-[11px] text-surface-400 mt-1">
                      In-memory radix cache + WebP/AVIF worker pool
                    </div>
                  </div>
                  <div className="min-w-0 p-3 rounded-lg bg-surface-950/70 border border-surface-800">
                    <div className="text-xs font-mono text-brand-300 font-semibold">shared_static</div>
                    <div className="text-[11px] text-surface-400 mt-1">
                      Cumulative chunk pool + mark-and-sweep across 5 releases
                    </div>
                  </div>
                  <div className="min-w-0 p-3 rounded-lg bg-surface-950/70 border border-surface-800">
                    <div className="text-xs font-mono text-warning-300 font-semibold">Instant Rollback</div>
                    <div className="text-[11px] text-surface-400 mt-1">
                      {`{unix-ts}-{shortSha}`} directories + S3 manifest parity
                    </div>
                  </div>
                </div>
              </div>
              <div className="flex flex-wrap gap-x-5 gap-y-2.5">
                <Link
                  href="/docs/vps/chunk-pooling"
                  className="inline-flex items-center gap-1.5 text-xs font-medium text-brand-400 hover:text-brand-300"
                >
                  Static Chunk Pooling docs
                  <ArrowRight className="w-3.5 h-3.5" aria-hidden="true" />
                </Link>
                <Link
                  href="/docs/vps/rollback"
                  className="inline-flex items-center gap-1.5 text-xs font-medium text-brand-400 hover:text-brand-300"
                >
                  Instant Rollback docs
                  <ArrowRight className="w-3.5 h-3.5" aria-hidden="true" />
                </Link>
              </div>
            </div>

            {/* Zero-Touch CI/CD & Previews (12 cols) */}
            <div className="lg:col-span-12 min-w-0 p-4 sm:p-6 bg-surface-900/40 border border-surface-800 rounded-2xl flex flex-col lg:flex-row lg:items-center justify-between gap-5 sm:gap-6">
              <div className="max-w-2xl min-w-0">
                <div className="inline-flex items-center gap-2 text-info-400 font-mono text-xs uppercase tracking-wider mb-2">
                  <GitBranch className="w-4 h-4 shrink-0" aria-hidden="true" />
                  Zero-Touch CI/CD &amp; Ephemeral PR Previews
                </div>
                <h3 className="text-xl font-bold text-white mb-2">
                  Target-aware GitHub Actions &amp; GitLab CI with PR drift gating
                </h3>
                <p className="text-sm text-surface-300 leading-relaxed">
                  Run <code className="text-info-300 font-mono text-xs">nextdeploy generate-ci --preview</code>{" "}
                  to emit checksum-verified workflows that detect your target from{" "}
                  <code className="text-info-300 font-mono text-xs">nextdeploy.yml</code> at runtime,
                  gate PRs with <code className="text-info-300 font-mono text-xs">nextdeploy plan --detailed-exitcode</code>{" "}
                  (exit code 2 on drift), and spin up isolated <code className="text-info-300 font-mono text-xs">pr-&lt;N&gt;</code>{" "}
                  Cloudflare preview Workers that auto-destroy on PR close.
                </p>
              </div>
              <div className="min-w-0 flex flex-col sm:flex-row lg:flex-col gap-3 shrink-0">
                <code className="block px-3.5 py-2.5 rounded-lg bg-surface-950 border border-surface-800 font-mono text-xs text-surface-200 overflow-x-auto max-w-full whitespace-nowrap">
                  nextdeploy generate-ci --preview
                </code>
                <Link
                  href="/docs/operations/ci-cd"
                  className="inline-flex items-center gap-1.5 text-xs font-medium text-brand-400 hover:text-brand-300"
                >
                  CI/CD Pipelines docs
                  <ArrowRight className="w-3.5 h-3.5" aria-hidden="true" />
                </Link>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ─── 3. ENGINEERING COMPARISON MATRIX ─── */}
      <section
        aria-labelledby="compare-heading"
        className="py-12 sm:py-20 lg:py-24 border-b border-surface-800/80"
      >
        <div className="max-w-7xl mx-auto px-4 sm:px-6">
          <div className="max-w-3xl mb-8 sm:mb-10">
            <p className="text-xs font-mono font-semibold tracking-wider uppercase text-brand-400 mb-2">
              Tradeoffs in the Open
            </p>
            <h2
              id="compare-heading"
              className="text-2xl sm:text-4xl font-bold text-white tracking-tight"
            >
              NextDeploy vs. Managed PaaS vs. DIY Scripts
            </h2>
          </div>

          {/* Mobile stacked comparison cards */}
          <div className="md:hidden space-y-3">
            {COMPARISON_ROWS.map((row) => (
              <div
                key={row.feature}
                className="rounded-xl border border-surface-800 bg-surface-900/40 p-4"
              >
                <h3 className="font-semibold text-white text-sm mb-3">
                  {row.feature}
                </h3>
                <div className="rounded-lg bg-brand-500/10 border border-brand-500/25 p-3 mb-3">
                  <div className="text-[10px] font-mono uppercase tracking-wider text-brand-400 mb-1">
                    NextDeploy
                  </div>
                  <div className="flex items-start gap-2 text-xs text-brand-100 leading-relaxed">
                    <Check className="w-4 h-4 text-brand-400 shrink-0 mt-0.5" aria-hidden="true" />
                    <span>{row.nextdeploy}</span>
                  </div>
                </div>
                <dl className="space-y-2 text-xs">
                  <div className="flex flex-col gap-0.5">
                    <dt className="font-mono text-[10px] uppercase tracking-wider text-surface-500">
                      Managed PaaS
                    </dt>
                    <dd className="text-surface-300">{row.paas}</dd>
                  </div>
                  <div className="flex flex-col gap-0.5 pt-2 border-t border-surface-800/70">
                    <dt className="font-mono text-[10px] uppercase tracking-wider text-surface-500">
                      DIY Scripts / Docker
                    </dt>
                    <dd className="text-surface-400">{row.diy}</dd>
                  </div>
                </dl>
              </div>
            ))}
          </div>

          {/* Desktop comparison table */}
          <div className="hidden md:block overflow-x-auto rounded-2xl border border-surface-800 bg-surface-900/30">
            <table className="w-full text-left border-collapse min-w-[680px]">
              <thead>
                <tr className="border-b border-surface-800 bg-surface-900/80 text-xs font-mono uppercase tracking-wider">
                  <th className="py-3.5 px-4 text-surface-400">Capability</th>
                  <th className="py-3.5 px-4 text-brand-400">NextDeploy</th>
                  <th className="py-3.5 px-4 text-surface-300">Managed PaaS</th>
                  <th className="py-3.5 px-4 text-surface-400">DIY Scripts / Docker</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-surface-800/80 text-xs sm:text-sm">
                {COMPARISON_ROWS.map((row) => (
                  <tr key={row.feature} className="hover:bg-surface-900/40 transition-colors">
                    <td className="py-3.5 px-4 font-medium text-white">{row.feature}</td>
                    <td className="py-3.5 px-4 text-brand-200">
                      <span className="inline-flex items-start gap-2">
                        <Check className="w-4 h-4 text-brand-400 shrink-0 mt-0.5" aria-hidden="true" />
                        <span>{row.nextdeploy}</span>
                      </span>
                    </td>
                    <td className="py-3.5 px-4 text-surface-300">{row.paas}</td>
                    <td className="py-3.5 px-4 text-surface-400">
                      <span className="inline-flex items-start gap-2">
                        <Minus className="w-4 h-4 text-surface-500 shrink-0 mt-0.5" aria-hidden="true" />
                        <span>{row.diy}</span>
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </section>

      {/* ─── 4. VIDEO WALKTHROUGH + ARCHITECTURE GUIDES ─── */}
      <section
        aria-labelledby="walkthrough-heading"
        className="py-12 sm:py-20 lg:py-24 border-b border-surface-800/80"
      >
        <div className="max-w-7xl mx-auto px-4 sm:px-6">
          <div className="grid grid-cols-1 lg:grid-cols-12 gap-8 items-start">
            <div className="lg:col-span-6 min-w-0">
              <p className="text-xs font-mono font-semibold tracking-wider uppercase text-brand-400 mb-2">
                See It in Action
              </p>
              <h2
                id="walkthrough-heading"
                className="text-2xl sm:text-3xl font-bold text-white tracking-tight mb-3"
              >
                Zero-Downtime VPS Deployment Walkthrough
              </h2>
              <p className="text-sm text-surface-400 mb-5 leading-relaxed">
                Watch how <code className="text-brand-300 font-mono text-xs">nextdeploy init</code>,{" "}
                <code className="text-brand-300 font-mono text-xs">prepare</code>, and{" "}
                <code className="text-brand-300 font-mono text-xs">ship</code> take a Next.js
                application from local code to a hardened Linux VPS with automatic TLS.
              </p>
              <div className="aspect-video w-full rounded-xl overflow-hidden border border-surface-800 bg-surface-900/50">
                <iframe
                  width="100%"
                  height="100%"
                  loading="lazy"
                  src="https://www.youtube.com/embed/9C8WNZSS5o0"
                  title="How to deploy Next.js to a Linux VPS with NextDeploy"
                  allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
                  allowFullScreen
                  className="w-full h-full border-none"
                />
              </div>
            </div>

            <div className="lg:col-span-6 min-w-0">
              <div className="flex items-center justify-between gap-3 mb-4">
                <div>
                  <p className="text-xs font-mono font-semibold tracking-wider uppercase text-brand-400 mb-1">
                    Documentation
                  </p>
                  <h3 className="text-2xl font-bold text-white tracking-tight">
                    Deep-dive architecture guides
                  </h3>
                </div>
                <Layers className="w-5 h-5 text-brand-400 shrink-0" aria-hidden="true" />
              </div>

              <div className="grid grid-cols-1 gap-2 min-w-0">
                {ARCHITECTURE_GUIDES.map((guide) => (
                  <Link
                    key={guide.href}
                    href={guide.href}
                    className="group flex items-center justify-between gap-3 px-3.5 sm:px-4 py-3 sm:py-2.5 rounded-xl bg-surface-900/40 border border-surface-800 hover:border-brand-500/40 hover:bg-surface-900/80 transition-colors min-w-0"
                  >
                    <div className="flex items-center gap-3 min-w-0">
                      <span className="font-mono text-xs text-brand-400 shrink-0">
                        {guide.n}
                      </span>
                      <span className="text-sm text-surface-200 group-hover:text-white truncate">
                        {guide.title}
                      </span>
                    </div>
                    <ArrowRight
                      className="w-4 h-4 text-surface-500 group-hover:text-brand-400 group-hover:translate-x-0.5 transition-all shrink-0"
                      aria-hidden="true"
                    />
                  </Link>
                ))}
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* ─── 5. FINAL INSTALL & CTA ─── */}
      <section aria-labelledby="cta-heading" className="py-12 sm:py-20 lg:py-24">
        <div className="max-w-4xl mx-auto px-4 sm:px-6 text-center">
          <h2
            id="cta-heading"
            className="text-2xl sm:text-4xl font-bold text-white tracking-tight mb-4"
          >
            Take control of your Next.js deployments.
          </h2>
          <p className="text-surface-400 text-sm sm:text-base mb-8 max-w-xl mx-auto">
            Install the static binary and ship to your own Linux VPS, AWS, or Cloudflare Workers in
            minutes.
          </p>

          <div className="mb-10">
            <InstallTabs />
          </div>

          <div className="flex flex-col sm:flex-row items-center justify-center gap-3 mb-8">
            <Link
              href="/docs"
              className="px-6 py-3 w-full sm:w-auto rounded-lg bg-brand-500 hover:bg-brand-400 text-surface-950 font-semibold text-sm transition-colors inline-flex items-center justify-center gap-2"
            >
              Read the Documentation
              <ArrowRight className="w-4 h-4" aria-hidden="true" />
            </Link>
            <a
              href="https://github.com/aynaash/NextDeploy"
              target="_blank"
              rel="noopener noreferrer"
              className="px-6 py-3 w-full sm:w-auto rounded-lg bg-surface-900 hover:bg-surface-800 text-surface-200 font-medium text-sm border border-surface-800 hover:border-surface-700 transition-colors inline-flex items-center justify-center gap-2"
            >
              <Github className="w-4 h-4" aria-hidden="true" />
              <Star className="w-3.5 h-3.5 text-highlight-400" aria-hidden="true" />
              Star NextDeploy on GitHub
            </a>
          </div>

          <p className="text-xs font-mono text-surface-500 tracking-wide">
            Open source · MIT Licensed · Self-hosted · Multi-target deployment
          </p>
        </div>
      </section>
    </div>
  );
}
