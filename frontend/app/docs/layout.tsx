import React from "react"
import type { Metadata } from "next"
import DocsFooter from '@/components/docs-footer'
import DocsSidebar from '@/components/docs-sidebar'

export const metadata: Metadata = {
  title: {
    default: 'Documentation',
    template: '%s | NextDeploy Docs',
  },
  description: 'Comprehensive documentation for NextDeploy: self-hosted Next.js deployment engine for Linux VPS, AWS Lambda, and Cloudflare Workers.',
  openGraph: {
    title: 'NextDeploy Documentation',
    description: 'Comprehensive documentation for NextDeploy: self-hosted Next.js deployment engine for Linux VPS, AWS Lambda, and Cloudflare Workers.',
    type: 'website',
    siteName: 'NextDeploy',
    locale: 'en_US',
    images: [
      {
        url: '/og.png',
        width: 1200,
        height: 630,
        alt: 'NextDeploy Documentation',
      },
    ],
  },
  twitter: {
    card: 'summary_large_image',
    title: 'NextDeploy Documentation',
    description: 'Comprehensive documentation for NextDeploy: self-hosted Next.js deployment engine for Linux VPS, AWS Lambda, and Cloudflare Workers.',
    images: ['/og.png'],
  },
}

export default function DocsLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <div className="min-h-screen bg-surface-950 lg:flex">
      <DocsSidebar />
      <main className="flex-1 min-w-0 relative">
        <div className="py-8 sm:py-10 px-4 sm:px-6 lg:px-12 max-w-4xl">
          {children}
          <DocsFooter />
        </div>
      </main>
    </div>
  )
}
