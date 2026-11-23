'use client'

import Link from 'next/link'
import { usePathname } from 'next/navigation'
import { ChevronDown, Menu, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import Logo from './logo'

import { docCategories } from '@/lib/docs-config'

function SidebarNav({
  pathname,
  expanded,
  toggle,
  onNavigate,
}: {
  pathname: string
  expanded: string[]
  toggle: (title: string) => void
  onNavigate?: () => void
}) {
  const isActive = (href: string) => pathname === href

  return (
    <>
      <Link href="/" onClick={onNavigate} className="block mb-8 hover:opacity-90 transition-opacity">
        <Logo variant="full" size="sm" subtitle="Documentation" />
      </Link>

      <nav className="space-y-4">
        {docCategories.map((category) => (
          <div key={category.title}>
            <button
              onClick={() => toggle(category.title)}
              className="w-full flex items-center justify-between px-3 py-2 text-sm font-medium text-gray-300 hover:text-white transition-colors"
            >
              {category.title}
              <ChevronDown
                className={`w-4 h-4 transition-transform ${expanded.includes(category.title) ? 'rotate-180' : ''
                  }`}
              />
            </button>

            {expanded.includes(category.title) && (
              <div className="ml-2 space-y-1 mt-2">
                {category.items.map((item) => (
                  <Link
                    key={item.href}
                    href={item.href}
                    onClick={onNavigate}
                    className={`block px-3 py-2 rounded text-sm transition-colors ${isActive(item.href)
                        ? 'bg-emerald-600/20 text-emerald-400 font-medium'
                        : 'text-gray-400 hover:text-gray-200 hover:bg-slate-800/50'
                      }`}
                  >
                    {item.title}
                  </Link>
                ))}
              </div>
            )}
          </div>
        ))}
      </nav>
    </>
  )
}

export default function DocsSidebar() {
  const pathname = usePathname()
  const [expanded, setExpanded] = useState<string[]>(['Getting Started'])
  const [open, setOpen] = useState(false)

  const toggle = (title: string) => {
    setExpanded((prev) =>
      prev.includes(title)
        ? prev.filter((t) => t !== title)
        : [...prev, title]
    )
  }

  useEffect(() => {
    setOpen(false)
  }, [pathname])

  useEffect(() => {
    document.body.style.overflow = open ? 'hidden' : ''
    return () => {
      document.body.style.overflow = ''
    }
  }, [open])

  return (
    <>
      {/* Desktop sidebar */}
      <aside className="hidden lg:block w-64 bg-slate-900/50 border-r border-slate-800 p-6 sticky top-0 h-screen overflow-y-auto flex-shrink-0">
        <SidebarNav pathname={pathname} expanded={expanded} toggle={toggle} />
      </aside>

      {/* Mobile docs sub-bar (sits below global Navbar) */}
      <div className="lg:hidden sticky top-16 z-30 flex items-center justify-between px-4 h-12 bg-slate-950/90 backdrop-blur-md border-b border-slate-800">
        <span className="text-xs font-semibold text-gray-400 uppercase tracking-wider">
          Documentation
        </span>
        <button
          type="button"
          onClick={() => setOpen(true)}
          aria-label="Open documentation menu"
          className="flex items-center gap-2 px-3 py-1.5 rounded-md text-sm text-gray-300 hover:text-white hover:bg-slate-800 transition-colors border border-slate-700"
        >
          <Menu className="w-4 h-4" />
          Menu
        </button>
      </div>

      {/* Mobile drawer */}
      {open && (
        <div
          className="lg:hidden fixed inset-0 z-50 bg-slate-950/70 backdrop-blur-sm"
          onClick={() => setOpen(false)}
        >
          <aside
            className="absolute top-0 left-0 h-full w-[85%] max-w-xs bg-slate-950 border-r border-slate-800 p-6 overflow-y-auto"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-6">
              <span className="text-sm font-semibold text-gray-400 uppercase tracking-wider">
                Navigation
              </span>
              <button
                type="button"
                onClick={() => setOpen(false)}
                aria-label="Close menu"
                className="p-1.5 rounded-md text-gray-400 hover:text-white hover:bg-slate-800 transition-colors"
              >
                <X className="w-5 h-5" />
              </button>
            </div>
            <SidebarNav
              pathname={pathname}
              expanded={expanded}
              toggle={toggle}
              onNavigate={() => setOpen(false)}
            />
          </aside>
        </div>
      )}
    </>
  )
}
