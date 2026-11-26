import Link from "next/link";
import { Github, Twitter } from "lucide-react";
import Logo from "./logo";

export default function Footer() {
  return (
    <footer className="py-12 sm:py-16 bg-surface-950 border-t border-surface-800">
      <div className="max-w-6xl mx-auto px-4 sm:px-6 grid grid-cols-1 sm:grid-cols-2 md:grid-cols-4 gap-8 md:gap-10">
        <div className="sm:col-span-2 md:col-span-1">
          <Logo variant="horizontal" size="sm" className="mb-3" />
          <p className="text-muted-500 text-sm leading-relaxed">
            Open-source, self-hosted Next.js deployment engine. Deploy to Linux
            VPS, AWS, or Cloudflare Workers without platform lock-in.
          </p>
        </div>

        <div>
          <h3 className="text-xs font-mono font-semibold uppercase tracking-wider text-surface-300 mb-3">
            Targets
          </h3>
          <ul className="space-y-2 text-sm">
            <li>
              <Link
                href="/docs/getting-started/vps-quickstart"
                className="text-muted-400 hover:text-white transition-colors"
              >
                Linux VPS
              </Link>
            </li>
            <li>
              <Link
                href="/docs/getting-started/aws-quickstart"
                className="text-muted-400 hover:text-white transition-colors"
              >
                AWS Serverless
              </Link>
            </li>
            <li>
              <Link
                href="/docs/getting-started/cloudflare-quickstart"
                className="text-muted-400 hover:text-white transition-colors"
              >
                Cloudflare Workers
              </Link>
            </li>
          </ul>
        </div>

        <div>
          <h3 className="text-xs font-mono font-semibold uppercase tracking-wider text-surface-300 mb-3">
            Documentation
          </h3>
          <ul className="space-y-2 text-sm">
            <li>
              <Link
                href="/docs/getting-started/installation"
                className="text-muted-400 hover:text-white transition-colors"
              >
                Getting Started
              </Link>
            </li>
            <li>
              <Link
                href="/docs/concepts/metadata"
                className="text-muted-400 hover:text-white transition-colors"
              >
                Build Metadata
              </Link>
            </li>
            <li>
              <Link
                href="/docs/concepts/security"
                className="text-muted-400 hover:text-white transition-colors"
              >
                Security Model
              </Link>
            </li>
            <li>
              <Link
                href="/docs/cli/reference"
                className="text-muted-400 hover:text-white transition-colors"
              >
                CLI Reference
              </Link>
            </li>
            <li>
              <Link
                href="/routes"
                className="text-muted-400 hover:text-white transition-colors"
              >
                Next.js Routes Lab
              </Link>
            </li>
          </ul>
        </div>

        <div>
          <h3 className="text-xs font-mono font-semibold uppercase tracking-wider text-surface-300 mb-3">
            Community
          </h3>
          <ul className="space-y-2 text-sm">
            <li>
              <a
                href="https://github.com/aynaash/NextDeploy"
                target="_blank"
                rel="noopener noreferrer"
                className="text-muted-400 hover:text-white transition-colors inline-flex items-center gap-2"
              >
                <Github className="w-4 h-4" aria-hidden="true" />
                GitHub
              </a>
            </li>
            <li>
              <a
                href="https://discord.gg/xd9Cub9fm"
                target="_blank"
                rel="noopener noreferrer"
                className="text-muted-400 hover:text-white transition-colors"
              >
                Discord Community
              </a>
            </li>
            <li>
              <a
                href="https://twitter.com/hersiyussuf"
                target="_blank"
                rel="noopener noreferrer"
                className="text-muted-400 hover:text-white transition-colors inline-flex items-center gap-2"
              >
                <Twitter className="w-4 h-4" aria-hidden="true" />
                X / Twitter
              </a>
            </li>
          </ul>
        </div>
      </div>

      <div className="max-w-6xl mx-auto px-4 sm:px-6 mt-10 border-t border-surface-800/60 pt-6 flex flex-col sm:flex-row items-center justify-between gap-4 text-center sm:text-left">
        <p className="text-muted-600 text-xs">
          © 2026 NextDeploy · Open-source under MIT License
        </p>
        <div className="flex items-center gap-4">
          <a
            href="https://github.com/aynaash/NextDeploy"
            target="_blank"
            rel="noopener noreferrer"
            aria-label="NextDeploy on GitHub"
            className="text-muted-500 hover:text-white transition-colors"
          >
            <Github className="w-4 h-4" aria-hidden="true" />
          </a>
          <a
            href="https://twitter.com/hersiyussuf"
            target="_blank"
            rel="noopener noreferrer"
            aria-label="NextDeploy creator on X / Twitter"
            className="text-muted-500 hover:text-white transition-colors"
          >
            <Twitter className="w-4 h-4" aria-hidden="true" />
          </a>
        </div>
      </div>
    </footer>
  );
}
