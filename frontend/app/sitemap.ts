import type { MetadataRoute } from "next";
import { listDocSlugs } from "@/lib/docs-loader";

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const baseUrl = process.env.NEXT_PUBLIC_SITE_URL || "https://nextdeploy.org";
  const now = new Date();

  const staticRoutes: MetadataRoute.Sitemap = [
    {
      url: `${baseUrl}`,
      lastModified: now,
      changeFrequency: "daily",
      priority: 1.0,
    },
    {
      url: `${baseUrl}/docs`,
      lastModified: now,
      changeFrequency: "daily",
      priority: 0.9,
    },
    {
      url: `${baseUrl}/login`,
      lastModified: now,
      changeFrequency: "monthly",
      priority: 0.3,
    },
  ];

  try {
    const docSlugs = await listDocSlugs();
    const docRoutes: MetadataRoute.Sitemap = docSlugs.map((slugSegments) => {
      const slugPath = slugSegments.join("/");
      return {
        url: `${baseUrl}/docs/${slugPath}`,
        lastModified: now,
        changeFrequency: "weekly" as const,
        priority: 0.8,
      };
    });

    return [...staticRoutes, ...docRoutes];
  } catch {
    return staticRoutes;
  }
}
