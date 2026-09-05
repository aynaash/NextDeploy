# `@nextdeploy/adapter`

A Next.js [deployment adapter](https://nextjs.org/docs/app/api-reference/adapters)
whose only job is to serialize Next's build output to disk as a versioned
envelope that NextDeploy's Go side reads.

**Status: spec. Not implemented yet.**

## Scope

This package does one thing: `onBuildComplete` → a deterministic, versioned
JSON file. It contains **no mapping, no Cloudflare knowledge, no inference**.
Deciding that a route with `initialRevalidate` needs a KV namespace is the Go
side's job, and keeping that decision out of here is what stops this package
from becoming a second implementation of the product.

If it ever exceeds ~300 lines of real logic, mapping has leaked into it.

Requires **Next.js 16.2+** (the release where the Adapter API went stable).

## Why this exists

Today `shared/nextcore` reconstructs the build by reading Next's *private*
manifests — `prerenderManifest`, `routesManifest`, `appPathRoutesManifest`,
`reactLoadableManifest`. That is the archaeology that breaks on Next releases
and produces most of the recurring bug tax in this repo.

`onBuildComplete` provides the same information as a typed, versioned, public
contract with a documented breaking-change policy. The swap happens at the
input: `RouteInfo` survives roughly as-is; everything that derives it goes.

## Usage (once implemented)

```js
// next.config.mjs
export default {
  adapterPath: '@nextdeploy/adapter',
}
```

Or `NEXT_ADAPTER_PATH=@nextdeploy/adapter`. Writes
`.nextdeploy/build-output.json`.

## The interface

```ts
{
  name: 'nextdeploy',

  // Do nothing here in v1 beyond recording the phase. Every config mutation
  // is a divergence between what the user wrote and what actually built.
  async modifyConfig(config, { phase, nextVersion, projectDir }) { … },

  async onBuildComplete({
    routing,      // beforeMiddleware, middlewareMatchers, beforeFiles,
                  // afterFiles, dynamicRoutes, onMatch, fallback, rsc,
                  // shouldNormalizeNextData
    outputs,      // pages, pagesApi, appPages, appRoutes, prerenders,
                  // staticFiles, middleware
    projectDir, repoRoot, distDir, config, nextVersion, buildId,
  }) { … },
}
```

## Five decisions to get right up front

### 1. Normalize every path

`filePath`, and the *values* in `assets` / `wasmAssets`, are absolute and
machine-specific. The envelope travels — into artifacts, into CI, potentially
to the daemon. Rewrite all of them relative to `repoRoot` before writing, and
fail if any path escapes `repoRoot`.

### 2. Whitelist `context.config`, never dump it

It is the fully resolved Next config: large, and capable of carrying secrets.
Take only what is needed — `basePath`, `assetPrefix`, `images`, `i18n`,
`output`, `trailingSlash`. Same whitelist-not-blacklist call as `SafeConfig`
in `shared/config`.

### 3. Deterministic output

Sort array entries and object keys before serializing. The same build must
produce a byte-identical envelope so config fingerprints and content hashes
stay stable — the reason `sortedKeys` exists in `cloudflare_bindings.go`.

### 4. Versioned envelope, validated on the Go side

```jsonc
{
  "schemaVersion": 1,
  "adapterVersion": "0.1.0",
  "nextVersion": "16.3.4",
  "buildId": "…",
  "paths":   { "projectDir": ".", "distDir": ".next" },
  "config":  { /* whitelisted subset */ },
  "routing": { /* passthrough */ },
  "outputs": { /* passthrough, paths normalized */ }
}
```

Go refuses to read a field before checking `schemaVersion` — the same pattern
as `NextCorePayload.ValidateSchema`. That check exists because a decoder turns
version skew into zero values, and a zero value is indistinguishable from a
real answer.

### 5. Fail loudly, never partially

Write to a temp file and rename atomically. If the output shape is
unrecognized, throw with the offending field named and write nothing. A
half-written `build-output.json` decoded into zero values is exactly what the
versioning exists to prevent.

## What the envelope replaces

| From the adapter | Replaces |
|---|---|
| `staticFiles[].immutableHash` | the immutable/mutable guesswork in `partitionAssets` |
| `assets` + `assetsHashes` | our own content-hash pass for R2 skip-upload |
| `prerenders[].groupId`, `config.allowQuery` / `allowHeader` / `bypassFor`, `fallback.initialRevalidate` | the hand-rolled ISR detail — and `initialRevalidate` is the time-based revalidation the current runtime cannot do |
| `routing.middlewareMatchers` (`sourceRegex`, `has`, `missing`) | `runtime_src/middleware_match.mjs` |
| `routing.dynamicRoutes`, `beforeFiles`, `afterFiles`, `fallback` | `runtime_src/route_match.mjs`, `route_trie.dev.mjs` |
| `outputs.*` | `RouteInfo` **and** `NextBuildMetadata` — the manifest archaeology |

**Unconfirmed:** `prerenders` shows no tags field in the reference. Verify
against a real build before designing the ISR tag store around it.

## Types across the boundary

Make the **schema** the source of truth, not either language. Hand-write
`schema/envelope.v1.json`, generate the TS type and the Go struct from it,
check both in, and fail CI when regeneration produces a diff.

Inside the TS, import Next's own output types so a shape change in a Next major
breaks this build rather than someone's deploy.

## Testing

- Next ships a compatibility test harness for adapters — that is the baseline.
- **Golden file:** run against a fixture, snapshot the envelope, diff on every
  Next upgrade. Wire it into `.github/workflows/nextjs-canary-matrix.yml`,
  which already scaffolds `next@latest` and `next@canary` weekly — a shape
  change then surfaces as a failing snapshot on a Monday rather than in
  production.

## Milestones

1. Skeleton + `onBuildComplete` writing a raw, unnormalized envelope. Prove the
   hook fires at all.
2. Normalization, safe-config, determinism, atomic write.
3. Schema + codegen both directions, wired into CI.
4. Go reads and validates it; `nextcore` grows a second input path.
5. Golden test into the canary matrix.
6. **Only then** delete `scanner.go`, `manifest.go`, `version_detect.go`.

Keep 6 last. Running both paths side by side for a while is how you prove the
envelope actually carries everything the old code derived.
