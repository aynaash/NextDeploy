// Shim for Next.js internal async storages (work, action, request).
// Bridges Next's internal storage accessors to nextcompile's ALS context.
import { AsyncLocalStorage } from "node:async_hooks";
import { getContext } from "../context.mjs";

export const workAsyncStorage = new AsyncLocalStorage();
export const actionAsyncStorage = new AsyncLocalStorage();

export const requestAsyncStorage = {
  getStore() {
    try {
      const ctx = getContext();
      return {
        headers: ctx.responseHeaders,
        cookies: ctx.request?.headers?.get("cookie") || "",
        mutableCookies: ctx.setCookies,
        draftMode: ctx.draft,
      };
    } catch {
      return undefined;
    }
  },
  run(store, fn) {
    return fn();
  },
};

export const afterTaskAsyncStorage = new AsyncLocalStorage();
