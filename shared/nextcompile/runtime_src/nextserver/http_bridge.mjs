// Web Fetch <-> Node HTTP streaming bridge for running NextNodeServer inside
// Cloudflare Workers (workerd with nodejs_compat).
//
// Converts an incoming Fetch `Request` into a Node `IncomingMessage`-compatible
// Readable stream and pipes Node `ServerResponse` writes directly into a Web
// `ReadableStream` `Response` as soon as headers are flushed — giving true
// zero-buffering progressive streaming for App Router Suspense & loading.tsx.

import { Readable } from "node:stream";
import { EventEmitter } from "node:events";

const NULL_BODY_STATUSES = new Set([101, 204, 205, 304]);

/**
 * Build a Node IncomingMessage-compatible Readable from a Fetch Request.
 */
export function requestToIncomingMessage(request) {
  const url = new URL(request.url);
  const pathWithQuery = url.pathname + url.search;

  const headers = {};
  const rawHeaders = [];
  for (const [k, v] of request.headers.entries()) {
    const lower = k.toLowerCase();
    rawHeaders.push(k, v);
    if (lower === "set-cookie") {
      if (!headers[lower]) headers[lower] = [];
      headers[lower].push(v);
    } else if (headers[lower] !== undefined) {
      headers[lower] = `${headers[lower]}, ${v}`;
    } else {
      headers[lower] = v;
    }
  }
  if (!headers.host) {
    headers.host = url.host;
    rawHeaders.push("host", url.host);
  }

  let reader = null;
  let bodyDone = !request.body || request.method === "GET" || request.method === "HEAD";

  const req = new Readable({
    async read() {
      if (bodyDone) {
        this.push(null);
        return;
      }
      try {
        if (!reader) {
          reader = request.body.getReader();
        }
        const { done, value } = await reader.read();
        if (done) {
          bodyDone = true;
          this.push(null);
        } else {
          this.push(Buffer.from(value.buffer, value.byteOffset, value.byteLength));
        }
      } catch (err) {
        this.destroy(err);
      }
    },
  });

  req.method = request.method;
  req.url = pathWithQuery;
  req.headers = headers;
  req.rawHeaders = rawHeaders;
  req.httpVersion = "1.1";
  req.httpVersionMajor = 1;
  req.httpVersionMinor = 1;
  req.complete = bodyDone;
  req.socket = {
    encrypted: url.protocol === "https:",
    remoteAddress: request.headers.get("cf-connecting-ip") || "127.0.0.1",
    remotePort: 443,
    destroyed: false,
    setTimeout() {},
    setNoDelay() {},
    setKeepAlive() {},
  };
  req.connection = req.socket;
  req.on("end", () => {
    req.complete = true;
  });

  return req;
}

/**
 * Create a Node ServerResponse-compatible writable object paired with a
 * Promise<Response> that resolves as soon as headers are flushed (so
 * streaming bodies begin transmitting immediately).
 */
