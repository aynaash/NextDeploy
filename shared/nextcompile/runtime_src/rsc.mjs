// React Server Components rendering.
//
// Bridge from a compiled Next App Router page to a streaming HTML Response.
// Delegates Flight encoding to the vendored react-server-dom-webpack bundle
// (see runtime/vendor/README.md for the vendoring contract).
//
// Pipeline:
//   1. Check PPR — not yet implemented; return 501 with clear message.
//   2. Load the page module, its layout chain, and its client manifest
//      in parallel.
//   3. Compose layouts root → leaf → page.
//   4. Pass composed tree + clientModules to renderToReadableStream.
//   5. Hand the Flight stream to ssr.mjs, which renders it to real HTML and
//      inlines the same payload as self.__next_f so the browser can hydrate.
//   6. Emit the response with context-propagated headers/cookies.
//
// Scope intentionally narrow:
//   - Layouts are assumed to be plain React components taking { children }.
//     Suspense boundaries and loading.js interleave come with proper
//     streaming protocol work.
//   - Client manifest is passed verbatim as Flight's bundlerConfig.
//     `clientModules` is the shape Next emits; renderToReadableStream
//     consumes it directly.
//   - When the vendored SSR builds are absent (a bundle compiled before SSR
//     vendoring landed), step 5 degrades to the original Flight-only shell:
//     a blank but non-erroring page, same as the previous behaviour.

import { runWithContext, createRequestContext } from "./context.mjs";
import {
  renderToHtml,
  buildSSRManifest,
  bootstrapScriptURLs,
  stylesheetLinkTags,
  ssrLoadFailure,
  htmlEscapeJsonString,
} from "./ssr.mjs";
import { resolveMetadata, renderMetadataTags } from "./metadata.mjs";

let vendoredLoader = null;
let vendoredLoadError = null;

async function loadVendored() {
  if (vendoredLoader) return vendoredLoader;
  if (vendoredLoadError) return null;
  try {
    vendoredLoader = await import("./vendor/react-server-dom-webpack/server.edge.mjs");
    return vendoredLoader;
  } catch (err) {
    vendoredLoadError = err;
    return null;
  }
}

/**
 * Render a Server Component page module to a streaming Response.
 *
 * @param {object} entry       dispatch-table entry with load, loadLayouts, loadClientManifest, ppr
 * @param {Request} request
 * @param {Record<string, any>} env
 * @param {{ waitUntil: (p: Promise<any>) => void }} ctx
 * @param {{ params, searchParams }} routeCtx
 * @param {object} [manifest]  runtime manifest; supplies assetPrefix/basePath
 *                             so hydration script URLs match where the client
 *                             chunks were actually uploaded.
 * @returns {Promise<Response>}
 */
