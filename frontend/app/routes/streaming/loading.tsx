import RouteLabShell from "@/components/route-lab-shell";

export default function StreamingLoading() {
  return (
    <RouteLabShell
      title="Progressive RSC Streaming & Loading"
      subtitle="Streaming initial route shell..."
      pattern="/routes/streaming"
      mode="STREAM"
    >
      <div className="grid sm:grid-cols-3 gap-4 animate-pulse">
        {[1, 2, 3].map((idx) => (
          <div
            key={idx}
            className="h-32 rounded-xl border border-surface-800 bg-surface-900/60"
          />
        ))}
      </div>
    </RouteLabShell>
  );
}
