"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { RefreshCw } from "lucide-react";

export default function RevalidateButton() {
  const router = useRouter();
  const [busy, setBusy] = useState(false);
  const [status, setStatus] = useState<string | null>(null);

  const trigger = async () => {
    setBusy(true);
    setStatus(null);
    try {
      const res = await fetch("/api/routes/revalidate", { method: "POST" });
      const data = (await res.json()) as {
        revalidated?: boolean;
        timestamp?: string;
      };
      setStatus(
        res.ok
          ? `Revalidated at ${data.timestamp ?? "now"} — refreshing view...`
          : `HTTP ${res.status}`,
      );
      router.refresh();
    } catch (err) {
      setStatus(err instanceof Error ? err.message : "Revalidate failed");
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="flex flex-wrap items-center gap-3">
      <button
        type="button"
        onClick={trigger}
        disabled={busy}
        className="inline-flex items-center gap-2 px-4 py-2 rounded-lg bg-brand-500 hover:bg-brand-400 disabled:opacity-60 text-surface-950 text-xs font-semibold transition-colors"
      >
        <RefreshCw
          className={`w-3.5 h-3.5 ${busy ? "animate-spin" : ""}`}
          aria-hidden="true"
        />
        {busy ? "Revalidating..." : "Trigger On-Demand Revalidate"}
      </button>
      {status && (
        <span className="text-xs font-mono text-cyan-300">{status}</span>
      )}
    </div>
  );
}
