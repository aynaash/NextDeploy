import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import RouteLabShell from "@/components/route-lab-shell";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Scoped notFound() Demo",
  description: "Tests notFound() and scoped not-found.tsx HTTP 404 rendering.",
};

export default async function NotFoundDemoPage({
  searchParams,
}: {
  searchParams: Promise<{ trigger?: string }>;
}) {
  const { trigger } = await searchParams;
  if (trigger === "1") {
    notFound();
  }

  return (
    <RouteLabShell
      title="Scoped notFound() Boundary"
      subtitle="Renders normally with HTTP 200 by default, and invokes next/navigation notFound() when ?trigger=1 is present."
      pattern="/routes/not-found-demo"
      mode="SSR"
    >
      <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-6 space-y-4">
        <p className="text-sm text-surface-300 leading-relaxed">
          Clicking below calls <code className="text-brand-300 font-mono">notFound()</code>{" "}
          on the server, rendering <code className="text-brand-300 font-mono">app/routes/not-found-demo/not-found.tsx</code>{" "}
          with an HTTP <code className="text-rose-300 font-mono">404</code> status.
        </p>
        <Link
          href="/routes/not-found-demo?trigger=1"
          className="inline-flex items-center px-4 py-2 rounded-lg bg-rose-500/15 border border-rose-500/40 text-rose-300 hover:bg-rose-500/25 text-xs font-mono font-semibold transition-colors"
        >
          Trigger Scoped 404 (?trigger=1) →
        </Link>
      </div>
    </RouteLabShell>
  );
}
