export interface GalleryItem {
  id: string;
  title: string;
  tag: string;
  summary: string;
  details: string;
}

export const GALLERY_ITEMS: GalleryItem[] = [
  {
    id: "worker-01",
    title: "NextNodeServer Worker Bridge",
    tag: "Compute",
    summary: "Streams App Router RSC responses through a Web↔Node HTTP bridge.",
    details:
      "Runs Next.js's own NextNodeServer inside Cloudflare Workers using a build-time Virtual Filesystem (vfs.mjs) and chunk registry (chunks.mjs).",
  },
  {
    id: "r2-assets",
    title: "Fingerprinted R2 Static Assets",
    tag: "Storage",
    summary: "Content-hashed chunks shared; mutable assets fingerprinted per release.",
    details:
      "Mutable public and prerendered HTML files are stored under deterministic release prefixes in R2 so instant Worker rollbacks read matching assets.",
  },
  {
    id: "kv-guard",
    title: "KV Edge Protection Guard",
    tag: "Security",
    summary: "Rate-limiting and HMAC session auth executed ahead of app code.",
    details:
      "Generated from nextdeploy.yml protection rules: guards /app/* with session cookies and enforces 60 req/min rate limits on /api/* via RATE_LIMIT KV.",
  },
];

export function findGalleryItem(id: string): GalleryItem {
  return (
    GALLERY_ITEMS.find((item) => item.id === id) ?? {
      id,
      title: `Custom Item: ${id}`,
      tag: "Dynamic",
      summary: "Dynamically matched intercepting route item.",
      details:
        "Even arbitrary item IDs not listed in the preset gallery resolve cleanly in both modal interception and direct full-page SSR.",
    }
  );
}
