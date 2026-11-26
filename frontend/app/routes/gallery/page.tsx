import Link from "next/link";
import RouteLabShell from "@/components/route-lab-shell";
import { GALLERY_ITEMS } from "./items";

export default function GalleryPage() {
  return (
    <RouteLabShell
      title="Intercepting Routes & @modal Slot"
      subtitle="Soft-navigating to an item intercepts /routes/gallery/item/[id] inside the @modal overlay. Direct visits or full page reloads render the standalone item page."
      pattern="/routes/gallery -> (.)item/[id]"
      mode="SSR"
    >
      <div className="grid sm:grid-cols-3 gap-4">
        {GALLERY_ITEMS.map((item) => (
          <div
            key={item.id}
            className="rounded-xl border border-surface-800 bg-surface-900/70 p-5 flex flex-col justify-between gap-4"
          >
            <div>
              <span className="inline-block px-2 py-0.5 rounded bg-brand-500/15 border border-brand-500/30 text-[11px] font-mono text-brand-300 mb-2">
                {item.tag}
              </span>
              <h2 className="text-base font-semibold text-white">
                {item.title}
              </h2>
              <p className="mt-1.5 text-xs text-surface-300 leading-relaxed">
                {item.summary}
              </p>
            </div>
            <div className="flex flex-wrap items-center gap-3 pt-3 border-t border-surface-800 text-xs font-mono">
              <Link
                href={`/routes/gallery/item/${item.id}`}
                className="text-brand-400 hover:text-brand-300 font-semibold"
              >
                Open Intercepted Modal →
              </Link>
              <a
                href={`/routes/gallery/item/${item.id}`}
                className="text-surface-400 hover:text-white"
              >
                Direct Page
              </a>
            </div>
          </div>
        ))}
      </div>
    </RouteLabShell>
  );
}
