export const dynamic = "force-dynamic";

export default function ParallelMainPage() {
  return (
    <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-5 space-y-2">
      <div className="flex items-center justify-between text-xs font-mono">
        <span className="text-brand-300 font-semibold">children slot</span>
        <span className="text-surface-500">{new Date().toISOString()}</span>
      </div>
      <p className="text-sm text-surface-300">
        Primary <code className="text-brand-300 font-mono">page.tsx</code> content
        rendered simultaneously with <code className="text-cyan-300 font-mono">@metrics</code>{" "}
        and <code className="text-amber-300 font-mono">@logs</code> slots below.
      </p>
    </div>
  );
}
