"use client";

import { useEffect, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import { X } from "lucide-react";

export default function ModalFrame({
  title,
  itemId,
  children,
}: {
  title: string;
  itemId: string;
  children: ReactNode;
}) {
  const router = useRouter();

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        router.back();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [router]);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label={title}
      onClick={() => router.back()}
      className="fixed inset-0 z-50 flex items-center justify-center bg-surface-950/80 backdrop-blur-sm p-4"
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-lg rounded-2xl border border-brand-500/40 bg-surface-900 p-6 shadow-2xl space-y-4"
      >
        <div className="flex items-start justify-between gap-4">
          <div>
            <div className="text-[11px] font-mono uppercase tracking-wider text-brand-400">
              Intercepted via @modal/(.)item/[id]
            </div>
            <h2 className="text-xl font-bold text-white mt-1">{title}</h2>
          </div>
          <button
            type="button"
            onClick={() => router.back()}
            aria-label="Close modal"
            className="p-1.5 rounded-lg text-surface-400 hover:text-white hover:bg-surface-800 transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {children}

        <div className="pt-3 border-t border-surface-800 flex items-center justify-between text-xs font-mono">
          <button
            type="button"
            onClick={() => router.back()}
            className="text-surface-300 hover:text-white"
          >
            ← Back to Gallery
          </button>
          <a
            href={`/routes/gallery/item/${encodeURIComponent(itemId)}`}
            className="text-brand-400 hover:text-brand-300"
          >
            Hard Reload as Full Page →
          </a>
        </div>
      </div>
    </div>
  );
}
