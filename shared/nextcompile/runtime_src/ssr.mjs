// SSR layer — Flight stream → streaming HTML the browser can paint AND hydrate.
//
// This is layer 2 of Next's three-layer App Router pipeline:
//
//   1. RSC        React tree → Flight stream            rsc.mjs + server.edge
//   2. SSR        Flight     → HTML + inline Flight     ← THIS FILE
//   3. Hydration  HTML + self.__next_f + client chunks → live React   (browser)
//
// Layer 3 is deliberately NOT ours. Next already compiled its client runtime
// (react-dom/client, the App Router hydrator, react-server-dom-webpack/client.
// browser) into .next/static/chunks/*, and DeployStatic uploads those to R2.
// Our only job is to emit the two things that runtime expects:
//
//   a) the same ordered <script src> bootstrap list Next would have emitted
//      (from ModuleRef.BootstrapChunks — see scanner.go:bootstrapChunksForRoute)
//   b) the Flight payload framed as self.__next_f pushes
//
// Then Next's own hydrator takes over. The cost is a version coupling to Next's
// HTML contract, accepted deliberately: reimplementing the chunk graph and
// client-reference resolution would be far more code and would break on every
// Next minor.
//
// SCOPE (M1): the Flight stream is fully resolved before HTML rendering starts,
// so the response is not progressively streamed. That is a milestone boundary,
// not an oversight — see renderToHtml's note on the `use()` pattern that
// unlocks Suspense/loading.js streaming (M3).

// --- vendored dependency loading -------------------------------------------

let ssrDeps = null;
let ssrLoadError = null;

/**
 * Lazily import the two vendored builds SSR needs, caching both the success
 * and the failure. Mirrors rsc.mjs's loadVendored: a missing vendor bundle is
 * a degradation, never a throw, so an older bundle that predates SSR
 * vendoring still serves the Flight-only shell instead of 500ing.
 *
 * @returns {Promise<{client: any, dom: any} | null>} null when unavailable.
 */
export async function loadSSRDeps() {
  if (ssrDeps) return ssrDeps;
  if (ssrLoadError) return null;
  try {
    const [client, dom] = await Promise.all([
      import("./vendor/react-server-dom-webpack/client.edge.mjs"),
      import("./vendor/react-dom/server.edge.mjs"),
    ]);
    if (typeof client?.createFromReadableStream !== "function") {
      throw new TypeError(
        "vendored react-server-dom-webpack/client.edge exports no createFromReadableStream",
      );
    }
    if (typeof dom?.renderToReadableStream !== "function") {
      throw new TypeError(
        "vendored react-dom/server.edge exports no renderToReadableStream",
      );
    }
    ssrDeps = { client, dom };
    return ssrDeps;
  } catch (err) {
    ssrLoadError = err;
    return null;
  }
}

/** The error that made loadSSRDeps give up, for diagnostics. */
export function ssrLoadFailure() {
  return ssrLoadError;
}

// --- manifest plumbing ------------------------------------------------------

/**
 * Derive the ssrManifest that client.edge needs from Next's
 * page_client-reference-manifest. Distinct from the `clientModules`
 * bundlerConfig that server.edge consumes: that one maps source paths →
 * client references for ENCODING, this one maps client reference IDs →
 * loadable modules for DECODING on the server.
 *
 * Both halves live in the same manifest file (verified against a real
 * Next 15.0.3 build — see testdata/fixtures/next15-appdir).
 *
 * @param {any} clientManifest the imported JSON module (or its .default)
 */
export function buildSSRManifest(clientManifest) {
  const m = clientManifest?.default || clientManifest || {};
  return {
    // edgeSSRModuleMapping is populated ONLY for the edge runtime; a webpack
    // build (which is what we force for Cloudflare — see ensureNextBuild)
    // emits it as {} and puts everything in ssrModuleMapping. Preferring
    // "edge" unconditionally therefore hands React an empty moduleMap, which
    // resolves every "use client" boundary to nothing and throws mid-render.
    // Pick the first NON-EMPTY mapping instead of the first defined one.
    moduleMap: firstNonEmpty(m.edgeSSRModuleMapping, m.ssrModuleMapping) || {},
    moduleLoading: m.moduleLoading ?? null,
  };
}

