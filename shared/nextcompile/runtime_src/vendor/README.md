# Bundle vendor directory

Files under `_nextdeploy/runtime/vendor/` are copied into each compiled
worker bundle at build time, not at request time. The Cloudflare Workers
runtime has no `npm`, no filesystem, and no dynamic resolver — anything
the runtime imports must already be inside the bundle when it ships.

## What lives here

`react-server-dom-webpack/server.edge.mjs`
:   Vendored by `VendorRSC` in `shared/nextcompile/vendor.go`. Resolved
    from the Next standalone tree's `node_modules`, then byte-copied
    into the bundle. Imported lazily by `runtime_src/rsc.mjs` to encode
    React Flight streams for App Router pages.

`react-server-dom-webpack/client.edge.mjs`
:   SSR companion. Deserializes a Flight stream back into a React element
    on the worker, so the SSR layer can render it to HTML.

`react-dom/server.edge.mjs`
:   SSR companion. Streams a React element to HTML via
    `renderToReadableStream`.

`*.cjs` (alongside any of the above)
:   The concrete implementation, when the installed React publishes that
    build as CommonJS. See "Module format" below.

## Module format — why you may see `.cjs` files

React 19 publishes **`react-dom` and `react-server-dom-webpack` as CJS
only**: there is no `esm/` directory, and the flat `server.edge.js` /
`client.edge.js` at the package root are conditional shims whose body is
`require("./cjs/<pkg>-<entry>.production.js")`.

Vendoring such a shim on its own copies a module whose relative `require`
points at a file that was never copied — it resolves to nothing. So
`vendorEdgeBuild` ranks the concrete `cjs/…production.js` **above** the flat
shim, writes it as `<entry>.cjs`, and generates a small `<entry>.mjs`
re-export facade next to it:

```js
export * from "./server.edge.cjs";
export { default } from "./server.edge.cjs";
```

That keeps the `.mjs` specifier stable, so runtime modules (e.g. `rsc.mjs`'s
`import("./vendor/react-server-dom-webpack/server.edge.mjs")`) never have to
branch on how React happened to be published. esbuild statically analyzes
React's `exports.foo = …` assignments, and Node's `cjs-module-lexer` does the
same for the `node --test` path, so named bindings resolve in both.

When a package *does* ship `esm/` (React 18), the payload is written as
`<entry>.mjs` directly and no shim is generated.

> **Known gap for the SSR renderer (not this layer).** React's *server*
> build (`react-server-dom-webpack/server.edge`) throws at module load
> unless the `react-server` export condition is enabled, and the Worker
> bundle is currently built with `--conditions=workerd,worker,node`
> (`cloudflare_adapter.go`). Vendoring puts the right bytes in the bundle;
> making the RSC server build *evaluate* is the SSR milestone's problem, and
> it is complicated by `client.edge` needing that condition to be **off**.

This directory is otherwise empty in the source tree. It only fills up
inside the per-build output directory after `VendorRSC` runs.

## Why vendoring at all

React Server Components needs `react-server-dom-webpack` at the edge.
Three options were considered:

1. **`npm install` in the worker** — impossible, Workers has no npm.
2. **Re-publish a forked package** — drift; users' React version must
    match exactly.
3. **Copy the package the user already installed** — what we do.

Option 3 keeps the user's React/Next versions authoritative. The
package the build resolves is the package the runtime executes.

## Lookup contract

`VendorRSC` walks up from the standalone directory looking for
`node_modules/react-server-dom-webpack` (and, for the react-dom companion,
`node_modules/react-dom`), capped at 5 levels — which handles pnpm and
workspace layouts. Build flavor preference, first hit wins:

    esm/<pkg>-<entry>.production.js     ESM, no interop needed
    <entry>.production.js               flat ESM (React 18)
    cjs/<pkg>-<entry>.production.js     CJS implementation (React 19)
    …the same three for .development…
    <entry>.js                          flat shim, last resort

Failures surface as one of five sentinels, so the error names the exact
build that is absent:

    ErrRSCPackageNotFound        react-server-dom-webpack not installed
    ErrRSCServerEdgeNotFound     installed, but publishes no server.edge
    ErrRSCClientEdgeNotFound     installed, but publishes no client.edge
    ErrReactDOMPackageNotFound   react-dom not installed
    ErrReactDOMServerNotFound    installed, but publishes no server.edge

The compiler only treats any of them as fatal when
`manifest.Features.RSC == true` — pages-only apps skip vendoring silently.

Every vendored file, including the generated `.mjs` shims, is folded into
the bundle content hash. If a companion were omitted, bumping react-dom
19.0 → 19.1 would leave the hash unchanged, the adapter would skip the
redeploy, and the Worker would keep serving the old React.

## When this directory is regenerated

Every build. `ExtractRuntime` writes the embedded source files first;
`VendorRSC` lays the package on top. Both are idempotent — running a
second build into the same output directory overwrites in place.
