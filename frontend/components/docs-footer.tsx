'use client'

import { Copy, CheckCircle2 } from 'lucide-react'
import { useState } from 'react'

export default function DocsFooter() {
    const [copied, setCopied] = useState(false)

    const copyAsMarkdown = async () => {
        try {
            const targetElement = document.querySelector('.prose')
            if (!targetElement) {
                console.error("Documentation content (.prose) not found on page")
                return
            }

            // Dynamically load Turndown to keep client bundle small initially
            const TurndownService = (await import('turndown')).default
            const turndownService = new TurndownService({
                headingStyle: 'atx',
                codeBlockStyle: 'fenced'
            })

            // Convert the HTML content of the documentation to markdown
            const markdown = turndownService.turndown(targetElement.innerHTML)

            await navigator.clipboard.writeText(markdown)

            setCopied(true)
            setTimeout(() => setCopied(false), 2000)
        } catch (err) {
            console.error('Failed to copy text: ', err)
        }
    }

    return (
        <div className="mt-16 pt-8 border-t border-slate-800 pb-12">
            <div className="flex justify-center pt-8">
                <button
                    onClick={copyAsMarkdown}
                    className="flex items-center gap-2 text-xs font-medium bg-slate-800/50 hover:bg-slate-800 text-gray-300 hover:text-emerald-400 px-4 py-2 border border-slate-700/50 rounded-md transition-all group"
                >
                    {copied ? (
                        <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />
                    ) : (
                        <Copy className="w-3.5 h-3.5 group-hover:scale-110 transition-transform" />
                    )}
                    {copied ? 'Copied to Clipboard' : 'Copy as Markdown for LLM'}
                </button>
            </div>
        </div>
    )
}