export async function renderRSC(entry, request, env, ctx, routeCtx, manifest) {
  // B7 — PPR marker. Our renderer doesn't implement the static-shell +
  // dynamic-holes protocol; surface a clear 501 so operators know why the
  // deploy won't handle this page yet.
  if (entry.ppr) {
    return pprNotImplemented();
  }

  const url = new URL(request.url);
  const reqCtx = createRequestContext(request, env, url, routeCtx.params, ctx);

  return runWithContext(reqCtx, async () => {
    const vendored = await loadVendored();
    if (!vendored) {
      return vendorMissingResponse();
    }

    // Load module + layouts + client manifest in parallel.
    const loadLayouts = Array.isArray(entry.loadLayouts) ? entry.loadLayouts : [];
    const [pageModule, clientManifest, ...layoutModules] = await Promise.all([
      entry.load(),
      entry.loadClientManifest ? entry.loadClientManifest() : Promise.resolve(null),
      ...loadLayouts.map((fn) => fn()),
    ]);

    const PageComponent = await resolveComponent(pageModule);
    if (typeof PageComponent !== "function") {
      return new Response(describeMissingComponent(pageModule, entry), {
        status: 500,
        headers: { "content-type": "text/plain; charset=utf-8" },
      });
    }

    const rawParams = reqCtx.params || {};
    const params = Promise.resolve(rawParams);
    Object.assign(params, rawParams);

    const rawSearchParams = Object.fromEntries(url.searchParams);
    const searchParams = Promise.resolve(rawSearchParams);
    Object.assign(searchParams, rawSearchParams);

    // Build the rendering tree — layouts wrap the page, root-first.
    const treeProps = {
      params,
      searchParams,
    };
    let tree;
    try {
      let layouts = [];
      if (layoutModules.length > 0) {
        for (const m of layoutModules) {
          const comp = await resolveComponent(m);
          if (typeof comp === "function") layouts.push(comp);
        }
      } else {
        const rootTree = pageModule.tree || pageModule.routeModule?.userland?.loaderTree;
        if (rootTree) {
          layouts = await extractLayoutsFromTree(rootTree);
        }
      }
      tree = await buildLayoutTree(layouts, PageComponent, treeProps);
    } catch (err) {
      const msg = err?.digest || err?.message || String(err);
      if (msg.includes("NEXT_HTTP_ERROR_FALLBACK;404")) {
        return new Response("<!DOCTYPE html><html><head><title>404 Not Found</title></head><body><h1>404 Not Found</h1></body></html>", {
          status: 404,
          headers: { "content-type": "text/html; charset=utf-8" },
        });
      }
      if (typeof msg === "string" && msg.startsWith("NEXT_REDIRECT")) {
        const parts = msg.split(";");
        const redirectUrl = parts[2] || "/";
        const redirectStatus = parseInt(parts[3], 10) || 307;
        return Response.redirect(new URL(redirectUrl, request.url).toString(), redirectStatus);
      }
      return new Response(
        "nextcompile: layout composition failed:\n" + (err?.stack || String(err)),
        { status: 500 },
      );
    }

    // Pull the bundlerConfig out of the client manifest. Next emits it as
    // `.default.clientModules` for bundler consumption; some minors use
    // top-level `.clientModules`. Try both.
    const bundlerConfig =
      (clientManifest &&
        (clientManifest.default?.clientModules || clientManifest.clientModules)) ||
      {};

    let flightStream;
    try {
      flightStream = await vendored.renderToReadableStream(tree, bundlerConfig);
    } catch (err) {
      return new Response(
        "nextcompile RSC render failure:\n" + (err?.stack || String(err)),
        {
          status: 500,
          headers: { "content-type": "text/plain; charset=utf-8" },
        },
      );
    }

    // Resolve `metadata` / `generateMetadata` across the same chain we just
    // rendered. Next does this inside app-render, which we bypass — without it
    // the page ships with no <title> or Open Graph tags whatsoever.
    let metaMarkup = "";
    try {
      const pageUserland = pageModule?.routeModule?.userland || pageModule?.userland || pageModule;
      const chain = [...layoutModules, pageUserland];
      metaMarkup = renderMetadataTags(await resolveMetadata(chain, treeProps));
    } catch (err) {
      console.error("[nextcompile metadata]", err?.stack || String(err));
    }

    // Layer 2 — SSR. Renders the Flight stream to HTML and inlines the same
    // payload as self.__next_f so Next's own client runtime can hydrate it.
    const htmlStream = await renderHtmlOrShell(
      flightStream,
      entry,
      clientManifest,
      manifest,
      reqCtx,
      metaMarkup,
    );

    const respHeaders = new Headers(reqCtx.responseHeaders);
    respHeaders.set("content-type", "text/html; charset=utf-8");
    if (!respHeaders.has("cache-control")) {
      respHeaders.set("cache-control", "private, no-cache, no-store, must-revalidate");
    }
    for (const cookie of reqCtx.setCookies) {
      respHeaders.append("set-cookie", cookie);
    }

    return new Response(htmlStream, { status: 200, headers: respHeaders });
  });
}

/**
 * Pick the React component out of a compiled Next page module. Next
 * commonly puts it on default; some legacy emits use `Page`. Production
 * route modules put it on routeModule.userland.default or inside tree.
 */
