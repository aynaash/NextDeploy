import React from 'react'
import Link from 'next/link'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import rehypeHighlight from 'rehype-highlight'

import DocPrevNext from '@/components/doc-prev-next'

export type DocStatus = 'draft' | 'in-progress' | 'ready'

export interface DocMeta {
  title: string
  lede?: string
  status?: DocStatus
  sourceRef?: string
  related?: { title: string; href: string }[]
}

const statusStyles: Record<DocStatus, string> = {
  draft: 'border-amber-500/30 text-amber-300 bg-amber-500/10',
  'in-progress': 'border-blue-500/30 text-blue-300 bg-blue-500/10',
  ready: 'border-emerald-500/30 text-emerald-300 bg-emerald-500/10',
}

export default function DocMd({ meta, body }: { meta: DocMeta; body: string }) {
  return (
    <article
      className="
        prose prose-invert max-w-none prose-sm sm:prose-base
        prose-headings:text-white prose-headings:font-semibold
        prose-h1:text-3xl sm:prose-h1:text-4xl prose-h1:font-bold prose-h1:mb-4 prose-h1:mt-0
        prose-h2:text-xl sm:prose-h2:text-2xl prose-h2:mt-10 sm:prose-h2:mt-12 prose-h2:mb-3
        prose-h3:text-base sm:prose-h3:text-lg prose-h3:mt-6 sm:prose-h3:mt-8 prose-h3:mb-2
        prose-p:text-gray-300 prose-p:leading-relaxed
        prose-strong:text-white
        prose-a:text-emerald-400 prose-a:no-underline hover:prose-a:text-emerald-300
        prose-code:text-emerald-300 prose-code:bg-slate-900 prose-code:px-1.5 prose-code:py-0.5 prose-code:rounded prose-code:before:content-none prose-code:after:content-none prose-code:font-normal prose-code:break-words
        prose-pre:bg-slate-900 prose-pre:border prose-pre:border-slate-800 prose-pre:p-4 prose-pre:rounded-xl prose-pre:overflow-x-auto prose-pre:text-xs sm:prose-pre:text-sm
        prose-pre:[&_code]:bg-transparent prose-pre:[&_code]:p-0 prose-pre:[&_code]:text-gray-200
        prose-blockquote:border-l-amber-400/50 prose-blockquote:bg-slate-900/40 prose-blockquote:py-1 prose-blockquote:px-4 prose-blockquote:rounded-r-lg prose-blockquote:not-italic prose-blockquote:text-gray-400
        prose-ul:text-gray-300 prose-ol:text-gray-300 prose-li:marker:text-gray-600
        prose-hr:border-slate-800
        prose-table:text-gray-300 prose-th:text-white prose-th:border-slate-700 prose-td:border-slate-800
        prose-img:rounded-xl
      "
    >
      <header className="not-prose mb-8">
        <div className="flex flex-wrap items-center justify-between gap-3 mb-4">
          <h1 className="text-2xl sm:text-3xl md:text-4xl font-bold text-white m-0 break-words">{meta.title}</h1>
          {meta.status && (
            <span
              className={`text-[10px] uppercase tracking-wider px-3 py-1 rounded-full border ${statusStyles[meta.status]}`}
            >
              {meta.status}
            </span>
          )}
        </div>
        {meta.lede && (
          <p className="text-gray-300 text-lg leading-relaxed mb-3 m-0">{meta.lede}</p>
        )}
        {meta.sourceRef && (
          <p className="text-gray-500 text-xs font-mono m-0">
            Source reference: {meta.sourceRef}
          </p>
        )}
        <hr className="border-slate-800 mt-8 mb-0" />
      </header>

      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        rehypePlugins={[[rehypeHighlight, { detect: true, ignoreMissing: true }]]}
      >
        {body}
      </ReactMarkdown>

      {meta.related && meta.related.length > 0 && (
        <section className="not-prose mt-12 pt-8 border-t border-slate-800">
          <h2 className="text-2xl font-bold text-white mb-4">Related</h2>
          <ul className="space-y-2 list-none p-0 m-0">
            {meta.related.map(({ title, href }) => (
              <li key={href}>
                <Link
                  href={href}
                  className="text-emerald-400 hover:text-emerald-300 transition-colors"
                >
                  → {title}
                </Link>
              </li>
            ))}
          </ul>
        </section>
      )}

      <div className="not-prose">
        <DocPrevNext />
      </div>
    </article>
  )
}
