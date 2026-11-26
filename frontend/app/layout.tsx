import React from "react";
import type { Metadata, Viewport } from "next";
import { Geist, Geist_Mono } from "next/font/google";
import { Analytics } from "@vercel/analytics/next";
import Navbar from "@/components/navbar";
import "./globals.css";

const geist = Geist({
  subsets: ["latin"],
  variable: "--font-geist-sans",
});
const geistMono = Geist_Mono({
  subsets: ["latin"],
  variable: "--font-geist-mono",
});

const siteUrl = process.env.NEXT_PUBLIC_SITE_URL || "https://nextdeploy.org";

export const viewport: Viewport = {
  themeColor: [
    { media: "(prefers-color-scheme: dark)", color: "#020617" },
    { media: "(prefers-color-scheme: light)", color: "#020617" },
  ],
  width: "device-width",
  initialScale: 1,
  maximumScale: 5,
  colorScheme: "dark",
};

export const metadata: Metadata = {
  metadataBase: new URL(siteUrl),
  title: {
    default: "NextDeploy — Self-Hosted Next.js Deployment Engine",
    template: "%s | NextDeploy",
  },
  description:
    "Open-source self-hosted Next.js deployment engine. Deploy Next.js to Linux VPS, AWS Lambda, or Cloudflare Workers with zero-downtime rollouts and rollback.",
  applicationName: "NextDeploy",
  authors: [
    { name: "Yussuf Hersi", url: "https://twitter.com/hersiyussuf" },
    { name: "NextDeploy Team", url: siteUrl },
  ],
  generator: "Next.js",
  keywords: [
    "NextDeploy",
    "Next.js",
    "self-host Next.js",
    "self-hosted",
    "Next.js deployment",
    "Next.js deployment engine",
    "open-source Vercel alternative",
    "Vercel alternative",
    "deploy Next.js to VPS",
    "deploy Next.js to AWS Lambda",
    "deploy Next.js to Cloudflare Workers",
    "Linux VPS",
    "Cloudflare Workers",
    "AWS Lambda",
    "atomic deployments",
    "zero-downtime",
    "ISR",
    "build intelligence",
    "Caddy",
    "Caddy reverse proxy",
    "Coraza WAF",
    "systemd",
    "Turbopack",
    "Agent Trace",
    "open-source devops",
    "Docker-free deployment",
    "Next.js 15",
    "Next.js 16",
  ],
  referrer: "origin-when-cross-origin",
  creator: "NextDeploy",
  publisher: "NextDeploy",
  formatDetection: {
    email: false,
    address: false,
    telephone: false,
  },
  alternates: {
    canonical: "/",
    languages: {
      "en-US": "/",
    },
  },
  openGraph: {
    type: "website",
    locale: "en_US",
    url: siteUrl,
    siteName: "NextDeploy",
    title: "NextDeploy — Self-Hosted Next.js Deployment Engine",
    description:
      "Open-source self-hosted Next.js deployment engine. Deploy Next.js to Linux VPS, AWS Lambda, or Cloudflare Workers with zero-downtime rollouts and rollback.",
    images: [
      {
        url: "/og.png",
        width: 1200,
        height: 630,
        alt: "NextDeploy — Self-Hosted Next.js Deployment Engine",
        type: "image/png",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "NextDeploy — Self-Hosted Next.js Deployment Engine",
    description:
      "Open-source self-hosted Next.js deployment engine. Deploy Next.js to Linux VPS, AWS Lambda, or Cloudflare Workers with zero-downtime rollouts and rollback.",
    creator: "@hersiyussuf",
    site: "@hersiyussuf",
    images: ["/og.png"],
  },
  icons: {
    icon: [
      { url: "/favicon.ico", sizes: "any" },
      { url: "/icon.svg", type: "image/svg+xml" },
      { url: "/favicon-32x32.png", sizes: "32x32", type: "image/png" },
      { url: "/favicon-16x16.png", sizes: "16x16", type: "image/png" },
    ],
    apple: [
      { url: "/apple-touch-icon.png", sizes: "180x180", type: "image/png" },
    ],
    shortcut: ["/favicon.ico"],
  },
  manifest: "/site.webmanifest",
  category: "technology",
  classification: "Developer Tools / Infrastructure Software",
  other: {
    "msapplication-TileColor": "#020617",
    "msapplication-TileImage": "/mstile-144x144.png",
    "msapplication-config": "/browserconfig.xml",
  },
  robots: {
    index: true,
    follow: true,
    googleBot: {
      index: true,
      follow: true,
      "max-video-preview": -1,
      "max-image-preview": "large",
      "max-snippet": -1,
    },
  },
  verification: {
    google: process.env.NEXT_PUBLIC_GOOGLE_SITE_VERIFICATION,
  },
};

const jsonLd = {
  "@context": "https://schema.org",
  "@graph": [
    {
      "@type": "Organization",
      "@id": `${siteUrl}/#organization`,
      name: "NextDeploy",
      url: siteUrl,
      logo: {
        "@type": "ImageObject",
        url: `${siteUrl}/logo.png`,
      },
      sameAs: [
        "https://github.com/aynaash/NextDeploy",
        "https://twitter.com/hersiyussuf",
        "https://discord.gg/xd9Cub9fm",
      ],
    },
    {
      "@type": "WebSite",
      "@id": `${siteUrl}/#website`,
      url: siteUrl,
      name: "NextDeploy",
      description: "Self-Hosted Next.js Deployment Engine",
      inLanguage: "en-US",
      publisher: {
        "@id": `${siteUrl}/#organization`,
      },
    },
    {
      "@type": "SoftwareApplication",
      "@id": `${siteUrl}/#software`,
      name: "NextDeploy",
      applicationCategory: "DeveloperApplication",
      operatingSystem: "Linux, macOS, Windows",
      description:
        "Open-source self-hosted Next.js deployment engine with build intelligence, route classification, zero-downtime atomic deployments, instant rollback, Caddy reverse proxy, Coraza WAF, and multi-target support (Linux VPS, AWS Lambda, Cloudflare Workers).",
      offers: {
        "@type": "Offer",
        price: "0",
        priceCurrency: "USD",
      },
      softwareVersion: "1.0.0",
      url: siteUrl,
      codeRepository: "https://github.com/aynaash/NextDeploy",
      downloadUrl: "https://github.com/aynaash/NextDeploy/releases",
      license: "https://opensource.org/licenses/MIT",
      sameAs: ["https://github.com/aynaash/NextDeploy"],
      featureList: [
        "Build intelligence via structured metadata.json",
        "Route classification for static, SSR, ISR, API, and middleware routes",
        "Linux VPS deployments with atomic port switching and Caddy HTTPS",
        "AWS serverless deployments via Lambda, S3, CloudFront, ACM, and Secrets Manager",
        "Cloudflare Workers deployments via NextDeploy runtime",
        "Instant rollback and incremental builds",
        "Hardened systemd isolation, Coraza WAF, and HMAC-SHA256 command signing",
      ],
      author: {
        "@id": `${siteUrl}/#organization`,
      },
      publisher: {
        "@id": `${siteUrl}/#organization`,
      },
    },
  ],
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en" className={`${geist.variable} ${geistMono.variable}`}>
      <head>
        <script
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(jsonLd) }}
        />
      </head>
      <body className="font-sans antialiased text-surface-100 bg-surface-950 min-h-screen overflow-x-clip selection:bg-brand-500 selection:text-surface-950">
        <Navbar />
        {children}
        <Analytics />
      </body>
    </html>
  );
}
