"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import {
  Activity,
  CheckCircle2,
  ExternalLink,
  Play,
  Radio,
  RefreshCw,
  Terminal,
  XCircle,
} from "lucide-react";
import {
  ROUTE_CATEGORIES,
  ROUTES_CATALOG,
  type RouteCategory,
  type RouteMode,
  type RouteSpec,
} from "@/lib/routes-catalog";

interface ProbeResult {
  status: number | "REDIRECT";
  ok: boolean;
  latencyMs: number;
  poweredBy: string | null;
  contentType: string | null;
  snippet: string;
}

const MODE_BADGE: Record<RouteMode, string> = {
  SSG: "bg-emerald-500/10 text-emerald-300 border-emerald-500/30",
  ISR: "bg-cyan-500/10 text-cyan-300 border-cyan-500/30",
  SSR: "bg-brand-500/15 text-brand-300 border-brand-500/30",
  STREAM: "bg-sky-500/10 text-sky-300 border-sky-500/30",
  ACTION: "bg-amber-500/10 text-amber-300 border-amber-500/30",
  API: "bg-indigo-500/10 text-indigo-300 border-indigo-500/30",
  EDGE: "bg-rose-500/10 text-rose-300 border-rose-500/30",
  META: "bg-surface-800 text-surface-200 border-surface-700",
};

function isHardNavigationSpec(spec: RouteSpec): boolean {
  return (
    spec.href.startsWith("/api/") ||
    spec.href === "/sitemap.xml" ||
    spec.href === "/app" ||
    spec.id === "redirect-page"
  );
}

