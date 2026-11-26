import type { Metadata } from "next";
import Link from "next/link";
import RouteLabShell from "@/components/route-lab-shell";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Nested Dynamic Route",
  description: "Multi-segment dynamic route /routes/nested/[category]/[id].",
};

interface NestedDynamicPageProps {
  params: Promise<{ category: string; id: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}

export default async function NestedDynamicPage({
  params,
  searchParams,
}: NestedDynamicPageProps) {
  const [{ category, id }, query] = await Promise.all([params, searchParams]);

  return (
    <RouteLabShell
      title={`Nested Route: ${category} / ${id}`}
      subtitle="Multi-parameter App Router dynamic matching across two independent path segments and async searchParams."
      pattern="/routes/nested/[category]/[id]"
      mode="SSR"
    >
      <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-6 space-y-5">
        <div className="grid sm:grid-cols-3 gap-4 text-xs font-mono">
          <div className="p-4 rounded-lg bg-surface-950 border border-surface-800">
            <div className="text-surface-500 uppercase mb-1">category</div>
            <div className="text-brand-300 text-sm break-all">{category}</div>
          </div>
          <div className="p-4 rounded-lg bg-surface-950 border border-surface-800">
            <div className="text-surface-500 uppercase mb-1">id</div>
            <div className="text-cyan-300 text-sm break-all">{id}</div>
          </div>
          <div className="p-4 rounded-lg bg-surface-950 border border-surface-800">
            <div className="text-surface-500 uppercase mb-1">searchParams</div>
            <div className="text-surface-200 text-sm break-all">
              {JSON.stringify(query)}
            </div>
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          {[
            { category: "workers", id: "42", q: "env=edge" },
            { category: "r2", id: "assets-v1", q: "cache=immutable" },
            { category: "kv", id: "rate-limit", q: "ttl=60" },
          ].map((ex) => (
            <Link
              key={`${ex.category}-${ex.id}`}
              href={`/routes/nested/${ex.category}/${ex.id}?${ex.q}`}
              className="px-3 py-1.5 rounded-lg bg-surface-950 border border-surface-800 hover:border-surface-700 text-xs font-mono text-surface-300 hover:text-white transition-colors"
            >
              /nested/{ex.category}/{ex.id}?{ex.q}
            </Link>
          ))}
        </div>
      </div>
    </RouteLabShell>
  );
}
