import { notFound } from 'next/navigation'
import type { Metadata } from 'next'

import DocMd from '@/components/doc-md'
import { loadDoc, listDocSlugs } from '@/lib/docs-loader'

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
  return {
    title: doc.meta.title,
    description: doc.meta.lede || `Learn about ${doc.meta.title} in the NextDeploy documentation.`,
    alternates: {
      canonical: `/docs/${slugPath}`,
    },
    openGraph: {
      title: `${doc.meta.title} — NextDeploy Docs`,
      description: doc.meta.lede || `Learn about ${doc.meta.title} in the NextDeploy documentation.`,
      url: `/docs/${slugPath}`,
      type: 'article',
    },
  }
}

export default async function DocCatchallPage({ params }: PageParams) {
  const { slug } = await params
  const doc = await loadDoc(slug)
  if (!doc) notFound()
  return <DocMd meta={doc.meta} body={doc.body} />
}
