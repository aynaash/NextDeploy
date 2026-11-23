// Typed accessor for Cloudflare bindings + secrets.
//
// At the edge, NextDeploy's runtime exposes the full Worker `env` on
// globalThis.__nextdeployEnv before any app code runs. This helper reads it so
// server code / proxy.ts can reach DB, R2, AI, KV and secrets without importing
// runtime internals. Secrets that are plain strings are also on process.env.
export interface R2Object {
  key: string;
  size: number;
  etag: string;
  httpMetadata?: Record<string, unknown>;
  customMetadata?: Record<string, string>;
}

export interface R2ObjectBody extends R2Object {
  body: ReadableStream;
  arrayBuffer(): Promise<ArrayBuffer>;
  text(): Promise<string>;
  json<T = unknown>(): Promise<T>;
  blob(): Promise<Blob>;
}

export interface R2Bucket {
  head(key: string): Promise<R2Object | null>;
  get(
    key: string,
    options?: Record<string, unknown>,
  ): Promise<R2ObjectBody | null>;
  put(
    key: string,
    value:
      | ReadableStream
      | ArrayBuffer
      | ArrayBufferView
      | string
      | Blob
      | null,
    options?: Record<string, unknown>,
  ): Promise<R2Object | null>;
  delete(keys: string | string[]): Promise<void>;
  list(options?: Record<string, unknown>): Promise<{
    objects: R2Object[];
    truncated: boolean;
    cursor?: string;
    delimitedPrefixes: string[];
  }>;
}

export interface KVNamespace {
  get(key: string, options?: unknown): Promise<any>;
  put(
    key: string,
    value: string | ArrayBuffer | ArrayBufferView | ReadableStream,
    options?: {
      expiration?: number;
      expirationTtl?: number;
      metadata?: unknown;
    },
  ): Promise<void>;
  delete(key: string): Promise<void>;
  list(options?: {
    prefix?: string;
    limit?: number;
    cursor?: string;
  }): Promise<{
    keys: Array<{ name: string; expiration?: number; metadata?: unknown }>;
    list_complete: boolean;
    cursor?: string;
  }>;
}

export interface CloudflareEnv {
  // Database binding name depends on the chosen variant (D1: env.DB).
  DB: any;
  ASSETS: R2Bucket;
  AI: any;
  RATE_LIMIT: KVNamespace;
  AUTH_SECRET: string;
  [key: string]: unknown;
}

export function getEnv(): CloudflareEnv {
  const env = (globalThis as any).__nextdeployEnv;
  if (!env) {
    throw new Error(
      "Cloudflare env not available — getEnv() must be called inside a request " +
        "(server component, route handler, action, or proxy.ts).",
    );
  }
  return env as CloudflareEnv;
}
