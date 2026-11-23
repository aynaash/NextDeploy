import fs from 'node:fs/promises'
import path from 'node:path'
import matter from 'gray-matter'

import type { DocMeta } from '@/components/doc-md'

export const CONTENT_ROOT = path.join(process.cwd(), 'content', 'docs')

export interface LoadedDoc {
  meta: DocMeta
  body: string
}

export async function loadDoc(slugSegments: string[]): Promise<LoadedDoc | null> {
  if (slugSegments.some(seg => seg.includes('..') || seg.includes('/'))) return null

  const baseFile = path.join(CONTENT_ROOT, ...slugSegments)
  const candidates = [`${baseFile}.md`, path.join(baseFile, 'index.md')]

  for (const filePath of candidates) {
    try {
      const raw = await fs.readFile(filePath, 'utf-8')
      const parsed = matter(raw)
      const data = parsed.data as Partial<DocMeta>
      if (!data.title) continue
      return {
        meta: {
          title: data.title,
          lede: data.lede,
          status: data.status,
          sourceRef: data.sourceRef,
          related: data.related,
        },
        body: parsed.content,
      }
    } catch (err: unknown) {
      const code = (err as NodeJS.ErrnoException).code
      if (code !== 'ENOENT' && code !== 'EISDIR') throw err
    }
  }

  return null
}

export async function listDocSlugs(): Promise<string[][]> {
  const results: string[][] = []
  await walk(CONTENT_ROOT, [])
  return results

  async function walk(dir: string, prefix: string[]) {
    let entries: import('node:fs').Dirent[]
    try {
      entries = await fs.readdir(dir, { withFileTypes: true })
    } catch (err: unknown) {
      const code = (err as NodeJS.ErrnoException).code
      if (code === 'ENOENT') return
      throw err
    }
    for (const entry of entries) {
      const next = [...prefix, entry.name.replace(/\.md$/, '')]
      if (entry.isDirectory()) {
        await walk(path.join(dir, entry.name), [...prefix, entry.name])
      } else if (entry.isFile() && entry.name.endsWith('.md')) {
        if (entry.name === 'index.md') {
          results.push(prefix)
        } else {
          results.push(next)
        }
      }
    }
  }
}
