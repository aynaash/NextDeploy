'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { ArrowLeft, ArrowRight } from 'lucide-react'

import { flatDocs } from '@/lib/docs-config'

export default function DocPrevNext() {
  const pathname = usePathname()
  const idx = flatDocs.findIndex(d => d.href === pathname)
  if (idx === -1) return null

  const prev = idx > 0 ? flatDocs[idx - 1] : null
  const next = idx < flatDocs.length - 1 ? flatDocs[idx + 1] : null

  return (
    <nav className="mt-16 pt-8 border-t border-slate-800 grid grid-cols-1 sm:grid-cols-2 gap-4">
      {prev ? (
        <Link
          href={prev.href}
          className="group flex items-center gap-3 px-5 py-4 rounded-xl border border-slate-800 bg-slate-900/30 hover:border-emerald-500/40 hover:bg-slate-900/60 transition-colors no-underline"
        >
          <ArrowLeft className="w-4 h-4 text-gray-500 group-hover:text-emerald-400 transition-colors flex-shrink-0" />
          <div className="min-w-0">
            <div className="text-[10px] uppercase tracking-wider text-gray-500">Previous</div>
            <div className="text-gray-200 group-hover:text-white text-sm font-medium truncate">
              {prev.title}
            </div>
          </div>
        </Link>
      ) : (
        <div aria-hidden className="hidden sm:block" />
      )}

      {next ? (
        <Link
          href={next.href}
          className="group flex items-center gap-3 px-5 py-4 rounded-xl border border-slate-800 bg-slate-900/30 hover:border-emerald-500/40 hover:bg-slate-900/60 transition-colors no-underline sm:text-right sm:justify-end"
        >
          <div className="min-w-0 sm:order-1">
            <div className="text-[10px] uppercase tracking-wider text-gray-500">Next</div>
            <div className="text-gray-200 group-hover:text-white text-sm font-medium truncate">
              {next.title}
            </div>
          </div>
          <ArrowRight className="w-4 h-4 text-gray-500 group-hover:text-emerald-400 transition-colors flex-shrink-0 sm:order-2" />
        </Link>
      ) : (
        <div aria-hidden className="hidden sm:block" />
      )}
    </nav>
  )
}
