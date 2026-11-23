'use client'

import { useState } from 'react'
import { Copy, Check } from 'lucide-react'

export function CopyButton({ text }: { text: string }) {
    const [copied, setCopied] = useState(false)

    const handleCopy = async () => {
        try {
            await navigator.clipboard.writeText(text)
            setCopied(true)
            setTimeout(() => setCopied(false), 2000)
        } catch (err) {
            console.error('Failed to copy:', err)
        }
    }

    return (
        <button
            onClick={handleCopy}
            className="absolute top-3 right-3 p-1.5 rounded-md bg-surface-800 hover:bg-surface-700 text-muted-400 hover:text-brand-400 transition-all"
            title="Copy to clipboard"
        >
            {copied ? <Check className="w-4 h-4 text-brand-400" /> : <Copy className="w-4 h-4" />}
        </button>
    )
}

export function CodeBlock({ code, className = '' }: { code: string; className?: string }) {
    return (
        <div className={`relative group ${className}`}>
            <pre className="bg-surface-900/80 border border-surface-800 rounded-lg p-4 overflow-x-auto pr-12 font-mono text-xs sm:text-sm">
                <code className="text-surface-200">{code}</code>
            </pre>
            <CopyButton text={code} />
        </div>
    )
}
