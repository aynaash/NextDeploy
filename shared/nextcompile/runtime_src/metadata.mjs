// App Router metadata → <head> tags.
//
// Why this exists: rsc.mjs composes the layout chain by hand rather than going
// through Next's app-render, so Next's metadata pipeline never executes. Every
// `export const metadata` and `export async function generateMetadata` in the
// app is simply absent from the response — pages ship with no <title>, no
// description, and no Open Graph tags at all.
//
// This resolves the chain the same way Next does (root → leaf, child wins) and
// renders the result as markup, which ssr.mjs splices into <head>.
//
// COVERAGE is a documented subset — the fields that affect SEO, link unfurling,
// and the browser tab, including `metadataBase` resolution for og/twitter image
// and canonical URLs. Not covered: file-convention metadata (opengraph-image.tsx
// et al, which needs a build-time scan), `alternates.languages`, `manifest`,
// `appleWebApp`, verification tokens. Unhandled keys are ignored, never crash.

/**
 * Walk the layout chain and the page, resolving each module's metadata and
 * merging root → leaf.
 *
 * @param {any[]} modules  layout modules root→leaf, page module LAST
 * @param {{params?: object, searchParams?: object}} props
 * @returns {Promise<object>} the merged metadata object
 */
export async function resolveMetadata(modules, props = {}) {
  let merged = {};
  for (const mod of modules) {
    if (!mod) continue;
    let own;
    try {
      own = await readModuleMetadata(mod, props, merged);
    } catch {
      // A throwing generateMetadata must not take the page down; Next would
      // surface it as an error boundary, and we have no equivalent yet.
      continue;
    }
    if (own && typeof own === "object") merged = mergeMetadata(merged, own);
  }
  return merged;
}

async function readModuleMetadata(mod, props, parentSoFar) {
  if (typeof mod.generateMetadata === "function") {
    // Next passes a `parent` promise as the 2nd arg; callers commonly await it
    // to extend inherited values. Resolved-so-far is the closest honest answer.
    return await mod.generateMetadata(props, Promise.resolve(parentSoFar));
  }
  if (mod.metadata && typeof mod.metadata === "object") return mod.metadata;
  return null;
}

/**
 * Merge a child's metadata over a parent's.
 *
 * Next's rule is per-top-level-key replacement, not a deep merge — a child
 * `openGraph` replaces the parent's wholesale rather than blending fields.
 * Title is the exception, because a parent's `template` has to be applied to
 * the child's string title.
 */
export function mergeMetadata(parent, child) {
  const out = { ...parent, ...child };
  out.title = mergeTitle(parent.title, child.title);
  if (out.title === undefined) delete out.title;
  return out;
}

function mergeTitle(parentTitle, childTitle) {
  const template = parentTitle && typeof parentTitle === "object" ? parentTitle.template : null;

  if (childTitle === undefined || childTitle === null) {
    // No child title: a parent's `default` applies, otherwise inherit as-is.
    if (parentTitle && typeof parentTitle === "object") {
      return parentTitle.default !== undefined ? parentTitle : parentTitle;
    }
    return parentTitle;
  }
  if (typeof childTitle === "object") {
    // `absolute` opts out of the parent template entirely.
    if (childTitle.absolute !== undefined) return childTitle;
    return childTitle;
  }
  if (template && typeof template === "string") {
    return template.includes("%s") ? template.replace("%s", childTitle) : template;
  }
  return childTitle;
}

/** Flatten a resolved title (string | {default,template,absolute}) to text. */
export function titleText(title) {
  if (title === undefined || title === null) return null;
  if (typeof title === "string") return title;
  if (typeof title === "object") {
    if (typeof title.absolute === "string") return title.absolute;
    if (typeof title.default === "string") return title.default;
  }
  return null;
}

// --- rendering --------------------------------------------------------------

const escapeText = (s) =>
  String(s).replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");

const escapeAttr = (s) =>
  String(s)
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");

/**
 * Resolve a possibly-relative URL against `metadataBase`.
 *
 * Next requires absolute URLs for og:image / twitter:image / canonical — most
 * link unfurlers (Slack, iMessage, Twitter) reject relative ones outright, so a
 * page with `images: ['/og.png']` and no resolution unfurls with no image at
 * all. `metadataBase` is what turns those into absolute URLs.
 *
 * Returns the input unchanged when there's no base or it's already absolute,
 * so this is safe to apply unconditionally.
 *
 * @param {string|URL|undefined} value
 * @param {string|URL|undefined} base  metadata.metadataBase
 */
