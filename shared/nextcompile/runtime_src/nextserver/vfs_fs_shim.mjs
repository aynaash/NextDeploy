// In-memory Virtual Filesystem shim for `fs` / `node:fs` in Cloudflare Workers.
//
// Next.js's NextNodeServer synchronously reads `.next/*.json` manifests,
// BUILD_ID, and package.json at startup and request time. The build-time VFS
// emitter packs those files into globalThis.__nextdeployVFS, and this shim
// serves them from memory with proper ENOENT semantics for missing paths.

import { Readable } from "node:stream";

function getVFS() {
  return globalThis.__nextdeployVFS || { files: {}, dirs: {} };
}

/**
 * Normalize an incoming Node fs path (string | URL | Buffer) into a clean
 * POSIX relative path matching keys emitted by EmitVirtualFS (e.g.
 * ".next/routes-manifest.json", "package.json").
 */
export function normalizeVFSPath(rawPath) {
  if (rawPath == null) return "";
  let s;
  if (rawPath instanceof URL) {
    s = rawPath.pathname;
  } else if (typeof rawPath === "string") {
    s = rawPath;
  } else if (typeof Buffer !== "undefined" && Buffer.isBuffer(rawPath)) {
    s = rawPath.toString("utf8");
  } else {
    s = String(rawPath);
  }
  s = s.replace(/\\/g, "/");

  // Resolve `.` and `..` segments cleanly.
  const parts = [];
  for (const seg of s.split("/")) {
    if (!seg || seg === ".") continue;
    if (seg === "..") {
      if (parts.length > 0) parts.pop();
    } else {
      parts.push(seg);
    }
  }
  const joined = parts.join("/");
  if (!joined) return "";

  // Direct hit first.
  const vfs = getVFS();
  if (Object.prototype.hasOwnProperty.call(vfs.files, joined) || Object.prototype.hasOwnProperty.call(vfs.dirs, joined)) {
    return joined;
  }

  // Strip arbitrary leading mount prefix (e.g. "var/task/.next/..." → ".next/...").
  const nextIdx = joined.indexOf(".next/");
  if (nextIdx > 0) {
    const sub = joined.slice(nextIdx);
    if (Object.prototype.hasOwnProperty.call(vfs.files, sub) || Object.prototype.hasOwnProperty.call(vfs.dirs, sub)) {
      return sub;
    }
  }
  if (joined === ".next" || joined.endsWith("/.next")) {
    if (Object.prototype.hasOwnProperty.call(vfs.dirs, ".next")) return ".next";
  }
  if (joined.endsWith("/package.json") && Object.prototype.hasOwnProperty.call(vfs.files, "package.json")) {
    return "package.json";
  }

  return joined;
}

function makeENOENT(op, p) {
  const err = new Error(`ENOENT: no such file or directory, ${op} '${p}'`);
  err.code = "ENOENT";
  err.errno = -2;
  err.syscall = op;
  err.path = String(p);
  return err;
}

function makeStat(isDir, size) {
  const now = new Date(0);
  return {
    isFile: () => !isDir,
    isDirectory: () => isDir,
    isSymbolicLink: () => false,
    isFIFO: () => false,
    isSocket: () => false,
    isCharacterDevice: () => false,
    isBlockDevice: () => false,
    size,
    mtime: now,
    atime: now,
    ctime: now,
    birthtime: now,
    mtimeMs: 0,
    atimeMs: 0,
    ctimeMs: 0,
    birthtimeMs: 0,
    mode: isDir ? 0o040755 : 0o100644,
  };
}

function encodeContent(str, options) {
  const enc = typeof options === "string" ? options : options?.encoding;
  if (enc) {
    return str;
  }
  if (typeof Buffer !== "undefined") {
    return Buffer.from(str, "utf8");
  }
  return new TextEncoder().encode(str);
}

export function existsSync(p) {
  const key = normalizeVFSPath(p);
  const vfs = getVFS();
  if (key === "") return true;
  return (
    Object.prototype.hasOwnProperty.call(vfs.files, key) ||
    Object.prototype.hasOwnProperty.call(vfs.dirs, key)
  );
}

export function readFileSync(p, options) {
  const key = normalizeVFSPath(p);
  const vfs = getVFS();
  if (!Object.prototype.hasOwnProperty.call(vfs.files, key)) {
    throw makeENOENT("open", p);
  }
  return encodeContent(vfs.files[key], options);
}