export function createStreamingServerResponse(method = "GET") {
  const emitter = new EventEmitter();
  const encoder = new TextEncoder();

  let statusCode = 200;
  let statusMessage = "";
  let headersSent = false;
  let writableEnded = false;
  let writableFinished = false;
  let streamController = null;
  let streamClosed = false;
  const headerMap = new Map(); // lowerName -> { name, value }

  let resolveResponse;
  const responsePromise = new Promise((resolve) => {
    resolveResponse = resolve;
  });

  const bodyStream = new ReadableStream({
    start(controller) {
      streamController = controller;
    },
    cancel() {
      streamClosed = true;
      emitter.emit("close");
    },
  });

  function toWebHeaders() {
    const out = new Headers();
    for (const { name, value } of headerMap.values()) {
      if (Array.isArray(value)) {
        for (const item of value) {
          out.append(name, String(item));
        }
      } else if (value !== undefined) {
        out.set(name, String(value));
      }
    }
    return out;
  }

  function commitHeaders() {
    if (headersSent) return;
    headersSent = true;
    const isNullBody = method === "HEAD" || NULL_BODY_STATUSES.has(statusCode);
    const webHeaders = toWebHeaders();
    if (isNullBody) {
      closeStream();
      resolveResponse(
        new Response(null, {
          status: statusCode,
          statusText: statusMessage || undefined,
          headers: webHeaders,
        }),
      );
      return;
    }
    resolveResponse(
      new Response(bodyStream, {
        status: statusCode,
        statusText: statusMessage || undefined,
        headers: webHeaders,
      }),
    );
  }

  function toUint8Array(chunk, encoding) {
    if (chunk == null) return null;
    if (chunk instanceof Uint8Array) {
      return new Uint8Array(chunk.buffer, chunk.byteOffset, chunk.byteLength);
    }
    if (typeof chunk === "string") {
      if (encoding && encoding !== "utf8" && encoding !== "utf-8" && typeof Buffer !== "undefined") {
        const b = Buffer.from(chunk, encoding);
        return new Uint8Array(b.buffer, b.byteOffset, b.byteLength);
      }
      return encoder.encode(chunk);
    }
    return encoder.encode(String(chunk));
  }

  function enqueueChunk(chunk, encoding) {
    if (streamClosed || method === "HEAD" || NULL_BODY_STATUSES.has(statusCode)) return;
    const bytes = toUint8Array(chunk, encoding);
    if (bytes && bytes.byteLength > 0) {
      streamController?.enqueue(bytes);
    }
  }

  function closeStream() {
    if (streamClosed) return;
    streamClosed = true;
    try {
      streamController?.close();
    } catch {
      // Already closed/errored
    }
  }

  const res = Object.defineProperties(
    emitter,
    Object.getOwnPropertyDescriptors({
      get statusCode() {
        return statusCode;
      },
      set statusCode(code) {
        statusCode = Number(code) || 200;
      },
      get statusMessage() {
        return statusMessage;
      },
      set statusMessage(msg) {
        statusMessage = String(msg || "");
      },
      get headersSent() {
        return headersSent;
      },
      get writableEnded() {
        return writableEnded;
      },
      get writableFinished() {
        return writableFinished;
      },
      get finished() {
        return writableFinished;
      },

      setHeader(name, value) {
        headerMap.set(String(name).toLowerCase(), { name: String(name), value });
        return res;
      },
      appendHeader(name, value) {
        const lower = String(name).toLowerCase();
        const existing = headerMap.get(lower);
        if (!existing) {
          headerMap.set(lower, { name: String(name), value });
        } else {
          const prev = Array.isArray(existing.value) ? existing.value : [existing.value];
          const next = Array.isArray(value) ? value : [value];
          existing.value = [...prev, ...next];
        }
        return res;
      },
      getHeader(name) {
        return headerMap.get(String(name).toLowerCase())?.value;
      },
      getHeaderNames() {
        return Array.from(headerMap.keys());
      },
      getHeaders() {
        const obj = Object.create(null);
        for (const [k, { value }] of headerMap) obj[k] = value;
        return obj;
      },
      hasHeader(name) {
        return headerMap.has(String(name).toLowerCase());
      },
      removeHeader(name) {
        headerMap.delete(String(name).toLowerCase());
      },

      writeHead(code, reasonOrHeaders, maybeHeaders) {
        statusCode = Number(code) || 200;
        let hdrs = maybeHeaders;
        if (typeof reasonOrHeaders === "string") {
          statusMessage = reasonOrHeaders;
        } else if (reasonOrHeaders && typeof reasonOrHeaders === "object") {
          hdrs = reasonOrHeaders;
        }
        if (hdrs) {
          if (Array.isArray(hdrs)) {
            for (let i = 0; i + 1 < hdrs.length; i += 2) {
              res.appendHeader(hdrs[i], hdrs[i + 1]);
            }
          } else {
            for (const [k, v] of Object.entries(hdrs)) {
              if (v !== undefined) res.setHeader(k, v);
            }
          }
        }
        commitHeaders();
        return res;
      },

      flushHeaders() {
        commitHeaders();
      },

      write(chunk, encodingOrCb, maybeCb) {
        const encoding = typeof encodingOrCb === "string" ? encodingOrCb : undefined;
        const cb = typeof encodingOrCb === "function" ? encodingOrCb : maybeCb;
        if (!headersSent) commitHeaders();
        enqueueChunk(chunk, encoding);
        if (typeof cb === "function") queueMicrotask(cb);
        return true;
      },

      end(chunkOrCb, encodingOrCb, maybeCb) {
        if (writableEnded) return res;
        let chunk = chunkOrCb;
        let encoding = encodingOrCb;
        let cb = maybeCb;
        if (typeof chunkOrCb === "function") {
          cb = chunkOrCb;
          chunk = undefined;
          encoding = undefined;
        } else if (typeof encodingOrCb === "function") {
          cb = encodingOrCb;
          encoding = undefined;
        }

        if (!headersSent) commitHeaders();
        if (chunk !== undefined) {
          enqueueChunk(chunk, encoding);
        }
        writableEnded = true;
        writableFinished = true;
        closeStream();
        emitter.emit("finish");
        emitter.emit("close");
        if (typeof cb === "function") queueMicrotask(cb);
        return res;
      },

      cork() {},
      uncork() {},
      destroy(err) {
        if (!streamClosed) {
          streamClosed = true;
          if (err) {
            try {
              streamController?.error(err);
            } catch {}
          } else {
            try {
              streamController?.close();
            } catch {}
          }
        }
        if (!headersSent) {
          headersSent = true;
          resolveResponse(new Response(err ? String(err) : "Internal Server Error", { status: 500 }));
        }
        emitter.emit("close");
        return res;
      },
    }),
  );

  return { res, responsePromise };
}

/**
 * Execute a Node-style `(req, res) => Promise<void>` handler (such as
 * NextNodeServer's `getRequestHandler()`) against a Fetch Request and return
 * a streaming Web Response.
 */
export async function invokeNodeHandler(handler, request, ctx) {
  const req = requestToIncomingMessage(request);
  const { res, responsePromise } = createStreamingServerResponse(request.method);

  const handlerPromise = Promise.resolve()
    .then(() => handler(req, res))
    .then(() => {
      if (!res.writableEnded) {
        res.end();
      }
    })
    .catch((err) => {
      console.error("[nextcompile nextserver handler error]", err?.stack || String(err));
      if (!res.headersSent) {
        res.statusCode = 500;
        res.setHeader("content-type", "text/plain; charset=utf-8");
        res.end("Internal Server Error");
      } else {
        res.destroy(err);
      }
    });

  // Keep the Worker isolate alive while the streaming response finishes.
  if (typeof ctx?.waitUntil === "function") {
    ctx.waitUntil(handlerPromise);
  }

  return await responsePromise;
}
