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
    default: "NextDeploy — Self-Hosted Next.js Deployment Platform",
    template: "%s | NextDeploy",
  },
  description:
    "Deploy Next.js to your own Linux VPS, Cloudflare Workers, or AWS Lambda with zero lock-in. Complete build intelligence, zero-downtime atomic rollouts, OWASP CRS WAF, and automated SSL.",
  applicationName: "NextDeploy",
  authors: [
    { name: "Yussuf Hersi", url: "https://twitter.com/hersiyussuf" },
    { name: "NextDeploy Team", url: siteUrl },
  ],
  generator: "Next.js",
  keywords: [
    "Next.js",
    "self-hosted",
    "Next.js deployment",
    "Vercel alternative",
    "Linux VPS",
    "Cloudflare Workers",
    "AWS Lambda",
    "Turbopack",
    "zero-downtime",
    "Caddy reverse proxy",
    "Coraza WAF",
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
    title: "NextDeploy — Self-Hosted Next.js Deployment Platform",
    description:
      "Deploy Next.js to your own Linux VPS, Cloudflare Workers, or AWS Lambda with zero lock-in, zero downtime, and complete transparency.",
    images: [
      {
        url: "/og.png",
        width: 1200,
        height: 630,
        alt: "NextDeploy — Self-Hosted Next.js Deployment Platform",
        type: "image/png",
      },
    ],
  },
  twitter: {
    card: "summary_large_image",
    title: "NextDeploy — Self-Hosted Next.js Deployment Platform",
    description:
      "Deploy Next.js to your own Linux VPS, Cloudflare Workers, or AWS Lambda with zero lock-in.",
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
      "@type": "WebSite",
      "@id": `${siteUrl}/#website`,
      url: siteUrl,
      name: "NextDeploy",
      description: "Self-Hosted Next.js Deployment Platform",
      inLanguage: "en-US",
      potentialAction: {
        "@type": "SearchAction",
        target: `${siteUrl}/docs?q={search_term_string}`,
        "query-input": "required name=search_term_string",
      },
    },
    {
      "@type": "SoftwareApplication",
      "@id": `${siteUrl}/#software`,
      name: "NextDeploy",
      applicationCategory: "DeveloperApplication",
      operatingSystem: "Linux, macOS, Windows",
      description:
        "Open-source self-hosted Next.js deployment engine with build intelligence, zero-downtime atomic port swaps, Caddy reverse proxy, and multi-target support (VPS, Cloudflare, AWS).",
      offers: {
        "@type": "Offer",
        price: "0",
        priceCurrency: "USD",
      },
      softwareVersion: "0.20.0",
      url: siteUrl,
      author: {
        "@type": "Organization",
        name: "NextDeploy",
        url: siteUrl,
        logo: `${siteUrl}/logo.png`,
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
      <body className="font-sans antialiased text-slate-100 bg-slate-950 min-h-screen selection:bg-emerald-500 selection:text-slate-950">
        <Navbar />
        {children}
        <Analytics />
      </body>
    </html>
  );
}
