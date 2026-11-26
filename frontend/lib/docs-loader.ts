// Docs are pre-parsed at build time by scripts/build-docs-index.mjs into
// docs-content.generated.json. No fs / gray-matter at runtime, so this module is
// safe to evaluate inside a Cloudflare Worker (which has no project filesystem).
import type { DocMeta } from '@/components/doc-md'
import docsIndex from './docs-content.generated.json'

export interface LoadedDoc {
  meta: DocMeta
  body: string
}

interface IndexedDoc {
  slug: string[]
  meta: DocMeta
  body: string
}

const DOCS = docsIndex as unknown as Record<string, IndexedDoc>

export async function loadDoc(slugSegments: string[]): Promise<LoadedDoc | null> {
  if (slugSegments.some(seg => seg.includes('..') || seg.includes('/'))) return null
  const doc = DOCS[slugSegments.join('/')]
  if (!doc) return null
  return { meta: doc.meta, body: doc.body }
}

export async function listDocSlugs(): Promise<string[][]> {
  return Object.values(DOCS).map(doc => doc.slug)
}
