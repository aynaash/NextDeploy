// M3 SCAFFOLD — progressive streaming for Suspense / loading.js.
//
// Dev-only by suffix: `.dev.mjs` means isDevOnlyRuntimeFile (runtime.go) keeps
// this out of every deployed Worker and out of the content hash, exactly like
// route_trie.dev.mjs. It exists to pin the design and the missing prerequisite
// in executable form rather than prose.
//
// ---------------------------------------------------------------------------
// THE PROBLEM
// ---------------------------------------------------------------------------
// ssr.mjs (M1) does:
//
//     const element = await client.createFromReadableStream(forSSR, {...});
//     const html    = await dom.renderToReadableStream(element, {...});
//
// The `await` is the issue. It resolves the ENTIRE Flight payload before HTML
// rendering starts, so:
//   - Suspense boundaries can't flush progressively,
//   - loading.js never appears (there is no gap to show it in),
//   - TTFB is the full server render, not the shell.
//
// ---------------------------------------------------------------------------
// THE FIX, AND WHY IT ISN'T IN M1
// ---------------------------------------------------------------------------
// React's intended pattern hands the *pending* root to Fizz and lets Suspense
// do the waiting:
//
//     const rootPromise = client.createFromReadableStream(forSSR, {...});
//     function Root() { return use(rootPromise); }
//     dom.renderToReadableStream(createElement(Root), {...});
//
// `use` and `createElement` come from `react` itself — which is the blocker.
//
// THE BLOCKER, STATED PRECISELY
// -----------------------------
// React ships two different builds of `react` selected by export condition:
// the `react-server` build (no useState/useEffect, for RSC) and the standard
// one (for SSR/client). rsc.mjs's `server.edge` needs the former; this needs
// the latter.
//
// The conflict is NOT in our own import statement — it is INSIDE the vendored
// bundles. Both `react-server-dom-webpack/*.edge` and `react-dom/server.edge`
// do a bare `require("react")` internally, and esbuild resolves one `react`
// per bundle. So vendoring react and importing it by relative path from these
// modules does NOT fix it: the vendored React DOM still reaches for whichever
// single `react` esbuild picked. (An earlier note in this file claimed
// otherwise; it was wrong.)
//
// THREE REAL OPTIONS, roughly by increasing effort:
//
//   (a) Rewrite the internal requires at vendor time. In vendorEdgeBuild,
//       rewrite `require("react")` in each copied file to a relative specifier
//       pointing at the matching vendored build:
//         react-server-dom-webpack/server.edge → ../react/react.react-server.cjs
//         react-dom/server.edge                → ../react/index.cjs
//       Cheapest, but it is string surgery on third-party output — pin it with
//       a test that greps the vendored files for a surviving bare require.
//
//   (b) An esbuild plugin resolving `react` per importer. Correct and less
//       brittle, but the adapter currently shells out to `npx esbuild` with CLI
//       flags only (cloudflare_adapter.go:runEsbuild); a plugin means moving to
//       esbuild's JS API, which is a bigger change to the build path.
//
//   (c) Two bundles — an RSC worker and an SSR worker — each with its own
//       conditions. Architecturally cleanest, matches how Next separates them,
//       and by far the most work.
//
// Whichever is chosen, the streaming swap below is the easy part and comes last.
//
// Verify with: an async server component behind <Suspense> with a loading.js —
// the fallback must paint before the data resolves.

/**
 * Wrap a pending Flight root so Fizz suspends on it instead of the caller
 * awaiting it. `React` is injected rather than imported precisely because the
 * vendoring in step 1 above does not exist yet.
 *
 * @param {{use: Function, createElement: Function}} React vendored standard build
 * @param {Promise<any>} rootPromise from client.edge.createFromReadableStream
 * @returns {any} a React element suitable for renderToReadableStream
 */
export function createStreamingRoot(React, rootPromise) {
  if (!React || typeof React.use !== "function" || typeof React.createElement !== "function") {
    throw new TypeError(
      "createStreamingRoot needs a vendored standard-condition React with use() and createElement(); " +
        "see the PREREQUISITE block in ssr_streaming.dev.mjs",
    );
  }
  const Root = () => React.use(rootPromise);
  return React.createElement(Root);
}

/**
 * The M3 replacement for the awaited block in ssr.mjs's renderToHtml.
 *
 * Deliberately mirrors that function's shape so the eventual swap is a small,
 * reviewable diff rather than a rewrite. Not wired to anything.
 */
export async function renderToHtmlStreaming(deps, React, flightForSSR, bootstrap, reqCtx) {
  // NOTE the absent `await` — this is the entire behavioural difference.
  const rootPromise = deps.client.createFromReadableStream(flightForSSR, {
    ssrManifest: bootstrap.ssrManifest || { moduleMap: {}, moduleLoading: null },
    nonce: bootstrap.nonce,
  });

  return deps.dom.renderToReadableStream(createStreamingRoot(React, rootPromise), {
    bootstrapScripts: bootstrap.scripts?.length ? bootstrap.scripts : undefined,
    bootstrapScriptContent: bootstrap.flightInit,
    nonce: bootstrap.nonce,
    signal: reqCtx?.abortSignal,
    onError: bootstrap.onError,
  });
}
