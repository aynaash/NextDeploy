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

function releaseAssetKey(relPath, assetVersion) {
  const clean = String(relPath || "").replace(/^\/+/, "");
  return assetVersion ? `_nextdeploy/releases/${assetVersion}/${clean}` : clean;
}

function reviveSerializedBuffers(value) {
  if (!value || typeof value !== "object") return value;
  if (
    value.rscData &&
    typeof value.rscData === "object" &&
    value.rscData.type === "Buffer" &&
    Array.isArray(value.rscData.data) &&
    typeof Buffer !== "undefined"
  ) {
    return { ...value, rscData: Buffer.from(value.rscData.data) };
  }
  return value;
}

async function readR2Text(obj) {
  if (!obj) return "";
  if (typeof obj.text === "function") return await obj.text();
  if (typeof obj.body === "string") return obj.body;
  return await new Response(obj.body).text();
}

async function readR2Buffer(obj) {
  if (!obj) return undefined;
  if (typeof obj.arrayBuffer === "function") {
    const ab = await obj.arrayBuffer();
    return typeof Buffer !== "undefined" ? Buffer.from(ab) : new Uint8Array(ab);
  }
  if (typeof obj.body === "string") {
    return typeof Buffer !== "undefined" ? Buffer.from(obj.body, "utf8") : new TextEncoder().encode(obj.body);
  }
  const ab = await new Response(obj.body).arrayBuffer();
  return typeof Buffer !== "undefined" ? Buffer.from(ab) : new Uint8Array(ab);
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

  async get(key, ctx = {}) {
    const env = getEnv();
    const l1 = l1Cache.get(key);
    if (l1) {
      if (!(await isEntryInvalidatedByTags(l1.tags, l1.lastModified, env))) {
        return { value: l1.value, lastModified: l1.lastModified };
      }
      l1Cache.delete(key);
    }

    if (!env?.ASSETS || typeof env.ASSETS.get !== "function") return null;
    try {
      const obj = await env.ASSETS.get(cacheObjectKey(key, this.assetVersion));
      if (obj) {
        const parsed = typeof obj.json === "function"
          ? await obj.json()
          : JSON.parse(await readR2Text(obj));
        if (parsed && typeof parsed === "object") {
          if (!(await isEntryInvalidatedByTags(parsed.tags, parsed.lastModified, env))) {
            parsed.value = reviveSerializedBuffers(parsed.value);
            l1Cache.set(key, parsed);
            return { value: parsed.value, lastModified: parsed.lastModified };
          }
        }
      }
    } catch {
      // Fall through to build-time prerendered R2 lookup.
    }

    // Fallback for build-time pre-rendered SSG/ISR routes stored in R2
    // (_nextdeploy/releases/<assetVersion>/<route>.html and optional .rsc).
    // Serves both HTML and client-side RSC Flight navigations for
    // generateStaticParams routes without hitting NoFallbackError.
    if (ctx?.kind !== "FETCH" && typeof key === "string" && key.startsWith("/")) {
      try {
        const clean = key === "/" ? "index" : key.replace(/^\/+/, "");
        const htmlObj = await env.ASSETS.get(releaseAssetKey(`${clean}.html`, this.assetVersion));
        if (htmlObj) {
          const [html, rscObj] = await Promise.all([
            readR2Text(htmlObj),
            env.ASSETS.get(releaseAssetKey(`${clean}.rsc`, this.assetVersion)).catch(() => null),
          ]);
          const rscData = rscObj ? await readR2Buffer(rscObj) : undefined;
          const lastModified =
            Number(htmlObj.customMetadata?.revalidatedAt) ||
            (htmlObj.uploaded instanceof Date ? htmlObj.uploaded.getTime() : Date.now());
          const entry = {
            value: {
              kind: "APP_PAGE",
              html,
              rscData,
              headers: {},
              postponed: undefined,
              status: key === "/_not-found" ? 404 : undefined,
            },
            lastModified,
            tags: [],
          };
          l1Cache.set(key, entry);
          return { value: entry.value, lastModified: entry.lastModified };
        }
      } catch {
        // Non-fatal
      }
    }

    return null;
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

    if (typeof env?.ASSETS?.put !== "function") return;
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