export async function resolveComponent(mod) {
  if (!mod) return undefined;
  if (typeof mod === "function") return mod;
  if (typeof mod.default === "function") return mod.default;
  if (typeof mod.Page === "function") return mod.Page;
  if (typeof mod.Component === "function") return mod.Component;

  // Next's internal route module userland export.
  const userland =
    mod.routeModule?.userland ||
    mod.routeModule?.definition?.userland ||
    mod.userland;
  if (userland) {
    if (typeof userland === "function") return userland;
    if (typeof userland.default === "function") return userland.default;
    if (typeof userland.Page === "function") return userland.Page;
    if (typeof userland.Component === "function") return userland.Component;
  }

  // Next's recursive App Router tree layout representation.
  const tree = mod.tree || mod.routeModule?.userland?.loaderTree;
  if (tree) {
    const fromTree = await extractComponentFromTree(tree);
    if (typeof fromTree === "function") return fromTree;
  }

  return undefined;
}

async function extractComponentFromTree(tree) {
  if (!tree || !Array.isArray(tree)) return undefined;
  const components = tree[2];
  if (components && components.page) {
    const pageLoader = Array.isArray(components.page) ? components.page[0] : components.page;
    if (typeof pageLoader === "function") {
      try {
        let res = pageLoader();
        if (res && typeof res.then === "function") {
          res = await res;
        }
        if (typeof res === "function") return res;
        if (res && typeof res.default === "function") return res.default;
        if (res && typeof res.Page === "function") return res.Page;
      } catch {}
    }
  }
  const parallelRoutes = tree[1];
  if (parallelRoutes && typeof parallelRoutes === "object") {
    for (const sub of Object.values(parallelRoutes)) {
      const found = await extractComponentFromTree(sub);
      if (found) return found;
    }
  }
  return undefined;
}

async function extractLayoutsFromTree(t) {
  const layouts = [];
  async function walk(node) {
    if (!node || !Array.isArray(node)) return;
    const comps = node[2];
    if (comps && comps.layout) {
      const loader = Array.isArray(comps.layout) ? comps.layout[0] : comps.layout;
      if (typeof loader === "function") {
        try {
          let res = loader();
          if (res && typeof res.then === "function") res = await res;
          const comp = typeof res === "function" ? res : res?.default;
          if (typeof comp === "function") layouts.push(comp);
        } catch {}
      }
    }
    const parallel = node[1];
    if (parallel && typeof parallel === "object") {
      for (const sub of Object.values(parallel)) {
        await walk(sub);
      }
    }
  }
  await walk(t);
  return layouts;
}

/**
 * Explain a module that carried no renderable component.
 *
 * This is the single most likely first failure of the whole RSC path, so the
 * message earns its length. `resolveComponent` expects a plain component on
 * `default`; a production `next build` may instead emit Next's internal route
 * module (`routeModule` / `tree` / `pages`), which this renderer does not know
 * how to drive. Detecting that shape specifically turns a generic 500 into an
 * actionable one, rather than leaving someone to guess from a key list.
 */
export function describeMissingComponent(mod, entry) {
  const keys = mod ? Object.keys(mod) : [];
  const nextInternals = ["routeModule", "tree", "pages", "workAsyncStorage", "handler"].filter(
    (k) => keys.includes(k),
  );

  let body =
    "nextcompile: page module exported no renderable component.\n\n" +
    `Route:    ${entry?.compiled ?? "(unknown)"}\n` +
    `Exports:  ${keys.length ? keys.join(", ") : "(none)"}\n\n`;

  if (nextInternals.length > 0) {
    body +=
      "This module is Next's INTERNAL route module (found: " +
      nextInternals.join(", ") +
      "), not a\n" +
      "plain component. nextcompile's renderer composes layouts itself and needs a\n" +
      "component on `default`; it cannot drive Next's route module yet.\n\n" +
      "This is a known architectural gap, not a misconfiguration — see\n" +
      "yussuf.md §6(a).\n";
  } else {
    body +=
      "Expected a component on `default` (or `Page` / `Component`).\n" +
      "If this page renders in `next dev`, the compiled output shape differs from\n" +
      "what this renderer assumes — report the export list above.\n";
  }
  return body;
}

