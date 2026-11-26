export const dynamic = "force-dynamic";

export default function ParallelLogsSlot() {
  const now = new Date().toISOString();
  const lines = [
    `[${now}] [edge-guard] public path /routes/parallel allowed`,
    `[${now}] [proxy.ts] injected x-powered-by: nextdeploy-frontend`,
    `[${now}] [nextserver] composed parallel slots: children, @metrics, @logs`,
  ];

  return (
    <div className="rounded-xl border border-amber-500/30 bg-surface-900/70 p-5 space-y-3 h-full">
      <div className="flex items-center justify-between text-xs font-mono">
        <span className="text-amber-300 font-semibold">@logs slot</span>
        <span className="text-surface-500">live server render</span>
      </div>
      <div className="rounded-lg bg-surface-950 border border-surface-800 p-3 space-y-1.5 font-mono text-[11px] text-surface-300 overflow-x-auto">
        {lines.map((line, i) => (
          <div key={i} className="whitespace-nowrap">
            {line}
          </div>
        ))}
      </div>
    </div>
  );
}
