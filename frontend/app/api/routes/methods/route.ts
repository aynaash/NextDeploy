import { NextRequest, NextResponse } from "next/server";

export const dynamic = "force-dynamic";

async function handleMethod(request: NextRequest, method: string) {
  const query = Object.fromEntries(request.nextUrl.searchParams.entries());
  let body: unknown = null;

  if (method !== "GET" && method !== "HEAD" && method !== "OPTIONS") {
    try {
      const text = await request.text();
      if (text) {
        try {
          body = JSON.parse(text);
        } catch {
          body = text;
        }
      }
    } catch {
      body = null;
    }
  }

  return NextResponse.json({
    ok: true,
    method,
    pathname: request.nextUrl.pathname,
    query,
    body,
    timestamp: new Date().toISOString(),
  });
}

export async function GET(request: NextRequest) {
  return handleMethod(request, "GET");
}

export async function POST(request: NextRequest) {
  return handleMethod(request, "POST");
}

export async function PUT(request: NextRequest) {
  return handleMethod(request, "PUT");
}

export async function PATCH(request: NextRequest) {
  return handleMethod(request, "PATCH");
}

export async function DELETE(request: NextRequest) {
  return handleMethod(request, "DELETE");
}

export async function HEAD() {
  return new NextResponse(null, {
    status: 200,
    headers: { "x-nextdeploy-head": "ok" },
  });
}

export async function OPTIONS() {
  return new NextResponse(null, {
    status: 204,
    headers: {
      Allow: "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS",
    },
  });
}
