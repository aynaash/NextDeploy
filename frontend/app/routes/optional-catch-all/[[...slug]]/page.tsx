import type { Metadata } from "next";
import Link from "next/link";
import RouteLabShell from "@/components/route-lab-shell";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Optional Catch-All Route",
  description:
    "Optional catch-all route /routes/optional-catch-all/[[...slug]] matching zero or more segments.",
};

interface OptionalCatchAllProps {
  params: Promise<{ slug?: string[] }>;
}

export default async function OptionalCatchAllPage({
  params,
}: OptionalCatchAllProps) {
  const { slug } = await params;
  const segments = slug ?? [];
  const isRootMatch = slug === undefined;

  return (
    <RouteLabShell
      title="Optional Catch-All ([[...slug]])"
      subtitle="Matches both the base route (/routes/optional-catch-all) when slug is undefined and any deeper nested segment list."
      pattern="/routes/optional-catch-all/[[...slug]]"
      mode="SSR"
    >
      <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-6 space-y-5">
        <div className="grid sm:grid-cols-3 gap-4 text-xs font-mono">
          <div className="p-4 rounded-lg bg-surface-950 border border-surface-800">
            <div className="text-surface-500 uppercase mb-1">Match Type</div>
            <div className="text-brand-300 text-sm">
              {isRootMatch ? "Root (slug === undefined)" : "Nested segments"}
            </div>
          </div>
          <div className="p-4 rounded-lg bg-surface-950 border border-surface-800">
            <div className="text-surface-500 uppercase mb-1">Segment Count</div>
            <div className="text-surface-200 text-sm">{segments.length}</div>
          </div>
          <div className="p-4 rounded-lg bg-surface-950 border border-surface-800">
            <div className="text-surface-500 uppercase mb-1">JSON Value</div>
            <div className="text-surface-200 text-sm break-all">
              {JSON.stringify(slug ?? null)}
            </div>
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          <Link
            href="/routes/optional-catch-all"
            className={`px-3 py-1.5 rounded-lg border text-xs font-mono ${
              isRootMatch
                ? "bg-brand-500/15 border-brand-500/40 text-brand-300"
                : "bg-surface-950 border-surface-800 text-surface-300 hover:text-white"
            }`}
          >
            /optional-catch-all (root)
          </Link>
          <Link
            href="/routes/optional-catch-all/edge/worker/v1"
            className="px-3 py-1.5 rounded-lg bg-surface-950 border border-surface-800 text-xs font-mono text-surface-300 hover:text-white"
          >
            /optional-catch-all/edge/worker/v1
          </Link>
        </div>
      </div>
    </RouteLabShell>
  );
}
