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

function bytesToBase64(bytes) {
  if (typeof Buffer !== "undefined") {
    return Buffer.from(bytes.buffer, bytes.byteOffset, bytes.byteLength).toString("base64");
  }
  let bin = "";
  for (let i = 0; i < bytes.length; i++) {
    bin += String.fromCharCode(bytes[i]);
  }
  return btoa(bin);
}

function base64ToBuffer(b64) {
  if (typeof Buffer !== "undefined") {
    return Buffer.from(b64, "base64");
  }
  const bin = atob(b64);
  const out = new Uint8Array(bin.length);
  for (let i = 0; i < bin.length; i++) {
    out[i] = bin.charCodeAt(i);
  }
  return out;
}

function serializeBinary(value) {
  if (typeof Buffer !== "undefined" && Buffer.isBuffer(value)) {
    return { __ndType: "Buffer", base64: bytesToBase64(value) };
  }
  if (value instanceof Uint8Array) {
    return { __ndType: "Buffer", base64: bytesToBase64(value) };
  }
  return value;
}

function reviveBinary(value) {
  if (typeof Buffer !== "undefined" && Buffer.isBuffer(value)) {
    return value;
  }
  if (value instanceof Uint8Array) {
    return typeof Buffer !== "undefined"
      ? Buffer.from(value.buffer, value.byteOffset, value.byteLength)
      : value;
  }
  if (!value || typeof value !== "object") {
    return value;
  }
  if (value.__ndType === "Buffer" && typeof value.base64 === "string") {
    return base64ToBuffer(value.base64);
  }
  if (value.type === "Buffer" && Array.isArray(value.data)) {
    return typeof Buffer !== "undefined" ? Buffer.from(value.data) : new Uint8Array(value.data);
  }
  const keys = Object.keys(value);
  if (keys.length > 0 && keys.every((k, idx) => k === String(idx))) {
    const arr = new Uint8Array(keys.length);
    for (let i = 0; i < keys.length; i++) {
      arr[i] = value[keys[i]];
    }
    return typeof Buffer !== "undefined" ? Buffer.from(arr.buffer, arr.byteOffset, arr.byteLength) : arr;
  }
  return value;
}

function serializeCacheValue(value) {
  if (!value || typeof value !== "object") return value;
  const out = { ...value };
  if (out.rscData != null) {
    out.rscData = serializeBinary(out.rscData);
  }
  if (out.segmentData instanceof Map) {
    out.segmentData = {
      __ndType: "Map",
      entries: Array.from(out.segmentData.entries(), ([k, v]) => [k, serializeBinary(v)]),
    };
  }
  return out;
}

function reviveCacheValue(value) {
  if (!value || typeof value !== "object") return value;
  const out = { ...value };
  if (out.rscData != null) {
    out.rscData = reviveBinary(out.rscData);
  }
  if (out.segmentData instanceof Map) {
    const revived = new Map();
    for (const [k, v] of out.segmentData.entries()) {
      revived.set(k, reviveBinary(v));
    }
    out.segmentData = revived;
  } else if (out.segmentData && out.segmentData.__ndType === "Map" && Array.isArray(out.segmentData.entries)) {
    const revived = new Map();
    for (const [k, v] of out.segmentData.entries) {
      revived.set(k, reviveBinary(v));
    }
    out.segmentData = revived;
  } else if (out.segmentData !== undefined) {
    // Guard against stale legacy R2 JSON entries where Map serialized to `{}`.
    out.segmentData = undefined;
  }
  return out;
}

function readRouteMetaFromVFS(clean) {
  const files = globalThis.__nextdeployVFS?.files;
  if (!files) return null;
  const exactKey = `.next/server/app/${clean}.meta`;
  let raw = Object.prototype.hasOwnProperty.call(files, exactKey) ? files[exactKey] : undefined;
  if (raw === undefined) {
    const suffix = `/server/app/${clean}.meta`;
    for (const k of Object.keys(files)) {
      if (k.endsWith(suffix)) {
        raw = files[k];
        break;
      }
    }
  }
  if (typeof raw !== "string" || !raw) return null;
  try {
    return JSON.parse(raw);
  } catch {
    return null;
  }
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
            parsed.value = reviveCacheValue(parsed.value);
            l1Cache.set(key, parsed);
            return { value: parsed.value, lastModified: parsed.lastModified };
          }
        }
      }
    } catch {
      // Fall through to build-time prerendered R2 lookup.
    }

    // Fallback for build-time pre-rendered SSG/ISR routes stored in R2
    // (_nextdeploy/releases/<assetVersion>/<route>.html, optional .rsc, and
    // Next.js 16 .segments/**/*.segment.rsc).
    // Serves HTML, client-side RSC Flight navigations, and segment prefetches
    // without hitting NoFallbackError or Missing SegmentData errors.
    if (ctx?.kind !== "FETCH" && typeof key === "string" && key.startsWith("/")) {
      try {
        const clean = key === "/" ? "index" : key.replace(/^\/+/, "");
        const htmlObj = await env.ASSETS.get(releaseAssetKey(`${clean}.html`, this.assetVersion));
        if (htmlObj) {
          const meta = readRouteMetaFromVFS(clean);
          const segmentPaths = Array.isArray(meta?.segmentPaths) ? meta.segmentPaths : [];
          const [html, rscObj, ...segObjs] = await Promise.all([
            readR2Text(htmlObj),
            env.ASSETS.get(releaseAssetKey(`${clean}.rsc`, this.assetVersion)).catch(() => null),
            ...segmentPaths.map((segPath) => {
              const relSeg = String(segPath).replace(/^\/+/, "");
              return env.ASSETS.get(
                releaseAssetKey(`${clean}.segments/${relSeg}.segment.rsc`, this.assetVersion),
              ).catch(() => null);
            }),
          ]);
          const rscData = rscObj ? await readR2Buffer(rscObj) : undefined;
          let segmentData;
          if (segmentPaths.length > 0) {
            segmentData = new Map();
            for (let i = 0; i < segmentPaths.length; i++) {
              const segObj = segObjs[i];
              if (segObj) {
                const segBuf = await readR2Buffer(segObj);
                if (segBuf !== undefined) {
                  segmentData.set(segmentPaths[i], segBuf);
                }
              }
            }
          }
          const lastModified =
            Number(htmlObj.customMetadata?.revalidatedAt) ||
            (htmlObj.uploaded instanceof Date ? htmlObj.uploaded.getTime() : Date.now());
          const entry = {
            value: {
              kind: "APP_PAGE",
              html,
              rscData,
              headers: meta && typeof meta.headers === "object" && meta.headers ? meta.headers : {},
              postponed: meta?.postponed ?? undefined,
              status: key === "/_not-found" ? 404 : (meta?.status ?? undefined),
              segmentData,
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
    const serializedEntry = {
      value: serializeCacheValue(data),
      lastModified: entry.lastModified,
      tags,
    };
    const writePromise = env.ASSETS.put(
      cacheObjectKey(key, this.assetVersion),
      JSON.stringify(serializedEntry),
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
