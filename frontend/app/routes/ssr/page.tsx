import type { Metadata } from "next";
import Link from "next/link";
import { cookies, headers } from "next/headers";
import RouteLabShell from "@/components/route-lab-shell";

export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "Dynamic SSR Route",
  description:
    "Per-request SSR inspecting searchParams, headers(), cookies(), and Cloudflare bindings.",
};

interface SsrPageProps {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}

export default async function SsrRoutePage({ searchParams }: SsrPageProps) {
  const [resolvedParams, headerList, cookieStore] = await Promise.all([
    searchParams,
    headers(),
    cookies(),
  ]);

  const cfEnv = (globalThis as { __nextdeployEnv?: Record<string, unknown> })
    .__nextdeployEnv;

  const bindings = {
    ASSETS: Boolean(cfEnv?.ASSETS),
    RATE_LIMIT: Boolean(cfEnv?.RATE_LIMIT),
    AI: Boolean(cfEnv?.AI),
    AUTH_SECRET: Boolean(cfEnv?.AUTH_SECRET),
  };

  const inspectedHeaders = {
    host: headerList.get("host") ?? "(unset)",
    "x-powered-by": headerList.get("x-powered-by") ?? "(not set by proxy yet)",
    "cf-ray": headerList.get("cf-ray") ?? "(local / non-CF)",
    "cf-ipcountry": headerList.get("cf-ipcountry") ?? "(local / non-CF)",
    "user-agent": headerList.get("user-agent") ?? "(unset)",
  };

  const demoCookie = cookieStore.get("nextdeploy_route_demo")?.value ?? "(not set)";

  return (
    <RouteLabShell
      title="Dynamic Server-Side Rendering (SSR)"
      subtitle="Forced dynamic rendering on every request. Awaits Next.js 16 async searchParams, headers(), and cookies(), plus safe presence checks for Cloudflare Worker bindings."
      pattern="/routes/ssr"
      mode="SSR"
    >
      <div className="grid sm:grid-cols-2 gap-4">
        <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-5">
          <h2 className="text-xs font-mono uppercase tracking-wider text-surface-400 mb-3">
            Request Snapshot
          </h2>
          <dl className="space-y-2 text-xs font-mono">
            <div>
              <dt className="text-surface-500">Rendered At</dt>
              <dd className="text-brand-300">{new Date().toISOString()}</dd>
            </div>
            <div>
              <dt className="text-surface-500">Cookie (nextdeploy_route_demo)</dt>
              <dd className="text-surface-200 break-all">{demoCookie}</dd>
            </div>
            <div>
              <dt className="text-surface-500">Query Params</dt>
              <dd className="text-surface-200 break-all">
                {JSON.stringify(resolvedParams)}
              </dd>
            </div>
          </dl>
          <div className="mt-4 pt-3 border-t border-surface-800">
            <Link
              href="/routes/actions"
              className="text-xs font-mono text-brand-400 hover:text-brand-300"
            >
              Set demo cookie via Server Action →
            </Link>
          </div>
        </div>

        <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-5">
          <h2 className="text-xs font-mono uppercase tracking-wider text-surface-400 mb-3">
            Cloudflare Worker Bindings (Presence Only)
          </h2>
          <ul className="space-y-2 text-xs font-mono">
            {Object.entries(bindings).map(([name, present]) => (
              <li
                key={name}
                className="flex items-center justify-between px-3 py-1.5 rounded bg-surface-950 border border-surface-800"
              >
                <span className="text-surface-200">{name}</span>
                <span
                  className={
                    present ? "text-emerald-400 font-semibold" : "text-surface-500"
                  }
                >
                  {present ? "BOUND" : "ABSENT (local)"}
                </span>
              </li>
            ))}
          </ul>
        </div>
      </div>

      <div className="rounded-xl border border-surface-800 bg-surface-900/70 p-5">
        <h2 className="text-xs font-mono uppercase tracking-wider text-surface-400 mb-3">
          Inspected Request Headers
        </h2>
        <pre className="text-xs font-mono text-surface-200 bg-surface-950 border border-surface-800 rounded-lg p-4 overflow-x-auto">
          {JSON.stringify(inspectedHeaders, null, 2)}
        </pre>
      </div>
    </RouteLabShell>
  );
}
