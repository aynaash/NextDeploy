import crypto from "crypto";
import os from "os";

export const dynamic = "force-dynamic";

export async function GET() {
  const start = process.hrtime.bigint();

  // Simulate heavy CPU computation (crypto key derivation) to stress vCPU
  let hash = "nextdeploy-enterprise-stress-test-payload";
  for (let i = 0; i < 5000; i++) {
    hash = crypto.createHash("sha256").update(hash + i).digest("hex");
  }

  const durationMs = Number(process.hrtime.bigint() - start) / 1_000_000;

  return Response.json({
    status: "ok",
    release: process.env.APP_RELEASE_VERSION || "v1.0.0",
    hostname: os.hostname(),
    pid: process.pid,
    uptime: process.uptime(),
    compute_duration_ms: durationMs.toFixed(2),
    hash_sample: hash.substring(0, 16),
  });
}
