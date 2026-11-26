// Built-in Node.js module registry for Turbopack `Context.externalRequire`
// calls inside Next.js server chunks on Cloudflare Workers (`nodejs_compat`).
//
// Because esbuild marks `node:*` imports external, dynamic `require(expr)` calls
// inside `[turbopack]_runtime.js` cannot be resolved statically at bundle time.
// Importing these supported `node:*` modules here ensures esbuild emits valid
// static ESM imports and `__ndDynRequire` can return them synchronously.

import * as assertMod from "node:assert";
import * as asyncHooksMod from "node:async_hooks";
import * as bufferMod from "node:buffer";
import * as cryptoMod from "node:crypto";
import * as diagnosticsChannelMod from "node:diagnostics_channel";
import * as eventsMod from "node:events";
import * as pathMod from "node:path";
import * as querystringMod from "node:querystring";
import * as streamMod from "node:stream";
import * as stringDecoderMod from "node:string_decoder";
import * as timersMod from "node:timers";
import * as urlMod from "node:url";
import * as utilMod from "node:util";
import * as zlibMod from "node:zlib";

import fsShim, { promises as fsPromisesShim } from "./vfs_fs_shim.mjs";

function unwrapCjs(mod) {
  if (mod && (typeof mod.default === "function" || (typeof mod.default === "object" && mod.default !== null))) {
    return mod.default;
  }
  return mod;
}

const osStub = {
  EOL: "\n",
  platform: () => "linux",
  arch: () => "x64",
  tmpdir: () => "/tmp",
  homedir: () => "/",
  hostname: () => "cloudflare-worker",
  cpus: () => [{ model: "workerd", speed: 1000 }],
  totalmem: () => 134217728,
  freemem: () => 67108864,
  networkInterfaces: () => ({}),
  constants: {},
};

/**
 * Resolve a Node built-in module specifier (with or without `node:` prefix).
 * Returns `undefined` when `spec` is not a registered built-in.
 */
export function loadBuiltinModule(spec) {
  if (typeof spec !== "string" || !spec) return undefined;
  const bare = spec.startsWith("node:") ? spec.slice(5) : spec;

  switch (bare) {
    case "fs":
      return fsShim;
    case "fs/promises":
      return fsPromisesShim;
    case "path":
      return unwrapCjs(pathMod);
    case "path/posix":
      return pathMod.posix || unwrapCjs(pathMod).posix;
    case "path/win32":
      return pathMod.win32 || unwrapCjs(pathMod).win32;
    case "assert":
      return unwrapCjs(assertMod);
    case "assert/strict":
      return assertMod.strict || unwrapCjs(assertMod).strict || unwrapCjs(assertMod);
    case "async_hooks":
      return unwrapCjs(asyncHooksMod);
    case "buffer":
      return unwrapCjs(bufferMod);
    case "crypto":
      return unwrapCjs(cryptoMod);
    case "diagnostics_channel":
      return unwrapCjs(diagnosticsChannelMod);
    case "events":
      return unwrapCjs(eventsMod);
    case "querystring":
      return unwrapCjs(querystringMod);
    case "stream":
      return unwrapCjs(streamMod);
    case "stream/web":
      return {
        ReadableStream: globalThis.ReadableStream,
        WritableStream: globalThis.WritableStream,
        TransformStream: globalThis.TransformStream,
        ByteLengthQueuingStrategy: globalThis.ByteLengthQueuingStrategy,
        CountQueuingStrategy: globalThis.CountQueuingStrategy,
      };
    case "string_decoder":
      return unwrapCjs(stringDecoderMod);
    case "timers":
      return unwrapCjs(timersMod);
    case "url":
      return unwrapCjs(urlMod);
    case "util":
      return unwrapCjs(utilMod);
    case "util/types":
      return utilMod.types || unwrapCjs(utilMod).types;
    case "zlib":
      return unwrapCjs(zlibMod);
    case "process":
      return globalThis.process;
    case "console":
      return globalThis.console;
    case "os":
      return osStub;
    default:
      return undefined;
  }
}
