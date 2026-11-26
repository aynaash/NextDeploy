"use client";

import { useState } from "react";

export default function ErrorTrigger({ initialBoom }: { initialBoom: boolean }) {
  const [boom, setBoom] = useState(initialBoom);

  if (boom) {
    throw new Error(
      "Simulated NextDeploy route render error — caught by scoped error.tsx",
    );
  }

  return (
    <button
      type="button"
      onClick={() => setBoom(true)}
      className="px-4 py-2 rounded-lg bg-rose-500/15 border border-rose-500/40 hover:bg-rose-500/25 text-rose-300 text-xs font-mono font-semibold transition-colors"
    >
      Throw Render Error (Test error.tsx)
    </button>
  );
}
