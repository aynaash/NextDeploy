import type { Metadata } from "next";
import RouteLabShell from "@/components/route-lab-shell";

export const dynamic = "force-static";

export const metadata: Metadata = {
  title: "Route Group Demo",
  description:
    "App Router parenthesized route group (grouped) applying a scoped layout without affecting the URL.",
};

export default function GroupedRoutePage() {
  return (
    <RouteLabShell
      title="Route Group Layout ((grouped))"
      subtitle="Lives at app/routes/(grouped)/grouped/page.tsx while resolving publicly at /routes/grouped."
      pattern="/routes/(grouped)/grouped"
      mode="SSG"
    >
      <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-6 space-y-3 text-sm text-surface-300">
        <p>
          The emerald banner at the top of this page comes exclusively from{" "}
          <code className="text-brand-300 font-mono">
            app/routes/(grouped)/layout.tsx
          </code>
          . Other routes under <code className="text-brand-300 font-mono">/routes/*</code>{" "}
          do not inherit it.
        </p>
        <p className="text-xs font-mono text-surface-400">
          Public URL: <span className="text-brand-300">/routes/grouped</span>
        </p>
      </div>
    </RouteLabShell>
  );
}
