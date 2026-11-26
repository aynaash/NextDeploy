import Link from "next/link";
import RouteLabShell from "@/components/route-lab-shell";

export default function ScopedNotFound() {
  return (
    <RouteLabShell
      title="404 — Scoped Not-Found Boundary"
      subtitle="Rendered by app/routes/not-found-demo/not-found.tsx in response to notFound()."
      pattern="/routes/not-found-demo?trigger=1"
      mode="SSR"
    >
      <div className="rounded-xl border border-rose-500/30 bg-surface-900/80 p-6 space-y-4">
        <div className="inline-block px-2.5 py-1 rounded bg-rose-500/15 border border-rose-500/30 text-xs font-mono text-rose-300">
          HTTP 404 Not Found
        </div>
        <p className="text-sm text-surface-300">
          This confirms that <code className="text-brand-300 font-mono">notFound()</code>{" "}
          resolves the nearest <code className="text-brand-300 font-mono">not-found.tsx</code>{" "}
          boundary and preserves the 404 HTTP status code.
        </p>
        <div className="flex flex-wrap gap-3 text-xs font-mono">
          <Link
            href="/routes/not-found-demo"
            className="px-3 py-1.5 rounded-lg bg-brand-500 text-surface-950 font-semibold"
          >
            Recover to /routes/not-found-demo
          </Link>
          <Link
            href="/routes"
            className="px-3 py-1.5 rounded-lg bg-surface-950 border border-surface-800 text-surface-300 hover:text-white"
          >
            Back to Routes Lab
          </Link>
        </div>
      </div>
    </RouteLabShell>
  );
}
