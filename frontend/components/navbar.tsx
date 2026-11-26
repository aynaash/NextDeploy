import Link from "next/link";
import { Suspense } from "react";
import GitHubStarButton from "./github-star-button";
import MobileNav from "./mobile-nav";
import Logo from "./logo";

export default function Navbar() {
  return (
    <nav className="sticky top-0 z-50 w-full border-b border-surface-800 bg-surface-950/80 backdrop-blur-md">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 h-16 flex items-center justify-between">
        <div className="flex items-center gap-8">
          <Link href="/" className="flex items-center group transition-transform hover:scale-[1.02]">
            <Logo variant="horizontal" size="sm" />
          </Link>

          <div className="hidden md:flex items-center gap-6">
            <Link
              href="/docs"
              className="text-sm font-medium text-muted-400 hover:text-white transition-colors"
            >
              Documentation
            </Link>
            <Link
              href="/routes"
              className="text-sm font-medium text-muted-400 hover:text-white transition-colors"
            >
              Routes Lab
            </Link>
            <a
              href="https://github.com/aynaash/NextDeploy"
              target="_blank"
              rel="noopener noreferrer"
              className="text-sm font-medium text-muted-400 hover:text-white transition-colors"
            >
              GitHub
            </a>
          </div>
        </div>

        <div className="flex items-center gap-3 sm:gap-4">
          <div className="hidden sm:block">
            <Suspense
              fallback={
                <div className="w-32 h-9 bg-surface-800 rounded-lg animate-pulse" />
              }
            >
              <GitHubStarButton />
            </Suspense>
          </div>

          <Link
            href="/docs"
            className="hidden sm:flex items-center justify-center px-4 py-2 rounded-lg bg-brand-500 hover:bg-brand-400 text-surface-950 text-sm font-semibold transition-colors"
          >
            Get Started
          </Link>

          <MobileNav />
        </div>
      </div>
    </nav>
  );
}
