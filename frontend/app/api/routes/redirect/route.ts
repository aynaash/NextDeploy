import { NextRequest, NextResponse } from "next/server";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  return NextResponse.redirect(
    new URL("/api/routes/methods?redirected=1", request.url),
    307,
  );
}
