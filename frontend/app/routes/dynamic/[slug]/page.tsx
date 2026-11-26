import type { Metadata } from "next";
import Link from "next/link";
import RouteLabShell from "@/components/route-lab-shell";

const PRESET_SLUGS = ["cloudflare", "vps", "aws"] as const;

interface DynamicSlugPageProps {
  params: Promise<{ slug: string }>;
}

export const dynamicParams = true;

export async function generateStaticParams() {
  return PRESET_SLUGS.map((slug) => ({ slug }));
}

export async function generateMetadata({
  params,
}: DynamicSlugPageProps): Promise<Metadata> {
  const { slug } = await params;
  return {
    title: `Dynamic Slug: ${slug}`,
    description: `Dynamic [slug] route rendered for "${slug}" on NextDeploy.`,
  };
}

export default async function DynamicSlugPage({ params }: DynamicSlugPageProps) {
  const { slug } = await params;
  const isPrerendered = (PRESET_SLUGS as readonly string[]).includes(slug);

  return (
    <RouteLabShell
      title={`Dynamic Segment: ${slug}`}
      subtitle="Demonstrates generateStaticParams() combined with dynamicParams = true and async generateMetadata()."
      pattern="/routes/dynamic/[slug]"
      mode={isPrerendered ? "SSG" : "SSR"}
    >
      <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-6 space-y-5">
        <div className="grid sm:grid-cols-3 gap-4 text-xs font-mono">
          <div className="p-4 rounded-lg bg-surface-950 border border-surface-800">
            <div className="text-surface-500 uppercase mb-1">Resolved slug</div>
            <div className="text-brand-300 text-sm break-all">{slug}</div>
          </div>
          <div className="p-4 rounded-lg bg-surface-950 border border-surface-800">
            <div className="text-surface-500 uppercase mb-1">Resolution Path</div>
            <div className="text-surface-200 text-sm">
              {isPrerendered ? "Pre-rendered (R2)" : "On-demand SSR (Worker)"}
            </div>
          </div>
          <div className="p-4 rounded-lg bg-surface-950 border border-surface-800">
            <div className="text-surface-500 uppercase mb-1">Rendered At</div>
            <div className="text-surface-200 text-sm">
              {new Date().toISOString()}
            </div>
          </div>
        </div>

        <div>
          <div className="text-xs font-mono text-surface-400 mb-2">
            Switch between pre-rendered and on-demand slugs:
          </div>
          <div className="flex flex-wrap gap-2">
            {[...PRESET_SLUGS, "edge-worker-live", "turbopack-16"].map((item) => (
              <Link
                key={item}
                href={`/routes/dynamic/${encodeURIComponent(item)}`}
                className={`px-3 py-1.5 rounded-lg text-xs font-mono border transition-colors ${
                  item === slug
                    ? "bg-brand-500/15 border-brand-500/40 text-brand-300"
                    : "bg-surface-950 border-surface-800 text-surface-300 hover:border-surface-700 hover:text-white"
                }`}
              >
                {item}
              </Link>
            ))}
          </div>
        </div>
      </div>
    </RouteLabShell>
  );
}
