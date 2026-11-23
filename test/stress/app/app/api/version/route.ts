import os from "os";

export const dynamic = "force-dynamic";

export async function GET() {
  return Response.json({
    version: process.env.APP_RELEASE_VERSION || "v1.0.0",
    hostname: os.hostname(),
    pid: process.pid,
    timestamp: Date.now(),
  });
}
