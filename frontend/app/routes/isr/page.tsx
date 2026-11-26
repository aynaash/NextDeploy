import type { Metadata } from "next";
import RouteLabShell from "@/components/route-lab-shell";
import RevalidateButton from "./revalidate-button";

export const revalidate = 30;

export const metadata: Metadata = {
  title: "ISR Route (revalidate = 30)",
  description:
    "Incremental Static Regeneration with 30s time window and on-demand revalidatePath().",
};

export default function IsrRoutePage() {
  const generatedAt = new Date().toISOString();

  return (
    <RouteLabShell
      title="Incremental Static Regeneration (ISR)"
      subtitle="Configured with revalidate = 30. Cached in R2 with revalidation metadata and refreshed asynchronously in the background or on-demand via revalidatePath('/routes/isr')."
      pattern="/routes/isr"
      mode="ISR"
    >
      <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-6 space-y-5">
        <div className="grid sm:grid-cols-2 gap-4 font-mono text-xs">
          <div className="p-4 rounded-lg bg-surface-950 border border-surface-800">
            <div className="text-surface-500 uppercase tracking-wider mb-1">
              Last Generated At
            </div>
            <div className="text-cyan-300 text-sm">{generatedAt}</div>
          </div>
          <div className="p-4 rounded-lg bg-surface-950 border border-surface-800">
            <div className="text-surface-500 uppercase tracking-wider mb-1">
              Revalidation Window
            </div>
            <div className="text-surface-200 text-sm">
              export const revalidate = 30 (seconds)
            </div>
          </div>
        </div>

        <RevalidateButton />

        <p className="text-sm text-surface-300 leading-relaxed">
          Clicking the button calls{" "}
          <code className="text-brand-300 font-mono">POST /api/routes/revalidate</code>,
          which runs <code className="text-brand-300 font-mono">revalidatePath(&quot;/routes/isr&quot;)</code>{" "}
          and refreshes the route.
        </p>
      </div>
    </RouteLabShell>
  );
}
