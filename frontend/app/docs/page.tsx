import { CodeBlock } from "@/components/copy-button";
import DocPrevNext from "@/components/doc-prev-next";
import GitHubStarButton from "@/components/github-star-button";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import {
  AlertTriangle,
  Check,
  Cloud,
  FileText,
  Lock,
  Play,
  Server,
  Share2,
  Shield,
  Terminal,
} from "lucide-react";
import type { Metadata } from "next";

function VideoPlaceholder({
  title,
  badge = "Tutorial Video Placement",
}: {
  title: string;
  badge?: string;
}) {
  return (
    <div className="w-full bg-surface-900/40 border border-surface-800 rounded-xl p-8 flex flex-col items-center justify-center text-center mb-8">
      <div className="w-12 h-12 rounded-full bg-surface-800 border border-surface-700 flex items-center justify-center mb-3">
        <Play className="w-5 h-5 text-surface-300 ml-0.5" />
      </div>
      <p className="text-white font-medium text-sm">
        {title}
      </p>
      <div className="text-surface-500 text-xs mt-1 font-mono">{badge}</div>
    </div>
  );
}

function YouTubeEmbed({ id, title }: { id: string; title: string }) {
  return (
    <div className="aspect-video w-full mb-8 rounded-xl overflow-hidden border border-surface-800 bg-surface-900/50">
      <iframe
        width="100%"
        height="100%"
        src={`https://www.youtube.com/embed/${id}`}
        title={title}
        allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
        allowFullScreen
        className="w-full h-full"
      ></iframe>
    </div>
  );
}

