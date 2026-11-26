export interface DocItem {
    title: string
    href: string
}

export interface DocCategory {
    title: string
    items: DocItem[]
}

export const docCategories: DocCategory[] = [
    {
        title: 'Getting Started',
        items: [
            { title: 'Overview', href: '/docs' },
            { title: 'Installation', href: '/docs/getting-started/installation' },
            { title: 'Requirements', href: '/docs/getting-started/requirements' },
            { title: 'VPS Quickstart', href: '/docs/getting-started/vps-quickstart' },
            { title: 'AWS Quickstart', href: '/docs/getting-started/aws-quickstart' },
            { title: 'Cloudflare Quickstart', href: '/docs/getting-started/cloudflare-quickstart' },
        ],
    },
    {
        title: 'Concepts',
        items: [
            { title: 'How It Works', href: '/docs/concepts/how-it-works' },
            { title: 'Build Metadata', href: '/docs/concepts/metadata' },
            { title: 'Architecture', href: '/docs/concepts/architecture' },
            { title: 'Security Model', href: '/docs/concepts/security' },
        ],
    },
    {
        title: 'VPS',
        items: [
            { title: 'Overview', href: '/docs/vps/overview' },
            { title: 'Daemon', href: '/docs/vps/daemon' },
            { title: 'Go Sidecar', href: '/docs/vps/go-sidecar' },
            { title: 'Zero-Downtime Cutover', href: '/docs/vps/atomic-cutover' },
            { title: 'Chunk Pooling', href: '/docs/vps/chunk-pooling' },
            { title: 'systemd Hardening', href: '/docs/vps/systemd-hardening' },
            { title: 'Caddy & TLS', href: '/docs/vps/caddy' },
            { title: 'Rollback', href: '/docs/vps/rollback' },
        ],
    },
    {
        title: 'AWS',
        items: [
            { title: 'Lambda', href: '/docs/aws/lambda' },
            { title: 'CloudFront', href: '/docs/aws/cloudfront' },
            { title: 'Lambda Bridge', href: '/docs/aws/lambda-bridge' },
            { title: 'ISR', href: '/docs/aws/isr' },
            { title: 'Image Optimization', href: '/docs/aws/image-optimization' },
        ],
    },
    {
        title: 'Cloudflare',
        items: [
            { title: 'Overview', href: '/docs/cloudflare/overview' },
            { title: 'Quickstart', href: '/docs/cloudflare/quickstart' },
            { title: 'API Token Permissions', href: '/docs/cloudflare/tokens' },
            { title: 'Workers Runtime', href: '/docs/cloudflare/workers' },
            { title: 'Bindings', href: '/docs/cloudflare/bindings' },
            { title: 'R2', href: '/docs/cloudflare/r2' },
            { title: 'Hyperdrive', href: '/docs/cloudflare/hyperdrive' },
            { title: 'Queues', href: '/docs/cloudflare/queues' },
            { title: 'Durable Objects', href: '/docs/cloudflare/durable-objects' },
            { title: 'KV / Vectorize / AI', href: '/docs/cloudflare/kv-vectorize-ai' },
            { title: 'Cloudflare DNS', href: '/docs/cloudflare/dns' },
        ],
    },
    {
        title: 'Secrets',
        items: [
            { title: 'Overview', href: '/docs/secrets/overview' },
            { title: 'Encryption', href: '/docs/secrets/encryption' },
            { title: 'Doppler', href: '/docs/secrets/doppler' },
            { title: 'Lambda Extension', href: '/docs/secrets/lambda-extension' },
        ],
    },
    {
        title: 'DNS',
        items: [
            { title: 'DNS Configuration', href: '/docs/dns' },
        ],
    },
    {
        title: 'CLI',
        items: [
            { title: 'Command Reference', href: '/docs/cli/reference' },
            { title: 'Trace & Takeover', href: '/docs/cli/trace' },
            { title: 'Self-Update', href: '/docs/cli/self-update' },
        ],
    },
    {
        title: 'Agent Trace',
        items: [
            { title: 'Agent-First Infra', href: '/docs/agent-trace/agent-first-infra' },
            { title: 'Architecture & Continuity', href: '/docs/agent-trace/architecture' },
            { title: 'CLI & Human Takeover', href: '/docs/agent-trace/cli-takeover' },
        ],
    },
    {
        title: 'MCP & Agents',
        items: [
            { title: 'MCP Server', href: '/docs/mcp/server' },
            { title: 'Tool Schemas & API', href: '/docs/mcp/tools' },
            { title: 'Agent Rules & Prompts', href: '/docs/mcp/prompt-engineering' },
        ],
    },
    {
        title: 'Configuration',
        items: [
            { title: 'YAML Reference', href: '/docs/config/reference' },
            { title: 'Examples', href: '/docs/config/examples' },
        ],
    },
    {
        title: 'Operations',
        items: [
            { title: 'Monitoring', href: '/docs/operations/monitoring' },
            { title: 'Audit Log', href: '/docs/operations/audit-log' },
            { title: 'CI/CD', href: '/docs/operations/ci-cd' },
        ],
    },
    {
        title: 'Advanced',
        items: [
            { title: 'NextCompile (CF runtime)', href: '/docs/advanced/nextcompile' },
            { title: 'Next.js 16 Turbopack on CF', href: '/docs/advanced/nextjs-16-turbopack-on-cloudflare' },
            { title: 'Auto-Provisioning', href: '/docs/advanced/auto-provisioning' },
            { title: 'Local Development & Air', href: '/docs/advanced/local-development' },
        ],
    },
    {
        title: 'Reference',
        items: [
            { title: 'Limitations', href: '/docs/reference/limitations' },
            { title: 'Troubleshooting', href: '/docs/reference/troubleshooting' },
            { title: 'FAQ', href: '/docs/reference/faq' },
        ],
    },
    {
        title: 'Roadmap',
        items: [
            { title: 'Roadmap', href: '/docs/roadmap' },
        ],
    },
]

export const flatDocs: DocItem[] = docCategories.flatMap(category => category.items)
