'use client'

import React, { useEffect, useId, useState } from 'react'

let mermaidInitialized = false

export default function MermaidDiagram({ chart }: { chart: string }) {
  const rawId = useId()
  const safeId = `mermaid-${rawId.replace(/[^a-zA-Z0-9_-]/g, '')}`
  const [svg, setSvg] = useState<string | null>(null)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    let cancelled = false

    async function renderChart() {
      try {
        const mod = await import('mermaid')
        const mermaid = mod.default
        if (!mermaidInitialized) {
          mermaid.initialize({
            startOnLoad: false,
            theme: 'base',
            securityLevel: 'loose',
            fontFamily: 'var(--font-sans), ui-sans-serif, system-ui, sans-serif',
            themeVariables: {
              darkMode: true,
              background: '#0f172a',
              primaryColor: '#1e293b',
              primaryTextColor: '#f8fafc',
              primaryBorderColor: '#10b981',
              lineColor: '#64748b',
              secondaryColor: '#0f172a',
              secondaryTextColor: '#e2e8f0',
              secondaryBorderColor: '#334155',
              tertiaryColor: '#020617',
              tertiaryTextColor: '#cbd5e1',
              tertiaryBorderColor: '#334155',
              clusterBkg: '#0b1220',
              clusterBorder: '#334155',
              edgeLabelBackground: '#0f172a',
              actorBkg: '#1e293b',
              actorBorder: '#10b981',
              actorTextColor: '#f8fafc',
              actorLineColor: '#475569',
              signalColor: '#cbd5e1',
              signalTextColor: '#f8fafc',
              noteBkgColor: '#1e293b',
              noteTextColor: '#e2e8f0',
              noteBorderColor: '#3b82f6',
            },
          })
          mermaidInitialized = true
        }

        const { svg: rendered } = await mermaid.render(safeId, chart.trim())
        if (!cancelled) {
          setSvg(rendered)
          setFailed(false)
        }
      } catch (err) {
        console.error('Mermaid render error:', err)
        if (!cancelled) {
          setFailed(true)
        }
      }
    }

    renderChart()
    return () => {
      cancelled = true
    }
  }, [chart, safeId])

  if (failed) {
    return (
      <pre className="not-prose my-6 p-4 bg-surface-900 border border-surface-800 rounded-xl overflow-x-auto text-xs text-muted-300 font-mono">
        <code>{chart}</code>
      </pre>
    )
  }

  if (!svg) {
    return (
      <div className="not-prose my-6 p-6 bg-surface-900/50 border border-surface-800 rounded-2xl text-xs text-muted-500 font-mono text-center">
        Rendering diagram…
      </div>
    )
  }

  return (
    <div
      className="not-prose my-6 p-4 sm:p-6 bg-surface-900/50 border border-surface-800 rounded-2xl overflow-x-auto flex justify-center [&_svg]:max-w-full [&_svg]:h-auto"
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  )
}
