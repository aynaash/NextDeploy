import type { Metadata } from "next";
import Link from "next/link";
import RouteLabShell from "@/components/route-lab-shell";
import { findGalleryItem } from "../../items";

export const dynamic = "force-dynamic";

interface ItemPageProps {
  params: Promise<{ id: string }>;
}

export async function generateMetadata({
  params,
}: ItemPageProps): Promise<Metadata> {
  const { id } = await params;
  const item = findGalleryItem(id);
  return {
    title: item.title,
    description: item.summary,
  };
}

export default async function GalleryItemFullPage({ params }: ItemPageProps) {
  const { id } = await params;
  const item = findGalleryItem(id);

  return (
    <RouteLabShell
      title={item.title}
      subtitle="Standalone full-page render when navigated to directly or hard-reloaded."
      pattern="/routes/gallery/item/[id]"
      mode="SSR"
      backHref="/routes/gallery"
      backLabel="Back to Gallery"
    >
      <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-6 space-y-4">
        <div className="flex items-center gap-2 text-xs font-mono">
          <span className="px-2 py-0.5 rounded bg-brand-500/15 border border-brand-500/30 text-brand-300">
            {item.tag}
          </span>
          <span className="text-surface-400">id: {item.id}</span>
        </div>
        <p className="text-sm text-surface-200 leading-relaxed">
          {item.details}
        </p>
        <div className="pt-3 border-t border-surface-800 text-xs font-mono">
          <Link
            href="/routes/gallery"
            className="text-brand-400 hover:text-brand-300"
          >
            Return to Gallery to test soft modal interception →
          </Link>
        </div>
      </div>
    </RouteLabShell>
  );
}
