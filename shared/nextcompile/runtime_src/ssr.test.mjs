// Tests for the SSR layer (Flight → HTML). Run with: node --test ssr.test.mjs
//
// Deliberately covers only the pure/compositional surface — manifest
// derivation, URL building, Flight framing, and stream composition. The
// renderToHtml path itself needs the vendored React builds, which exist only
// inside a compiled bundle; it stays integration-verified (curl a deployed
// route), exactly as the plan's M1 verification step describes.
import { test } from "node:test";
import assert from "node:assert";

import {
  buildSSRManifest,
  bootstrapScriptURLs,
  htmlEscapeJsonString,
  flightChunkScript,
  composeHtmlWithFlight,
  stylesheetLinkTags,
  createHeadInjector,
  renderToHtml,
  loadSSRDeps,
  ssrLoadFailure,
  FLIGHT_INIT,
} from "./ssr.mjs";

// --- helpers ----------------------------------------------------------------

function streamOf(...chunks) {
  const enc = new TextEncoder();
  return new ReadableStream({
    start(c) {
      for (const ch of chunks) c.enqueue(typeof ch === "string" ? enc.encode(ch) : ch);
      c.close();
    },
  });
}

function failingStream(err) {
  return new ReadableStream({
    start(c) {
      c.error(err);
    },
  });
}

async function readAll(stream) {
  const dec = new TextDecoder();
  const reader = stream.getReader();
  let out = "";
  for (;;) {
    const { value, done } = await reader.read();
    if (done) break;
    out += dec.decode(value, { stream: true });
  }
  return out + dec.decode();
}

// --- buildSSRManifest -------------------------------------------------------

// The regression this pins: a webpack build (what we force for Cloudflare)
// emits edgeSSRModuleMapping as {} and fills ssrModuleMapping. Preferring
// "edge" because it sounds more edge-appropriate yields an empty moduleMap,
// and every "use client" boundary fails to resolve at render time.
test("buildSSRManifest skips an EMPTY edgeSSRModuleMapping for the populated one", () => {
  const got = buildSSRManifest({
    edgeSSRModuleMapping: {},
    ssrModuleMapping: { 492: { "*": { id: "6166", chunks: [] } } },
    moduleLoading: { prefix: "/_next/", crossOrigin: null },
  });
  assert.deepStrictEqual(got.moduleMap, { 492: { "*": { id: "6166", chunks: [] } } });
  assert.deepStrictEqual(got.moduleLoading, { prefix: "/_next/", crossOrigin: null });
});

test("buildSSRManifest prefers edge mapping when it is actually populated", () => {
  const got = buildSSRManifest({
    edgeSSRModuleMapping: { 1: { "*": { id: "edge" } } },
    ssrModuleMapping: { 2: { "*": { id: "node" } } },
  });
  assert.deepStrictEqual(got.moduleMap, { 1: { "*": { id: "edge" } } });
});

test("buildSSRManifest unwraps a JSON module's .default", () => {
  const got = buildSSRManifest({ default: { ssrModuleMapping: { 7: {} }, moduleLoading: null } });
  assert.deepStrictEqual(got.moduleMap, { 7: {} });
});

test("buildSSRManifest tolerates null/empty manifests", () => {
  for (const input of [null, undefined, {}, { default: null }]) {
    const got = buildSSRManifest(input);
    assert.deepStrictEqual(got.moduleMap, {}, `input: ${JSON.stringify(input)}`);
    assert.strictEqual(got.moduleLoading, null);
  }
});

// --- bootstrapScriptURLs ----------------------------------------------------

test("bootstrapScriptURLs maps dist-relative chunks onto /_next/ URLs in order", () => {
  const got = bootstrapScriptURLs([
    "static/chunks/webpack-5adebf9f62dc3001.js",
    "static/chunks/main-app-da1490cb1b408188.js",
    "static/chunks/app/page-e1e2856be569970d.js",
  ]);
  assert.deepStrictEqual(got, [
    "/_next/static/chunks/webpack-5adebf9f62dc3001.js",
    "/_next/static/chunks/main-app-da1490cb1b408188.js",
    "/_next/static/chunks/app/page-e1e2856be569970d.js",
  ]);
});