function firstNonEmpty(...maps) {
  for (const m of maps) {
    if (m && typeof m === "object" && Object.keys(m).length > 0) return m;
  }
  return null;
}

/**
 * Turn dist-relative bootstrap chunk paths into browser-fetchable URLs.
 *
 * BootstrapChunks holds paths relative to distDir, exactly as
 * app-build-manifest.json lists them ("static/chunks/webpack-<hash>.js").
 * The public URL is <assetPrefix|basePath>/_next/<chunk>, which is the same
 * key space serve.mjs reads out of R2.
 *
 * @param {string[]} chunks
 * @param {{assetPrefix?: string, basePath?: string}} opts
 * @returns {string[]}
 */
export function bootstrapScriptURLs(chunks, opts = {}) {
  if (!Array.isArray(chunks) || chunks.length === 0) return [];
  const { assetPrefix = "", basePath = "" } = opts;
  // assetPrefix wins when both are set — it exists precisely to point assets
  // at a different origin than the app's own path prefix.
  const prefix = String(assetPrefix || basePath || "").replace(/\/+$/, "");
  return chunks
    .filter((c) => typeof c === "string" && c.length > 0)
    .map((c) => `${prefix}/_next/${c.replace(/^\/+/, "")}`);
}

/**
 * Build the <link rel="stylesheet"> tags for a route, in cascade order.
 *
 * Necessary because rsc.mjs composes layouts by hand instead of going through
 * Next's app-render, so Next's own stylesheet injection never executes. Without
 * these the page hydrates perfectly and looks completely unstyled.
 *
 * @param {string[]} cssChunks dist-relative paths from entryCSSFiles
 * @param {{assetPrefix?: string, basePath?: string, nonce?: string}} opts
 * @returns {string} concatenated tags, or "" when there is no CSS
 */
export function stylesheetLinkTags(cssChunks, opts = {}) {
  const urls = bootstrapScriptURLs(cssChunks, opts);
  if (urls.length === 0) return "";
  const nonceAttr = opts.nonce ? ` nonce="${opts.nonce}"` : "";
  return urls
    .map((href) => `<link rel="stylesheet" href="${escapeAttr(href)}"${nonceAttr}>`)
    .join("");
}