export function resolveMetadataURL(value, base) {
  if (value === undefined || value === null) return value;
  const str = value instanceof URL ? value.href : String(value);
  if (!base) return str;
  // Already absolute (has a scheme, or protocol-relative) — leave it alone.
  if (/^[a-z][a-z0-9+.-]*:/i.test(str) || str.startsWith("//")) return str;
  try {
    return new URL(str, base instanceof URL ? base : String(base)).href;
  } catch {
    // A malformed base must not lose the URL entirely.
    return str;
  }
}

const metaName = (name, content) =>
  content === undefined || content === null || content === ""
    ? ""
    : `<meta name="${escapeAttr(name)}" content="${escapeAttr(content)}">`;

const metaProp = (prop, content) =>
  content === undefined || content === null || content === ""
    ? ""
    : `<meta property="${escapeAttr(prop)}" content="${escapeAttr(content)}">`;

/**
 * Render resolved metadata to head markup.
 *
 * Output order mirrors Next's: title first, then description and the rest.
 * Everything is escaped for its context — metadata is app-authored but
 * routinely interpolates user data (post titles, product names).
 *
 * @param {object} metadata
 * @returns {string} markup, or "" when there is nothing to emit
 */
export function renderMetadataTags(metadata) {
  if (!metadata || typeof metadata !== "object") return "";
  const base = metadata.metadataBase;
  const out = [];

  const title = titleText(metadata.title);
  if (title) out.push(`<title>${escapeText(title)}</title>`);

  out.push(metaName("description", metadata.description));

  if (Array.isArray(metadata.keywords)) {
    out.push(metaName("keywords", metadata.keywords.join(", ")));
  } else if (typeof metadata.keywords === "string") {
    out.push(metaName("keywords", metadata.keywords));
  }

  if (typeof metadata.robots === "string") out.push(metaName("robots", metadata.robots));
  if (typeof metadata.authors === "object" && metadata.authors) {
    for (const a of [].concat(metadata.authors)) {
      if (a?.name) out.push(metaName("author", a.name));
    }
  }
  if (metadata.themeColor) out.push(metaName("theme-color", metadata.themeColor));

  const canonical = resolveMetadataURL(metadata.alternates?.canonical, base);
  if (canonical) out.push(`<link rel="canonical" href="${escapeAttr(canonical)}">`);

  out.push(renderOpenGraph(metadata.openGraph, base));
  out.push(renderTwitter(metadata.twitter, base));
  out.push(renderIcons(metadata.icons));

  return out.filter(Boolean).join("");
}

function renderOpenGraph(og, base) {
  if (!og || typeof og !== "object") return "";
  const out = [
    metaProp("og:title", titleText(og.title)),
    metaProp("og:description", og.description),
    metaProp("og:url", resolveMetadataURL(og.url, base)),
    metaProp("og:site_name", og.siteName),
    metaProp("og:type", og.type),
    metaProp("og:locale", og.locale),
  ];
  // images: string | {url,...} | array of either
  for (const img of [].concat(og.images ?? [])) {
    const url = typeof img === "string" ? img : img?.url;
    if (!url) continue;
    out.push(metaProp("og:image", resolveMetadataURL(url, base)));
    if (typeof img === "object") {
      out.push(metaProp("og:image:width", img.width));
      out.push(metaProp("og:image:height", img.height));
      out.push(metaProp("og:image:alt", img.alt));
    }
  }
  return out.filter(Boolean).join("");
}

function renderTwitter(tw, base) {
  if (!tw || typeof tw !== "object") return "";
  const out = [
    metaName("twitter:card", tw.card),
    metaName("twitter:title", titleText(tw.title)),
    metaName("twitter:description", tw.description),
    metaName("twitter:site", tw.site),
    metaName("twitter:creator", tw.creator),
  ];
  for (const img of [].concat(tw.images ?? [])) {
    const url = typeof img === "string" ? img : img?.url;
    if (url) out.push(metaName("twitter:image", resolveMetadataURL(url, base)));
  }
  return out.filter(Boolean).join("");
}

function renderIcons(icons) {
  if (!icons) return "";
  // Shorthand: a bare string or array is the favicon.
  if (typeof icons === "string" || Array.isArray(icons)) {
    return []
      .concat(icons)
      .map((i) => {
        const url = typeof i === "string" ? i : i?.url;
        return url ? `<link rel="icon" href="${escapeAttr(url)}">` : "";
      })
      .filter(Boolean)
      .join("");
  }
  if (typeof icons !== "object") return "";
  const rels = { icon: "icon", shortcut: "shortcut icon", apple: "apple-touch-icon" };
  const out = [];
  for (const [key, rel] of Object.entries(rels)) {
    for (const i of [].concat(icons[key] ?? [])) {
      const url = typeof i === "string" ? i : i?.url;
      if (url) out.push(`<link rel="${rel}" href="${escapeAttr(url)}">`);
    }
  }
  return out.join("");
}
