// Edge verification for the Next.js ecosystem canary matrix.
//
// Loads the *bundled* Worker (`.nextdeploy-cf/worker.mjs`, the esbuild output —
// NOT the pre-bundle `worker_entry.mjs`, whose bare `next/*` aliases only
// resolve through esbuild) into Miniflare and fires real requests at it. The
// assertions are intentionally limited to behavior NextDeploy actually
// implements, so a failure means genuine drift, not a wrong expectation:
//
//   - O(1) route dispatch renders the home route as streamed HTML
//   - the edge guard's IP denylist returns 403 (the guard does IP allow/deny +
//     KV rate-limit + HMAC auth — there is NO content/SQL WAF at the edge; that
//     lives in Coraza on the Caddy/VPS path)
//   - Server Actions (if the fixture has any) resolve through the action loader
//
// Paths and binding names are passed in by the workflow via env so this file
// stays decoupled from the fixture layout.
import { afterAll, beforeAll, describe, expect, test } from "vitest";
import { Miniflare } from "miniflare";
import { existsSync, readFileSync } from "node:fs";
import { join } from "node:path";

const WORKER = process.env.WORKER_BUNDLE_PATH;
const FIXTURE = process.env.CANARY_FIXTURE_DIR ?? process.cwd();
// IP seeded into the deny list of the generated nextdeploy.yml.
const DENIED_IP = process.env.CANARY_DENIED_IP ?? "192.0.2.10";

describe("NextDeploy Cloudflare compilation verification", () => {
  let mf: Miniflare;

  beforeAll(async () => {
    if (!WORKER || !existsSync(WORKER)) {
      throw new Error(
        `worker bundle not found at WORKER_BUNDLE_PATH=${WORKER}; did the compile step run?`,
      );
    }
    mf = new Miniflare({
      modules: true,
      scriptPath: WORKER,
      compatibilityDate: "2025-04-01",
      compatibilityFlags: ["nodejs_compat_v2"],
      // The generator emits an R2 binding for static assets and a KV namespace
      // for the guard's rate limiter / sessions. Names must match the bindings
      // block of the generated nextdeploy.yml — keep these in sync with the
      // workflow's config-generation step.
      r2Buckets: ["ASSETS"],
      kvNamespaces: ["SECURITY_KV"],
      bindings: { AUTH_SECRET: "canary-test-secret" },
    });
    // Surface module-init crashes (a common drift symptom) eagerly.
    await mf.ready;
  });

  afterAll(async () => {
    await mf?.dispose();
  });

  test("O(1) route dispatcher serves the home route as HTML", async () => {
    const res = await mf.dispatchFetch("http://localhost/");
    expect(res.status).toBe(200);
    expect(res.headers.get("content-type") ?? "").toContain("text/html");
    const body = await res.text();
    expect(body.toLowerCase()).toContain("<!doctype html>");
  });

  test("edge guard denylist returns 403 for a blocked IP", async () => {
    const res = await mf.dispatchFetch("http://localhost/", {
      headers: { "CF-Connecting-IP": DENIED_IP },
    });
    expect(res.status).toBe(403);
  });

  test("Server Action loader resolves a real action id", async () => {
    // Read an actual id from the generated manifest instead of a placeholder —
    // a fabricated id can't distinguish "loader works" from "404 for unknown".
    const manifestPath = join(
      FIXTURE,
      ".next",
      "standalone",
      ".nextdeploy-cf",
      "_nextdeploy",
      "action_manifest.json",
    );
    if (!existsSync(manifestPath)) {
      // Default create-next-app fixtures ship no Server Actions; nothing to
      // assert. Logged as skipped rather than passing silently.
      console.warn("no action_manifest.json; skipping Server Action drift check");
      return;
    }
    const manifest = JSON.parse(readFileSync(manifestPath, "utf8"));
    const actionId = Object.keys(manifest?.actions ?? manifest ?? {})[0];
    if (!actionId) {
      console.warn("action manifest empty; skipping Server Action drift check");
      return;
    }
    const res = await mf.dispatchFetch("http://localhost/", {
      method: "POST",
      headers: {
        "Next-Action": actionId,
        "content-type": "text/plain;charset=UTF-8",
      },
      body: "[]",
    });
    // Drift in the action serialization contract surfaces as a 500; any handled
    // status (200 action result, 303 redirect, 4xx validation) proves the
    // loader matched and dispatched.
    expect(res.status).not.toBe(500);
  });
});
