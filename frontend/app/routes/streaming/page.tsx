import { Suspense } from "react";
import type { Metadata } from "next";
import RouteLabShell from "@/components/route-lab-shell";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Streaming RSC Route",
  description:
    "Progressive React Server Component streaming with staggered Suspense boundaries.",
};

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function SlowStreamPanel({
  label,
  delayMs,
  detail,
}: {
  label: string;
  delayMs: number;
  detail: string;
}) {
  await sleep(delayMs);
  return (
    <div className="rounded-xl border border-brand-500/30 bg-surface-900/80 p-5 space-y-2">
      <div className="flex items-center justify-between text-xs font-mono">
        <span className="text-brand-300 font-semibold">{label}</span>
        <span className="text-emerald-400">+{delayMs}ms</span>
      </div>
      <p className="text-xs text-surface-300 leading-relaxed">{detail}</p>
      <div className="text-[11px] font-mono text-surface-500">
        Flushed at {new Date().toISOString()}
      </div>
    </div>
  );
}

function PanelFallback({ label, delayMs }: { label: string; delayMs: number }) {
  return (
    <div className="rounded-xl border border-surface-800 bg-surface-900/40 p-5 space-y-2 animate-pulse">
      <div className="flex items-center justify-between text-xs font-mono">
        <span className="text-surface-400">{label}</span>
        <span className="text-surface-500">streaming ({delayMs}ms)...</span>
      </div>
      <div className="h-4 w-3/4 rounded bg-surface-800" />
      <div className="h-3 w-1/2 rounded bg-surface-800/80" />
    </div>
  );
}

export default function StreamingRoutePage() {
  const shellTime = new Date().toISOString();

  return (
    <RouteLabShell
      title="Progressive RSC Streaming & Suspense"
      subtitle="The outer shell flushes immediately while three async Server Components stream in progressively at 120ms, 320ms, and 550ms."
      pattern="/routes/streaming"
      mode="STREAM"
    >
      <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-4 text-xs font-mono flex flex-wrap items-center justify-between gap-2">
        <span className="text-surface-400">Immediate Shell Timestamp</span>
        <span className="text-brand-300">{shellTime}</span>
      </div>

      <div className="grid sm:grid-cols-3 gap-4">
        <Suspense fallback={<PanelFallback label="Stage 1: Edge Manifest" delayMs={120} />}>
          <SlowStreamPanel
            label="Stage 1: Edge Manifest"
            delayMs={120}
            detail="Resolves route classification and static asset manifest chunk."
          />
        </Suspense>

        <Suspense fallback={<PanelFallback label="Stage 2: KV Rate Check" delayMs={320} />}>
          <SlowStreamPanel
            label="Stage 2: KV Rate Check"
            delayMs={320}
            detail="Simulates mid-latency KV or Durable Object metadata fetch."
          />
        </Suspense>

        <Suspense fallback={<PanelFallback label="Stage 3: Origin Query" delayMs={550} />}>
          <SlowStreamPanel
            label="Stage 3: Origin Query"
            delayMs={550}
            detail="Simulates slowest backend boundary flushing last in the Flight stream."
          />
        </Suspense>
      </div>
    </RouteLabShell>
  );
}
