import type { Metadata } from "next";
import Hero from "@/components/hero";
import Features from "@/components/features";
import Footer from "@/components/footer";

const siteUrl = process.env.NEXT_PUBLIC_SITE_URL || "https://nextdeploy.org";

export const metadata: Metadata = {
  alternates: {
    canonical: "/",
  },
};

const homeJsonLd = {
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "HowTo",
      name: "How to deploy Next.js with NextDeploy",
      description:
        "From your Next.js project to production infrastructure on Linux VPS, AWS, or Cloudflare Workers.",
      step: [
        {
          "@type": "HowToStep",
          position: 1,
          name: "Analyse",
          text: "Run nextdeploy build to inspect your Next.js output, classify routes, identify integrations, fingerprint assets, and generate deployment metadata.",
        },
        {
          "@type": "HowToStep",
          position: 2,
          name: "Validate",
          text: "Check deployment requirements, configuration, secrets, build constraints, and application metadata before shipping.",
        },
        {
          "@type": "HowToStep",
          position: 3,
          name: "Deploy",
          text: "Run nextdeploy ship to build when necessary, package your application, and deploy it to your selected infrastructure target.",
        },
        {
          "@type": "HowToStep",
          position: 4,
          name: "Verify",
          text: "Health checks, deployment status, release tracking, and rollback mechanisms keep production deployments reliable.",
        },
      ],
    },
    {
      "@type": "VideoObject",
      name: "How to deploy Next.js to a Linux VPS with NextDeploy",
      description:
        "Step-by-step tutorial showing zero-downtime Next.js deployment to a self-hosted Linux VPS using NextDeploy.",
      thumbnailUrl: "https://i.ytimg.com/vi/9C8WNZSS5o0/hqdefault.jpg",
      uploadDate: "2025-01-01T00:00:00Z",
      embedUrl: "https://www.youtube.com/embed/9C8WNZSS5o0",
      contentUrl: "https://www.youtube.com/watch?v=9C8WNZSS5o0",
      publisher: {
        "@id": `${siteUrl}/#organization`,
      },
    },
  ],
};

export default function Home() {
  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(homeJsonLd) }}
      />
      <main className="min-h-screen overflow-x-clip">
        <Hero />
        <Features />
        <Footer />
      </main>
    </>
  );
}
