import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'NextDeploy',
  description: 'Deploy Next.js apps to VPS, AWS Lambda, or Cloudflare Workers from one config file.',
  base: process.env.VITEPRESS_BASE || '/NextDeploy/',
  cleanUrls: true,
  lastUpdated: true,

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/NextDeploy/logo.svg' }],
    ['meta', { name: 'theme-color', content: '#3b82f6' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:title', content: 'NextDeploy — Zero-Downtime Next.js Deployments' }],
    ['meta', { property: 'og:description', content: 'Deploy Next.js apps to a VPS, AWS Lambda, or Cloudflare Workers from one config file and one command. Open source, self-hosted, no vendor lock-in.' }]
  ],

  themeConfig: {
    logo: '/logo.svg',
    siteTitle: 'NextDeploy',

    nav: [
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'Agent Trace', link: '/guide/agent-first-infra' },
      { text: 'Targets', link: '/targets/vps' },
      { text: 'CLI Reference', link: '/cli/overview' },
      { text: 'Configuration', link: '/guide/configuration' },
      { text: 'Internals', link: '/internals/atomic-cutover' },
      {
        text: 'v0.14.0',
        items: [
          { text: 'Changelog', link: 'https://github.com/aynaash/NextDeploy/blob/main/CHANGELOG.md' },
          { text: 'Contributing', link: 'https://github.com/aynaash/NextDeploy/blob/main/CODE_QUALITY.md' }
        ]
      }
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Getting Started',
          items: [
            { text: 'Quickstart', link: '/guide/getting-started' },
            { text: 'Core Architecture', link: '/guide/architecture' },
            { text: 'Configuration Reference', link: '/guide/configuration' },
            { text: 'Agent-First Infra & Handoff', link: '/guide/agent-first-infra' },
            { text: 'Multi-Server Stress Testing', link: '/guide/multiserver-stress-testing' }
          ]
        },
        {
          text: 'Deployment Targets',
          items: [
            { text: 'VPS (Caddy + nextdeployd)', link: '/targets/vps' },
            { text: 'Cloudflare Workers (nextcompile)', link: '/targets/cloudflare' },
            { text: 'AWS (Lambda + S3 + CloudFront)', link: '/targets/aws' }
          ]
        }
      ],

      '/targets/': [
        {
          text: 'Deployment Targets',
          items: [
            { text: 'VPS & Go Sidecar', link: '/targets/vps' },
            { text: 'Cloudflare Workers', link: '/targets/cloudflare' },
            { text: 'AWS Serverless', link: '/targets/aws' }
          ]
        }
      ],

      '/cli/': [
        {
          text: 'CLI Reference',
          items: [
            { text: 'Overview & Global Flags', link: '/cli/overview' },
            { text: 'nextdeploy init', link: '/cli/init' },
            { text: 'nextdeploy plan', link: '/cli/plan' },
            { text: 'nextdeploy build', link: '/cli/build' },
            { text: 'nextdeploy ship', link: '/cli/ship' },
            { text: 'nextdeploy rollback', link: '/cli/rollback' },
            { text: 'nextdeploy status', link: '/cli/status' },
            { text: 'nextdeploy logs', link: '/cli/logs' },
            { text: 'nextdeploy secrets', link: '/cli/secrets' },
            { text: 'nextdeploy prepare', link: '/cli/prepare' },
            { text: 'nextdeploy inspect', link: '/cli/inspect' },
            { text: 'nextdeploy generate-ci', link: '/cli/ci-cd' },
            { text: 'nextdeploy trace & takeover', link: '/cli/trace' }
          ]
        }
      ],

      '/internals/': [
        {
          text: 'Architecture & Internals',
          items: [
            { text: 'NextServer Cloudflare Engine', link: '/internals/nextserver-cloudflare-engine' },
            { text: 'Zero-Downtime Cutover', link: '/internals/atomic-cutover' },
            { text: 'Cumulative Chunk Pooling', link: '/internals/chunk-pooling' },
            { text: 'Go High-Performance Sidecar', link: '/internals/go-sidecar' },
            { text: 'Agent Trace & Handoff', link: '/internals/agent-trace' },
            { text: 'Local Development & Mage', link: '/internals/local-development' }
          ]
        }
      ]
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/aynaash/NextDeploy' },
      { icon: 'discord', link: 'https://discord.gg/xd9Cub9fm' }
    ],

    search: {
      provider: 'local',
      options: {
        detailedView: true
      }
    },

    editLink: {
      pattern: 'https://github.com/aynaash/NextDeploy/edit/main/docs/:path',
      text: 'Edit this page on GitHub'
    },

    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2026 NextDeploy Contributors'
    }
  }
})
