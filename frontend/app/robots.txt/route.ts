export const dynamic = "force-static";
export const revalidate = 3600;

export async function GET() {
  const baseUrl = process.env.NEXT_PUBLIC_SITE_URL || "https://nextdeploy.org";

  const robotsTxt = `User-agent: *
Allow: /
Disallow: /api/
Disallow: /app/

Host: ${baseUrl}
Sitemap: ${baseUrl}/sitemap.xml
`;

  return new Response(robotsTxt, {
    status: 200,
    headers: {
      "Content-Type": "text/plain; charset=utf-8",
      "Cache-Control": "public, max-age=3600, s-maxage=3600",
    },
  });
}
