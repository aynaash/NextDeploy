import { headers } from "next/headers";
import { NextResponse } from "next/server";
import buildMetadata from "@/lib/build-metadata.json";

export const dynamic = "force-dynamic";

export async function GET() {
  const headerList = await headers();
  const cfEnv = (globalThis as { __nextdeployEnv?: Record<string, unknown> })
    .__nextdeployEnv;

  const isWorkersRuntime = typeof cfEnv !== "undefined";

  return NextResponse.json(
    {
      ...buildMetadata,
      live_edge: {
        served_at: new Date().toISOString(),
        runtime: isWorkersRuntime ? "cloudflare-workers" : "node-local",
        bindings: {
          ASSETS: Boolean(cfEnv?.ASSETS),
          RATE_LIMIT: Boolean(cfEnv?.RATE_LIMIT),
          AI: Boolean(cfEnv?.AI),
          AUTH_SECRET: Boolean(cfEnv?.AUTH_SECRET),
        },
        request_headers: {
          host: headerList.get("host") ?? null,
          cf_ray: headerList.get("cf-ray") ?? null,
          cf_ipcountry: headerList.get("cf-ipcountry") ?? null,
          cf_colo: headerList.get("cf-colo") ?? null,
          x_powered_by: headerList.get("x-powered-by") ?? null,
        },
      },
    },
    {
      status: 200,
      headers: {
        "Cache-Control": "no-store",
        "x-nextdeploy-metadata-source": "build-snapshot+live-edge",
      },
    },
  );
}
