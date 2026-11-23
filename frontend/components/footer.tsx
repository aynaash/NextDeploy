import { Github, Twitter } from "lucide-react";
import Logo from "./logo";

export default function Footer() {
  return (
    <footer className="py-10 sm:py-12 bg-surface-950 border-t border-surface-800">
      <div className="max-w-5xl mx-auto px-4 sm:px-6 flex flex-col md:flex-row items-center justify-between gap-6 text-center md:text-left">
        <div className="flex flex-col items-center md:items-start">
          <Logo variant="horizontal" size="sm" className="mb-2" />
          <p className="text-muted-500 text-sm">
            Self-hosted Next.js deployment engine. Open-source, transparent &amp; zero lock-in.
          </p>
        </div>

        <div className="flex items-center gap-6">
          <a href="/docs" className="text-muted-400 hover:text-white text-sm transition-colors">
            Docs
          </a>
          <a
            href="https://github.com/aynaash/NextDeploy"
            target="_blank"
            rel="noopener noreferrer"
            className="text-muted-400 hover:text-white transition-colors"
          >
            <Github className="w-5 h-5" />
          </a>
          <a
            href="https://twitter.com/hersiyussuf"
            target="_blank"
            rel="noopener noreferrer"
            className="text-muted-400 hover:text-white transition-colors"
          >
            <Twitter className="w-5 h-5" />
          </a>
        </div>
      </div>

      <div className="max-w-5xl mx-auto px-4 sm:px-6 mt-8 border-t border-surface-800/60 pt-6">
        <p className="text-muted-600 text-xs text-center">
          © 2026 NextDeploy · Open-source under MIT License
        </p>
      </div>
    </footer>
  );
}
