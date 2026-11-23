'use client'

import { BookOpen, ArrowRight, Code, Rocket, Bug, Shield } from 'lucide-react'

const docSections = [
  {
    title: 'Getting Started',
    icon: Rocket,
    description: 'Deploy your first app in 5 minutes',
    items: [
      'Installation & Setup',
      'Quick Start Guide',
      'First Deployment'
    ]
  },
  {
    title: 'Configuration',
    icon: Code,
    description: 'Configure your deployments',
    items: [
      'Environment Variables',
      'Environment Configuration',
      'Secret Management'
    ]
  },
  {
    title: 'Advanced',
    icon: Shield,
    description: 'Master advanced features',
    items: [
      'Blue-Green Deployments',
      'CI/CD Integration',
      'Database Setup'
    ]
  },
  {
    title: 'Troubleshooting',
    icon: Bug,
    description: 'Solve common issues',
    items: [
      'Common Problems',
      'FAQ',
      'Debug Guide'
    ]
  }
]

export default function DocsPreview() {
  return (
    <section id="docs" className="py-24 bg-slate-900">
      <div className="max-w-7xl mx-auto px-6">
        <div className="text-center mb-16">
          <h2 className="text-4xl md:text-5xl font-bold text-white mb-4">
            Documentation
          </h2>
          <p className="text-gray-300 text-lg">
            Everything you need to deploy with confidence
          </p>
        </div>

        <div className="grid md:grid-cols-2 gap-6 mb-12">
          {docSections.map((section, i) => (
            <div
              key={i}
              className="bg-slate-800/50 backdrop-blur-sm border border-slate-700 rounded-lg p-8 hover:border-emerald-600/50 hover:bg-slate-800/80 transition-all duration-300 group"
            >
              <div className="flex items-start gap-4 mb-6">
                <section.icon className="w-8 h-8 text-emerald-400 group-hover:text-emerald-300 transition-colors flex-shrink-0" />
                <div className="flex-1">
                  <h3 className="text-xl font-semibold text-white mb-1">
                    {section.title}
                  </h3>
                  <p className="text-gray-400 text-sm">
                    {section.description}
                  </p>
                </div>
              </div>

              <ul className="space-y-2 mb-6">
                {section.items.map((item, j) => (
                  <li key={j} className="flex items-center gap-2 text-gray-300 text-sm">
                    <div className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                    {item}
                  </li>
                ))}
              </ul>

              <button className="text-emerald-400 hover:text-emerald-300 text-sm font-medium flex items-center gap-2 group/btn transition-colors">
                Learn more
                <ArrowRight className="w-4 h-4 group-hover/btn:translate-x-1 transition-transform" />
              </button>
            </div>
          ))}
        </div>

        {/* CTA Section */}
        <div className="bg-gradient-to-r from-emerald-600/20 to-teal-600/20 border border-emerald-500/20 rounded-lg p-8 md:p-12 text-center">
          <h3 className="text-2xl md:text-3xl font-bold text-white mb-4">
            Ready to Deploy?
          </h3>
          <p className="text-gray-300 mb-8 max-w-2xl mx-auto">
            Start your journey to self-hosted Next.js deployments today. Join thousands of developers taking control of their infrastructure.
          </p>
          <button className="px-8 py-3 bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white font-semibold rounded-lg transition-all transform hover:scale-105">
            Start Deploying Now
          </button>
        </div>
      </div>
    </section>
  )
}