// Attribute-context escaping for URLs we splice into raw HTML. Chunk names are
// build-generated, but assetPrefix is user config and reaches this unvalidated.
function escapeAttr(s) {
  return String(s)
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

/**
 * Splice markup in immediately after the opening <head> tag of a byte stream.
 *
 * React offers no "bootstrapStyles" hook, and building the tags as React
 * elements would mean importing `react` into this module — pulling a second
 * React resolution into a bundle that already carries the react-server one.
 * Rewriting the byte stream keeps that conflict out of M1/M2 entirely.
 *
 * Buffers only until <head> is found (or the budget is blown), then passes
 * everything through untouched.
 */
export function createHeadInjector(markup, budgetBytes = 65536) {
  const encoder = new TextEncoder();
  const decoder = new TextDecoder("utf-8");
  let done = !markup;
  let buffered = [];
  let bufferedLen = 0;

  const concat = () => {
    const all = new Uint8Array(bufferedLen);
    let at = 0;
    for (const b of buffered) {
      all.set(b, at);
      at += b.length;
    }
    return all;
  };

  return {
    /** @returns {Uint8Array[]} zero or more chunks to emit */
    push(chunk) {
      if (done) return [chunk];

      buffered.push(chunk);
      bufferedLen += chunk.length;
      const all = concat();
      const text = decoder.decode(all, { stream: true });

      // Match <head>, <head ...>, but never <header>.
      const m = /<head(\s[^>]*)?>/i.exec(text);
      if (m) {
        done = true;
        buffered = [];
        const at = m.index + m[0].length;
        const out = encoder.encode(text.slice(0, at) + markup + text.slice(at));
        return [out];
      }
      if (bufferedLen > budgetBytes) {
        // No <head> in a document this far in — a fragment render, or a shape
        // we don't recognize. Give up and stream verbatim rather than risk
        // corrupting the output.
        done = true;
        buffered = [];
        return [all];
      }
      return [];
    },
    /** Flush whatever is still buffered when the source ends. */
    flush() {
      if (bufferedLen === 0) return [];
      const all = concat();
      buffered = [];
      bufferedLen = 0;
      return [all];
    },
  };
}

// --- Flight inlining --------------------------------------------------------

// Escape the characters that could terminate the enclosing <script> element or
// be reinterpreted as JS line terminators. Without this, page data containing
// "</script>" breaks out of the tag — an XSS vector, not just a rendering bug.
// Values match Next's htmlEscapeJsonString so the payload stays byte-compatible.
const ESCAPE_LOOKUP = {
  "&": "\\u0026",
  ">": "\\u003e",
  "<": "\\u003c",
  ["\u2028"]: "\\u2028",
  ["\u2029"]: "\\u2029",
};
const ESCAPE_REGEX = /[&><\u2028\u2029]/g;

export function htmlEscapeJsonString(str) {
  return str.replace(ESCAPE_REGEX, (m) => ESCAPE_LOOKUP[m]);
}

// The bootstrap marker Next pushes before any data chunk. Emitted via
// bootstrapScriptContent so React places it with the bootstrap scripts.
export const FLIGHT_INIT = "(self.__next_f=self.__next_f||[]).push([0])";

/**
 * Frame one Flight chunk as the <script> tag Next's client runtime consumes.
 *
 * Note the self-initializing `(self.__next_f=self.__next_f||[])` on EVERY
 * chunk, where Next only uses it for the first. Our flight scripts are
 * interleaved into React's HTML stream and can land before React flushes
 * bootstrapScriptContent; a bare `self.__next_f.push(...)` would then throw
 * "Cannot read properties of undefined". The idempotent form costs ~30 bytes
 * a chunk and removes the ordering hazard outright.
 */
export function flightChunkScript(text, nonce) {
  const attr = nonce ? ` nonce="${nonce}"` : "";
  return `<script${attr}>(self.__next_f=self.__next_f||[]).push([1,${htmlEscapeJsonString(
    JSON.stringify(text),
  )}])</script>`;
}

/**
 * Merge React's HTML stream with the Flight payload, emitting each Flight
 * chunk as a <script> tag once the HTML has begun.
 *
 * The gate matters: injecting a <script> before React's first flush would put
 * it ahead of <!DOCTYPE html>, which throws the document into quirks mode.
 *
 * @param {ReadableStream<Uint8Array>} htmlStream
 * @param {ReadableStream<Uint8Array>} flightStream
 * @param {{nonce?: string, headMarkup?: string}} opts
 * @returns {ReadableStream<Uint8Array>}
 */
export function composeHtmlWithFlight(htmlStream, flightStream, opts = {}) {
  const { nonce, headMarkup } = opts;
  const headInjector = createHeadInjector(headMarkup);
  const encoder = new TextEncoder();
  // Non-fatal decoding: a Flight chunk can split a multi-byte character, so
  // decode with stream:true and flush the remainder at the end.
  const decoder = new TextDecoder("utf-8");

  return new ReadableStream({
    async start(controller) {
      let done = false;
      const emit = (bytes) => {
        if (!done) controller.enqueue(bytes);
      };

      let markHtmlStarted;
      const htmlStarted = new Promise((resolve) => {
        markHtmlStarted = resolve;
      });

      const pumpHtml = (async () => {
        const reader = htmlStream.getReader();
        try {
          for (;;) {
            const { value, done: fin } = await reader.read();
            if (fin) break;
            // The injector may withhold early chunks while it hunts for
            // <head>; only unblock the flight pump once bytes actually land,
            // so no <script> can precede the doctype.
            for (const out of headInjector.push(value)) {
              emit(out);
              markHtmlStarted();
            }
          }
          for (const out of headInjector.flush()) {
            emit(out);
            markHtmlStarted();
          }
        } finally {
          // Unblock the flight pump even if the HTML stream produced nothing,
          // otherwise an empty render would hang the response forever.
          markHtmlStarted();
          reader.releaseLock();
        }
      })();

      const pumpFlight = (async () => {
        await htmlStarted;
        const reader = flightStream.getReader();
        try {
          for (;;) {
            const { value, done: fin } = await reader.read();
            if (fin) break;
            const text = decoder.decode(value, { stream: true });
            if (text) emit(encoder.encode(flightChunkScript(text, nonce)));
          }
          const tail = decoder.decode();
          if (tail) emit(encoder.encode(flightChunkScript(tail, nonce)));
        } finally {
          reader.releaseLock();
        }
      })();

      try {
        await Promise.all([pumpHtml, pumpFlight]);
      } catch (err) {
        done = true;
        controller.error(err);
        return;
      }
      done = true;
      controller.close();
    },
  });
}

// --- the SSR entry point ----------------------------------------------------

/**
 * Render a Flight stream to an HTML stream carrying its own hydration data.
 *
 * @param {ReadableStream<Uint8Array>} flightStream  from server.edge
 * @param {{
 *   scripts?: string[],       ordered bootstrap script URLs
 *   headMarkup?: string,      spliced in after <head> (stylesheet links)
 *   ssrManifest?: object,     from buildSSRManifest
 *   nonce?: string,
 *   onError?: (err: any) => void,
 * }} bootstrap
 * @param {object} [reqCtx]  request context; supplies an abort signal if present
 * @returns {Promise<ReadableStream<Uint8Array> | null>} null when the vendored
 *   SSR builds are unavailable, so the caller can degrade to the Flight shell.
 */
export async function renderToHtml(flightStream, bootstrap = {}, reqCtx = undefined) {
  const deps = await loadSSRDeps();
  if (!deps) return null;

  const onError =
    typeof bootstrap.onError === "function"
      ? bootstrap.onError
      : (err) => console.error("[nextcompile ssr]", err?.stack || String(err));

  // One Flight stream, two consumers: SSR decodes it into a React tree, and
  // the browser needs the identical bytes to hydrate against. tee() is what
  // lets both read it without re-rendering the RSC pass.
  const [forSSR, forInline] = flightStream.tee();

  // M1 resolves the tree fully before rendering. The streaming form wraps the
  // pending root in a component that calls React's use() hook so Suspense
  // boundaries flush progressively — that needs `react` itself vendored into
  // the SSR (non-react-server) condition, which is M3's scope. Awaiting here
  // is correct-but-unstreamed, and keeps M1 free of a second React vendoring.
  const element = await deps.client.createFromReadableStream(forSSR, {
    ssrManifest: bootstrap.ssrManifest || { moduleMap: {}, moduleLoading: null },
    nonce: bootstrap.nonce,
  });

  const htmlStream = await deps.dom.renderToReadableStream(element, {
    // React emits these as <script src async> in the shell, in array order.
    // Order is load-bearing: polyfills → webpack → framework → main-app →
    // page chunks, exactly as app-build-manifest.json lists them. A wrong
    // order surfaces as ChunkLoadError, not a silent degradation.
    bootstrapScripts: bootstrap.scripts && bootstrap.scripts.length ? bootstrap.scripts : undefined,
    bootstrapScriptContent: FLIGHT_INIT,
    nonce: bootstrap.nonce,
    signal: reqCtx?.abortSignal,
    onError,
  });

  return composeHtmlWithFlight(htmlStream, forInline, {
    nonce: bootstrap.nonce,
    headMarkup: bootstrap.headMarkup,
  });
}
