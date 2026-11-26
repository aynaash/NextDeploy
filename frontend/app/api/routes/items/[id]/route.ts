import { NextRequest, NextResponse } from "next/server";

export const dynamic = "force-dynamic";

function inspectBindings() {
  const env = (globalThis as { __nextdeployEnv?: Record<string, unknown> })
    .__nextdeployEnv;
  return {
    ASSETS: Boolean(env?.ASSETS),
    RATE_LIMIT: Boolean(env?.RATE_LIMIT),
    AI: Boolean(env?.AI),
    AUTH_SECRET: Boolean(env?.AUTH_SECRET),
  };
}

export async function GET(
  request: NextRequest,
  context: { params: Promise<{ id: string }> },
) {
  const { id } = await context.params;
  return NextResponse.json({
    ok: true,
    method: "GET",
    id,
    query: Object.fromEntries(request.nextUrl.searchParams.entries()),
    bindings: inspectBindings(),
    timestamp: new Date().toISOString(),
  });
}

export async function POST(
  request: NextRequest,
  context: { params: Promise<{ id: string }> },
) {
  const { id } = await context.params;
  let body: unknown = null;
  try {
    body = await request.json();
  } catch {
    body = null;
  }
  return NextResponse.json({
    ok: true,
    method: "POST",
    id,
    body,
    bindings: inspectBindings(),
    timestamp: new Date().toISOString(),
  });
}
