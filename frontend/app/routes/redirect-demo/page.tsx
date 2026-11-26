import { redirect } from "next/navigation";

export const dynamic = "force-dynamic";

export default function RedirectDemoPage() {
  redirect("/routes/ssr?from=redirect-demo");
}
