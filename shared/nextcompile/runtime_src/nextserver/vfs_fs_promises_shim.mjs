// `fs/promises` / `node:fs/promises` entry backed by the in-memory VFS.
import { promises, constants } from "./vfs_fs_shim.mjs";

export const {
  readFile,
  stat,
  lstat,
  realpath,
  readdir,
  access,
  mkdir,
  writeFile,
  unlink,
  rm,
} = promises;

export { constants };
export default promises;
