# `shared/telemetry`

The CLI side of the anonymous "apps shipped" counter. On a successful
`nextdeploy ship`, `RecordShipSuccess` sends one signed, anonymous event
(opt-out via `DO_NOT_TRACK=1`, `NEXTDEPLOY_TELEMETRY=0`, or
`nextdeploy telemetry off`). Events are Ed25519-signed with a key compiled only
into official release binaries, so the server can reject forged posts. The rest
of this file is the **frontend integration guide**.

---

# Ship-counter telemetry — frontend integration guide

Audience: the **nextdeployfrontend** team. This wires the CLI's anonymous
"a project shipped" telemetry into a Cloudflare **D1** store and surfaces the
total on the hero ("🚀 N apps shipped with NextDeploy").

The CLI side is already built (`shared/telemetry` in the NextDeploy repo). This
doc covers the three pieces you own: the **ingest endpoint**, the **D1 store**,
and the **hero read**.

---

## 1. What the CLI sends

On a successful `nextdeploy ship`, the CLI POSTs JSON to
`https://nextdeploy.org/api/telemetry` (override: `NEXTDEPLOY_TELEMETRY_URL`):

```jsonc
{
  "id":      "b3f1...e9",   // random per-install UUID — NOT identifying
  "event":   "ship.success",
  "target":  "cloudflare",  // vps | aws | cloudflare | other
  "version": "v0.12.2",
  "os":      "linux",
  "arch":    "amd64",
  "ts":      1718467200,     // unix seconds
  "nonce":   "9a4c...77"     // random per-event — replay/dedup key
}
```

Header on signed builds:
```
X-NextDeploy-Signature: ed25519=<base64 signature>
```

It carries **no** project name, domain, repo, paths, env, or secrets — keep it
that way; never log the source IP against the payload.

## 2. Anti-spoofing — this is the important part

The whole point is that **a random `curl` cannot inflate the counter**. The CLI
signs each event with an **Ed25519 private key compiled into official release
binaries only**. You verify with the matching **public key** (safe to commit).

**Canonical message** — sign/verify exactly these 8 fields joined by `|`, in
this order (must byte-match the Go client `canonical()`):

```
id|event|target|version|os|arch|ts|nonce
```

> ⚠️ Honest threat model: the signing key lives inside a public, downloadable
> binary, so a determined attacker could extract it via reverse engineering.
> This is **not** unbreakable — no open-source self-reporting client can be. It
> *does* stop scripted/casual spoofing, and combined with the server checks
> below (freshness + nonce dedup + rate limit + shape) it makes inflating a
> vanity counter not worth the effort. If you ever need hard guarantees, the
> only real path is server-side verification that the deployment exists — which
> trades away anonymity; treat that as a separate opt-in "verified deploy".

### Generate the keypair (one time)

```bash
# Private key seed (32 bytes) → base64. Keep secret.
openssl genpkey -algorithm ed25519 -outform DER \
  | tail -c 32 | base64    # → TELEMETRY_SIGNING_KEY

# Public key → base64 (commit this).
# Easiest is a tiny node script (matches what the verifier uses):
node -e '
  const { generateKeyPairSync } = require("crypto");
  const { publicKey, privateKey } = generateKeyPairSync("ed25519");
  const pub = publicKey.export({type:"spki",format:"der"}).subarray(-32);
  const seed = privateKey.export({type:"pkcs8",format:"der"}).subarray(-32);
  console.log("PRIVATE (TELEMETRY_SIGNING_KEY):", seed.toString("base64"));
  console.log("PUBLIC  (commit in frontend):   ", pub.toString("base64"));
'
```

- **Private** → add as repo secret **`TELEMETRY_SIGNING_KEY`** in the
  **NextDeploy** repo (Settings → Secrets → Actions). The release workflow
  already passes it to GoReleaser; the next release then ships signed binaries.
- **Public** → put in the frontend as `TELEMETRY_PUBLIC_KEY` (env or constant).

Until the secret exists, release builds are **unsigned** — the endpoint should
reject unsigned events (or bucket them as `unverified`, uncounted).

## 3. D1 schema

`wrangler.toml`:
```toml
[[d1_databases]]
binding = "TELEMETRY_DB"
database_name = "nextdeploy-telemetry"
database_id = "<from: wrangler d1 create nextdeploy-telemetry>"
```

Migration (`wrangler d1 execute nextdeploy-telemetry --file ./schema.sql`):
```sql
CREATE TABLE IF NOT EXISTS ship_events (
  nonce      TEXT PRIMARY KEY,          -- per-event → idempotent dedup
  id         TEXT NOT NULL,             -- install id
  target     TEXT NOT NULL,
  version    TEXT NOT NULL,
  os         TEXT,
  arch       TEXT,
  ts         INTEGER NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_ship_events_created ON ship_events(created_at);
-- The hero number is COUNT(*); add a daily rollup later if you want charts.
```