/**
 * Compose layouts around the page. Layouts are applied root → leaf so that
 * the root layout is outermost in the final tree.
 *
 * Each compiled layout exports a component that takes { children }. We
 * invoke them as functions (not JSX) because this runtime doesn't depend
 * on JSX syntax at module top level.
 */
async function buildLayoutTree(layouts, PageComponent, props) {
  let node = await PageComponent(props);
  // Apply layouts innermost first — because our layouts array is
  // root → leaf, we iterate in reverse so the root wraps everything.
  for (let i = layouts.length - 1; i >= 0; i--) {
    const LayoutComponent = layouts[i];
    if (typeof LayoutComponent !== "function") continue;
    node = await LayoutComponent({ ...props, children: node });
  }
  return node;
}

/**
 * Render the Flight stream to hydratable HTML, degrading to the Flight-only
 * shell if the SSR layer can't run.
 *
 * The stream is teed up front rather than after a failure: once renderToHtml
 * has started consuming it the stream is locked, so there would be nothing
 * left to build a fallback shell from. Teeing costs one buffered copy and
 * makes the fallback always available; the unused branch is cancelled so it
 * doesn't retain the payload.
 */
async function renderHtmlOrShell(
  flightStream,
  entry,
  clientManifest,
  manifest,
  reqCtx,
  headExtra = "",
) {
  const [primary, fallback] = flightStream.tee();

  const assetOpts = {
    assetPrefix: manifest?.assetPrefix,
    basePath: manifest?.basePath,
  };
  let bootstrapChunks = entry.bootstrap;
  if (!bootstrapChunks || bootstrapChunks.length === 0) {
    const rawChunks = [];
    if (Array.isArray(manifest?.rootMainFiles)) {
      rawChunks.push(...manifest.rootMainFiles);
    }
    const cm = clientManifest?.default || clientManifest || {};
    const clientModules = cm.clientModules || {};
    for (const mod of Object.values(clientModules)) {
      if (Array.isArray(mod?.chunks)) {
        for (const c of mod.chunks) {
          if (typeof c === "string" && c.endsWith(".js") && !rawChunks.includes(c)) {
            rawChunks.push(c);
          }
        }
      }
    }
    if (rawChunks.length > 0) {
      bootstrapChunks = rawChunks;
    }
  }
  const scripts = bootstrapScriptURLs(bootstrapChunks, assetOpts);
  // Metadata before stylesheets: <title>/<meta> are what a crawler or preview
  // bot reads out of a truncated head, so they should never sit behind a
  // stylesheet list.
  const headMarkup = headExtra + stylesheetLinkTags(entry.css, { ...assetOpts, nonce: reqCtx?.nonce });

  try {
    const html = await renderToHtml(
      primary,
      {
        scripts,
        headMarkup,
        ssrManifest: buildSSRManifest(clientManifest),
        nonce: reqCtx?.nonce,
      },
      reqCtx,
    );
    if (html) {
      safeCancel(fallback);
      return html;
    }
    // Vendored SSR builds absent — an older bundle. Not an error.
    console.warn(
      "[nextcompile ssr] SSR runtime unavailable, serving Flight shell (page will not hydrate):",
      String(ssrLoadFailure()),
    );
  } catch (err) {
    // A render failure must not 500 the route: a blank-but-served page beats
    // an error page, and the Flight payload is still on the wire for debugging.
    console.error(
      "[nextcompile ssr] render failed, falling back to Flight shell:",
      err?.stack || String(err),
    );
  }

  safeCancel(primary);
  return wrapFlightInHtmlShell(fallback, reqCtx, headMarkup, scripts);
}

