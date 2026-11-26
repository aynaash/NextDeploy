import type { Metadata } from "next";
import { cookies } from "next/headers";
import RouteLabShell from "@/components/route-lab-shell";
import ActionsClient from "./actions-client";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Server Actions Demo",
  description:
    "Tests Next.js Server Actions using Web Crypto SHA-256, cookies(), and revalidatePath().",
};

export default async function ActionsPage() {
  const store = await cookies();
  const currentCookie = store.get("nextdeploy_route_demo")?.value ?? null;

  return (
    <RouteLabShell
      title="Server Actions ('use server')"
      subtitle="Exercises three Server Actions with React 19 useActionState: Web Crypto hashing, cookie mutation, and on-demand ISR path revalidation."
      pattern="/routes/actions"
      mode="ACTION"
    >
      <ActionsClient currentCookie={currentCookie} />
    </RouteLabShell>
  );
}