test("bootstrapScriptURLs honours assetPrefix and strips its trailing slash", () => {
  const got = bootstrapScriptURLs(["static/chunks/a.js"], {
    assetPrefix: "https://cdn.example.com/",
  });
  assert.deepStrictEqual(got, ["https://cdn.example.com/_next/static/chunks/a.js"]);
});

test("bootstrapScriptURLs falls back to basePath, and assetPrefix wins over it", () => {
  assert.deepStrictEqual(bootstrapScriptURLs(["static/a.js"], { basePath: "/docs" }), [
    "/docs/_next/static/a.js",
  ]);
  assert.deepStrictEqual(
    bootstrapScriptURLs(["static/a.js"], { basePath: "/docs", assetPrefix: "https://cdn.io" }),
    ["https://cdn.io/_next/static/a.js"],
  );
});

test("bootstrapScriptURLs never emits a doubled slash", () => {
  const got = bootstrapScriptURLs(["/static/chunks/a.js"], { assetPrefix: "/p/" });
  assert.deepStrictEqual(got, ["/p/_next/static/chunks/a.js"]);
});

test("bootstrapScriptURLs returns [] for empty/absent input", () => {
  assert.deepStrictEqual(bootstrapScriptURLs([]), []);
  assert.deepStrictEqual(bootstrapScriptURLs(undefined), []);
  assert.deepStrictEqual(bootstrapScriptURLs(null), []);
});

// --- Flight framing ---------------------------------------------------------

test("htmlEscapeJsonString neutralizes script-terminating and line-separator chars", () => {
  assert.strictEqual(htmlEscapeJsonString("<"), "\\u003c");
  assert.strictEqual(htmlEscapeJsonString(">"), "\\u003e");
  assert.strictEqual(htmlEscapeJsonString("&"), "\\u0026");
  assert.strictEqual(htmlEscapeJsonString("\u2028"), "\\u2028");
  assert.strictEqual(htmlEscapeJsonString("\u2029"), "\\u2029");
  assert.strictEqual(htmlEscapeJsonString("plain"), "plain");
});

// The XSS case: RSC payload data containing a literal </script> must not be
// able to close the inlining <script> element.
test("flightChunkScript cannot be broken out of with </script>", () => {
  const out = flightChunkScript('4:["</script><img src=x onerror=alert(1)>"]');
  assert.ok(!out.slice(0, -"</script>".length).includes("</script>"), `leaked: ${out}`);
  assert.ok(out.includes("\\u003c/script\\u003e"), `not escaped: ${out}`);
  assert.ok(out.endsWith("</script>"));
});

test("flightChunkScript emits the self-initializing push form", () => {
  // Every chunk re-initializes because our scripts interleave with React's
  // stream and can precede bootstrapScriptContent.
  const out = flightChunkScript("1:hello");
  assert.ok(out.startsWith("<script>(self.__next_f=self.__next_f||[]).push([1,"), out);
  assert.ok(out.includes('"1:hello"'), out);
});

test("flightChunkScript threads a CSP nonce onto the tag", () => {
  const out = flightChunkScript("x", "abc123");
  assert.ok(out.startsWith('<script nonce="abc123">'), out);
});

test("FLIGHT_INIT is the [0] bootstrap marker and is self-initializing", () => {
  assert.strictEqual(FLIGHT_INIT, "(self.__next_f=self.__next_f||[]).push([0])");
});

// --- composeHtmlWithFlight --------------------------------------------------

test("composeHtmlWithFlight emits HTML first, then the flight payload", async () => {
  const html = streamOf("<!DOCTYPE html><html><body>", "<div>hi</div></body></html>");
  const flight = streamOf('0:["hello"]\n');
  const out = await readAll(composeHtmlWithFlight(html, flight));

  assert.ok(out.startsWith("<!DOCTYPE html>"), `must not precede the doctype:\n${out}`);
  assert.ok(out.includes("<div>hi</div>"), out);
  assert.ok(out.includes("self.__next_f"), out);
  // No flight script may appear before the document has opened — a <script>
  // ahead of <!DOCTYPE html> puts the browser into quirks mode.
  assert.ok(out.indexOf("self.__next_f") > out.indexOf("<!DOCTYPE html>"), out);
});

