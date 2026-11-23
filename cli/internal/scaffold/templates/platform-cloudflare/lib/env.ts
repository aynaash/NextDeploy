// Typed accessor for Cloudflare bindings + secrets.
//
// At the edge, NextDeploy's runtime exposes the full Worker `env` on
// globalThis.__nextdeployEnv before any app code runs. This helper reads it so
// server code / middleware.ts / proxy.ts can reach DB, R2, AI, KV and secrets without importing
// runtime internals. Secrets that are plain strings are also on process.env.
// Minimal structural interfaces so existing TypeScript projects compile even
// without `@cloudflare/workers-types` installed. When `@cloudflare/workers-types`
// is installed, native R2Bucket / KVNamespace / D1Database / Hyperdrive types
// are assignable to these shapes.
export interface R2BucketLike {
  get(key: string): Promise<unknown>;
  put(key: string, value: unknown): Promise<unknown>;
  delete(key: string): Promise<void>;
}

export interface KVNamespaceLike {
  get(key: string): Promise<string | null>;
  put(key: string, value: string, options?: { expirationTtl?: number }): Promise<void>;
  delete(key: string): Promise<void>;
}

export interface CloudflareEnv {
  // Optional database binding: D1 (D1Database) or Hyperdrive ({ connectionString: string }).
  // Absent when scaffolded with --db=none.
  DB?: any;
  ASSETS: R2BucketLike;
  AI: any;
  RATE_LIMIT: KVNamespaceLike;
  AUTH_SECRET: string;
  [key: string]: unknown;
}

export function getEnv(): CloudflareEnv {
  const env = (globalThis as any).__nextdeployEnv;
  if (!env) {
    throw new Error(
      "Cloudflare env not available — getEnv() must be called inside a request " +
        "(server component, route handler, action, or middleware.ts / proxy.ts).",
    );
  }
  return env as CloudflareEnv;
}
