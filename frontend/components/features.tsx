const features = [
  {
    title: "Native Execution",
    description: "No Docker, no overhead. Runs your Next.js app directly with Node/Bun via systemd.",
  },
  {
    title: "Zero-Downtime Deploys",
    description: "Daemon-managed port swaps with automatic rollback if health checks fail.",
  },
  {
    title: "VPS & Serverless",
    description: "Deploy to any Linux VPS (Hetzner, DigitalOcean, AWS) or natively to AWS Lambda + S3.",
  },
  {
    title: "Encrypted Secrets",
    description: "Remote secret management from your local CLI — never touch .env files on the server again.",
  },
  {
    title: "Live Logs & Status",
    description: "Real-time log streaming and PID/memory tracking streamed via SSH from the daemon.",
  },
  {
    title: "Full Ownership",
    description: "Your server, your secrets, your logs. Zero vendor lock-in, total transparency.",
  },
];

export default function Features() {
  return (
    <section className="py-16 sm:py-24 bg-surface-950 border-t border-surface-800/80">
      <div className="max-w-4xl mx-auto px-4 sm:px-6">
        <div className="text-center mb-10 sm:mb-16">
          <h2 className="text-3xl sm:text-4xl font-bold text-white tracking-tight mb-3">
            Why NextDeploy?
          </h2>
          <p className="text-surface-400 text-sm sm:text-base">
            Built for developers who want full control without the complexity.
          </p>
        </div>

        <div className="grid sm:grid-cols-2 gap-4">
          {features.map((feature, i) => (
            <div
              key={i}
              className="p-5 bg-surface-900/40 border border-surface-800/90 hover:border-surface-700 rounded-lg transition-colors group"
            >
              <div className="flex items-start gap-3">
                <div className="w-1.5 h-1.5 mt-2 rounded-full bg-brand-400 shrink-0" />
                <div>
                  <h3 className="text-base font-semibold text-white mb-1">
                    {feature.title}
                  </h3>
                  <p className="text-surface-400 text-sm leading-relaxed">
                    {feature.description}
                  </p>
                </div>
              </div>
            </div>
          ))}
        </div>

        <div className="mt-10 sm:mt-12 text-center flex flex-wrap gap-3 sm:gap-4 justify-center">
          <a
            href="/docs"
            className="px-5 py-2.5 rounded-lg bg-brand-500 hover:bg-brand-400 text-surface-950 font-semibold text-sm transition-colors"
          >
            Read the Docs →
          </a>
          <a
            href="https://github.com/aynaash/NextDeploy"
            target="_blank"
            rel="noopener noreferrer"
            className="px-5 py-2.5 rounded-lg bg-surface-900 hover:bg-surface-800 text-surface-200 font-medium text-sm border border-surface-800 hover:border-surface-700 transition-colors"
          >
            View on GitHub
          </a>
        </div>
      </div>
    </section>
  );
}
