import type { Metadata } from "next";
import RouteLabShell from "@/components/route-lab-shell";
import ErrorTrigger from "./error-trigger";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Error Boundary Demo",
  description: "Tests scoped error.tsx error boundary and reset() recovery.",
};

export default async function ErrorDemoPage({
  searchParams,
}: {
  searchParams: Promise<{ boom?: string }>;
}) {
  const { boom } = await searchParams;

  return (
    <RouteLabShell
      title="Error Boundary & reset() Recovery"
      subtitle="Demonstrates scoped error.tsx isolation inside the Cloudflare NextNodeServer runtime."
      pattern="/routes/error-demo"
      mode="SSR"
    >
      <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-6 space-y-4">
        <p className="text-sm text-surface-300 leading-relaxed">
          Click the trigger below to throw a React render exception inside{" "}
          <code className="text-brand-300 font-mono">ErrorTrigger</code>. Next.js
          catches it in{" "}
          <code className="text-brand-300 font-mono">app/routes/error-demo/error.tsx</code>{" "}
          and lets you recover via <code className="text-brand-300 font-mono">reset()</code>.
        </p>
        <ErrorTrigger initialBoom={boom === "1"} />
      </div>
    </RouteLabShell>
  );
}