## 4. Ingest endpoint — `app/api/telemetry/route.ts`

Uses `@noble/ed25519` (edge-safe; `npm i @noble/ed25519`). Runs on the Workers
runtime.

```ts
import { verify } from "@noble/ed25519";

export const runtime = "edge";

const PUBLIC_KEY = process.env.TELEMETRY_PUBLIC_KEY!;          // base64, 32 bytes
const TARGETS = new Set(["vps", "aws", "cloudflare", "other"]);
const MAX_SKEW = 10 * 60;                                      // 10 minutes

const b64 = (s: string) => Uint8Array.from(atob(s), c => c.charCodeAt(0));

export async function POST(req: Request) {
  let ev: any;
  try { ev = await req.json(); } catch { return new Response("bad json", { status: 400 }); }

  // 1. Shape validation — reject anything off-spec.
  if (ev?.event !== "ship.success" || !TARGETS.has(ev?.target)
      || typeof ev?.id !== "string" || typeof ev?.nonce !== "string"
      || typeof ev?.ts !== "number" || typeof ev?.version !== "string") {
    return new Response("bad shape", { status: 400 });
  }

  // 2. Freshness — drop stale/replayed-from-the-future events.
  const now = Math.floor(Date.now() / 1000);
  if (Math.abs(now - ev.ts) > MAX_SKEW) return new Response("stale", { status: 400 });

  // 3. Signature — the gate against forged "fake deploy" posts.
  const sig = (req.headers.get("X-NextDeploy-Signature") ?? "").replace(/^ed25519=/, "");
  if (!sig) return new Response("unsigned", { status: 401 });
  const msg = new TextEncoder().encode(
    [ev.id, ev.event, ev.target, ev.version, ev.os, ev.arch, ev.ts, ev.nonce].join("|"),
  );
  const ok = await verify(b64(sig), msg, b64(PUBLIC_KEY)).catch(() => false);
  if (!ok) return new Response("bad signature", { status: 401 });

  // 4. Idempotent insert — nonce PK dedups replays of a valid event.
  const db = (process.env as any).TELEMETRY_DB; // or getRequestContext().env.TELEMETRY_DB
  await db.prepare(
    `INSERT OR IGNORE INTO ship_events (nonce,id,target,version,os,arch,ts,created_at)
     VALUES (?,?,?,?,?,?,?,?)`,
  ).bind(ev.nonce, ev.id, ev.target, ev.version, ev.os ?? "", ev.arch ?? "", ev.ts, now).run();

  return new Response(null, { status: 204 });
}
```

Add a **Cloudflare Rate Limiting rule** on `/api/telemetry` (e.g. 30 req/min/IP)
as the volume backstop — signature verification stops forgery, rate limiting
stops a valid build being looped.

## 5. Stats + hero

`app/api/stats/route.ts` (cache so the hero never hits D1 on every render):
```ts
export const runtime = "edge";
export const revalidate = 60;

export async function GET() {
  const db = (process.env as any).TELEMETRY_DB;
  const row = await db.prepare(`SELECT COUNT(*) AS n FROM ship_events`).first<{ n: number }>();
  return Response.json({ shipped: row?.n ?? 0 },
    { headers: { "Cache-Control": "public, s-maxage=60, stale-while-revalidate=300" } });
}
```

Hero (server component — keep D1 server-side):
```tsx
async function ShipCount() {
  const { shipped } = await fetch("https://nextdeploy.org/api/stats",
    { next: { revalidate: 60 } }).then(r => r.json());
  return <p className="text-sm text-muted-foreground">🚀 {shipped.toLocaleString()} apps shipped with NextDeploy</p>;
}
```

Optional: an animated count-up on the client, seeded with the server value so
it's SSR-safe and degrades without JS.

## 6. Checklist

- [ ] `wrangler d1 create nextdeploy-telemetry` + run the migration
- [ ] Generate the Ed25519 keypair (§2)
- [ ] `TELEMETRY_SIGNING_KEY` (private) → **NextDeploy** repo Actions secret
- [ ] `TELEMETRY_PUBLIC_KEY` (public) → frontend env
- [ ] Ship `/api/telemetry` + `/api/stats` + bind `TELEMETRY_DB`
- [ ] Add the CF rate-limit rule on `/api/telemetry`
- [ ] Drop the count into the hero
- [ ] Cut a NextDeploy release so signed binaries exist, then `nextdeploy ship` once and confirm a row lands + the hero ticks up