test("composeHtmlWithFlight emits one script per flight chunk", async () => {
  const html = streamOf("<!DOCTYPE html><html></html>");
  const flight = streamOf("1:a\n", "2:b\n", "3:c\n");
  const out = await readAll(composeHtmlWithFlight(html, flight));
  assert.strictEqual(out.match(/self\.__next_f=self\.__next_f\|\|\[\]/g)?.length, 3, out);
  for (const frag of ['"1:a\\n"', '"2:b\\n"', '"3:c\\n"']) {
    assert.ok(out.includes(frag), `missing ${frag} in:\n${out}`);
  }
});

// Regression guard on the markHtmlStarted gate: it is resolved in a finally
// block precisely so an empty HTML stream can't deadlock the flight pump.
test("composeHtmlWithFlight does not hang when the HTML stream is empty", async () => {
  const out = await readAll(composeHtmlWithFlight(streamOf(), streamOf("1:x\n")));
  assert.ok(out.includes('"1:x\\n"'), out);
});

test("composeHtmlWithFlight completes when there is no flight data", async () => {
  const out = await readAll(composeHtmlWithFlight(streamOf("<html></html>"), streamOf()));
  assert.strictEqual(out, "<html></html>");
});

test("composeHtmlWithFlight reassembles a multi-byte char split across chunks", async () => {
  // "€" is E2 82 AC; splitting it must not yield replacement characters.
  const euro = new TextEncoder().encode("0:\u20ac\n");
  const flight = streamOf(euro.slice(0, 3), euro.slice(3));
  const out = await readAll(composeHtmlWithFlight(streamOf("<html>"), flight));
  assert.ok(out.includes("\u20ac"), `lost the multi-byte char:\n${out}`);
  assert.ok(!out.includes("\ufffd"), `replacement char present:\n${out}`);
});

test("composeHtmlWithFlight surfaces an HTML stream error", async () => {
  const boom = new Error("render exploded");
  await assert.rejects(
    () => readAll(composeHtmlWithFlight(failingStream(boom), streamOf("1:x\n"))),
    /render exploded/,
  );
});

test("composeHtmlWithFlight surfaces a flight stream error", async () => {
  const boom = new Error("flight exploded");
  await assert.rejects(
    () => readAll(composeHtmlWithFlight(streamOf("<html>"), failingStream(boom))),
    /flight exploded/,
  );
});

// --- stylesheets (M2) -------------------------------------------------------

test("stylesheetLinkTags builds cascade-ordered <link> tags", () => {
  const out = stylesheetLinkTags(["static/css/root.css", "static/css/page.css"]);
  assert.strictEqual(
    out,
    '<link rel="stylesheet" href="/_next/static/css/root.css">' +
      '<link rel="stylesheet" href="/_next/static/css/page.css">',
  );
});

test("stylesheetLinkTags honours assetPrefix and nonce", () => {
  const out = stylesheetLinkTags(["static/css/a.css"], {
    assetPrefix: "https://cdn.io",
    nonce: "n1",
  });
  assert.strictEqual(out, '<link rel="stylesheet" href="https://cdn.io/_next/static/css/a.css" nonce="n1">');
});

test("stylesheetLinkTags returns empty string when there is no CSS", () => {
  assert.strictEqual(stylesheetLinkTags([]), "");
  assert.strictEqual(stylesheetLinkTags(undefined), "");
});

// assetPrefix is user-supplied config spliced into a raw HTML attribute.
test("stylesheetLinkTags escapes an attribute break-out in assetPrefix", () => {
  const out = stylesheetLinkTags(["a.css"], { assetPrefix: '"><script>alert(1)</script>' });
  assert.ok(!out.includes("<script>"), `attribute escaped out: ${out}`);
  assert.ok(out.includes("&quot;"), out);
});

test("createHeadInjector splices markup directly after <head>", () => {
  const inj = createHeadInjector("<link rel=stylesheet>");
  const enc = new TextEncoder();
  const dec = new TextDecoder();
  const out = inj.push(enc.encode("<!DOCTYPE html><html><head><title>x</title></head>"));
  assert.strictEqual(
    dec.decode(out[0]),
    "<!DOCTYPE html><html><head><link rel=stylesheet><title>x</title></head>",
  );
});

