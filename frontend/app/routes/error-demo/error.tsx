"use client";

import Link from "next/link";
import { useEffect } from "react";
import RouteLabShell from "@/components/route-lab-shell";

export default function ScopedErrorBoundary({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    console.error("ScopedErrorBoundary caught:", error);
  }, [error]);

  return (
    <RouteLabShell
      title="Error Boundary Caught Render Failure"
      subtitle="Rendered by app/routes/error-demo/error.tsx without crashing the rest of the application."
      pattern="/routes/error-demo (error.tsx)"
      mode="SSR"
    >
      <div className="rounded-xl border border-rose-500/30 bg-surface-900/80 p-6 space-y-4">
        <div className="text-xs font-mono text-rose-300 bg-surface-950 border border-surface-800 rounded-lg p-3 break-all">
          {error.message}
          {error.digest ? ` (digest: ${error.digest})` : ""}
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <button
            type="button"
            onClick={() => reset()}
            className="px-4 py-2 rounded-lg bg-brand-500 hover:bg-brand-400 text-surface-950 text-xs font-semibold transition-colors"
          >
            Recover via reset()
          </button>
          <Link
            href="/routes/error-demo"
            className="px-4 py-2 rounded-lg bg-surface-950 border border-surface-800 text-surface-300 hover:text-white text-xs font-mono"
          >
            Clean Reload
          </Link>
        </div>
      </div>
    </RouteLabShell>
  );
}