export default function RoutesMatrixClient() {
  const [activeCategory, setActiveCategory] = useState<RouteCategory | "All">(
    "All",
  );
  const [results, setResults] = useState<Record<string, ProbeResult>>({});
  const [probingId, setProbingId] = useState<string | null>(null);
  const [runningAll, setRunningAll] = useState(false);

  // Dynamic URL playground state
  const [dynSlug, setDynSlug] = useState("cloudflare-dogfood");
  const [nestedCat, setNestedCat] = useState("workers");
  const [nestedId, setNestedId] = useState("99");
  const [catchAllPath, setCatchAllPath] = useState("edge/r2/assets/v2");
  const [optPath, setOptPath] = useState("edge/worker");
  const [ssrQuery, setSsrQuery] = useState("source=playground");

  // API Verb tester state
  const [apiPayload, setApiPayload] = useState(
    '{"target":"cloudflare","dogfood":true}',
  );
  const [apiBusy, setApiBusy] = useState(false);
  const [apiResponse, setApiResponse] = useState<string | null>(null);

  // SSE stream tester state
  const [sseBusy, setSseBusy] = useState(false);
  const [sseChunks, setSseChunks] = useState<string[]>([]);

  const filteredSpecs = useMemo(() => {
    if (activeCategory === "All") return ROUTES_CATALOG;
    return ROUTES_CATALOG.filter((s) => s.category === activeCategory);
  }, [activeCategory]);

  const groupedSpecs = useMemo(() => {
    const groups = new Map<RouteCategory, RouteSpec[]>();
    for (const cat of ROUTE_CATEGORIES) {
      const items = filteredSpecs.filter((s) => s.category === cat);
      if (items.length > 0) groups.set(cat, items);
    }
    return groups;
  }, [filteredSpecs]);

  const stats = useMemo(() => {
    const vals = Object.values(results);
    const passed = vals.filter((r) => r.ok).length;
    const failed = vals.length - passed;
    const avgLatency =
      vals.length > 0
        ? Math.round(vals.reduce((acc, r) => acc + r.latencyMs, 0) / vals.length)
        : 0;
    return { total: vals.length, passed, failed, avgLatency };
  }, [results]);

  async function probeOne(spec: RouteSpec): Promise<void> {
    setProbingId(spec.id);
    const start = performance.now();
    try {
      const res = await fetch(spec.probeUrl, {
        method: spec.probeMethod,
        redirect: spec.probeRedirect ?? "follow",
        cache: "no-store",
        headers:
          spec.probeMethod === "POST" && spec.probeBody
            ? { "Content-Type": "application/json" }
            : undefined,
        body: spec.probeBody,
      });
      const latencyMs = Math.max(1, Math.round(performance.now() - start));
      const isOpaqueRedirect = res.type === "opaqueredirect" || res.status === 0;
      const status: number | "REDIRECT" = isOpaqueRedirect
        ? "REDIRECT"
        : res.status;
      const ok = isOpaqueRedirect
        ? spec.expectedStatus.some((s) => s >= 300 && s < 400)
        : spec.expectedStatus.includes(res.status);

      let snippet = "";
      if (isOpaqueRedirect) {
        snippet = "Edge/server redirect intercepted (redirect: manual)";
      } else if (spec.id === "api-stream") {
        const text = await res.text();
        snippet = text.split("\n").slice(0, 2).join(" ");
      } else {
        const ct = res.headers.get("content-type") ?? "";
        const text = await res.text();
        if (ct.includes("application/json")) {
          snippet = text.slice(0, 140);
        } else {
          const titleMatch = text.match(/<title>([^<]+)<\/title>/i);
          snippet = titleMatch
            ? `<title>${titleMatch[1]}</title>`
            : text.replace(/\s+/g, " ").slice(0, 120);
        }
      }

      setResults((prev) => ({
        ...prev,
        [spec.id]: {
          status,
          ok,
          latencyMs,
          poweredBy: res.headers.get("x-powered-by"),
          contentType: res.headers.get("content-type"),
          snippet,
        },
      }));
    } catch (err) {
      const latencyMs = Math.max(1, Math.round(performance.now() - start));
      setResults((prev) => ({
        ...prev,
        [spec.id]: {
          status: 0,
          ok: false,
          latencyMs,
          poweredBy: null,
          contentType: null,
          snippet: err instanceof Error ? err.message : "Network error",
        },
      }));
    } finally {
      setProbingId(null);
    }
  }

  async function runAllProbes() {
    if (runningAll) return;
    setRunningAll(true);
    try {
      for (const spec of ROUTES_CATALOG) {
        await probeOne(spec);
      }
    } finally {
      setRunningAll(false);
    }
  }

  async function testApiVerb(
    method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE" | "HEAD" | "OPTIONS",
  ) {
    setApiBusy(true);
    setApiResponse(null);
    const start = performance.now();
    try {
      const hasBody = method === "POST" || method === "PUT" || method === "PATCH";
      const res = await fetch(`/api/routes/methods?verb=${method}`, {
        method,
        headers: hasBody ? { "Content-Type": "application/json" } : undefined,
        body: hasBody ? apiPayload : undefined,
      });
      const ms = Math.max(1, Math.round(performance.now() - start));
      const text = method === "HEAD" || method === "OPTIONS" ? "(empty body)" : await res.text();
      setApiResponse(
        `HTTP ${res.status} (${ms}ms) · x-powered-by=${res.headers.get("x-powered-by") ?? "none"}\n${text}`,
      );
    } catch (err) {
      setApiResponse(err instanceof Error ? err.message : "Request failed");
    } finally {
      setApiBusy(false);
    }
  }

  async function runSseTest() {
    if (sseBusy) return;
    setSseBusy(true);
    setSseChunks([]);
    try {
      const res = await fetch("/api/routes/stream", { cache: "no-store" });
      if (!res.body) {
        setSseChunks(["No response body stream available"]);
        return;
      }
      const reader = res.body.getReader();
      const decoder = new TextDecoder();
      while (true) {
        const { value, done } = await reader.read();
        if (done) break;
        const chunk = decoder.decode(value, { stream: true }).trim();
        if (chunk) {
          setSseChunks((prev) => [...prev, chunk]);
        }
      }
    } catch (err) {
      setSseChunks([err instanceof Error ? err.message : "Stream error"]);
    } finally {
      setSseBusy(false);
    }
  }

  const sanitizedCatchAll = catchAllPath
    .split("/")
    .map((s) => s.trim())
    .filter(Boolean)
    .map(encodeURIComponent)
    .join("/");

  const sanitizedOpt = optPath
    .split("/")
    .map((s) => s.trim())
    .filter(Boolean)
    .map(encodeURIComponent)
    .join("/");

  return (
    <div className="space-y-10">
      {/* Controls & Live Probe Summary */}
      <div className="rounded-2xl border border-surface-800 bg-surface-900/80 p-4 sm:p-5 space-y-4">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="flex flex-wrap items-center gap-3">
            <button
              type="button"
              onClick={runAllProbes}
              disabled={runningAll}
              className="inline-flex items-center gap-2 px-4 py-2.5 rounded-lg bg-brand-500 hover:bg-brand-400 disabled:opacity-60 text-surface-950 text-xs font-semibold transition-colors"
            >
              {runningAll ? (
                <RefreshCw className="w-3.5 h-3.5 animate-spin" aria-hidden="true" />
              ) : (
                <Play className="w-3.5 h-3.5" aria-hidden="true" />
              )}
              {runningAll
                ? "Probing Routes..."
                : `Run All Live Probes (${ROUTES_CATALOG.length})`}
            </button>

            {stats.total > 0 && (
              <div className="flex flex-wrap items-center gap-2 text-xs font-mono">
                <span className="px-2.5 py-1 rounded-md bg-emerald-500/10 border border-emerald-500/30 text-emerald-300">
                  {stats.passed} passed
                </span>
                {stats.failed > 0 && (
                  <span className="px-2.5 py-1 rounded-md bg-rose-500/10 border border-rose-500/30 text-rose-300">
                    {stats.failed} failed
                  </span>
                )}
                <span className="px-2.5 py-1 rounded-md bg-surface-950 border border-surface-800 text-surface-300">
                  avg {stats.avgLatency}ms
                </span>
              </div>
            )}
          </div>

          <span className="text-[11px] font-mono text-surface-500">
            Probes run sequentially to stay well under KV 60 req/min limit
          </span>
        </div>

        <div className="flex flex-wrap gap-1.5 pt-2 border-t border-surface-800/80">
          {(["All", ...ROUTE_CATEGORIES] as const).map((cat) => (
            <button
              key={cat}
              type="button"
              onClick={() => setActiveCategory(cat)}
              className={`px-3 py-1.5 rounded-lg text-xs font-mono transition-colors border ${
                activeCategory === cat
                  ? "bg-brand-500/15 border-brand-500/40 text-brand-300"
                  : "bg-surface-950 border-surface-800 text-surface-400 hover:text-white hover:border-surface-700"
              }`}
            >
              {cat}
            </button>
          ))}
        </div>
      </div>

      {/* Categorized Route Matrix */}
      <div className="space-y-8">
        {Array.from(groupedSpecs.entries()).map(([category, specs]) => (
          <section key={category} className="space-y-4">
            <div className="flex items-center justify-between border-b border-surface-800/80 pb-2">
              <h2 className="text-lg font-bold text-white tracking-tight">
                {category}
              </h2>
              <span className="text-xs font-mono text-surface-500">
                {specs.length} {specs.length === 1 ? "route" : "routes"}
              </span>
            </div>

            <div className="grid md:grid-cols-2 gap-4">
              {specs.map((spec) => {
                const probe = results[spec.id];
                const isProbing = probingId === spec.id;
                const hardNav = isHardNavigationSpec(spec);

                return (
                  <div
                    key={spec.id}
                    className="rounded-xl border border-surface-800 bg-surface-900/70 p-5 flex flex-col justify-between gap-4 hover:border-surface-700 transition-colors"
                  >
                    <div className="space-y-2.5">
                      <div className="flex items-center justify-between gap-2 flex-wrap">
                        <span
                          className={`px-2 py-0.5 rounded-full border text-[10px] font-mono font-semibold uppercase tracking-wider ${MODE_BADGE[spec.mode]}`}
                        >
                          {spec.mode}
                        </span>
                        <code className="text-[11px] font-mono text-brand-300 bg-surface-950 border border-surface-800 px-2 py-0.5 rounded truncate max-w-full">
                          {spec.pattern}
                        </code>
                      </div>

                      <h3 className="text-base font-semibold text-white">
                        {spec.title}
                      </h3>

                      <p className="text-xs text-surface-300 leading-relaxed">
                        {spec.description}
                      </p>

                      <p className="text-[11px] font-mono text-surface-400 bg-surface-950/70 border border-surface-800/80 rounded-lg px-3 py-2">
                        CF: {spec.cfNotes}
                      </p>

                      {probe && (
                        <div
                          className={`rounded-lg border p-3 text-[11px] font-mono space-y-1 ${
                            probe.ok
                              ? "border-emerald-500/30 bg-emerald-500/5 text-emerald-200"
                              : "border-rose-500/30 bg-rose-500/5 text-rose-200"
                          }`}
                        >
                          <div className="flex items-center justify-between gap-2">
                            <span className="inline-flex items-center gap-1.5 font-semibold">
                              {probe.ok ? (
                                <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />
                              ) : (
                                <XCircle className="w-3.5 h-3.5 text-rose-400" />
                              )}
                              {spec.probeMethod} → {probe.status}
                            </span>
                            <span className="text-surface-400">
                              {probe.latencyMs}ms
                              {probe.poweredBy ? ` · ${probe.poweredBy}` : ""}
                            </span>
                          </div>
                          <div className="text-surface-400 truncate">
                            {probe.snippet}
                          </div>
                        </div>
                      )}
                    </div>

                    <div className="flex items-center justify-between gap-3 pt-3 border-t border-surface-800">
                      {hardNav ? (
                        <a
                          href={spec.href}
                          className="inline-flex items-center gap-1.5 text-xs font-mono text-brand-400 hover:text-brand-300 font-semibold"
                        >
                          Open Route
                          <ExternalLink className="w-3 h-3" aria-hidden="true" />
                        </a>
                      ) : (
                        <Link
                          href={spec.href}
                          className="inline-flex items-center gap-1.5 text-xs font-mono text-brand-400 hover:text-brand-300 font-semibold"
                        >
                          Open Route →
                        </Link>
                      )}

                      <button
                        type="button"
                        onClick={() => probeOne(spec)}
                        disabled={isProbing || runningAll}
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-surface-950 hover:bg-surface-800 border border-surface-700 text-xs font-mono text-surface-200 disabled:opacity-50 transition-colors"
                      >
                        <Activity className="w-3 h-3 text-brand-400" />
                        {isProbing ? "Probing..." : "Probe"}
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>
          </section>
        ))}
      </div>

      {/* Interactive Playgrounds */}
      <div className="grid lg:grid-cols-2 gap-6">
        {/* Dynamic Segment Playground */}
        <section className="rounded-2xl border border-surface-800 bg-surface-900/70 p-6 space-y-5">
          <div className="flex items-center gap-2 text-sm font-semibold text-white">
            <Terminal className="w-4 h-4 text-brand-400" aria-hidden="true" />
            Dynamic URL Playground
          </div>

          <div className="space-y-4 text-xs font-mono">
            <div className="space-y-1.5">
              <label className="text-surface-400 block">
                /routes/dynamic/[slug]
              </label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={dynSlug}
                  onChange={(e) => setDynSlug(e.target.value)}
                  className="flex-1 px-3 py-2 rounded-lg bg-surface-950 border border-surface-800 text-white focus:outline-none focus:border-brand-500"
                />
                <Link
                  href={`/routes/dynamic/${encodeURIComponent(dynSlug || "edge")}`}
                  className="px-3 py-2 rounded-lg bg-brand-500 hover:bg-brand-400 text-surface-950 font-semibold shrink-0"
                >
                  Visit
                </Link>
              </div>
            </div>

            <div className="space-y-1.5">
              <label className="text-surface-400 block">
                /routes/nested/[category]/[id]
              </label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={nestedCat}
                  onChange={(e) => setNestedCat(e.target.value)}
                  className="w-1/2 px-3 py-2 rounded-lg bg-surface-950 border border-surface-800 text-white focus:outline-none focus:border-brand-500"
                />
                <input
                  type="text"
                  value={nestedId}
                  onChange={(e) => setNestedId(e.target.value)}
                  className="w-1/2 px-3 py-2 rounded-lg bg-surface-950 border border-surface-800 text-white focus:outline-none focus:border-brand-500"
                />
                <Link
                  href={`/routes/nested/${encodeURIComponent(nestedCat || "app")}/${encodeURIComponent(nestedId || "1")}`}
                  className="px-3 py-2 rounded-lg bg-brand-500 hover:bg-brand-400 text-surface-950 font-semibold shrink-0"
                >
                  Visit
                </Link>
              </div>
            </div>

            <div className="space-y-1.5">
              <label className="text-surface-400 block">
                /routes/catch-all/[...segments]
              </label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={catchAllPath}
                  onChange={(e) => setCatchAllPath(e.target.value)}
                  className="flex-1 px-3 py-2 rounded-lg bg-surface-950 border border-surface-800 text-white focus:outline-none focus:border-brand-500"
                />
                <Link
                  href={`/routes/catch-all/${sanitizedCatchAll || "one"}`}
                  className="px-3 py-2 rounded-lg bg-brand-500 hover:bg-brand-400 text-surface-950 font-semibold shrink-0"
                >
                  Visit
                </Link>
              </div>
            </div>

            <div className="space-y-1.5">
              <label className="text-surface-400 block">
                /routes/optional-catch-all/[[...slug]] (leave empty for root)
              </label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={optPath}
                  onChange={(e) => setOptPath(e.target.value)}
                  className="flex-1 px-3 py-2 rounded-lg bg-surface-950 border border-surface-800 text-white focus:outline-none focus:border-brand-500"
                />
                <Link
                  href={
                    sanitizedOpt
                      ? `/routes/optional-catch-all/${sanitizedOpt}`
                      : "/routes/optional-catch-all"
                  }
                  className="px-3 py-2 rounded-lg bg-brand-500 hover:bg-brand-400 text-surface-950 font-semibold shrink-0"
                >
                  Visit
                </Link>
              </div>
            </div>

            <div className="space-y-1.5">
              <label className="text-surface-400 block">
                /routes/ssr?query
              </label>
              <div className="flex gap-2">
                <input
                  type="text"
                  value={ssrQuery}
                  onChange={(e) => setSsrQuery(e.target.value)}
                  className="flex-1 px-3 py-2 rounded-lg bg-surface-950 border border-surface-800 text-white focus:outline-none focus:border-brand-500"
                />
                <Link
                  href={`/routes/ssr?${ssrQuery}`}
                  className="px-3 py-2 rounded-lg bg-brand-500 hover:bg-brand-400 text-surface-950 font-semibold shrink-0"
                >
                  Visit
                </Link>
              </div>
            </div>
          </div>
        </section>

        {/* API Method & SSE Stream Tester */}
        <section className="rounded-2xl border border-surface-800 bg-surface-900/70 p-6 space-y-5">
          <div className="flex items-center gap-2 text-sm font-semibold text-white">
            <Radio className="w-4 h-4 text-cyan-400" aria-hidden="true" />
            Live API Verb &amp; SSE Stream Tester
          </div>

          <div className="space-y-3 text-xs font-mono">
            <label className="text-surface-400 block">
              JSON Body for POST / PUT / PATCH → <code>/api/routes/methods</code>
            </label>
            <input
              type="text"
              value={apiPayload}
              onChange={(e) => setApiPayload(e.target.value)}
              className="w-full px-3 py-2 rounded-lg bg-surface-950 border border-surface-800 text-white focus:outline-none focus:border-cyan-500"
            />

            <div className="flex flex-wrap gap-2">
              {(
                ["GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"] as const
              ).map((verb) => (
                <button
                  key={verb}
                  type="button"
                  disabled={apiBusy}
                  onClick={() => testApiVerb(verb)}
                  className="px-3 py-1.5 rounded-lg bg-surface-950 hover:bg-surface-800 border border-surface-700 text-surface-200 disabled:opacity-50"
                >
                  {verb}
                </button>
              ))}
            </div>

            {apiResponse && (
              <pre className="p-3 rounded-lg bg-surface-950 border border-surface-800 text-[11px] text-emerald-300 overflow-x-auto whitespace-pre-wrap">
                {apiResponse}
              </pre>
            )}
          </div>

          <div className="pt-4 border-t border-surface-800 space-y-3 text-xs font-mono">
            <div className="flex items-center justify-between gap-2">
              <span className="text-surface-400">
                SSE ReadableStream → <code>/api/routes/stream</code>
              </span>
              <button
                type="button"
                onClick={runSseTest}
                disabled={sseBusy}
                className="px-3 py-1.5 rounded-lg bg-sky-500/15 hover:bg-sky-500/25 border border-sky-500/30 text-sky-300 disabled:opacity-50"
              >
                {sseBusy ? "Streaming..." : "Stream SSE Chunks"}
              </button>
            </div>

            {sseChunks.length > 0 && (
              <div className="p-3 rounded-lg bg-surface-950 border border-surface-800 space-y-1.5 text-[11px] text-sky-200 max-h-44 overflow-y-auto">
                {sseChunks.map((c, idx) => (
                  <div key={idx} className="whitespace-pre-wrap break-all">
                    {c}
                  </div>
                ))}
              </div>
            )}
          </div>
        </section>
      </div>
    </div>
  );
}