// Cancelling a stream that renderToHtml already locked throws synchronously;
// neither that nor a rejected cancel should affect the response.
function safeCancel(stream) {
  try {
    const p = stream.cancel();
    if (p && typeof p.catch === "function") p.catch(() => {});
  } catch {
    /* already locked or cancelled — nothing to reclaim */
  }
}

/**
 * HTML shell around the Flight stream that enables client-side hydration.
 *
 * Emits:
 *   1. Sets doctype + charset + stylesheets + resolved metadata tags.
 *   2. Emits placeholder <div id="__next"> for hydration targets.
 *   3. Frames Flight chunks into (self.__next_f=self.__next_f||[]).push(...)
 *      scripts so Next's client hydrator receives the component payload.
 *   4. Emits bootstrap <script src="..." async> tags for webpack, main-app,
 *      and page chunks so the browser executes Next.js hydration.
 */
function wrapFlightInHtmlShell(flightStream, reqCtx, headMarkup = "", scripts = []) {
  const encoder = new TextEncoder();
  const decoder = new TextDecoder("utf-8");
  const nonce = reqCtx?.nonce;
  const nonceAttr = nonce ? ` nonce="${nonce}"` : "";

  const shellPrefix = encoder.encode(
    `<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">${headMarkup}</head><body><div id="__next"></div><script${nonceAttr}>(self.__next_f=self.__next_f||[]).push([0])</script>`,
  );

  const scriptTags = (scripts || [])
    .map((src) => `<script src="${src}" async${nonceAttr}></script>`)
    .join("");
  const shellSuffix = encoder.encode(`${scriptTags}</body></html>`);

  return new ReadableStream({
    async start(controller) {
      controller.enqueue(shellPrefix);
      const reader = flightStream.getReader();
      while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        const text = decoder.decode(value, { stream: true });
        if (text) {
          const chunkScript = `<script${nonceAttr}>(self.__next_f=self.__next_f||[]).push([1,${htmlEscapeJsonString(
            JSON.stringify(text),
          )}])</script>`;
          controller.enqueue(encoder.encode(chunkScript));
        }
      }
      const tail = decoder.decode();
      if (tail) {
        const chunkScript = `<script${nonceAttr}>(self.__next_f=self.__next_f||[]).push([1,${htmlEscapeJsonString(
          JSON.stringify(tail),
        )}])</script>`;
        controller.enqueue(encoder.encode(chunkScript));
      }
      controller.enqueue(shellSuffix);
      controller.close();
    },
  });
}

function vendorMissingResponse() {
  const body =
    "nextcompile: React Server Components runtime not vendored.\n\n" +
    "This deployment contains App Router pages that use Server Components,\n" +
    "but the build did not include the vendored React server bundle at:\n" +
    "  _nextdeploy/runtime/vendor/react-server-dom-webpack/server.edge.mjs\n\n" +
    "Fix: invoke nextcompile's adapter build step with RSC vendoring enabled,\n" +
    "or ship the matching react-server-dom-webpack bundle for the detected\n" +
    "React version. See runtime/vendor/README.md for the exact steps.\n\n" +
    "Load error: " + String(vendoredLoadError);
  return new Response(body, {
    status: 501,
    headers: {
      "content-type": "text/plain; charset=utf-8",
      "cache-control": "no-store",
    },
  });
}

function pprNotImplemented() {
  const body =
    "nextcompile: Partial Prerendering (PPR) is not yet implemented.\n\n" +
    "This page is marked PPR in the compiled output, which means Next\n" +
    "expects the server to stream a static shell with dynamic holes\n" +
    "resolved inline. nextcompile's RSC renderer handles fully-dynamic\n" +
    "pages today; PPR support is tracked as a follow-up milestone.\n\n" +
    "Workaround: remove `experimental.ppr` or `experimental_ppr` from\n" +
    "this route's config and redeploy.";
  return new Response(body, {
    status: 501,
    headers: {
      "content-type": "text/plain; charset=utf-8",
      "cache-control": "no-store",
    },
  });
}
