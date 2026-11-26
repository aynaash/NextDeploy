import type { ReactNode } from "react";
import type { Metadata } from "next";
import RouteLabShell from "@/components/route-lab-shell";

export const metadata: Metadata = {
  title: "Parallel Routes (@metrics + @logs)",
  description:
    "App Router parallel routes rendering children, @metrics, and @logs slots together.",
};

export default function ParallelRoutesLayout({
  children,
  metrics,
  logs,
}: {
  children: ReactNode;
  metrics: ReactNode;
  logs: ReactNode;
}) {
  return (
    <RouteLabShell
      title="Parallel Routes (@metrics & @logs)"
      subtitle="Renders the primary children slot alongside two independent named parallel route slots in a single layout composition."
      pattern="/routes/parallel (@metrics, @logs)"
      mode="SSR"
    >
      <div>{children}</div>
      <div className="grid md:grid-cols-2 gap-4">
        <div>{metrics}</div>
        <div>{logs}</div>
      </div>
    </RouteLabShell>
  );
}
