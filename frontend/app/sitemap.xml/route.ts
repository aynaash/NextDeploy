import { flatDocs } from "@/lib/docs-config";

export const dynamic = "force-static";
export const revalidate = 3600;

export async function GET() {
  const baseUrl = process.env.NEXT_PUBLIC_SITE_URL || "https://nextdeploy.org";
  const today = new Date().toISOString().split("T")[0];

  const seen = new Set<string>();
  const entries: Array<{ loc: string; lastmod: string; changefreq: string; priority: string }> = [
    {
      loc: `${baseUrl}`,
      lastmod: today,
      changefreq: "daily",
      priority: "1.0",
    },
    {
      loc: `${baseUrl}/docs`,
      lastmod: today,
      changefreq: "daily",
      priority: "0.9",
    },
  ];

  seen.add("/");
  seen.add("/docs");

  for (const doc of flatDocs) {
    if (!seen.has(doc.href)) {
      seen.add(doc.href);
      entries.push({
        loc: `${baseUrl}${doc.href}`,
        lastmod: today,
        changefreq: "weekly",
        priority: "0.8",
      });
    }
  }

  const xmlUrls = entries
    .map(
      (e) => `  <url>
    <loc>${e.loc}</loc>
    <lastmod>${e.lastmod}</lastmod>
    <changefreq>${e.changefreq}</changefreq>
    <priority>${e.priority}</priority>
  </url>`
    )
    .join("\n");

  const sitemapXml = `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${xmlUrls}
</urlset>`;

  return new Response(sitemapXml, {
    status: 200,
    headers: {
      "Content-Type": "application/xml; charset=utf-8",
      "Cache-Control": "public, max-age=3600, s-maxage=3600",
    },
  });
}
