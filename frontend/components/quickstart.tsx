'use client'

import { useState } from 'react'
import { Copy, Check, Terminal, Server, Monitor } from 'lucide-react'

const installOptions = [
  {
    id: 'cli-unix',
    name: 'Mac/Linux CLI',
    icon: Terminal,
    cmd: 'curl -sSf https://nextdeploy.org/install.sh | sh',
    desc: 'Install NextDeploy CLI tool'
  },
  {
    id: 'cli-win',
    name: 'Windows CLI',
    icon: Monitor,
    cmd: 'curl.exe -sSfO https://nextdeploy.org/install.bat && install.bat',
    desc: 'Install CLI tool on Windows'
  },
  {
    id: 'daemon',
    name: 'Server Daemon',
    icon: Server,
    cmd: 'curl -sSf https://nextdeploy.org/daemon.sh | sh',
    desc: 'Install the Zero-Downtime Daemon'
  }
]

export default function QuickStart() {
  const [copied, setCopied] = useState(false)
  const [activeTab, setActiveTab] = useState(installOptions[0])

  const copyCmd = () => {
    navigator.clipboard.writeText(activeTab.cmd)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
  }

  return (
    <section className="py-16 sm:py-24 md:py-32 bg-gradient-to-br from-slate-900 via-slate-800 to-slate-900">
      <div className="max-w-4xl mx-auto px-4 sm:px-6">
        <div className="text-center mb-10 sm:mb-16">
          <h2 className="text-3xl sm:text-4xl md:text-5xl font-bold text-white mb-4">
            Zero-Touch Developer Experience
          </h2>
          <p className="text-gray-400 text-base sm:text-lg">
            From initialization to globally distributed production
          </p>
        </div>

        <div className="relative bg-gradient-to-br from-slate-950/80 to-slate-900/60 backdrop-blur-sm rounded-2xl border border-slate-700/50 overflow-hidden shadow-2xl">
          {/* Tabs header */}
          <div className="flex border-b border-slate-700/50 bg-slate-900/80 overflow-x-auto">
            {installOptions.map(option => {
              const Icon = option.icon
              const isActive = activeTab.id === option.id
              return (
                <button
                  key={option.id}
                  onClick={() => setActiveTab(option)}
                  className={`flex items-center gap-2 px-4 sm:px-6 py-3 sm:py-4 text-xs sm:text-sm font-medium transition-colors border-b-2 whitespace-nowrap ${isActive
                    ? 'text-emerald-400 border-emerald-500 bg-slate-800/50'
                    : 'text-gray-400 border-transparent hover:text-gray-200 hover:bg-slate-800/30'
                    }`}
                >
                  <Icon className="w-4 h-4" />
                  {option.name}
                </button>
              )
            })}
          </div>

          {/* Terminal header */}
          <div className="flex items-center justify-between px-4 sm:px-6 py-3 sm:py-4 bg-slate-800/40 border-b border-slate-700/50">
            <div className="flex items-center gap-3">
              <div className="w-3 h-3 rounded-full bg-red-500/80" />
              <div className="w-3 h-3 rounded-full bg-yellow-500/80" />
              <div className="w-3 h-3 rounded-full bg-emerald-500/80" />
              <span className="ml-4 text-sm text-gray-400 font-medium">install.sh</span>
            </div>
            <button
              onClick={copyCmd}
              className="flex items-center gap-2 px-4 py-1.5 text-sm text-gray-400 hover:text-emerald-300 hover:bg-slate-700/50 rounded-lg transition-all duration-200 font-medium"
            >
              {copied ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
              {copied ? 'Copied!' : 'Copy'}
            </button>
          </div>

          {/* Code Execution Block */}
          <div className="p-5 sm:p-8 md:p-10 font-mono text-sm">
            <div className="flex flex-col gap-4 group">
              <div className="flex items-start gap-3 sm:gap-4">
                <span className="text-gray-600 select-none flex-shrink-0 mt-1">$</span>
                <div className="flex-1 min-w-0">
                  <code className="text-emerald-300 break-all text-sm sm:text-base font-medium">{activeTab.cmd}</code>
                  <div className="text-gray-500 mt-4 flex items-center gap-2">
                    <span className="w-2 h-2 rounded-full bg-blue-500/50"></span>
                    {activeTab.desc}
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Step Badge */}
        <div className="mt-6 sm:mt-8 bg-emerald-500/10 border border-emerald-500/20 rounded-xl p-4 mx-auto flex items-center gap-3 max-w-2xl w-full text-emerald-300 text-sm">
          <Terminal className="w-4 h-4 shrink-0" />
          <p>Run <strong>nextdeploy init</strong> to analyze your project and scaffold your config instantly.</p>
        </div>

        {/* Features grid below terminal */}
        <div className="grid sm:grid-cols-3 gap-4 sm:gap-6 mt-10 sm:mt-12">
          <div className="text-center p-5 sm:p-6 rounded-xl border border-slate-700/50 hover:border-emerald-600/40 hover:bg-slate-900/30 transition-all duration-300 group">
            <div className="text-3xl sm:text-4xl font-bold text-emerald-400 mb-3 group-hover:text-emerald-300 transition-colors">Native</div>
            <p className="text-gray-400 group-hover:text-gray-300 text-sm font-medium transition-colors">Zero abstraction overhead</p>
          </div>
          <div className="text-center p-5 sm:p-6 rounded-xl border border-slate-700/50 hover:border-emerald-600/40 hover:bg-slate-900/30 transition-all duration-300 group">
            <div className="text-3xl sm:text-4xl font-bold text-emerald-400 mb-3 group-hover:text-emerald-300 transition-colors">0ms</div>
            <p className="text-gray-400 group-hover:text-gray-300 text-sm font-medium transition-colors">Zero-downtime shipping</p>
          </div>
          <div className="text-center p-5 sm:p-6 rounded-xl border border-slate-700/50 hover:border-emerald-600/40 hover:bg-slate-900/30 transition-all duration-300 group">
            <div className="text-3xl sm:text-4xl font-bold text-emerald-400 mb-3 group-hover:text-emerald-300 transition-colors">AWS</div>
            <p className="text-gray-400 group-hover:text-gray-300 text-sm font-medium transition-colors">Native Serverless moat</p>
          </div>
        </div>

      </div>
    </section>
  )
}
