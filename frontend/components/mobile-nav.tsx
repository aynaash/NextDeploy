"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import Link from "next/link";
import { Menu, X, Github } from "lucide-react";

export default function MobileNav() {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (open) {
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
    }
    return () => {
      document.body.style.overflow = "";
    };
  }, [open]);

  return (
    <div className="md:hidden">
      <button
        type="button"
        aria-label={open ? "Close menu" : "Open menu"}
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        className="p-2 -mr-2 rounded-md text-muted-300 hover:text-white hover:bg-surface-800 transition-colors"
      >
        {open ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
      </button>

      {open &&
        typeof document !== "undefined" &&
        createPortal(
          <div
            className="fixed inset-x-0 top-16 bottom-0 z-50 bg-surface-950/95 backdrop-blur-md border-t border-surface-800 overflow-y-auto md:hidden"
            onClick={() => setOpen(false)}
          >
            <nav
              className="flex flex-col p-6 gap-1 max-w-7xl mx-auto"
              onClick={(e) => e.stopPropagation()}
            >
              <Link
                href="/docs"
                onClick={() => setOpen(false)}
                className="px-4 py-3 rounded-lg text-base font-medium text-muted-200 hover:bg-surface-800 hover:text-white transition-colors"
              >
                Documentation
              </Link>
              <Link
                href="/routes"
                onClick={() => setOpen(false)}
                className="px-4 py-3 rounded-lg text-base font-medium text-muted-200 hover:bg-surface-800 hover:text-white transition-colors"
              >
                Routes Lab
              </Link>
              <a
                href="https://github.com/aynaash/NextDeploy"
                target="_blank"
                rel="noopener noreferrer"
                onClick={() => setOpen(false)}
                className="px-4 py-3 rounded-lg text-base font-medium text-muted-200 hover:bg-surface-800 hover:text-white transition-colors flex items-center gap-2"
              >
                <Github className="w-4 h-4" /> GitHub
              </a>
              <Link
                href="/docs"
                onClick={() => setOpen(false)}
                className="mt-3 px-4 py-3 rounded-lg bg-brand-500 hover:bg-brand-400 text-surface-950 text-base font-bold text-center transition-colors"
              >
                Get Started
              </Link>
            </nav>
          </div>,
          document.body
        )}
    </div>
  );
}
