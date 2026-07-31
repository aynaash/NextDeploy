# `next15-appdir` fixture

A **real** `next build` output (Next 15.0.3, React 19, App Router, webpack) —
captured, not hand-written. Only the files the compiler actually reads are kept;
the chunk `.js` files themselves are not, since nothing parses them.

```
app-build-manifest.json                        ← SSR M0 input: app-path → ordered client chunks
server/app-paths-manifest.json                 ← app-path → compiled server entry
server/app/page_client-reference-manifest.js   ← Flight bundlerConfig for "/"
```

## What the source app looks like

Deliberately shaped to exercise the cases that break naive key derivation:

```
app/layout.tsx                          root layout, renders <Nav/> ("use client")
app/nav.tsx                             "use client" island in the LAYOUT
app/counter.tsx                         "use client" island in a PAGE
app/page.tsx                            "/"            → renders <Counter/>
app/blog/layout.tsx                     nested layout
app/blog/[id]/page.tsx                  "/blog/[id]"   → dynamic segment
app/staff/(authed)/layout.tsx           layout inside a ROUTE GROUP
app/staff/(authed)/dashboard/page.tsx   "/staff/dashboard"  ← note the elided group
```

## The three facts this fixture pins

1. **Keys are app-paths, not URLs.** The root page is `/page`, the root layout is
   `/layout`. The trailing segment is the file role; the prefix is the route.

2. **Every entry repeats the shared runtime.** `webpack-*`, the framework chunk
   (`3f94662f-*`), the shared vendor chunk (`323-*`), and `main-app-*` appear in
   *every* entry — only the last element is entry-specific. So a per-route list
   must be deduped while preserving first-seen order.

3. **A route group segment appears in the key but NOT in the URL.** The page
   served at `/staff/dashboard` is keyed `/staff/(authed)/dashboard/page`, and
   its layout is `/staff/(authed)/layout` — there is no `/staff/layout`.

   Fact 3 is why `bootstrapChunksForRoute` keys off the **compiled path**
   (`server/app/staff/(authed)/dashboard/page.js`), which preserves the group,
   rather than off `ModuleRef.RoutePath`, which has already dropped it.
   Deriving keys from the URL silently yields an empty chunk list for every
   route-grouped page — a page that renders but never hydrates.

Note also that this production build emits **no** `server/app/**/layout.js`:
Next inlines layouts into the page bundle. So `ModuleRef.LayoutChain` is empty
here, and the ancestor layout *chunk* entries must be found by walking the
manifest key's own prefixes — not by consulting `LayoutChain`.

## Regenerating (e.g. when bumping Next)

```bash
mkdir fixture-app && cd fixture-app
npm init -y && npm i next@15 react@19 react-dom@19
# recreate the app/ tree above verbatim (the route group is the point)
npx next build
```

Then copy the three files listed at the top out of `.next/`. Re-run
`go test ./shared/nextcompile/ -run BootstrapChunks` — the chunk hashes in the
assertions are matched loosely (prefix/suffix), so a rebuild should not require
editing the tests unless the shared-chunk *count* changes.
