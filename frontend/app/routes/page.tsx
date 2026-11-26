import type { Metadata } from "next";
import Footer from "@/components/footer";
import RoutesMatrixClient from "@/components/routes-matrix-client";
import { ROUTE_CATEGORIES, ROUTES_CATALOG } from "@/lib/routes-catalog";

export const metadata: Metadata = {
  title: "Next.js Routes Lab",
  description:
    "Live Cloudflare Workers dogfood matrix exercising every Next.js 16 App Router route pattern, rendering mode, Server Action, and API Route Handler.",
  alternates: {
    canonical: "/routes",
  },
};

export default function RoutesHubPage() {
  return (
    <div className="min-h-screen flex flex-col bg-surface-950 nd-blueprint-grid">
      <main className="flex-1 w-full max-w-7xl mx-auto px-4 sm:px-6 py-10 sm:py-16">
        <header className="mb-10 pb-8 border-b border-surface-800/80">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-surface-900/90 border border-surface-700/80 text-xs font-mono text-brand-300 mb-4">
            Cloudflare Workers Dogfood Lab · nextdeploy.org
          </div>
          <h1 className="text-3xl sm:text-5xl font-bold text-white tracking-tight">
            Next.js App Router{" "}
            <span className="text-brand-400">Routes Matrix</span>
          </h1>
          <p className="mt-3 text-sm sm:text-base text-surface-300 max-w-3xl leading-relaxed">
            Every route pattern below is compiled by{" "}
            <code className="text-brand-300 font-mono">nextcompile</code> and
            served live by <code className="text-brand-300 font-mono">nextdeploy.org</code>{" "}
            on Cloudflare Workers + R2 + KV. Click any route to inspect it or run
            live HTTP probes directly from your browser.
          </p>

          <dl className="grid grid-cols-2 sm:grid-cols-4 gap-3 mt-6 max-w-2xl font-mono text-xs">
            <div className="px-3.5 py-2.5 rounded-lg bg-surface-900/70 border border-surface-800">
              <dt className="text-surface-500 uppercase text-[10px]">
                Probed Routes
              </dt>
              <dd className="text-lg font-bold text-white mt-0.5">
                {ROUTES_CATALOG.length}
              </dd>
            </div>
            <div className="px-3.5 py-2.5 rounded-lg bg-surface-900/70 border border-surface-800">
              <dt className="text-surface-500 uppercase text-[10px]">
                Categories
              </dt>
              <dd className="text-lg font-bold text-brand-300 mt-0.5">
                {ROUTE_CATEGORIES.length}
              </dd>
            </div>
            <div className="px-3.5 py-2.5 rounded-lg bg-surface-900/70 border border-surface-800">
              <dt className="text-surface-500 uppercase text-[10px]">
                API Handlers
              </dt>
              <dd className="text-lg font-bold text-cyan-300 mt-0.5">6</dd>
            </div>
            <div className="px-3.5 py-2.5 rounded-lg bg-surface-900/70 border border-surface-800">
              <dt className="text-surface-500 uppercase text-[10px]">
                Edge Guard
              </dt>
              <dd className="text-lg font-bold text-emerald-400 mt-0.5">
                KV + HMAC
              </dd>
            </div>
          </dl>
        </header>

        <RoutesMatrixClient />
      </main>
      <Footer />
    </div>
  );
}
