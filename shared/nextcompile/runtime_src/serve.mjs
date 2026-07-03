// Static + SSG asset serving from R2.
//
// Conventions upheld by the nextdeploy packager when uploading to R2:
//   - /_next/static/<hash>/...   →  keyed under "_next/static/<hash>/..."
//   - /public/foo.png            →  keyed under "foo.png"  (Next serves
//                                   /public content at the root)
//   - SSG/ISR HTML               →  keyed under the file path from the
//                                   manifest (e.g. "index.html",
//                                   "blog.html", "news.html")
//
// env.ASSETS is the R2 binding for the public bundle; it's declared
// automatically by cloudflare_bindings.go when compute routes exist.

/**
 * Serve a /_next/static or /public path from R2. Returns null on miss so
 * the dispatcher can fall through to a 404 or compute route.
 */
export async function serveStaticFromR2(env, pathname) {
  if (!env.ASSETS) return null;

  const key = r2KeyForStatic(pathname);
  if (!key) return null;

  const obj = await env.ASSETS.get(key);
  if (!obj) return null;

  const headers = new Headers();
  obj.writeHttpMetadata?.(headers);
  if (obj.httpEtag) headers.set("etag", obj.httpEtag);
  if (!headers.has("cache-control")) {
    headers.set(
      "cache-control",
      pathname.startsWith("/_next/static/")
        ? "public, max-age=31536000, immutable"
        : "public, max-age=300",
    );
  }
  return new Response(obj.body, { headers });
}

/**
 * Serve a public/* file requested at the bare root path (e.g.
 * /install.sh, /robots.txt, /favicon.ico). Next serves public/ at the
 * root, and the packager uploads those files to R2 keyed by their
 * basename — so the request path minus the leading slash is the R2
 * key. Restricted to paths whose final segment contains a "." to keep
 * real 404s from spending an R2 GET each.
 */
export async function serveRootPublicFromR2(env, pathname, manifest) {
  if (!env.ASSETS) return null;
  if (pathname.length < 2 || pathname.endsWith("/")) return null;
  const last = pathname.slice(pathname.lastIndexOf("/") + 1);
  if (!last.includes(".")) return null;

  const key = safeDecode(pathname.slice(1));
  // Never serve prerendered PAGE HTML here — those keys are the SSG/ISR route
  // targets, reachable only through the guarded dispatch path. Serving them at
  // their bare ".html" key (e.g. /dashboard.html) would bypass the edge guard,
  // which matched the request path "/dashboard.html", not the protected route
  // "/dashboard".
  if (isPrerenderedPageKey(key, manifest)) return null;

  const obj = await env.ASSETS.get(key);
  if (!obj) return null;

  const headers = new Headers();
  obj.writeHttpMetadata?.(headers);
  if (obj.httpEtag) headers.set("etag", obj.httpEtag);
  if (!headers.has("cache-control")) {
    headers.set("cache-control", "public, max-age=300");
  }
  return new Response(obj.body, { headers });
}

/**
 * isPrerenderedPageKey reports whether an R2 key is a prerendered SSG/ISR page
 * target (as opposed to a genuine public/ file). Used to keep page HTML out of
 * the unguarded root-public serving path — see serveRootPublicFromR2.
 */
function isPrerenderedPageKey(key, manifest) {
  const routes = manifest?.routes;
  if (!routes) return false;
  for (const bucket of ["ssg", "isr"]) {
    const table = routes[bucket];
    if (!table) continue;
    for (const target of Object.values(table)) {
      if (typeof target === "string" && target.replace(/^\/+/, "") === key) {
        return true;
      }
    }
  }
  return false;
}

/**
 * Serve a pre-rendered HTML file (SSG or ISR) from R2. The dispatcher
 * resolves the R2 key from manifest.routes.ssg/isr and passes it here.
 */
export async function serveSSGFromR2(env, htmlKey) {
  if (!env.ASSETS) return null;
  // Manifest entries are typically "/foo.html" — normalize to the bare key.
  const key = htmlKey.replace(/^\/+/, "");
  const obj = await env.ASSETS.get(key);
  if (!obj) return null;

  const headers = new Headers();
  obj.writeHttpMetadata?.(headers);
  if (!headers.has("content-type")) {
    headers.set("content-type", "text/html; charset=utf-8");
  }
  if (!headers.has("cache-control")) {
    headers.set("cache-control", "public, max-age=0, must-revalidate");
  }
  return new Response(obj.body, { headers });
}

function r2KeyForStatic(pathname) {
  let key;
  if (pathname.startsWith("/_next/static/")) {
    key = pathname.slice(1); // drop leading slash
  } else if (pathname.startsWith("/public/")) {
    key = pathname.slice("/public/".length);
  } else {
    // Next also serves public/* at root — these only reach here via a
    // caller that hinted "this is a public asset". Keep the contract tight:
    // return null so we don't accidentally 200 on non-static paths.
    return null;
  }
  // R2 keys are stored decoded (literal "[", "]", spaces, …), but browsers
  // request route-group / dynamic-segment chunk paths URL-encoded
  // (".../[...slug]/page.js" → ".../%5B...slug%5D/page.js"). Decode so the GET
  // matches the stored key — otherwise the chunk 404s and the page throws a
  // ChunkLoadError ("Application error: a client-side exception").
  return safeDecode(key);
}

// safeDecode URL-decodes a key, falling back to the raw value on a malformed
// escape sequence (never throws — a bad %xx must not 500 the asset path).
function safeDecode(s) {
  try {
    return decodeURIComponent(s);
  } catch {
    return s;
  }
}
