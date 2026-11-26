import type { ReactNode } from "react";

export default function GroupedRoutesLayout({
  children,
}: {
  children: ReactNode;
}) {
  return (
    <div>
      <div className="bg-brand-500/10 border-b border-brand-500/30 px-4 py-2 text-center text-xs font-mono text-brand-300">
        Scoped Route Group Layout active from{" "}
        <code>app/routes/(grouped)/layout.tsx</code> — notice{" "}
        <code>(grouped)</code> is omitted from the browser URL.
      </div>
      {children}
    </div>
  );
}
