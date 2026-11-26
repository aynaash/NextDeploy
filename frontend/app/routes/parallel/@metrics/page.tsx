export const dynamic = "force-dynamic";

export default function ParallelMetricsSlot() {
  return (
    <div className="rounded-xl border border-cyan-500/30 bg-surface-900/70 p-5 space-y-3 h-full">
      <div className="flex items-center justify-between text-xs font-mono">
        <span className="text-cyan-300 font-semibold">@metrics slot</span>
        <span className="text-surface-500">{new Date().toISOString()}</span>
      </div>
      <dl className="grid grid-cols-2 gap-3 text-xs font-mono">
        <div className="p-3 rounded-lg bg-surface-950 border border-surface-800">
          <dt className="text-surface-500">Runtime</dt>
          <dd className="text-surface-100 mt-0.5">workerd / edge</dd>
        </div>
        <div className="p-3 rounded-lg bg-surface-950 border border-surface-800">
          <dt className="text-surface-500">Asset Store</dt>
          <dd className="text-surface-100 mt-0.5">Cloudflare R2</dd>
        </div>
        <div className="p-3 rounded-lg bg-surface-950 border border-surface-800">
          <dt className="text-surface-500">Cutover</dt>
          <dd className="text-emerald-400 mt-0.5">Atomic</dd>
        </div>
        <div className="p-3 rounded-lg bg-surface-950 border border-surface-800">
          <dt className="text-surface-500">Rate Limit</dt>
          <dd className="text-surface-100 mt-0.5">60 req/min KV</dd>
        </div>
      </dl>
    </div>
  );
}
