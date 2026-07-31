// Tests for App Router metadata resolution. Run with: node --test metadata.test.mjs
import { test } from "node:test";
import assert from "node:assert";

import {
  resolveMetadata,
  mergeMetadata,
  titleText,
  renderMetadataTags,
  resolveMetadataURL,
} from "./metadata.mjs";

// --- resolution -------------------------------------------------------------

test("resolveMetadata reads a static metadata export", async () => {
  const got = await resolveMetadata([{ metadata: { title: "Home", description: "d" } }]);
  assert.deepStrictEqual(got, { title: "Home", description: "d" });
});

test("resolveMetadata awaits generateMetadata and passes props", async () => {
  const mod = {
    async generateMetadata({ params }) {
      return { title: `Post ${params.id}` };
    },
  };
  const got = await resolveMetadata([mod], { params: { id: "42" } });
  assert.strictEqual(got.title, "Post 42");
});

test("resolveMetadata prefers generateMetadata over a static export", async () => {
  const mod = { metadata: { title: "static" }, generateMetadata: () => ({ title: "dynamic" }) };
  assert.strictEqual((await resolveMetadata([mod])).title, "dynamic");
});

test("resolveMetadata merges root to leaf, child winning", async () => {
  const got = await resolveMetadata([
    { metadata: { title: "Site", description: "site desc", robots: "index" } },
    { metadata: { description: "page desc" } },
  ]);
  assert.strictEqual(got.description, "page desc");
  assert.strictEqual(got.robots, "index", "unset child keys inherit");
});

// A throwing generateMetadata is common in real apps (a failed fetch). It must
// degrade to the inherited metadata, not 500 the page.
test("resolveMetadata survives a throwing generateMetadata", async () => {
  const got = await resolveMetadata([
    { metadata: { title: "Root" } },
    {
      generateMetadata() {
        throw new Error("upstream down");
      },
    },
  ]);
  assert.strictEqual(got.title, "Root");
});

test("resolveMetadata ignores modules with no metadata at all", async () => {
  const got = await resolveMetadata([{ default: () => null }, null, undefined]);
  assert.deepStrictEqual(got, {});
});

test("generateMetadata receives a parent promise resolving to metadata so far", async () => {
  let seen = null;
  await resolveMetadata([
    { metadata: { title: "Root", description: "rd" } },
    {
      async generateMetadata(_props, parent) {
        seen = await parent;
        return {};
      },
    },
  ]);
  assert.strictEqual(seen.description, "rd");
});

// --- title templates --------------------------------------------------------

test("mergeMetadata applies a parent title template to a child string title", () => {
  const got = mergeMetadata({ title: { default: "Site", template: "%s | Site" } }, { title: "Blog" });
  assert.strictEqual(titleText(got.title), "Blog | Site");
});

test("a child title.absolute bypasses the parent template", () => {
  const got = mergeMetadata(
    { title: { template: "%s | Site" } },
    { title: { absolute: "Standalone" } },
  );
  assert.strictEqual(titleText(got.title), "Standalone");
});

test("a child with no title falls back to the parent default", () => {
  const got = mergeMetadata({ title: { default: "Site", template: "%s | Site" } }, {});
  assert.strictEqual(titleText(got.title), "Site");
});

test("a template without %s replaces the title wholesale", () => {
  const got = mergeMetadata({ title: { template: "Fixed" } }, { title: "ignored" });
  assert.strictEqual(titleText(got.title), "Fixed");
});

test("titleText handles every title shape", () => {
  assert.strictEqual(titleText("plain"), "plain");
  assert.strictEqual(titleText({ absolute: "abs" }), "abs");
  assert.strictEqual(titleText({ default: "def" }), "def");
  assert.strictEqual(titleText(undefined), null);
  assert.strictEqual(titleText({ template: "%s" }), null);
});

// Next replaces a top-level key wholesale rather than deep-merging it.
test("mergeMetadata replaces openGraph rather than blending it", () => {
  const got = mergeMetadata(
    { openGraph: { title: "p", description: "pd", siteName: "S" } },
    { openGraph: { title: "c" } },
  );
  assert.deepStrictEqual(got.openGraph, { title: "c" });
});

// --- rendering --------------------------------------------------------------

test("renderMetadataTags emits title first, then description", () => {
  const out = renderMetadataTags({ title: "T", description: "D" });
  assert.strictEqual(out, '<title>T</title><meta name="description" content="D">');
});

test("renderMetadataTags returns empty for absent/empty metadata", () => {
  assert.strictEqual(renderMetadataTags({}), "");
  assert.strictEqual(renderMetadataTags(null), "");
  assert.strictEqual(renderMetadataTags(undefined), "");
});

// Metadata routinely interpolates user data — a post title, a product name.
test("renderMetadataTags escapes a </title> break-out", () => {
  const out = renderMetadataTags({ title: "</title><script>alert(1)</script>" });
  assert.ok(!out.includes("<script>"), out);
  assert.ok(out.includes("&lt;/title&gt;"), out);
});

test("renderMetadataTags escapes a quote break-out in an attribute", () => {
  const out = renderMetadataTags({ description: '"><script>alert(1)</script>' });
  assert.ok(!out.includes("<script>"), out);
  assert.ok(out.includes("&quot;"), out);
});

test("renderMetadataTags joins keyword arrays", () => {
  assert.ok(renderMetadataTags({ keywords: ["a", "b"] }).includes('content="a, b"'));
  assert.ok(renderMetadataTags({ keywords: "solo" }).includes('content="solo"'));
});

