import type { Metadata } from "next";
import Link from "next/link";
import RouteLabShell from "@/components/route-lab-shell";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Catch-All Segment Route",
  description: "Arbitrary-depth catch-all route /routes/catch-all/[...segments].",
};

interface CatchAllPageProps {
  params: Promise<{ segments: string[] }>;
}

export default async function CatchAllPage({ params }: CatchAllPageProps) {
  const { segments } = await params;

  return (
    <RouteLabShell
      title="Catch-All Segment ([...segments])"
      subtitle="Captures one or more trailing URL segments as a string array, verified against Cloudflare Workers trie routing."
      pattern="/routes/catch-all/[...segments]"
      mode="SSR"
    >
      <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-6 space-y-5">
        <div className="grid sm:grid-cols-2 gap-4 text-xs font-mono">
          <div className="p-4 rounded-lg bg-surface-950 border border-surface-800">
            <div className="text-surface-500 uppercase mb-1">Segment Depth</div>
            <div className="text-brand-300 text-sm">{segments.length}</div>
          </div>
          <div className="p-4 rounded-lg bg-surface-950 border border-surface-800">
            <div className="text-surface-500 uppercase mb-1">Joined Path</div>
            <div className="text-surface-200 text-sm break-all">
              /{segments.join("/")}
            </div>
          </div>
        </div>

        <div>
          <div className="text-xs font-mono text-surface-400 mb-2">
            Clickable Breadcrumbs:
          </div>
          <div className="flex flex-wrap items-center gap-2 text-xs font-mono">
            {segments.map((seg, idx) => {
              const prefix = segments.slice(0, idx + 1).join("/");
              return (
                <Link
                  key={`${prefix}-${idx}`}
                  href={`/routes/catch-all/${prefix}`}
                  className="px-2.5 py-1 rounded bg-surface-950 border border-surface-800 hover:border-brand-500/40 text-brand-300"
                >
                  [{idx}] {seg}
                </Link>
              );
            })}
          </div>
        </div>

        <div className="flex flex-wrap gap-2 pt-2 border-t border-surface-800">
          <Link
            href="/routes/catch-all/one"
            className="px-3 py-1.5 rounded-lg bg-surface-950 border border-surface-800 text-xs font-mono text-surface-300 hover:text-white"
          >
            Depth 1 (/one)
          </Link>
          <Link
            href="/routes/catch-all/edge/r2/assets/v1/manifest"
            className="px-3 py-1.5 rounded-lg bg-surface-950 border border-surface-800 text-xs font-mono text-surface-300 hover:text-white"
          >
            Depth 5 (/edge/r2/assets/v1/manifest)
          </Link>
        </div>
      </div>
    </RouteLabShell>
  );
}
