// Cloudflare-backed Next.js CacheHandler for NextNodeServer.
//
// Implements Next.js's official incrementalCache CacheHandler contract:
//   - get(key, ctx)
//   - set(key, data, ctx)
//   - revalidateTag(tags)
//
// Storage tiers:
//   L1: In-isolate Map (instant hits during warm isolate lifetime)
//   L2: R2 (env.ASSETS under `_nextdeploy/cache/<assetVersion>/<key>.json`)
//       with background ctx.waitUntil writes so responses never wait on R2 PUT.
//   Tag index: In-memory Map + optional KV (env.NEXTCOMPILE_CACHE) timestamps
//       under `tag:<name>` so revalidateTag works across Worker isolates.

const l1Cache = new Map();
const localTagRevalidatedAt = new Map();

function getEnv() {
  return globalThis.__nextdeployEnv || {};
}

function getWorkerCtx() {
  return globalThis.__nextdeployWorkerCtx || null;
}

function cacheObjectKey(key, assetVersion) {
  const clean = String(key || "").replace(/^\/+/, "");
  const prefix = assetVersion ? `_nextdeploy/cache/${assetVersion}` : "_nextdeploy/cache";
  return `${prefix}/${clean}.json`;
}

async function isEntryInvalidatedByTags(tags, lastModified, env) {
  if (!Array.isArray(tags) || tags.length === 0) return false;
  const lm = Number(lastModified) || 0;
  for (const tag of tags) {
    const localAt = localTagRevalidatedAt.get(tag) || 0;
    if (localAt > lm) return true;
    if (env?.NEXTCOMPILE_CACHE) {
      try {
        const raw = await env.NEXTCOMPILE_CACHE.get(`tag:${tag}`);
        if (raw && Number(raw) > lm) {
          localTagRevalidatedAt.set(tag, Number(raw));
          return true;
        }
      } catch {
        // Non-fatal
      }
    }
  }
  return false;
}

export class CloudflareCacheHandler {
  constructor(options = {}) {
    this.options = options;
    this.assetVersion = options.assetVersion || globalThis.__nextdeployAssetVersion || "";
  }

  async get(key) {
    const env = getEnv();
    const l1 = l1Cache.get(key);
    if (l1) {
      if (!(await isEntryInvalidatedByTags(l1.tags, l1.lastModified, env))) {
        return { value: l1.value, lastModified: l1.lastModified };
      }
      l1Cache.delete(key);
    }

    if (!env?.ASSETS) return null;
    try {
      const obj = await env.ASSETS.get(cacheObjectKey(key, this.assetVersion));
      if (!obj) return null;
      const parsed = typeof obj.json === "function"
        ? await obj.json()
        : JSON.parse(await new Response(obj.body).text());
      if (!parsed || typeof parsed !== "object") return null;
      if (await isEntryInvalidatedByTags(parsed.tags, parsed.lastModified, env)) {
        return null;
      }
      l1Cache.set(key, parsed);
      return { value: parsed.value, lastModified: parsed.lastModified };
    } catch {
      return null;
    }
  }

  async set(key, data, ctx = {}) {
    const env = getEnv();
    const tags = Array.isArray(ctx?.tags) ? ctx.tags : [];
    const entry = {
      value: data,
      lastModified: Date.now(),
      tags,
    };
    l1Cache.set(key, entry);

    if (!env?.ASSETS) return;
    const writePromise = env.ASSETS.put(
      cacheObjectKey(key, this.assetVersion),
      JSON.stringify(entry),
      {
        httpMetadata: { contentType: "application/json" },
        customMetadata: { revalidatedAt: String(entry.lastModified) },
      },
    ).catch((err) => {
      console.error("[nextcompile cacheHandler R2 put failed]", err);
    });

    const workerCtx = getWorkerCtx();
    if (typeof workerCtx?.waitUntil === "function") {
      workerCtx.waitUntil(writePromise);
    } else {
      await writePromise;
    }
  }

  async revalidateTag(tags) {
    const list = Array.isArray(tags) ? tags : [tags];
    const now = Date.now();
    const env = getEnv();
    for (const tag of list) {
      if (!tag) continue;
      localTagRevalidatedAt.set(tag, now);
      for (const [k, entry] of l1Cache.entries()) {
        if (Array.isArray(entry.tags) && entry.tags.includes(tag)) {
          l1Cache.delete(k);
        }
      }
      if (env?.NEXTCOMPILE_CACHE) {
        const put = env.NEXTCOMPILE_CACHE.put(`tag:${tag}`, String(now), {
          expirationTtl: 7 * 24 * 60 * 60,
        }).catch(() => {});
        const workerCtx = getWorkerCtx();
        if (typeof workerCtx?.waitUntil === "function") {
          workerCtx.waitUntil(put);
        } else {
          await put;
        }
      }
    }
  }
}

export function resetLocalCacheForTests() {
  l1Cache.clear();
  localTagRevalidatedAt.clear();
}

export default CloudflareCacheHandler;
