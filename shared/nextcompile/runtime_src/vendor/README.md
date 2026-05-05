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
`node_modules/react-server-dom-webpack`, capped at 5 levels (handles
pnpm/workspace layouts). Build flavor preference: ESM production → ESM
development → legacy CJS.

Failure surfaces as `ErrRSCPackageNotFound`. The compiler only treats
that as fatal when `manifest.Features.RSC == true` — pages-only apps
skip vendoring silently.

## When this directory is regenerated

Every build. `ExtractRuntime` writes the embedded source files first;
`VendorRSC` lays the package on top. Both are idempotent — running a
second build into the same output directory overwrites in place.
