import Link from "next/link";
import type { ReactNode } from "react";
import { ArrowLeft } from "lucide-react";
import Footer from "@/components/footer";
import type { RouteMode } from "@/lib/routes-catalog";

const MODE_COLORS: Record<RouteMode, string> = {
  SSG: "bg-emerald-500/10 text-emerald-300 border-emerald-500/30",
  ISR: "bg-cyan-500/10 text-cyan-300 border-cyan-500/30",
  SSR: "bg-brand-500/15 text-brand-300 border-brand-500/30",
  STREAM: "bg-sky-500/10 text-sky-300 border-sky-500/30",
  ACTION: "bg-amber-500/10 text-amber-300 border-amber-500/30",
  API: "bg-indigo-500/10 text-indigo-300 border-indigo-500/30",
  EDGE: "bg-rose-500/10 text-rose-300 border-rose-500/30",
  META: "bg-surface-800 text-surface-200 border-surface-700",
};

interface RouteLabShellProps {
  title: string;
  subtitle: string;
  pattern: string;
  mode: RouteMode;
  backHref?: string;
  backLabel?: string;
  children: ReactNode;
}

export default function RouteLabShell({
  title,
  subtitle,
  pattern,
  mode,
  backHref = "/routes",
  backLabel = "Back to Routes Lab",
  children,
}: RouteLabShellProps) {
  return (
    <div className="min-h-screen flex flex-col bg-surface-950 nd-blueprint-grid">
      <main className="flex-1 w-full max-w-5xl mx-auto px-4 sm:px-6 py-10 sm:py-14">
        <div className="mb-6">
          <Link
            href={backHref}
            className="inline-flex items-center gap-2 text-xs font-mono text-surface-400 hover:text-brand-300 transition-colors"
          >
            <ArrowLeft className="w-3.5 h-3.5" aria-hidden="true" />
            {backLabel}
          </Link>
        </div>

        <header className="mb-8 pb-6 border-b border-surface-800/80">
          <div className="flex flex-wrap items-center gap-2.5 mb-3">
            <span
              className={`px-2.5 py-0.5 rounded-full border text-[11px] font-mono font-semibold uppercase tracking-wider ${MODE_COLORS[mode]}`}
            >
              {mode}
            </span>
            <code className="px-2.5 py-0.5 rounded-md bg-surface-900 border border-surface-800 text-xs font-mono text-brand-300">
              {pattern}
            </code>
          </div>
          <h1 className="text-2xl sm:text-4xl font-bold text-white tracking-tight">
            {title}
          </h1>
          <p className="mt-2 text-sm sm:text-base text-surface-300 leading-relaxed max-w-3xl">
            {subtitle}
          </p>
        </header>

        <div className="space-y-6">{children}</div>
      </main>
      <Footer />
    </div>
  );
}