test("renderMetadataTags emits canonical from alternates", () => {
  const out = renderMetadataTags({ alternates: { canonical: "https://x.io/p" } });
  assert.strictEqual(out, '<link rel="canonical" href="https://x.io/p">');
});

test("renderMetadataTags emits Open Graph as property= not name=", () => {
  const out = renderMetadataTags({ openGraph: { title: "T", type: "website" } });
  assert.ok(out.includes('<meta property="og:title" content="T">'), out);
  assert.ok(out.includes('<meta property="og:type" content="website">'), out);
});

test("renderMetadataTags handles every openGraph image shape", () => {
  const str = renderMetadataTags({ openGraph: { images: "https://x/i.png" } });
  assert.ok(str.includes('property="og:image" content="https://x/i.png"'), str);

  const rich = renderMetadataTags({
    openGraph: { images: [{ url: "https://x/a.png", width: 1200, height: 630, alt: "A" }] },
  });
  assert.ok(rich.includes('property="og:image:width" content="1200"'), rich);
  assert.ok(rich.includes('property="og:image:alt" content="A"'), rich);
});

test("renderMetadataTags emits Twitter card tags as name=", () => {
  const out = renderMetadataTags({
    twitter: { card: "summary", title: "T", images: ["https://x/i.png"] },
  });
  assert.ok(out.includes('<meta name="twitter:card" content="summary">'), out);
  assert.ok(out.includes('<meta name="twitter:image" content="https://x/i.png">'), out);
});

test("renderMetadataTags supports icon shorthand and the rel map", () => {
  assert.strictEqual(
    renderMetadataTags({ icons: "/favicon.ico" }),
    '<link rel="icon" href="/favicon.ico">',
  );
  const full = renderMetadataTags({
    icons: { icon: "/i.png", shortcut: "/s.ico", apple: [{ url: "/a.png" }] },
  });
  assert.ok(full.includes('<link rel="icon" href="/i.png">'), full);
  assert.ok(full.includes('<link rel="shortcut icon" href="/s.ico">'), full);
  assert.ok(full.includes('<link rel="apple-touch-icon" href="/a.png">'), full);
});

test("renderMetadataTags skips empty-string and null values", () => {
  assert.strictEqual(renderMetadataTags({ description: "", robots: null }), "");
});

test("end to end: chain resolves and renders", async () => {
  const resolved = await resolveMetadata(
    [
      { metadata: { title: { default: "Acme", template: "%s | Acme" }, description: "root" } },
      {
        async generateMetadata({ params }) {
          return { title: `Post ${params.slug}`, openGraph: { title: "OG" } };
        },
      },
    ],
    { params: { slug: "hello" } },
  );
  const out = renderMetadataTags(resolved);
  assert.ok(out.startsWith("<title>Post hello | Acme</title>"), out);
  assert.ok(out.includes('content="root"'), out);
  assert.ok(out.includes('property="og:title" content="OG"'), out);
});

// --- metadataBase -----------------------------------------------------------

test("resolveMetadataURL makes a relative URL absolute against the base", () => {
  assert.strictEqual(
    resolveMetadataURL("/og.png", "https://acme.com"),
    "https://acme.com/og.png",
  );
  assert.strictEqual(
    resolveMetadataURL("og.png", "https://acme.com/blog/"),
    "https://acme.com/blog/og.png",
  );
});

test("resolveMetadataURL leaves already-absolute URLs alone", () => {
  for (const abs of ["https://cdn.io/a.png", "http://x/a.png", "//cdn.io/a.png", "data:image/png;base64,x"]) {
    assert.strictEqual(resolveMetadataURL(abs, "https://acme.com"), abs);
  }
});

test("resolveMetadataURL is a no-op without a base", () => {
  assert.strictEqual(resolveMetadataURL("/og.png", undefined), "/og.png");
});

test("resolveMetadataURL accepts URL objects for value and base", () => {
  assert.strictEqual(
    resolveMetadataURL("/a.png", new URL("https://acme.com")),
    "https://acme.com/a.png",
  );
  assert.strictEqual(
    resolveMetadataURL(new URL("https://x.io/a.png"), "https://acme.com"),
    "https://x.io/a.png",
  );
});

// A bad base must not silently drop the image entirely.
test("resolveMetadataURL falls back to the raw value on a malformed base", () => {
  assert.strictEqual(resolveMetadataURL("/og.png", "not a url"), "/og.png");
});

test("renderMetadataTags resolves og/twitter images and canonical against metadataBase", () => {
  const out = renderMetadataTags({
    metadataBase: "https://acme.com",
    alternates: { canonical: "/post/1" },
    openGraph: { url: "/post/1", images: ["/og.png"] },
    twitter: { images: [{ url: "/tw.png" }] },
  });
  assert.ok(out.includes('href="https://acme.com/post/1"'), out);
  assert.ok(out.includes('property="og:url" content="https://acme.com/post/1"'), out);
  assert.ok(out.includes('property="og:image" content="https://acme.com/og.png"'), out);
  assert.ok(out.includes('name="twitter:image" content="https://acme.com/tw.png"'), out);
});

test("renderMetadataTags leaves relative URLs relative when no metadataBase is set", () => {
  const out = renderMetadataTags({ openGraph: { images: ["/og.png"] } });
  assert.ok(out.includes('content="/og.png"'), out);
});
