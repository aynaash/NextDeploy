# Fixture — `next15-appdir`

A **real** Next.js 15 App Router build, captured so the SSR guides (03→06) can
assert against genuine Next output instead of hand-mocked JSON. Unlike the
sibling `next14/`, `next15/` fixtures (which carry only
`server-reference-manifest.json` for Server Actions tests), this one carries the
artifacts the RSC **hydration** work needs.

## What the app is

The minimal app that still exercises a `"use client"` boundary — a server page
rendering a client counter:

```
app/layout.tsx   → root layout (server component)
app/page.tsx     → server page, imports ./counter
app/counter.tsx  → "use client" — useState button (the hydration target)
next.config.mjs  → { output: "standalone" }
```

Deps: `next@15.5.4`, `react@19.1.0`, `react-dom@19.1.0`.

## Captured artifacts

| File | Why it's here |
|---|---|
| `app-build-manifest.json` | **Guide 03 (SSR M0) input.** Per-entry ordered client bootstrap chunk list, keyed by app-path (`/page`, `/layout`, `/_not-found/page`). |
| `server/app-paths-manifest.json` | App-path → compiled entry (`/page` → `app/page.js`). Maps a route to its manifest keys. |
| `server/app/page_client-reference-manifest.js` | Next 15's side-effecting `globalThis.__RSC_MANIFEST[...]={…}` module — the Flight `bundlerConfig`. Exercises `scanner.extractRSCManifestJSON`. |

**Not** captured (add only if a later guide asserts on real bytes): the 12
compiled client chunks under `.next/static/chunks/` (~788 KB). M0/M1 need only
the manifests; M2 hydration can reference chunk *names* from
`app-build-manifest.json` without the bytes.

## Reproduce

```bash
mkdir ssr-fixture-app && cd ssr-fixture-app
# write the 5 files above, then:
npm install                      # next@15.5.4 react@19.1.0 react-dom@19.1.0
CI=1 NEXT_TELEMETRY_DISABLED=1 npx next build
# (the type-check step may warn about missing @types — the webpack compile,
#  which emits these manifests, completes before that and is all we need)
cp .next/app-build-manifest.json                         <fixture>/
cp .next/server/app-paths-manifest.json                  <fixture>/server/
cp .next/server/app/page_client-reference-manifest.js    <fixture>/server/app/
```

Regenerate when bumping the target Next version the SSR runtime supports.
