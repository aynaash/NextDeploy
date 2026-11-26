import type { Metadata } from "next";
import RouteLabShell from "@/components/route-lab-shell";

export const dynamic = "force-static";

export const metadata: Metadata = {
  title: "Static Route (SSG)",
  description: "Forced static Next.js route served from Cloudflare R2 assets.",
};

export default function StaticRoutePage() {
  const builtAt = new Date().toISOString();

  return (
    <RouteLabShell
      title="Static Prerender (SSG)"
      subtitle="Exported with dynamic = 'force-static'. Rendered once during next build and uploaded to Cloudflare R2 so requests never hit compute."
      pattern="/routes/static"
      mode="SSG"
    >
      <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-6 space-y-4">
        <div className="grid sm:grid-cols-2 gap-4 font-mono text-xs">
          <div className="p-4 rounded-lg bg-surface-950 border border-surface-800">
            <div className="text-surface-500 uppercase tracking-wider mb-1">
              Build-Time Timestamp
            </div>
            <div className="text-brand-300 text-sm">{builtAt}</div>
          </div>
          <div className="p-4 rounded-lg bg-surface-950 border border-surface-800">
            <div className="text-surface-500 uppercase tracking-wider mb-1">
              Route Segment Config
            </div>
            <div className="text-surface-200 text-sm">
              export const dynamic = &quot;force-static&quot;
            </div>
          </div>
        </div>
        <p className="text-sm text-surface-300 leading-relaxed">
          Reloading this page should show the exact same build-time timestamp until the
          next deployment. On Cloudflare Workers, NextDeploy serves this pre-rendered HTML
          directly from the R2 <code className="text-brand-300 font-mono">ASSETS</code>{" "}
          bucket.
        </p>
      </div>
    </RouteLabShell>
  );
}