export const metadata: Metadata = {
  title: "Documentation Overview",
  description:
    "Comprehensive guide to self-hosting Next.js applications on Linux VPS, Cloudflare Workers, or AWS Lambda with NextDeploy.",
  alternates: {
    canonical: "/docs",
  },
  openGraph: {
    title: "NextDeploy Documentation Overview",
    description:
      "Comprehensive guide to self-hosting Next.js applications on Linux VPS, Cloudflare Workers, or AWS Lambda with NextDeploy.",
    url: "/docs",
    type: "website",
    siteName: "NextDeploy",
    locale: "en_US",
    images: [
      {
        url: "/og.png",
        width: 1200,
        height: 630,
        alt: "NextDeploy Documentation Overview",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "NextDeploy Documentation Overview",
    description:
      "Comprehensive guide to self-hosting Next.js applications on Linux VPS, Cloudflare Workers, or AWS Lambda with NextDeploy.",
    images: ["/og.png"],
  },
};

export default function DocsIndex() {
  return (
    <div className="py-6 sm:py-12 prose prose-invert max-w-none">
      {/* ─── HEADER ─── */}
      <div className="flex flex-wrap items-center justify-between gap-4 mb-6">
        <h1 className="text-2xl sm:text-3xl md:text-4xl font-bold text-white m-0">
          NextDeploy Documentation
        </h1>
        <GitHubStarButton />
      </div>

      <p className="text-muted-300 text-base sm:text-lg mb-4">
        NextDeploy is a self-hosted deployment engine for Next.js. It builds a
        complete picture of your app at build time — routes, integrations,
        secrets, and assets — and uses that intelligence to deploy cleanly to your
        own VPS, AWS Lambda, or Cloudflare Workers without a platform in the
        middle.
      </p>
      <p className="text-muted-400 text-sm mb-8">
        No monthly platform bill. No vendor lock-in. Your infrastructure, your
        cloud accounts, your server.
      </p>

      <div className="bg-surface-900/60 border border-surface-800 rounded-lg p-4 mb-8 flex items-start gap-3">
        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-mono font-medium bg-brand-500/10 text-brand-400 border border-brand-500/20 shrink-0 mt-0.5">
          v1.0.0
        </span>
        <div className="text-sm text-surface-300 leading-relaxed">
          Production-ready for Next.js 14, 15, and 16 (Turbopack + Webpack) across Linux VPS, AWS Lambda, and Cloudflare Workers. Run{" "}
          <code className="text-surface-200 bg-surface-800/80 px-1.5 py-0.5 rounded font-mono text-xs">nextdeploy update</code> to get the
          latest binary.
        </div>
      </div>

      <hr className="border-surface-800 my-8 sm:my-10" />

      {/* ─── HOW IT ACTUALLY WORKS ─── */}
      <h2 className="text-2xl sm:text-3xl font-bold text-white mb-4">
        How it actually works
      </h2>
      <p className="text-muted-300 mb-6">
        Most deployment tools are transport layers they move your build from A

        to B. NextDeploy is different. Every{" "}
        <code className="text-brand-400 bg-surface-900 px-1.5 py-0.5 rounded">
          nextdeploy build
        </code>{" "}
        produces a{" "}
        <code className="text-brand-400 bg-surface-900 px-1.5 py-0.5 rounded">
          metadata.json
        </code>{" "}
        that statically deep  analyses your Next.js output before a single file is
        uploaded:
      </p>

      <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-4 mb-8">
        {[
          {
            title: "Route classification",
            desc: "Separates static, SSR, ISR, API, and middleware routes by parsing Next.js manifests in single metadata.json file . No config needed.",
            accent: "text-brand-400",
            border: "hover:border-brand-500/30",
          },
          {
            title: "Integration detection",
            desc: "Detects Stripe, Cloudinary, Google Analytics, YouTube, and other external origins from the build output  not source code.",
            accent: "text-info-400",
            border: "hover:border-info-500/30",
          },
          {
            title: "ISR awareness",
            desc: "Identifies ISR routes and revalidation intervals. Foundation for selective CDN invalidation instead of nuking the whole cache.",
            accent: "text-code-400",
            border: "hover:border-code-500/30",
          },
          {
            title: "Secret cross-referencing",
            desc: "Knows which integrations your app uses. Can warn before deploy if Stripe is detected but STRIPE_KEY is missing.",
            accent: "text-warning-400",
            border: "hover:border-warning-500/30",
          },
          {
            title: "Git provenance",
            desc: "Records commit hash and dirty state at build time. Every deploy is traceable in build lock file. Warns if you're shipping uncommitted changes.",
            accent: "text-danger-400",
            border: "hover:border-danger-500/30",
          },
          {
            title: "Security Sandboxing",
            desc: "Configures DynamicUser, ProtectSystem=strict, and PrivateTmp for process isolation. Minimal attack surface.",
            accent: "text-warning-500",
            border: "hover:border-warning-500/30",
          },
          {
            title: "WAF & HMAC",
            desc: "Integrated Coraza WAF (OWASP CRS) and HMAC-SHA256 request signing between CLI and daemon for vps deployment.",
            accent: "text-code-400",
            border: "hover:border-code-500/30",
          },
          {
            title: "Agent Trace & Continuity",
            desc: "Tracks the mental model, prompts, and diffs of AI coding agents (Antigravity, Claude Code, Cursor) with local UI (:4044) and 1-command developer takeover.",
            accent: "text-brand-400",
            border: "hover:border-brand-500/30",
          },
          {
            title: "Build fingerprint",
            desc: "Knows your package manager, entrypoint, output mode, Next.js version, and full asset inventory. Deploys exactly what your app needs.",
            accent: "text-muted-300",
            border: "hover:border-surface-600",
          },
        ].map(({ title, desc, accent, border }) => (
          <div
            key={title}
            className={`p-5 bg-surface-900/30 border border-surface-800 rounded-xl ${border} transition-colors`}
          >
            <h4 className={`${accent} font-semibold text-sm mb-2`}>{title}</h4>
            <p className="text-muted-400 text-sm leading-relaxed m-0">{desc}</p>
          </div>
        ))}
      </div>

      <div className="bg-surface-900/50 border border-surface-700 rounded-xl p-5 mb-8">
        <p className="text-muted-300 text-sm leading-relaxed m-0">
          This intelligence layer is what separates NextDeploy from a glorified
          deploy script. The metadata drives pre-deploy validation, targeted
          cache invalidation, deployment provenance, and  on the roadmap
          per-route CloudFront behavior configuration that eliminates
          unnecessary Lambda invocations entirely.
        </p>
      </div>

      <hr className="border-surface-800 my-8 sm:my-10" />

      {/* ─── WHAT IT HANDLES TODAY ─── */}
      <h2 className="text-2xl sm:text-3xl font-bold text-white mb-6">
        What it handles today
      </h2>
      <div className="grid md:grid-cols-2 gap-4 mb-8 text-muted-300">
        <ul className="space-y-3 list-none p-0 m-0">
          <li className="flex gap-3 items-start">
            <Check className="w-4 h-4 text-brand-400 shrink-0 mt-1" />
            <span>
              <strong>VPS deployments</strong> — atomic port-swap,
              zero-downtime, Caddy reverse proxy, Fail2Ban, HSTS
            </span>
          </li>
          <li className="flex gap-3 items-start">
            <Check className="w-4 h-4 text-brand-400 shrink-0 mt-1" />
            <span>
              <strong>Advanced isolation</strong> — DynamicUser,
              ProtectSystem=strict, PrivateTmp, and NoNewPrivileges
            </span>
          </li>
          <li className="flex gap-3 items-start">
            <Check className="w-4 h-4 text-brand-400 shrink-0 mt-1" />
            <span>
              <strong>AWS serverless</strong> — Lambda + S3 + CloudFront + ACM,
              fully provisioned on first deploy
            </span>
          </li>
          <li className="flex gap-3 items-start">
            <Check className="w-4 h-4 text-brand-400 shrink-0 mt-1" />
            <span>
              <strong>Build intelligence</strong> — static analysis of every
              Next.js build, route classification, integration detection
            </span>
          </li>
          <li className="flex gap-3 items-start">
            <Check className="w-4 h-4 text-brand-400 shrink-0 mt-1" />
            <span>
              <strong>Lambda bridge adapter</strong> — translates Lambda events
              to HTTP for the Next.js standalone server
            </span>
          </li>
        </ul>
        <ul className="space-y-3 list-none p-0 m-0">
          <li className="flex gap-3 items-start">
            <Check className="w-4 h-4 text-brand-400 shrink-0 mt-1" />
            <span>
              <strong>Secret management</strong> — encrypted locally, pushed to
              AWS Secrets Manager, injected via Lambda Extension at cold-start
            </span>
          </li>
          <li className="flex gap-3 items-start">
            <Check className="w-4 h-4 text-brand-400 shrink-0 mt-1" />
            <span>
              <strong>Audit logging</strong> — structured JSON logs of every
              command and system change for traceability
            </span>
          </li>
          <li className="flex gap-3 items-start">
            <Check className="w-4 h-4 text-brand-400 shrink-0 mt-1" />
            <span>
              <strong>Rollbacks</strong> — VPS ring buffer keeps last 5
              releases, serverless restores from S3 deployment history
            </span>
          </li>
          <li className="flex gap-3 items-start">
            <Check className="w-4 h-4 text-brand-400 shrink-0 mt-1" />
            <span>
              <strong>Deployment provenance</strong> — git commit, dirty state,
              Next.js version, and build timestamp on every deploy
            </span>
          </li>
          <li className="flex gap-3 items-start">
            <Check className="w-4 h-4 text-brand-400 shrink-0 mt-1" />
            <span>
              <strong>Self-updating CLI</strong> — single static binary, no
              runtime dependencies, updates via GitHub Releases
            </span>
          </li>
        </ul>
      </div>

      <div className="bg-surface-900/40 border border-surface-800 rounded-xl p-6 mb-8">
        <h4 className="text-white font-semibold mb-3 flex items-center gap-2 text-base">
          <AlertTriangle className="w-4 h-4 text-warning-400 shrink-0" /> Target
          considerations & constraints
        </h4>
        <ul className="space-y-2 text-sm text-surface-400 list-none p-0 m-0">
          <li className="flex items-start gap-2">
            <span className="text-surface-500">•</span>
            <span>
              <strong className="text-surface-200 font-medium">AWS Lambda package ceiling:</strong>{" "}
              250MB unzipped limit on AWS. Run <code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">nextdeploy inspect</code> to audit dependency weights before shipping.
            </span>
          </li>
          <li className="flex items-start gap-2">
            <span className="text-surface-500">•</span>
            <span>
              <strong className="text-surface-200 font-medium">AWS single-region compute:</strong>{" "}
              Lambda deploys to a single region (<code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">us-east-1</code> for CloudFront + ACM). For globally distributed zero-cold-start edge compute, deploy to the Cloudflare Workers target.
            </span>
          </li>
          <li className="flex items-start gap-2">
            <span className="text-surface-500">•</span>
            <span>
              <strong className="text-surface-200 font-medium">Selective CDN invalidation:</strong>{" "}
              Full invalidation and incremental cache updates are supported today. Fine-grained per-route CDN invalidation driven by ISR metadata diffs is in active development on the roadmap.
            </span>
          </li>
          <li className="flex items-start gap-2">
            <span className="text-surface-500">•</span>
            <span>
              <strong className="text-surface-200 font-medium">Supported vs planned targets:</strong>{" "}
              Linux VPS (Caddy + Go sidecar), Cloudflare Workers (NextServer engine), and AWS (Lambda + S3 + CloudFront) are live in production. GCP Cloud Run and Azure targets are planned.
            </span>
          </li>
        </ul>
      </div>

      <hr className="border-surface-800 my-8 sm:my-10" />

      {/* ─── REQUIREMENTS ─── */}
      <h2 className="text-2xl sm:text-3xl font-bold text-white mb-4">
        Requirements Before You Deploy
      </h2>
      <p className="text-muted-300 mb-4">
        NextDeploy requires a Next.js 14/15/16 app (with{" "}
        <code className="text-brand-400 bg-surface-900 px-1.5 py-0.5 rounded">
          output: &apos;standalone&apos;
        </code>{" "}
        recommended) and target-specific prerequisites. For full-stack apps on a Linux VPS, you need:
      </p>
      <div className="bg-surface-900/40 border border-surface-800 rounded-xl p-6 mb-6">
        <ul className="space-y-2.5 text-sm text-surface-300 list-none p-0 m-0">
          <li className="flex items-start gap-2.5">
            <Check className="w-4 h-4 text-brand-400 shrink-0 mt-0.5" />
            <span>
              <strong className="text-white">Linux VPS:</strong> Ubuntu or Debian server with public IPv4 and ports <code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">22</code>, <code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">80</code>, and <code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">443</code> open.
            </span>
          </li>
          <li className="flex items-start gap-2.5">
            <Check className="w-4 h-4 text-brand-400 shrink-0 mt-0.5" />
            <span>
              <strong className="text-white">Non-root <code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">sudo</code> user:</strong> A non-root SSH user (e.g. <code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">deploy</code> or <code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">ubuntu</code>). <code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">nextdeploy prepare</code> refuses <code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">root</code> by default.
            </span>
          </li>
          <li className="flex items-start gap-2.5">
            <Check className="w-4 h-4 text-brand-400 shrink-0 mt-0.5" />
            <span>
              <strong className="text-white">SSH private key path:</strong> Local SSH key (<code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">key_path: ~/.ssh/id_ed25519</code>) configured in <code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">nextdeploy.yml</code> for CLI SSH/SFTP access.
            </span>
          </li>
          <li className="flex items-start gap-2.5">
            <Check className="w-4 h-4 text-brand-400 shrink-0 mt-0.5" />
            <span>
              <strong className="text-white">Domain DNS:</strong> An <code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">A</code> record pointing your domain to the VPS IP so Caddy can issue Let&apos;s Encrypt TLS certificates.
            </span>
          </li>
          <li className="flex items-start gap-2.5">
            <Check className="w-4 h-4 text-brand-400 shrink-0 mt-0.5" />
            <span>
              <strong className="text-white">Secrets sync (Doppler recommended, or <code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">.env</code>):</strong> Sync runtime variables via <code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">doppler run -- nextdeploy secrets sync</code> or <code className="text-xs bg-surface-800 px-1 py-0.5 rounded text-surface-200">nextdeploy secrets load .env.production</code>.
            </span>
          </li>
        </ul>
        <p className="text-xs text-surface-400 mt-4 mb-0">
          Using AWS or Cloudflare Workers? See the full{" "}
          <a href="/docs/getting-started/requirements" className="text-brand-400 hover:text-brand-300">
            Requirements Guide →
          </a>
        </p>
      </div>

      <hr className="border-surface-800 my-8 sm:my-10" />

      {/* ─── INSTALLATION ─── */}
      <h2 className="text-2xl sm:text-3xl font-bold text-white mb-6">Installation</h2>

      <h3 className="text-lg sm:text-xl font-semibold text-white mt-8 mb-3">
        CLI (your machine)
      </h3>
      <p className="text-muted-300 mb-4">
        A single static binary. No Node, no runtime dependencies.
      </p>
      <Tabs defaultValue="linux" className="w-full mb-8">
        <TabsList className="bg-surface-900 border border-surface-700">
          <TabsTrigger value="linux">Linux / macOS</TabsTrigger>
          <TabsTrigger value="windows">Windows</TabsTrigger>
          <TabsTrigger value="go">Go (Source)</TabsTrigger>
        </TabsList>
        <TabsContent value="linux" className="mt-2">
          <CodeBlock code="curl -sSf https://nextdeploy.org/install.sh | sh" />
        </TabsContent>
        <TabsContent value="windows" className="mt-2">
          <CodeBlock
            code={`curl.exe -sSfO https://nextdeploy.org/install.bat && install.bat`}
          />
        </TabsContent>
        <TabsContent value="go" className="mt-2">
          <CodeBlock code="go install github.com/aynaash/NextDeploy/cli@latest" />
        </TabsContent>
      </Tabs>

      <h3 className="text-lg sm:text-xl font-semibold text-white mt-8 mb-3">
        Daemon (VPS only)
      </h3>
      <p className="text-muted-300 mb-4">
        Runs as a systemd service on your server. Handles health checks, atomic
        swaps, release management, and secret injection via a Unix socket
        control plane. <em>Not needed for serverless.</em>
      </p>
      <Tabs defaultValue="script" className="w-full mb-8">
        <TabsList className="bg-surface-900 border border-surface-700">
          <TabsTrigger value="script">Install Script</TabsTrigger>
          <TabsTrigger value="go">Go (Source)</TabsTrigger>
        </TabsList>
        <TabsContent value="script" className="mt-2">
          <CodeBlock code="curl -sSf https://nextdeploy.org/daemon.sh | sh" />
        </TabsContent>
        <TabsContent value="go" className="mt-2">
          <CodeBlock code="go install github.com/aynaash/NextDeploy/daemon/cmd/nextdeployd@main" />
        </TabsContent>
      </Tabs>

      <hr className="border-surface-800 my-8 sm:my-10" />

      {/* ─── VPS DEPLOYMENT ─── */}
      <h2 className="text-2xl sm:text-3xl font-bold text-white mb-2">VPS Deployment</h2>
      <p className="text-muted-400 mb-6 text-sm">
        Deploy to any Linux server you own or rent.
      </p>

      <h3 className="text-lg sm:text-xl font-semibold text-white mt-8 mb-3">
        1. Initialize
      </h3>
      <CodeBlock code="nextdeploy init" className="mb-4" />
      <p className="text-muted-300 mb-6">
        Creates{" "}
        <code className="bg-surface-900 px-2 py-1 rounded text-brand-400 font-mono">
          nextdeploy.yml
        </code>{" "}
        in your project root with sensible defaults.
      </p>

      <h3 className="text-lg sm:text-xl font-semibold text-white mt-8 mb-3">
        2. Configure
      </h3>
      <CodeBlock
        code={`version: "1.0"
target_type: vps

app:
  name: my-next-app
  domain: app.example.com
  port: 3000
  environment: production

servers:
  - name: prod
    host: 1.2.3.4
    port: 22
    username: deploy
    key_path: ~/.ssh/id_ed25519`}
        className="mb-8"
      />
      <p className="text-muted-400 text-xs mb-8 -mt-4">
        Use a non-root user with <code className="text-brand-400">sudo</code> privileges (for example <code className="text-brand-400">deploy</code> or <code className="text-brand-400">ubuntu</code>) and point <code className="text-brand-400">key_path</code> to your local SSH private key. Sync full-stack runtime secrets via <code className="text-brand-400">doppler run -- nextdeploy secrets sync</code> (recommended) or <code className="text-brand-400">nextdeploy secrets load .env.production</code>.
      </p>

      <YouTubeEmbed
        id="9C8WNZSS5o0"
        title="NextDeploy VPS Tutorial: Zero-Downtime Deployment"
      />

      <h3 className="text-lg sm:text-xl font-semibold text-white mt-8 mb-3">
        3. Prepare the server (once)
      </h3>
      <p className="text-muted-300 mb-4">
        Runs an Ansible playbook that installs Node, Bun, Caddy, Fail2Ban, and
        the NextDeploy daemon. Configures systemd with advanced isolation (
        <code className="text-brand-400">DynamicUser=yes</code>,{" "}
        <code className="text-brand-300">ProtectSystem=strict</code>), Unix
        socket permissions, and Caddy HTTPS automatically.
      </p>
      <CodeBlock code="nextdeploy prepare" className="mb-8" />

      <h3 className="text-lg sm:text-xl font-semibold text-white mt-8 mb-3">
        4. Deploy
      </h3>
      <CodeBlock code="nextdeploy ship" className="mb-4" />
      <p className="text-muted-300 mb-4">
        <code className="text-brand-400 bg-surface-900 px-1.5 py-0.5 rounded">
          ship
        </code>{" "}
        runs the full pipeline in one step — analyses your Next.js output,
        produces the metadata and artifact, uploads via SFTP, and signals the
        daemon for an atomic port-swap that's health checked before traffic cuts
        over. Incremental: skips the rebuild step when nothing has changed.
      </p>
      <p className="text-muted-400 text-sm mb-8">
        For CI pipelines that want a separate build stage, run{" "}
        <code className="text-brand-400 bg-surface-900 px-1.5 py-0.5 rounded">
          nextdeploy build
        </code>{" "}
        first — it produces the same artifact{" "}
        <code className="text-brand-400 bg-surface-900 px-1.5 py-0.5 rounded">
          ship
        </code>{" "}
        would have built. Add{" "}
        <code className="text-brand-400 bg-surface-900 px-1.5 py-0.5 rounded">
          --force
        </code>{" "}
        to either command to bypass the incremental cache.
      </p>

      <h3 className="text-lg sm:text-xl font-semibold text-white mt-8 mb-3">Rollback</h3>
      <p className="text-muted-300 mb-4">
        The daemon keeps your last 5 releases. Revert instantly:
      </p>
      <CodeBlock code="nextdeploy rollback" className="mb-8" />

      <hr className="border-surface-800 my-8 sm:my-10" />

      {/* ─── SERVERLESS DEPLOYMENT ─── */}
      <h2 className="text-2xl sm:text-3xl font-bold text-white mb-2">
        Serverless Deployment (AWS)
      </h2>
      <p className="text-muted-400 mb-6 text-sm">
        No servers, no daemon  Lambda + S3 + CloudFront, provisioned
        automatically.
      </p>

      <div className="bg-info-500/10 border border-info-500/20 rounded-lg p-4 mb-8 flex gap-3">
        <div className="text-info-400 mt-0.5">ℹ️</div>
        <div>
          <h4 className="text-info-400 font-semibold mb-1">
            What gets provisioned on first deploy
          </h4>
          <ul className="text-info-200/80 text-sm space-y-1 list-none p-0 m-0 mt-2">
            <li>
              • S3 bucket with correct MIME types and cache headers per asset
              type
            </li>
            <li>• Lambda function + IAM execution role</li>
            <li>
              • Lambda Function URL with CloudFront OAC — Lambda is never
              publicly exposed
            </li>
            <li>• ACM certificate for your custom domain</li>
            <li>• CloudFront distribution wired to both S3 and Lambda</li>
            <li>• AWS Secrets Manager namespace for your app</li>
          </ul>
          <p className="text-info-200/70 text-xs mt-3">
            <em>
              Prerequisite: AWS credentials in <code>~/.aws/credentials</code>{" "}
              with the permissions listed in the AWS Deployer Permissions
              section.
            </em>
          </p>
        </div>
      </div>

      <h3 className="text-lg sm:text-xl font-semibold text-white mt-8 mb-3">
        1. Configure
      </h3>
      <CodeBlock
        code={`version: "1.0"
target_type: serverless

app:
  name: my-next-app
  domain: app.example.com

serverless:
  provider: aws
  region: us-east-1      # ACM certificates must be us-east-1 for CloudFront`}
        className="mb-8"
      />

      <VideoPlaceholder
        title="Tutorial: Automated AWS Serverless Deployment"
        badge="Coming Soon"
      />

      <h3 className="text-lg sm:text-xl font-semibold text-white mt-8 mb-3">
        2. Add <code className="text-brand-400">output: 'standalone'</code>
      </h3>
      <CodeBlock
        code={`// next.config.mjs
const nextConfig = {
  output: 'standalone',
}
export default nextConfig`}
        className="mb-4"
      />
      <div className="bg-surface-900/50 border border-surface-700 rounded-xl p-5 mb-8">
        <h4 className="text-white font-semibold mb-2">Why standalone?</h4>
        <p className="text-muted-400 text-sm leading-relaxed m-0">
          Default Next.js builds include the full <code>node_modules</code> —
          often 500MB+, over Lambda's 250MB unzipped limit. Standalone mode
          traces your actual imports and produces a self-contained{" "}
          <code>server.js</code>. NextDeploy then injects a bridge adapter that
          translates Lambda invocation events into HTTP requests the Next.js
          server understands.
        </p>
      </div>

      <h3 className="text-lg sm:text-xl font-semibold text-white mt-8 mb-3">
        3. Deploy
      </h3>
      <CodeBlock code="nextdeploy ship" className="mb-4" />
      <p className="text-muted-300 mb-6">
        One command runs the full pipeline.{" "}
        <code className="bg-surface-900 px-2 py-1 rounded text-brand-400 font-mono">
          ship
        </code>{" "}
        builds your app (only when something changed), then:
      </p>

      <div className="grid md:grid-cols-2 gap-4 mb-8">
        {[
          {
            step: "1",
            title: "Validate build",
            desc: "Reads metadata.json — route types, integrations, ISR config, git state. Validates before uploading anything.",
          },
          {
            step: "2",
            title: "Push secrets",
            desc: "Syncs secrets to AWS Secrets Manager. They never touch the artifact or Lambda env vars directly.",
          },
          {
            step: "3",
            title: "Upload static assets",
            desc: "Syncs public/ and _next/static/ to S3 with correct MIME types and cache headers per asset type.",
          },
          {
            step: "4",
            title: "Deploy Lambda",
            desc: "Injects bridge adapter, zips .next/standalone/, updates Lambda via UpdateFunctionCode with retry logic for IAM propagation.",
          },
          {
            step: "5",
            title: "SSL + DNS guide",
            desc: "ACM cert requested. Detailed DNS guide generated in report. Re-run ship after DNS propagates.",
          },
          {
            step: "6",
            title: "Bust CDN cache",
            desc: "CloudFront /* invalidation triggered. Selective per-route invalidation using ISR metadata is on the roadmap.",
          },
        ].map(({ step, title, desc }) => (
          <div
            key={step}
            className="flex gap-4 p-4 bg-surface-900/30 border border-surface-800 rounded-xl"
          >
            <span className="shrink-0 w-8 h-8 rounded-full bg-brand-500/20 border border-brand-500/40 flex items-center justify-center text-brand-400 font-bold text-sm">
              {step}
            </span>
            <div>
              <h4 className="text-white font-semibold text-sm mb-1">{title}</h4>
              <p className="text-muted-400 text-sm leading-relaxed">{desc}</p>
            </div>
          </div>
        ))}
      </div>

      <hr className="border-surface-800 my-8 sm:my-10" />

      {/* ─── SECRETS ─── */}
      <h2 className="text-2xl sm:text-3xl font-bold text-white mb-2">Secret Management</h2>
      <p className="text-muted-400 mb-2 text-sm">
        Encrypted locally. Never in build artifacts. Never hardcoded in Lambda
        env vars.
      </p>
      <p className="text-muted-500 mb-6 text-xs italic">
        Commands from the CLI are now cryptographically signed using HMAC-SHA256
        to ensure daemon control plane integrity.
      </p>

      <CodeBlock
        code={`nextdeploy secrets set DATABASE_URL "postgres://user:pass@host/db"
nextdeploy secrets load
nextdeploy secrets list
nextdeploy secrets unset OLD_KEY`}
        className="mb-8"
      />

      <div className="bg-surface-900/50 border border-surface-700 rounded-xl p-6 mb-8">
        <h4 className="text-white font-semibold mb-3 flex items-center gap-2">
          <Lock className="w-4 h-4 text-code-400" /> How secrets are stored
        </h4>
        <ul className="space-y-3 text-sm text-muted-400 list-none p-0 m-0">
          <li className="flex gap-4 items-start">
            <span className="text-muted-500 shrink-0 w-16">VPS</span>
            <span>
              Encrypted on-disk at{" "}
              <code className="text-brand-400">/opt/nextdeploy/secrets/</code>{" "}
              with 0600 permissions. Decrypted and injected as environment
              variables at startup. Key is derived per-machine.
            </span>
          </li>
          <li className="flex gap-4 items-start">
            <span className="text-muted-500 shrink-0 w-16">AWS</span>
            <span>
              Pushed to AWS Secrets Manager under{" "}
              <code className="text-brand-400">
                nextdeploy/apps/&lt;name&gt;/production
              </code>
              . Lambda fetches them at cold-start via the AWS Secrets Extension
              — available as plain{" "}
              <code className="text-brand-400">process.env</code>. Only the
              ARN is set in Lambda, not the values.
            </span>
          </li>
        </ul>
      </div>

      <hr className="border-surface-800 my-8 sm:my-10" />

      {/* ─── DNS CONFIGURATION ─── */}
      <h2 className="text-2xl sm:text-3xl font-bold text-white mb-2">DNS Configuration</h2>
      <p className="text-muted-400 mb-6 text-sm">
        How to point your domain to your new infrastructure correctly.
      </p>

      <div className="bg-brand-500/10 border border-brand-500/20 rounded-lg p-5 mb-8">
        <h4 className="text-brand-400 font-semibold mb-2 flex items-center gap-2">
          <Share2 className="w-4 h-4" /> The "Loud" DNS Guide
        </h4>
        <p className="text-brand-200/70 text-sm leading-relaxed m-0">
          NextDeploy generates a{" "}
          <code className="text-brand-300 bg-surface-900 px-1.5 py-0.5 rounded">
            dns.md
          </code>{" "}
          file in your project root and a visual{" "}
          <strong>DNS Setup Guide</strong> in your deployment report. It
          explicitly tells you which records to add to your registrar.
        </p>
      </div>

      <div className="grid md:grid-cols-2 gap-8 mb-8">
        <div>
          <h3 className="text-lg sm:text-xl font-semibold text-white mb-3 flex items-center gap-2">
            <Server className="w-5 h-5 text-info-400" /> VPS Target
          </h3>
          <p className="text-muted-400 text-sm mb-4">
            Point your domain directly to your server IP.
          </p>
          <ul className="space-y-2 text-sm text-muted-300 list-none p-0">
            <li>
              • <strong className="text-white">A Record:</strong>{" "}
              <code className="text-info-400">@</code> &rarr;{" "}
              <code className="text-info-400">Server IP</code>
            </li>
            <li>
              • <strong className="text-white">CNAME:</strong>{" "}
              <code className="text-info-400">www</code> &rarr;{" "}
              <code className="text-info-400">@</code>
            </li>
          </ul>
          <p className="text-muted-500 text-xs mt-4 italic">
            Caddy handles SSL automatically via Let's Encrypt once DNS
            propagates.
          </p>
        </div>

        <div>
          <h3 className="text-lg sm:text-xl font-semibold text-white mb-3 flex items-center gap-2">
            <Cloud className="w-5 h-5 text-code-400" /> Serverless Target
          </h3>
          <p className="text-muted-400 text-sm mb-4">
            Requires two steps: verify ownership and route traffic.
          </p>
          <ul className="space-y-4 text-sm text-muted-300 list-none p-0">
            <li>
              <strong className="text-white block mb-1">
                1. SSL Validation (ACM)
              </strong>
              <span className="text-xs text-muted-500">
                Add the unique CNAME records provided in your report to prove
                ownership to AWS.
              </span>
            </li>
            <li>
              <strong className="text-white block mb-1">
                2. Traffic Routing
              </strong>
              <span className="text-xs text-muted-500">
                Point your domain to the CloudFront distribution:
              </span>
              <div className="mt-1">
                <code className="text-code-400">@</code> &rarr;{" "}
                <code className="text-code-400">xxxx.cloudfront.net</code>
              </div>
            </li>
          </ul>
        </div>
      </div>

      <hr className="border-surface-800 my-8 sm:my-10" />

      {/* ─── SECURITY & ISOLATION ─── */}
      <h2 className="text-2xl sm:text-3xl font-bold text-white mb-2">
        Security & Isolation
      </h2>
      <p className="text-muted-400 mb-6 text-sm">
        Hardened by default for both VPS and Serverless deployments.
      </p>

      <div className="grid md:grid-cols-2 gap-6 mb-8">
        {[
          {
            icon: <Lock className="w-5 h-5 text-code-400" />,
            title: "HMAC-SHA256 Command Signing",
            desc: "All CLI-to-Daemon commands require a cryptographic signature. A shared secret is generated during 'nextdeploy prepare', ensuring only authorized clients can control your apps.",
          },
          {
            icon: <Shield className="w-5 h-5 text-brand-400" />,
            title: "Coraza WAF Integration",
            desc: "Caddy includes an embedded Web Application Firewall using the OWASP Core Rule Set. It blocks SQLi, XSS, and common exploit attempts at the edge.",
          },
          {
            icon: <Server className="w-5 h-5 text-info-400" />,
            title: "Advanced Systemd Sandboxing",
            desc: "Apps run with DynamicUser=yes, ProtectSystem=strict, and PrivateTmp=yes. The process is sandboxed with minimal kernel attack surface and no filesystem access outside its data dir.",
          },
          {
            icon: <FileText className="w-5 h-5 text-warning-400" />,
            title: "Structured Audit Logging",
            desc: "Every deployment, secret change, and system command is recorded in JSON format at /var/log/nextdeployd/audit.log for full environment traceability.",
          },
          {
            icon: <Lock className="w-5 h-5 text-info-500" />,
            title: "Mutual TLS (mTLS)",
            desc: "Daemon communication supports an optional TCP+mTLS listener. For network-attached management, both client and server must present valid certificates.",
          },
          {
            icon: <Shield className="w-5 h-5 text-danger-500" />,
            title: "Atomic Permissions",
            desc: "Unix sockets are restricted to the 'nextdeploy' group with 0660 permissions, ensuring local privilege separation between users and the deployment plane.",
          },
        ].map(({ icon, title, desc }) => (
          <div
            key={title}
            className="p-5 bg-surface-900/30 border border-surface-800 rounded-xl hover:border-surface-700 transition-colors"
          >
            <div className="flex items-center gap-3 mb-3">
              {icon}
              <h4 className="text-white font-semibold m-0">{title}</h4>
            </div>
            <p className="text-muted-400 text-sm leading-relaxed m-0">{desc}</p>
          </div>
        ))}
      </div>

      <hr className="border-surface-800 my-8 sm:my-10" />

      {/* ─── NEXTCORE METADATA ─── */}
      <h2 className="text-2xl sm:text-3xl font-bold text-white mb-2">
        Deep Dive: NextCore Metadata
      </h2>
      <p className="text-muted-400 mb-6 text-sm">
        The "Brain" that makes NextDeploy provider-agnostic.
      </p>

      <p className="text-muted-300 mb-6">
        During <code className="text-brand-400">nextdeploy build</code>, the
        NextCore engine performs a deep inspection of your Next.js build output.
        It doesn't just copy files; it understands the <strong>intent</strong>{" "}
        of every route and asset.
      </p>

      <div className="bg-surface-900/50 border border-surface-700 rounded-xl p-6 mb-8">
        <h4 className="text-white font-semibold mb-4">
          Metadata Analysis Loop
        </h4>
        <div className="grid sm:grid-cols-2 gap-4 text-xs">
          <div className="p-3 bg-surface-900/80 border border-surface-800 rounded-lg">
            <div className="text-brand-400 font-bold mb-1">
              Manifest Scanning
            </div>
            <div className="text-muted-400">
              Parses .next/routes-manifest.json to map every route (Static,
              Dynamic, ISR, Middleware).
            </div>
          </div>
          <div className="p-3 bg-surface-900/80 border border-surface-800 rounded-lg">
            <div className="text-info-400 font-bold mb-1">
              Integration Detection
            </div>
            <div className="text-muted-400">
              Detects Stripe, Cloudinary, etc., by scanning chunks for external
              origin calls.
            </div>
          </div>
          <div className="p-3 bg-surface-900/80 border border-surface-800 rounded-lg">
            <div className="text-code-400 font-bold mb-1">
              Secret Cross-Ref
            </div>
            <div className="text-muted-400">
              Matches detected integrations against your stored secrets to warn
              of missing config.
            </div>
          </div>
          <div className="p-3 bg-surface-900/80 border border-surface-800 rounded-lg">
            <div className="text-warning-400 font-bold mb-1">Git Provenance</div>
            <div className="text-muted-400">
              Records commit hash, branch, and dirty state to ensure deployment
              traceability.
            </div>
          </div>
        </div>
      </div>

      <p className="text-muted-300 mb-6">
        The resulting{" "}
        <code className="text-brand-400 font-mono">metadata.json</code> serves
        as the source of truth for all providers:
      </p>

      <div className="grid md:grid-cols-2 gap-8 mb-8">
        <div className="p-5 bg-info-500/5 border border-info-500/10 rounded-xl">
          <h4 className="text-info-400 font-semibold mb-3 flex items-center gap-2">
            <Server className="w-4 h-4" /> VPS (nextdeployd)
          </h4>
          <ul className="space-y-2 text-sm text-muted-400 list-none p-0">
            <li>
              • Maps the entrypoint to a <strong>Systemd Sandbox</strong>
            </li>
            <li>
              • Allocates and persists <strong>Dynamic Ports</strong>
            </li>
            <li>
              • Translates Domain metadata to{" "}
              <strong>Caddyfile Directives</strong>
            </li>
          </ul>
        </div>
        <div className="p-5 bg-code-500/5 border border-code-500/10 rounded-xl">
          <h4 className="text-code-400 font-semibold mb-3 flex items-center gap-2">
            <Cloud className="w-4 h-4" /> AWS Serverless
          </h4>
          <ul className="space-y-2 text-sm text-muted-400 list-none p-0">
            <li>
              • Splits assets between <strong>S3 (Static)</strong> and{" "}
              <strong>Lambda (Dynamic)</strong>
            </li>
            <li>
              • Configures <strong>CloudFront Behaviors</strong> per route type
            </li>
            <li>
              • Maps secrets to <strong>AWS Secrets Manager</strong>
            </li>
          </ul>
        </div>
      </div>

      <hr className="border-surface-800 my-8 sm:my-10" />

      {/* ─── CLI REFERENCE ─── */}
      <h2 className="text-2xl sm:text-3xl font-bold text-white mb-6">CLI Reference</h2>
      <div className="space-y-3 mb-8">
        {[
          {
            cmd: "nextdeploy init",
            desc: "Scaffold nextdeploy.yml in your project",
          },
          {
            cmd: "nextdeploy ship",
            desc: "Build (incrementally) and deploy in one step — the typical command. Targets VPS or serverless based on nextdeploy.yml.",
          },
          {
            cmd: "nextdeploy ship --force",
            desc: "Force a full rebuild before deploying (skips the incremental cache).",
          },
          {
            cmd: "nextdeploy build",
            desc: "Run only the build stage — analyse Next.js output, classify routes, detect integrations, produce metadata.json and app.tar.gz. Useful for CI when you want build and deploy on separate runners.",
          },
          {
            cmd: "nextdeploy build --force",
            desc: "Force a clean rebuild from scratch.",
          },
          {
            cmd: "nextdeploy prepare",
            desc: "Provision a VPS — Caddy, daemon, Fail2Ban, Node, Bun, systemd (VPS only)",
          },
          {
            cmd: "nextdeploy rollback",
            desc: "Revert to the previous release (VPS ring buffer or S3 history)",
          },
          {
            cmd: "nextdeploy status",
            desc: "Show health, active release, route summary, and resource usage",
          },
          {
            cmd: "nextdeploy logs",
            desc: "Tail live application logs from the server",
          },
          {
            cmd: "nextdeploy secrets load",
            desc: "Load secrets from a .env file to your server securely",
          },
          {
            cmd: "nextdeploy secrets set KEY VALUE",
            desc: "Encrypt and store — routed to daemon store or AWS Secrets Manager automatically",
          },
          {
            cmd: "nextdeploy secrets list",
            desc: "List secret names (values always hidden)",
          },
          {
            cmd: "nextdeploy secrets unset KEY",
            desc: "Remove one or more secrets permanently",
          },
          {
            cmd: "nextdeploy update",
            desc: "Self-update the CLI binary from GitHub Releases",
          },
        ].map(({ cmd, desc }) => (
          <div
            key={cmd}
            className="flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-6 p-4 bg-surface-900/30 border border-surface-800 rounded-xl"
          >
            <code className="text-brand-400 font-mono text-xs sm:text-sm shrink-0 break-all sm:break-normal sm:whitespace-nowrap">
              {cmd}
            </code>
            <span className="text-muted-400 text-sm">{desc}</span>
          </div>
        ))}
      </div>

      <hr className="border-surface-800 my-8 sm:my-10" />

      {/* ─── CONFIG REFERENCE ─── */}
      <h2 className="text-2xl sm:text-3xl font-bold text-white mb-6">
        Configuration reference
      </h2>
      <Tabs defaultValue="vps" className="w-full mb-8">
        <TabsList className="bg-surface-900 border border-surface-700">
          <TabsTrigger value="vps">VPS</TabsTrigger>
          <TabsTrigger value="serverless">Serverless</TabsTrigger>
        </TabsList>
        <TabsContent value="vps" className="mt-2">
          <CodeBlock
            code={`version: "1.0"
target_type: vps

app:
  name: my-next-app         # Used for process naming and secret namespacing
  domain: app.example.com   # Caddy configures HTTPS automatically
  port: 3000
  environment: production

servers:
  - name: prod
    host: 1.2.3.4
    port: 22
    username: deploy
    ssh_key: ~/.ssh/id_rsa

secrets:
  provider: local           # "local" | "doppler" coming soon

logging:
  enabled: true`}
          />
        </TabsContent>
        <TabsContent value="serverless" className="mt-2">
          <CodeBlock
            code={`version: "1.0"
target_type: serverless

app:
  name: my-next-app
  domain: app.example.com

serverless:
  provider: aws             # "cloudflare" | "gcp" | "azure" coming soon
  region: us-east-1         # Must be us-east-1 for CloudFront + ACM
  # Optional — NextDeploy provisions these automatically on first ship:
  s3_bucket: my-nextjs-assets
  cloudfront_id: EXXXXXXXXXXXXXXXXX
  lambda_function_name: my-next-app-handler`}
          />
        </TabsContent>
      </Tabs>

      <hr className="border-surface-800 my-8 sm:my-10" />

      {/* ─── ARCHITECTURE ─── */}
      <h2 className="text-2xl sm:text-3xl font-bold text-white mb-6">Architecture</h2>

      <h3 className="text-lg sm:text-xl font-semibold text-white mt-8 mb-4">VPS</h3>
      <div className="my-6 p-3 sm:p-6 bg-surface-900/50 border border-surface-800 rounded-2xl flex flex-col items-center overflow-x-auto">
        <svg viewBox="0 0 860 390" className="w-full max-w-4xl min-w-[640px]">
          <defs>
            <marker
              id="vps-arrow-brand"
              markerWidth="8"
              markerHeight="6"
              refX="7"
              refY="3"
              orientation="auto"
            >
              <polygon points="0 0, 8 3, 0 6" fill="var(--color-brand-500)" />
            </marker>
            <marker
              id="vps-arrow-muted"
              markerWidth="8"
              markerHeight="6"
              refX="7"
              refY="3"
              orientation="auto"
            >
              <polygon points="0 0, 8 3, 0 6" fill="var(--color-surface-400)" />
            </marker>
            <marker
              id="vps-arrow-code"
              markerWidth="8"
              markerHeight="6"
              refX="7"
              refY="3"
              orientation="auto"
            >
              <polygon points="0 0, 8 3, 0 6" fill="var(--color-code-400)" />
            </marker>
            <marker
              id="vps-arrow-accent"
              markerWidth="8"
              markerHeight="6"
              refX="7"
              refY="3"
              orientation="auto"
            >
              <polygon points="0 0, 8 3, 0 6" fill="var(--color-accent-400)" />
            </marker>
          </defs>

          {/* Local / Client Boundary */}
          <rect
            x="18"
            y="24"
            width="184"
            height="342"
            rx="12"
            fill="none"
            stroke="var(--color-surface-700)"
            strokeWidth="1.5"
            strokeDasharray="4 4"
          />
          <text
            x="110"
            y="46"
            textAnchor="middle"
            fill="var(--color-surface-400)"
            fontSize="11"
            fontWeight="600"
          >
            Clients &amp; Local CLI
          </text>

          <rect
            x="34"
            y="70"
            width="152"
            height="62"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-info-500)"
            strokeWidth="2"
          />
          <text
            x="110"
            y="97"
            textAnchor="middle"
            fill="var(--color-surface-100)"
            fontSize="13"
            fontWeight="600"
          >
            Users / Browser
          </text>
          <text x="110" y="115" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            HTTPS :443
          </text>

          <rect
            x="34"
            y="278"
            width="152"
            height="62"
            rx="8"
            fill="var(--color-brand-900)"
            stroke="var(--color-brand-500)"
            strokeWidth="2"
          />
          <text
            x="110"
            y="305"
            textAnchor="middle"
            fill="var(--color-brand-50)"
            fontSize="13"
            fontWeight="600"
          >
            nextdeploy CLI
          </text>
          <text x="110" y="323" textAnchor="middle" fill="var(--color-brand-200)" fontSize="10">
            ship / rollback / prepare
          </text>

          {/* Production VPS Boundary */}
          <rect
            x="275"
            y="24"
            width="565"
            height="342"
            rx="12"
            fill="none"
            stroke="var(--color-surface-700)"
            strokeWidth="1.5"
            strokeDasharray="4 4"
          />
          <text
            x="557"
            y="46"
            textAnchor="middle"
            fill="var(--color-surface-400)"
            fontSize="11"
            fontWeight="600"
          >
            Production Linux VPS
          </text>

          <rect
            x="305"
            y="70"
            width="170"
            height="62"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-code-500)"
            strokeWidth="2"
          />
          <text
            x="390"
            y="97"
            textAnchor="middle"
            fill="var(--color-surface-100)"
            fontSize="13"
            fontWeight="600"
          >
            Caddy + Coraza WAF
          </text>
          <text x="390" y="115" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            Auto-TLS &amp; OWASP CRS
          </text>

          <rect
            x="305"
            y="278"
            width="170"
            height="62"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-info-500)"
            strokeWidth="2"
          />
          <text
            x="390"
            y="305"
            textAnchor="middle"
            fill="var(--color-surface-100)"
            fontSize="13"
            fontWeight="600"
          >
            nextdeployd Daemon
          </text>
          <text x="390" y="323" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            HMAC-SHA256 &amp; Audit Log
          </text>

          <rect
            x="590"
            y="56"
            width="220"
            height="52"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-brand-500)"
            strokeWidth="2"
          />
          <text
            x="700"
            y="79"
            textAnchor="middle"
            fill="var(--color-surface-100)"
            fontSize="12"
            fontWeight="600"
          >
            shared_static/ &amp; public/
          </text>
          <text x="700" y="96" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            Cumulative Chunks (NVMe sendfile)
          </text>

          <rect
            x="590"
            y="126"
            width="220"
            height="52"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-danger-500)"
            strokeWidth="2"
          />
          <text
            x="700"
            y="149"
            textAnchor="middle"
            fill="var(--color-surface-100)"
            fontSize="12"
            fontWeight="600"
          >
            Fail2Ban (caddy-waf)
          </text>
          <text x="700" y="166" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            Bans IPs from WAF audit.log
          </text>

          <rect
            x="590"
            y="196"
            width="220"
            height="56"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-accent-400)"
            strokeWidth="2"
            strokeDasharray="4 3"
          />
          <text
            x="700"
            y="220"
            textAnchor="middle"
            fill="var(--color-accent-300)"
            fontSize="12"
            fontWeight="600"
          >
            Go Sidecar (Optional)
          </text>
          <text x="700" y="237" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            HTML/RSC Cache &amp; /_next/image
          </text>

          <rect
            x="590"
            y="284"
            width="220"
            height="62"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-warning-500)"
            strokeWidth="2"
          />
          <text
            x="700"
            y="311"
            textAnchor="middle"
            fill="var(--color-surface-100)"
            fontSize="13"
            fontWeight="600"
          >
            Next.js Release
          </text>
          <text x="700" y="329" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            127.0.0.1:&lt;port&gt; (systemd)
          </text>

          {/* Edges */}
          <path
            d="M 186 101 L 303 101"
            fill="none"
            stroke="var(--color-code-400)"
            strokeWidth="2"
            markerEnd="url(#vps-arrow-code)"
          />
          <text x="238" y="91" textAnchor="middle" fill="var(--color-code-400)" fontSize="10" fontWeight="600">
            HTTPS :443
          </text>

          <path
            d="M 186 309 L 303 309"
            fill="none"
            stroke="var(--color-brand-500)"
            strokeWidth="2"
            markerEnd="url(#vps-arrow-brand)"
          />
          <text x="238" y="299" textAnchor="middle" fill="var(--color-brand-400)" fontSize="10" fontWeight="600">
            SSH / SFTP
          </text>

          <path
            d="M 390 278 L 390 134"
            fill="none"
            stroke="var(--color-surface-400)"
            strokeWidth="1.5"
            strokeDasharray="3 3"
            markerEnd="url(#vps-arrow-muted)"
          />
          <text x="398" y="208" textAnchor="start" fill="var(--color-surface-400)" fontSize="10">
            Validate &amp; Reload
          </text>

          <path
            d="M 475 82 L 588 82"
            fill="none"
            stroke="var(--color-brand-500)"
            strokeWidth="1.5"
            markerEnd="url(#vps-arrow-brand)"
          />
          <text x="532" y="72" textAnchor="middle" fill="var(--color-brand-400)" fontSize="10">
            /_next/static/*
          </text>

          <path
            d="M 475 100 L 542 100 L 542 152 L 588 152"
            fill="none"
            stroke="var(--color-surface-400)"
            strokeWidth="1.5"
            strokeDasharray="3 3"
            markerEnd="url(#vps-arrow-muted)"
          />
          <text x="564" y="143" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            audit.log
          </text>

          <path
            d="M 475 114 L 522 114 L 522 224 L 588 224"
            fill="none"
            stroke="var(--color-accent-400)"
            strokeWidth="1.5"
            markerEnd="url(#vps-arrow-accent)"
          />
          <text x="554" y="214" textAnchor="middle" fill="var(--color-accent-300)" fontSize="10">
            lb_policy first
          </text>

          <path
            d="M 700 252 L 700 282"
            fill="none"
            stroke="var(--color-accent-400)"
            strokeWidth="1.5"
            markerEnd="url(#vps-arrow-accent)"
          />
          <text x="710" y="271" textAnchor="start" fill="var(--color-accent-300)" fontSize="10">
            SSR / API
          </text>

          <path
            d="M 475 126 L 500 126 L 500 300 L 588 300"
            fill="none"
            stroke="var(--color-code-400)"
            strokeWidth="1.5"
            strokeDasharray="3 3"
            markerEnd="url(#vps-arrow-code)"
          />
          <text x="544" y="291" textAnchor="middle" fill="var(--color-code-400)" fontSize="10">
            Direct / Fallback
          </text>

          <path
            d="M 475 322 L 588 322"
            fill="none"
            stroke="var(--color-surface-400)"
            strokeWidth="1.5"
            strokeDasharray="3 3"
            markerEnd="url(#vps-arrow-muted)"
          />
          <text x="532" y="340" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            Probe + systemd
          </text>
        </svg>
      </div>

      <h3 className="text-lg sm:text-xl font-semibold text-white mt-8 mb-4">Serverless (AWS)</h3>
      <div className="my-6 p-3 sm:p-6 bg-surface-900/50 border border-surface-800 rounded-2xl flex flex-col items-center overflow-x-auto">
        <svg viewBox="0 0 860 320" className="w-full max-w-4xl min-w-[640px]">
          <defs>
            <marker
              id="aws-arrow-brand"
              markerWidth="8"
              markerHeight="6"
              refX="7"
              refY="3"
              orientation="auto"
            >
              <polygon points="0 0, 8 3, 0 6" fill="var(--color-brand-500)" />
            </marker>
            <marker
              id="aws-arrow-muted"
              markerWidth="8"
              markerHeight="6"
              refX="7"
              refY="3"
              orientation="auto"
            >
              <polygon points="0 0, 8 3, 0 6" fill="var(--color-surface-400)" />
            </marker>
            <marker
              id="aws-arrow-code"
              markerWidth="8"
              markerHeight="6"
              refX="7"
              refY="3"
              orientation="auto"
            >
              <polygon points="0 0, 8 3, 0 6" fill="var(--color-code-400)" />
            </marker>
          </defs>

          <rect
            x="18"
            y="24"
            width="184"
            height="272"
            rx="12"
            fill="none"
            stroke="var(--color-surface-700)"
            strokeWidth="1.5"
            strokeDasharray="4 4"
          />
          <text
            x="110"
            y="46"
            textAnchor="middle"
            fill="var(--color-surface-400)"
            fontSize="11"
            fontWeight="600"
          >
            Clients &amp; Local CLI
          </text>

          <rect
            x="34"
            y="64"
            width="152"
            height="60"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-info-500)"
            strokeWidth="2"
          />
          <text
            x="110"
            y="91"
            textAnchor="middle"
            fill="var(--color-surface-100)"
            fontSize="13"
            fontWeight="600"
          >
            Users / Browser
          </text>
          <text x="110" y="108" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            HTTPS Edge Request
          </text>

          <rect
            x="34"
            y="198"
            width="152"
            height="60"
            rx="8"
            fill="var(--color-brand-900)"
            stroke="var(--color-brand-500)"
            strokeWidth="2"
          />
          <text
            x="110"
            y="225"
            textAnchor="middle"
            fill="var(--color-brand-50)"
            fontSize="13"
            fontWeight="600"
          >
            nextdeploy CLI
          </text>
          <text x="110" y="242" textAnchor="middle" fill="var(--color-brand-200)" fontSize="10">
            AWS SDK v2 Deploy
          </text>

          <rect
            x="275"
            y="24"
            width="565"
            height="272"
            rx="12"
            fill="none"
            stroke="var(--color-surface-700)"
            strokeWidth="1.5"
            strokeDasharray="4 4"
          />
          <text
            x="557"
            y="46"
            textAnchor="middle"
            fill="var(--color-surface-400)"
            fontSize="11"
            fontWeight="600"
          >
            AWS Cloud
          </text>

          <rect
            x="305"
            y="64"
            width="175"
            height="60"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-code-500)"
            strokeWidth="2"
          />
          <text
            x="392"
            y="91"
            textAnchor="middle"
            fill="var(--color-surface-100)"
            fontSize="13"
            fontWeight="600"
          >
            CloudFront + ACM
          </text>
          <text x="392" y="108" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            CDN &amp; Edge TLS
          </text>

          <rect
            x="590"
            y="54"
            width="220"
            height="54"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-info-500)"
            strokeWidth="2"
          />
          <text
            x="700"
            y="78"
            textAnchor="middle"
            fill="var(--color-surface-100)"
            fontSize="13"
            fontWeight="600"
          >
            S3 Bucket (OAC)
          </text>
          <text x="700" y="95" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            Static Chunks &amp; Releases
          </text>

          <rect
            x="590"
            y="132"
            width="220"
            height="58"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-warning-500)"
            strokeWidth="2"
          />
          <text
            x="700"
            y="157"
            textAnchor="middle"
            fill="var(--color-surface-100)"
            fontSize="13"
            fontWeight="600"
          >
            Lambda + Bridge
          </text>
          <text x="700" y="174" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            SSR / API / Image Opt
          </text>

          <rect
            x="590"
            y="218"
            width="220"
            height="54"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-danger-500)"
            strokeWidth="2"
          />
          <text
            x="700"
            y="242"
            textAnchor="middle"
            fill="var(--color-surface-100)"
            fontSize="12"
            fontWeight="600"
          >
            AWS Secrets Manager
          </text>
          <text x="700" y="259" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            Encrypted App Secrets
          </text>

          {/* Edges */}
          <path
            d="M 186 94 L 303 94"
            fill="none"
            stroke="var(--color-code-400)"
            strokeWidth="2"
            markerEnd="url(#aws-arrow-code)"
          />
          <text x="238" y="84" textAnchor="middle" fill="var(--color-code-400)" fontSize="10" fontWeight="600">
            HTTPS
          </text>

          <path
            d="M 480 81 L 588 81"
            fill="none"
            stroke="var(--color-code-400)"
            strokeWidth="1.5"
            markerEnd="url(#aws-arrow-code)"
          />
          <text x="534" y="72" textAnchor="middle" fill="var(--color-code-400)" fontSize="10">
            /_next/static/*
          </text>

          <path
            d="M 480 108 L 536 108 L 536 152 L 588 152"
            fill="none"
            stroke="var(--color-code-400)"
            strokeWidth="1.5"
            markerEnd="url(#aws-arrow-code)"
          />
          <text x="562" y="143" textAnchor="middle" fill="var(--color-code-400)" fontSize="10">
            SSR / API
          </text>

          <path
            d="M 700 190 L 700 216"
            fill="none"
            stroke="var(--color-surface-400)"
            strokeWidth="1.5"
            strokeDasharray="3 3"
            markerEnd="url(#aws-arrow-muted)"
          />
          <text x="710" y="207" textAnchor="start" fill="var(--color-surface-400)" fontSize="10">
            Extension :2773
          </text>

          <path
            d="M 186 228 L 520 228 L 520 172 L 588 172"
            fill="none"
            stroke="var(--color-brand-500)"
            strokeWidth="2"
            markerEnd="url(#aws-arrow-brand)"
          />
          <path
            d="M 520 228 L 520 245 L 588 245"
            fill="none"
            stroke="var(--color-brand-500)"
            strokeWidth="2"
            markerEnd="url(#aws-arrow-brand)"
          />
          <text x="360" y="218" textAnchor="middle" fill="var(--color-brand-400)" fontSize="10" fontWeight="600">
            Push Secrets, S3 Assets &amp; Lambda Zip
          </text>
        </svg>
      </div>

      <h3 className="text-lg sm:text-xl font-semibold text-white mt-8 mb-4">
        Serverless (Cloudflare Workers)
      </h3>
      <div className="my-6 p-3 sm:p-6 bg-surface-900/50 border border-surface-800 rounded-2xl flex flex-col items-center overflow-x-auto">
        <svg viewBox="0 0 860 300" className="w-full max-w-4xl min-w-[640px]">
          <defs>
            <marker
              id="cf-arrow-brand"
              markerWidth="8"
              markerHeight="6"
              refX="7"
              refY="3"
              orientation="auto"
            >
              <polygon points="0 0, 8 3, 0 6" fill="var(--color-brand-500)" />
            </marker>
            <marker
              id="cf-arrow-code"
              markerWidth="8"
              markerHeight="6"
              refX="7"
              refY="3"
              orientation="auto"
            >
              <polygon points="0 0, 8 3, 0 6" fill="var(--color-code-400)" />
            </marker>
          </defs>

          <rect
            x="18"
            y="24"
            width="184"
            height="250"
            rx="12"
            fill="none"
            stroke="var(--color-surface-700)"
            strokeWidth="1.5"
            strokeDasharray="4 4"
          />
          <text
            x="110"
            y="46"
            textAnchor="middle"
            fill="var(--color-surface-400)"
            fontSize="11"
            fontWeight="600"
          >
            Clients &amp; Local CLI
          </text>

          <rect
            x="34"
            y="64"
            width="152"
            height="60"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-info-500)"
            strokeWidth="2"
          />
          <text
            x="110"
            y="91"
            textAnchor="middle"
            fill="var(--color-surface-100)"
            fontSize="13"
            fontWeight="600"
          >
            Users / Browser
          </text>
          <text x="110" y="108" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            HTTPS Edge Request
          </text>

          <rect
            x="34"
            y="184"
            width="152"
            height="60"
            rx="8"
            fill="var(--color-brand-900)"
            stroke="var(--color-brand-500)"
            strokeWidth="2"
          />
          <text
            x="110"
            y="211"
            textAnchor="middle"
            fill="var(--color-brand-50)"
            fontSize="13"
            fontWeight="600"
          >
            nextdeploy CLI
          </text>
          <text x="110" y="228" textAnchor="middle" fill="var(--color-brand-200)" fontSize="10">
            apply &amp; ship (nextcompile)
          </text>

          <rect
            x="275"
            y="24"
            width="565"
            height="250"
            rx="12"
            fill="none"
            stroke="var(--color-surface-700)"
            strokeWidth="1.5"
            strokeDasharray="4 4"
          />
          <text
            x="557"
            y="46"
            textAnchor="middle"
            fill="var(--color-surface-400)"
            fontSize="11"
            fontWeight="600"
          >
            Cloudflare Global Edge
          </text>

          <rect
            x="305"
            y="108"
            width="185"
            height="64"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-warning-500)"
            strokeWidth="2"
          />
          <text
            x="397"
            y="136"
            textAnchor="middle"
            fill="var(--color-surface-100)"
            fontSize="13"
            fontWeight="600"
          >
            Worker (NextServer)
          </text>
          <text x="397" y="154" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            workerd + nodejs_compat
          </text>

          <rect
            x="590"
            y="64"
            width="220"
            height="58"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-info-500)"
            strokeWidth="2"
          />
          <text
            x="700"
            y="89"
            textAnchor="middle"
            fill="var(--color-surface-100)"
            fontSize="13"
            fontWeight="600"
          >
            R2 Bucket (ASSETS)
          </text>
          <text x="700" y="106" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            Static Assets &amp; ISR Cache
          </text>

          <rect
            x="590"
            y="180"
            width="220"
            height="58"
            rx="8"
            fill="var(--color-surface-800)"
            stroke="var(--color-code-500)"
            strokeWidth="2"
          />
          <text
            x="700"
            y="205"
            textAnchor="middle"
            fill="var(--color-surface-100)"
            fontSize="13"
            fontWeight="600"
          >
            D1 / KV / Hyperdrive
          </text>
          <text x="700" y="222" textAnchor="middle" fill="var(--color-surface-400)" fontSize="10">
            Provisioned Edge Bindings
          </text>

          {/* Edges */}
          <path
            d="M 186 94 L 245 94 L 245 128 L 303 128"
            fill="none"
            stroke="var(--color-code-400)"
            strokeWidth="2"
            markerEnd="url(#cf-arrow-code)"
          />
          <text x="218" y="84" textAnchor="middle" fill="var(--color-code-400)" fontSize="10" fontWeight="600">
            HTTPS
          </text>

          <path
            d="M 186 214 L 245 214 L 245 154 L 303 154"
            fill="none"
            stroke="var(--color-brand-500)"
            strokeWidth="2"
            markerEnd="url(#cf-arrow-brand)"
          />
          <text x="224" y="232" textAnchor="middle" fill="var(--color-brand-400)" fontSize="10" fontWeight="600">
            Workers API
          </text>

          <path
            d="M 490 124 L 540 124 L 540 93 L 588 93"
            fill="none"
            stroke="var(--color-code-400)"
            strokeWidth="1.5"
            markerEnd="url(#cf-arrow-code)"
          />
          <text x="540" y="82" textAnchor="middle" fill="var(--color-code-400)" fontSize="10">
            /_next/static/*
          </text>

          <path
            d="M 490 156 L 540 156 L 540 209 L 588 209"
            fill="none"
            stroke="var(--color-code-400)"
            strokeWidth="1.5"
            markerEnd="url(#cf-arrow-code)"
          />
          <text x="540" y="226" textAnchor="middle" fill="var(--color-code-400)" fontSize="10">
            Edge Bindings
          </text>
        </svg>
      </div>

      <hr className="border-surface-800 my-8 sm:my-10" />

      {/* ─── UNDER THE HOOD ─── */}
      <h2 className="text-2xl sm:text-3xl font-bold text-white mb-6">Under the hood</h2>
      <div className="grid md:grid-cols-3 gap-6 my-8">
        <div className="p-6 bg-surface-900/40 border border-surface-800 rounded-2xl">
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2 m-0 font-sans">
            <Terminal className="w-5 h-5 text-brand-400" /> CLI
          </h3>
          <ul className="space-y-2 text-sm text-muted-400 list-none p-0">
            <li>• Next.js manifest parsing + route classification</li>
            <li>• Static integration detection from build output</li>
            <li>• HMAC-SHA256 request signing & verification</li>
            <li>• Build metadata generation per deploy</li>
            <li>• Encrypted local secret storage</li>
            <li>• Cloudflare API & R2, AWS SDK, or SFTP (VPS)</li>
            <li>• Self-update via GitHub Releases</li>
          </ul>
        </div>

        <div className="p-6 bg-surface-900/40 border border-surface-800 rounded-2xl">
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2 m-0 font-sans">
            <Server className="w-5 h-5 text-info-400" /> Daemon (VPS)
          </h3>
          <ul className="space-y-2 text-sm text-muted-400 list-none p-0">
            <li>• Systemd with socket & process isolation</li>
            <li>• DynamicUser & ProtectSystem sandboxing</li>
            <li>• Unix socket & TCP/mTLS control plane</li>
            <li>• Health check + port arbitration & persistence</li>
            <li>• Atomic hot-swap with instant rollback</li>
            <li>• Release buffer (last 5 builds)</li>
            <li>• Secret hot-reload without restart</li>
          </ul>
        </div>

        <div className="p-6 bg-surface-900/40 border border-surface-800 rounded-2xl">
          <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2 m-0 font-sans">
            <Cloud className="w-5 h-5 text-code-400" /> Serverless engine
          </h3>
          <ul className="space-y-2 text-sm text-muted-400 list-none p-0">
            <li>• Cloudflare Workers + native Turbopack + React 19</li>
            <li>• Streaming HTTP bridge + VFS runtime shims</li>
            <li>• AWS Lambda bridge adapter (event → HTTP)</li>
            <li>• CloudFront OAC + Lambda URL auth</li>
            <li>• R2 / S3 deployment history for rollback</li>
            <li>• Lambda version publishing & Worker instant cutover</li>
            <li>• Idempotent cloud resource provisioning</li>
          </ul>
        </div>
      </div>

      <hr className="border-surface-800 my-8 sm:my-10" />

      {/* ─── ROADMAP ─── */}
      <h2 className="text-2xl sm:text-3xl font-bold text-white mb-2">Roadmap</h2>
      <p className="text-muted-400 text-sm mb-6">
        Current shipping state and ongoing architectural milestones across targets:
      </p>

      <div className="space-y-2.5 mb-8">
        {[
          {
            label: "Cloudflare Workers — Next.js 16 Turbopack, R2, KV, Hyperdrive, streaming HTTP bridge",
            status: "v1.0 (Live)",
            done: true,
          },
          {
            label: "AWS — Lambda + S3 + CloudFront + ACM + Secrets Manager + ISR",
            status: "v1.0 (Live)",
            done: true,
          },
          {
            label:
              "Build intelligence — route classification, integration detection, ISR awareness, git provenance",
            status: "v1.0 (Live)",
            done: true,
          },
          {
            label:
              "VPS — Go sidecar, atomic cutover, chunk pooling, Caddy auto-TLS, systemd hardening",
            status: "v1.0 (Live)",
            done: true,
          },
          {
            label:
              "Pre-deploy validation — secret cross-referencing, git dirty warnings, Lambda & Worker size checks",
            status: "v1.0 (Live)",
            done: true,
          },
          {
            label: "Secret management — Doppler, AWS Secrets Manager, Cloudflare Secrets, Local encrypted store",
            status: "v1.0 (Live)",
            done: true,
          },
          {
            label:
              "Agent Trace — provider-independent continuity for AI coding agents, port 4044 dashboard, and 1-command takeover",
            status: "v1.0 (Live)",
            done: true,
          },
          {
            label:
              "Selective CDN invalidation — invalidate only changed routes using ISR metadata diff",
            status: "Planned",
            done: false,
          },
          {
            label:
              "Deployment dashboard — route map, build trends, history, provenance per deploy",
            status: "Planned",
            done: false,
          },
          {
            label: "GCP Cloud Run + GCS provider",
            status: "Planned",
            done: false,
          },
        ].map(({ label, status, done }) => (
          <div
            key={label}
            className="flex justify-between items-center p-3.5 rounded-lg border bg-surface-900/40 border-surface-800/90 hover:border-surface-700/80 transition-colors"
          >
            <span
              className={`text-sm ${done ? "text-surface-200" : "text-surface-400"}`}
            >
              {label}
            </span>
            <span
              className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded text-xs font-mono font-medium shrink-0 ml-4 ${done
                ? "bg-brand-500/10 text-brand-400 border border-brand-500/20"
                : "bg-surface-800/80 text-surface-400 border border-surface-700/80"
                }`}
            >
              <span
                className={`w-1.5 h-1.5 rounded-full ${done ? "bg-brand-400" : "bg-surface-500"
                  }`}
              />
              {status}
            </span>
          </div>
        ))}
      </div>

      <DocPrevNext />
    </div>
  );
}