export function statSync(p, opts) {
  const key = normalizeVFSPath(p);
  const vfs = getVFS();
  if (Object.prototype.hasOwnProperty.call(vfs.files, key)) {
    return makeStat(false, vfs.files[key].length);
  }
  if (key === "" || Object.prototype.hasOwnProperty.call(vfs.dirs, key)) {
    return makeStat(true, 0);
  }
  if (opts?.throwIfNoEntry === false) return undefined;
  throw makeENOENT("stat", p);
}

export function lstatSync(p, opts) {
  return statSync(p, opts);
}

export function realpathSync(p) {
  const key = normalizeVFSPath(p);
  if (!existsSync(key)) {
    throw makeENOENT("realpath", p);
  }
  return "/" + key;
}
realpathSync.native = realpathSync;

export function readdirSync(p, options) {
  const key = normalizeVFSPath(p);
  const vfs = getVFS();
  const entries = vfs.dirs[key];
  if (!entries) {
    throw makeENOENT("scandir", p);
  }
  if (options && typeof options === "object" && options.withFileTypes) {
    return entries.map((name) => {
      const childKey = key ? `${key}/${name}` : name;
      const isDir = Object.prototype.hasOwnProperty.call(vfs.dirs, childKey);
      return {
        name,
        isFile: () => !isDir,
        isDirectory: () => isDir,
        isSymbolicLink: () => false,
        isFIFO: () => false,
        isSocket: () => false,
        isCharacterDevice: () => false,
        isBlockDevice: () => false,
      };
    });
  }
  return [...entries];
}

export function accessSync(p) {
  if (!existsSync(p)) {
    throw makeENOENT("access", p);
  }
}

export function mkdirSync() {
  // No-op in read-only Worker VFS (Next occasionally calls mkdirSync for cache dirs).
}

export function writeFileSync() {
  // No-op in read-only Worker VFS.
}

export function unlinkSync() {}
export function rmSync() {}

export function createReadStream(p) {
  const data = readFileSync(p);
  return Readable.from([data]);
}

export function readFile(p, optsOrCb, maybeCb) {
  const cb = typeof optsOrCb === "function" ? optsOrCb : maybeCb;
  const opts = typeof optsOrCb === "function" ? undefined : optsOrCb;
  try {
    const out = readFileSync(p, opts);
    queueMicrotask(() => cb(null, out));
  } catch (err) {
    queueMicrotask(() => cb(err));
  }
}

export function stat(p, optsOrCb, maybeCb) {
  const cb = typeof optsOrCb === "function" ? optsOrCb : maybeCb;
  try {
    const out = statSync(p);
    queueMicrotask(() => cb(null, out));
  } catch (err) {
    queueMicrotask(() => cb(err));
  }
}

export function lstat(p, optsOrCb, maybeCb) {
  return stat(p, optsOrCb, maybeCb);
}

export function access(p, modeOrCb, maybeCb) {
  const cb = typeof modeOrCb === "function" ? modeOrCb : maybeCb;
  try {
    accessSync(p);
    queueMicrotask(() => cb(null));
  } catch (err) {
    queueMicrotask(() => cb(err));
  }
}

export function readdir(p, optsOrCb, maybeCb) {
  const cb = typeof optsOrCb === "function" ? optsOrCb : maybeCb;
  const opts = typeof optsOrCb === "function" ? undefined : optsOrCb;
  try {
    const out = readdirSync(p, opts);
    queueMicrotask(() => cb(null, out));
  } catch (err) {
    queueMicrotask(() => cb(err));
  }
}

export function realpath(p, optsOrCb, maybeCb) {
  const cb = typeof optsOrCb === "function" ? optsOrCb : maybeCb;
  try {
    const out = realpathSync(p);
    queueMicrotask(() => cb(null, out));
  } catch (err) {
    queueMicrotask(() => cb(err));
  }
}

export const constants = {
  F_OK: 0,
  R_OK: 4,
  W_OK: 2,
  X_OK: 1,
};

export const promises = {
  async readFile(p, options) {
    return readFileSync(p, options);
  },
  async stat(p, opts) {
    return statSync(p, opts);
  },
  async lstat(p, opts) {
    return lstatSync(p, opts);
  },
  async realpath(p) {
    return realpathSync(p);
  },
  async readdir(p, options) {
    return readdirSync(p, options);
  },
  async access(p) {
    accessSync(p);
  },
  async mkdir() {},
  async writeFile() {},
  async unlink() {},
  async rm() {},
  constants,
};

export default {
  normalizeVFSPath,
  existsSync,
  readFileSync,
  statSync,
  lstatSync,
  realpathSync,
  readdirSync,
  accessSync,
  mkdirSync,
  writeFileSync,
  unlinkSync,
  rmSync,
  createReadStream,
  readFile,
  stat,
  lstat,
  access,
  readdir,
  realpath,
  constants,
  promises,
};
