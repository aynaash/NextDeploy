import { NextResponse } from "next/server";

export const dynamic = "force-dynamic";

export async function GET() {
  return NextResponse.json({
    status: "healthy",
    timestamp: new Date().toISOString(),
    app: "__APP_NAME__",
    platform: "__PLATFORM_NAME__",
    framework: "Next.js 15 (React 19)",
    uptime: process.uptime ? Math.floor(process.uptime()) : 0,
  });
}
