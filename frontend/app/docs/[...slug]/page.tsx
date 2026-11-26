import { notFound } from 'next/navigation'
import type { Metadata } from 'next'

import DocMd from '@/components/doc-md'
import { loadDoc, listDocSlugs } from '@/lib/docs-loader'

const siteUrl = process.env.NEXT_PUBLIC_SITE_URL || 'https://nextdeploy.org'

interface PageParams {
  params: Promise<{ slug: string[] }>
}

export async function generateStaticParams() {
  const slugs = await listDocSlugs()
  return slugs.map(slug => ({ slug }))
}

export const dynamicParams = false

export async function generateMetadata({ params }: PageParams): Promise<Metadata> {
  const { slug } = await params
  const doc = await loadDoc(slug)
  if (!doc) return {}
  const slugPath = slug.join('/')
  const description = doc.meta.lede || `Learn about ${doc.meta.title} in the NextDeploy documentation.`
  const ogTitle = `${doc.meta.title} — NextDeploy Docs`

  return {
    title: doc.meta.title,
    description,
    alternates: {
      canonical: `/docs/${slugPath}`,
    },
    openGraph: {
      title: ogTitle,
      description,
      url: `/docs/${slugPath}`,
      type: 'article',
      siteName: 'NextDeploy',
      locale: 'en_US',
      images: [
        {
          url: '/og.png',
          width: 1200,
          height: 630,
          alt: ogTitle,
        },
      ],
    },
    twitter: {
      card: 'summary_large_image',
      title: ogTitle,
      description,
      images: ['/og.png'],
    },
  }
}

export default async function DocCatchallPage({ params }: PageParams) {
  const { slug } = await params
  const doc = await loadDoc(slug)
  if (!doc) notFound()

  const slugPath = slug.join('/')
  const docUrl = `${siteUrl}/docs/${slugPath}`
  const description = doc.meta.lede || `Learn about ${doc.meta.title} in the NextDeploy documentation.`

  const docJsonLd = {
    '@context': 'https://schema.org',
    '@graph': [
      {
        '@type': 'BreadcrumbList',
        itemListElement: [
          {
            '@type': 'ListItem',
            position: 1,
            name: 'Home',
            item: siteUrl,
          },
          {
            '@type': 'ListItem',
            position: 2,
            name: 'Docs',
            item: `${siteUrl}/docs`,
          },
          {
            '@type': 'ListItem',
            position: 3,
            name: doc.meta.title,
            item: docUrl,
          },
        ],
      },
      {
        '@type': 'TechArticle',
        headline: doc.meta.title,
        description,
        url: docUrl,
        mainEntityOfPage: {
          '@type': 'WebPage',
          '@id': docUrl,
        },
        publisher: {
          '@id': `${siteUrl}/#organization`,
        },
      },
    ],
  }

  return (
    <>
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(docJsonLd) }}
      />
      <DocMd meta={doc.meta} body={doc.body} />
    </>
  )
}