test("createHeadInjector handles <head> with attributes but not <header>", () => {
  const enc = new TextEncoder();
  const dec = new TextDecoder();

  const withAttrs = createHeadInjector("XX");
  assert.strictEqual(
    dec.decode(withAttrs.push(enc.encode('<html><head data-x="1">y'))[0]),
    '<html><head data-x="1">XXy',
  );

  // <header> must not be mistaken for <head> — a body-only fragment should
  // fall through to the give-up path instead of injecting mid-element.
  const headerOnly = createHeadInjector("XX", 16);
  const emitted = headerOnly.push(enc.encode("<header>a long body with no head</header>"));
  assert.ok(!dec.decode(emitted[0] ?? new Uint8Array()).includes("XX"));
});

test("createHeadInjector buffers across a chunk boundary that splits <head>", () => {
  const inj = createHeadInjector("XX");
  const enc = new TextEncoder();
  const dec = new TextDecoder();
  assert.deepStrictEqual(inj.push(enc.encode("<html><he")), []); // withheld
  const out = inj.push(enc.encode("ad><title>t</title>"));
  assert.strictEqual(dec.decode(out[0]), "<html><head>XX<title>t</title>");
});

test("createHeadInjector gives up past its budget and streams verbatim", () => {
  const inj = createHeadInjector("XX", 8);
  const enc = new TextEncoder();
  const dec = new TextDecoder();
  const out = inj.push(enc.encode("no head tag here at all"));
  assert.strictEqual(dec.decode(out[0]), "no head tag here at all");
  // Subsequent chunks pass straight through.
  assert.strictEqual(dec.decode(inj.push(enc.encode("more"))[0]), "more");
});

test("createHeadInjector with no markup is a pass-through", () => {
  const inj = createHeadInjector("");
  const enc = new TextEncoder();
  const chunk = enc.encode("<html><head>");
  assert.deepStrictEqual(inj.push(chunk), [chunk]);
});

test("composeHtmlWithFlight injects stylesheets into the head it streams", async () => {
  const html = streamOf("<!DOCTYPE html><html><head>", "</head><body>hi</body></html>");
  const out = await readAll(
    composeHtmlWithFlight(html, streamOf("1:x\n"), {
      headMarkup: '<link rel="stylesheet" href="/_next/static/css/a.css">',
    }),
  );
  assert.ok(out.includes('<head><link rel="stylesheet" href="/_next/static/css/a.css">'), out);
  // Still emits the flight payload, and still after the doctype.
  assert.ok(out.indexOf("self.__next_f") > out.indexOf("<!DOCTYPE html>"), out);
});

// The gate lives on emitted bytes, not on read chunks: the injector withholds
// early chunks, so gating on reads would let a flight <script> out first.
test("composeHtmlWithFlight keeps flight scripts after the doctype while injecting", async () => {
  const html = streamOf("<!DOCT", "YPE html><html><head></head><body>x</body></html>");
  const out = await readAll(
    composeHtmlWithFlight(html, streamOf("1:a\n"), { headMarkup: "<style>b{}</style>" }),
  );
  assert.ok(out.startsWith("<!DOCTYPE html>"), out);
  assert.ok(out.includes("<style>b{}</style>"), out);
});

// --- degradation ------------------------------------------------------------
//
// These run LAST: loadSSRDeps memoizes its failure, so once this file has
// probed for the (absent) vendor directory the negative result is cached.
// The source tree has no runtime_src/vendor/react-dom — vendoring populates it
// only inside a compiled bundle — so this exercises the real "older bundle
// without SSR builds" path rather than a simulated one.

test("loadSSRDeps returns null instead of throwing when vendor builds are absent", async () => {
  assert.strictEqual(await loadSSRDeps(), null);
  assert.ok(ssrLoadFailure(), "expected the load failure to be recorded for diagnostics");
});

test("renderToHtml returns null so callers can degrade to the Flight shell", async () => {
  const flight = streamOf("1:x\n");
  assert.strictEqual(await renderToHtml(flight, { scripts: [] }), null);
  // The stream must be left untouched — rsc.mjs still needs it for the shell.
  assert.strictEqual(flight.locked, false, "renderToHtml must not lock the stream it bailed on");
});
