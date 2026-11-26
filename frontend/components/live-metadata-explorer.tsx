"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  Activity,
  Check,
  ChevronDown,
  ChevronRight,
  Copy,
  FileCode2,
  Globe,
  HardDrive,
  Lock,
  Play,
  RefreshCw,
  Search,
  Server,
  ShieldCheck,
  X,
} from "lucide-react";
import initialSnapshot from "@/lib/build-metadata.json";
import "./landing-showcase.css";

export interface IsrDetailEntry {
  path: string;
  revalidate: number | null;
  tags: string[];
}

export interface LiveEdgeInfo {
  served_at: string;
  runtime: string;
  bindings: {
    ASSETS: boolean;
    RATE_LIMIT: boolean;
    AI: boolean;
    AUTH_SECRET: boolean;
  };
  request_headers: {
    host: string | null;
    cf_ray: string | null;
    cf_ipcountry: string | null;
    cf_colo: string | null;
    x_powered_by: string | null;
  };
}

export interface BuildMetadataSnapshot {
  schema_version: string;
  source: string;
  synced_at: string;
  generated_at: string;
  app_name: string;
  domain: string;
  output_mode: string;
  package_manager: string;
  dist_dir: string;
  export_dir: string;
  build_id: string;
  has_app_router: boolean;
  git_commit: string;
  git_dirty: boolean;
  config: {
    app_name: string;
    domain: string;
    port: number;
    environment: string;
    target_type: string;
  };
  build_lock: {
    git_commit: string;
    git_dirty: boolean;
    generated_at: string;
    metadata_file: string;
    config_hash: string | null;
  } | null;
  proxy: {
    file: string;
    runtime: string;
    matchers: string[];
  } | null;
  route_info: {
    counts: {
      static_routes: number;
      dynamic_routes: number;
      isr_routes: number;
      api_routes: number;
      public_assets: number;
    };
    static_routes: string[];
    dynamic_routes: string[];
    api_routes: string[];
    isr_detail: IsrDetailEntry[];
  };
  public_assets: string[];
  detected_features: Record<string, boolean | string[]>;
  edge_protection: {
    provider: string;
    compatibility_date: string;
    bindings: string[];
    enabled: boolean;
    public_paths: string[];
    protected_paths: string[];
    rate_limit_paths: string[];
    requests_per_minute: number;
  };
  live_edge?: LiveEdgeInfo;
}

type JsonSection =
  | "all"
  | "route_info"
  | "build_lock"
  | "detected_features"
  | "edge_protection"
  | "live_edge";

type NetworkTab = "traces" | "browser";

interface RouteClassification {
  bucket: string;
  detail: string;
  matchedKey: string | null;
  suggestedSection: JsonSection;
  edgeGuard: string;
  proxyMatched: boolean;
}

interface NetworkTraceEntry {
  id: string;
  source: "probe" | "browser";
  method: "GET";
  url: string;
  status: number;
  ok: boolean;
  latencyMs: number;
  bytes: number;
  protocol: string | null;
  contentType: string | null;
  poweredBy: string | null;
  cfRay: string | null;
  cacheControl: string | null;
  classification: RouteClassification;
  tracedAt: string;
}

function matchGlob(pattern: string, pathname: string): boolean {
  if (pattern.endsWith("/*")) {
    const prefix = pattern.slice(0, -1);
    return pathname === pattern.slice(0, -2) || pathname.startsWith(prefix);
  }
  return pattern === pathname;
}

function matchDynamicPattern(pattern: string, pathname: string): boolean {
  const cleanPattern = pattern.replace("/(.)", "/");
  const pSegs = cleanPattern.split("/").filter(Boolean);
  const uSegs = pathname.split("/").filter(Boolean);

  for (let i = 0; i < pSegs.length; i++) {
    const seg = pSegs[i];
    if (seg.startsWith("[[...") && seg.endsWith("]]")) return true;
    if (seg.startsWith("[...") && seg.endsWith("]")) return uSegs.length >= i + 1;
    if (i >= uSegs.length) return false;
    if (seg.startsWith("[") && seg.endsWith("]")) continue;
    if (seg !== uSegs[i]) return false;
  }
  return pSegs.length === uSegs.length;
}

function computeEdgeGuard(pathname: string, meta: BuildMetadataSnapshot): string {
  const prot = meta.edge_protection;
  if (!prot.enabled) return "DISABLED";
  if (prot.rate_limit_paths.some((p) => matchGlob(p, pathname))) {
    return `KV RATE-LIMIT (${prot.requests_per_minute}/m)`;
  }
  if (prot.protected_paths.some((p) => matchGlob(p, pathname))) {
    return "HMAC AUTH GUARD";
  }
  if (
    pathname.startsWith("/_next/") ||
    prot.public_paths.some((p) => matchGlob(p, pathname))
  ) {
    return "PUBLIC ALLOW";
  }
  return "PUBLIC / EDGE";
}

function classifyPath(
  rawUrl: string,
  meta: BuildMetadataSnapshot,
): RouteClassification {
  const pathname = rawUrl.split("?")[0] || "/";
  const proxyMatched =
    Boolean(meta.proxy) &&
    !pathname.startsWith("/_next/static") &&
    !pathname.startsWith("/_next/image") &&
    pathname !== "/favicon.ico";
  const edgeGuard = computeEdgeGuard(pathname, meta);

  if (pathname.startsWith("/_next/static/")) {
    return {
      bucket: "static_assets.next_static",
      detail: "Immutable R2 chunk (content-hashed)",
      matchedKey: null,
      suggestedSection: "all",
      edgeGuard,
      proxyMatched,
    };
  }

  if (meta.public_assets.includes(pathname)) {
    return {
      bucket: "public_assets · R2",
      detail: "Static file served from R2 ASSETS bucket",
      matchedKey: pathname,
      suggestedSection: "all",
      edgeGuard,
      proxyMatched,
    };
  }

  const matchedApi = meta.route_info.api_routes.find(
    (p) => p === pathname || matchDynamicPattern(p, pathname),
  );
  if (matchedApi) {
    return {
      bucket: "route_info.api_routes",
      detail: matchedApi,
      matchedKey: matchedApi,
      suggestedSection: "route_info",
      edgeGuard,
      proxyMatched,
    };
  }

  const isrMatch = meta.route_info.isr_detail.find((d) => d.path === pathname);
  if (isrMatch) {
    return {
      bucket: "route_info.isr_detail",
      detail: isrMatch.revalidate
        ? `ISR revalidate: ${isrMatch.revalidate}s`
        : "Prerendered HTML",
      matchedKey: isrMatch.path,
      suggestedSection: "route_info",
      edgeGuard,
      proxyMatched,
    };
  }

  const dynMatch = meta.route_info.dynamic_routes.find((p) =>
    matchDynamicPattern(p, pathname),
  );
  if (dynMatch) {
    return {
      bucket: "route_info.dynamic_routes",
      detail: `Pattern ${dynMatch}`,
      matchedKey: dynMatch,
      suggestedSection: "route_info",
      edgeGuard,
      proxyMatched,
    };
  }

  if (meta.route_info.static_routes.includes(pathname)) {
    return {
      bucket: "route_info.static_routes",
      detail: "Classified static/app route",
      matchedKey: pathname,
      suggestedSection: "route_info",
      edgeGuard,
      proxyMatched,
    };
  }

  return {
    bucket: "runtime-fallback",
    detail: "Resolved dynamically at Worker",
    matchedKey: null,
    suggestedSection: "all",
    edgeGuard,
    proxyMatched,
  };
}

function isProbableRouteString(val: string): boolean {
  if (!val.startsWith("/") || val.length > 90) return false;
  if (val.includes("(") || val.includes("*") || val.includes("[")) return false;
  return true;
}

const FILTER_EMPTY = Symbol("filter_empty");

function filterJsonValue(value: unknown, query: string): unknown {
  const q = query.trim().toLowerCase();
  if (!q) return value;

  if (Array.isArray(value)) {
    const out = value
      .map((item) => filterJsonValue(item, q))
      .filter((item) => item !== FILTER_EMPTY);
    return out.length > 0 ? out : FILTER_EMPTY;
  }

  if (value !== null && typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      if (k.toLowerCase().includes(q)) {
        out[k] = v;
        continue;
      }
      const sub = filterJsonValue(v, q);
      if (sub !== FILTER_EMPTY) {
        out[k] = sub;
      }
    }
    return Object.keys(out).length > 0 ? out : FILTER_EMPTY;
  }

  return String(value).toLowerCase().includes(q) ? value : FILTER_EMPTY;
}

function JsonTreeNode({
  name,
  value,
  depth,
  expandSignal,
  forceOpen,
  highlightedValue,
  onProbeRoute,
}: {
  name: string | null;
  value: unknown;
  depth: number;
  expandSignal: { version: number; open: boolean };
  forceOpen: boolean;
  highlightedValue: string | null;
  onProbeRoute: (path: string) => void;
}) {
  const isArray = Array.isArray(value);
  const isObject = value !== null && typeof value === "object" && !isArray;
  const isExpandable = isArray || isObject;

  const [open, setOpen] = useState<boolean>(() => forceOpen || depth < 2);

  useEffect(() => {
    if (forceOpen) {
      setOpen(true);
    }
  }, [forceOpen]);

  useEffect(() => {
    if (expandSignal.version > 0) {
      setOpen(expandSignal.open);
    }
  }, [expandSignal]);

  if (!isExpandable) {
    const isMatch =
      typeof value === "string" &&
      highlightedValue !== null &&
      value === highlightedValue;

    let rendered: React.ReactNode;
    if (typeof value === "string") {
      const clickable = isProbableRouteString(value);
      rendered = clickable ? (
        <button
          type="button"
          onClick={() => onProbeRoute(value)}
          title={`Trace live HTTP request to ${value}`}
          className="text-emerald-300 hover:text-brand-200 underline decoration-dotted underline-offset-2 text-left"
        >
          &quot;{value}&quot;
        </button>
      ) : (
        <span className="text-emerald-300 break-all">&quot;{value}&quot;</span>
      );
    } else if (typeof value === "number") {
      rendered = <span className="text-amber-300">{String(value)}</span>;
    } else if (typeof value === "boolean") {
      rendered = (
        <span className={value ? "text-violet-300" : "text-surface-400"}>
          {String(value)}
        </span>
      );
    } else {
      rendered = <span className="text-surface-500">null</span>;
    }

    return (
      <div
        className={`leading-relaxed pl-3 rounded transition-colors ${
          isMatch
            ? "nd-node-active border border-emerald-400/50 py-0.5 pr-2 my-0.5 inline-flex flex-wrap items-center gap-1.5"
            : ""
        }`}
      >
        {name !== null && (
          <span className="text-cyan-300">&quot;{name}&quot;: </span>
        )}
        {rendered}
        {isMatch && (
          <span className="px-1.5 py-0.2 rounded bg-emerald-400/20 text-emerald-200 text-[9px] font-semibold uppercase tracking-wider">
            MATCHED
          </span>
        )}
      </div>
    );
  }

  const entries = isArray
    ? (value as unknown[]).map((v, i) => [String(i), v] as const)
    : Object.entries(value as Record<string, unknown>);
  const openChar = isArray ? "[" : "{";
  const closeChar = isArray ? "]" : "}";
  const summaryText = isArray
    ? `${entries.length} items`
    : `${entries.length} keys`;

  return (
    <div className="leading-relaxed">
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        className="inline-flex items-center gap-1 hover:bg-surface-900 rounded px-1 -ml-1 text-left"
      >
        {open ? (
          <ChevronDown className="w-3 h-3 text-surface-500 shrink-0" />
        ) : (
          <ChevronRight className="w-3 h-3 text-surface-500 shrink-0" />
        )}
        {name !== null && (
          <span className="text-cyan-300">&quot;{name}&quot;: </span>
        )}
        <span className="text-surface-400">
          {open ? (
            openChar
          ) : (
            <>
              {openChar}{" "}
              <span className="text-surface-500 text-[10px]">{summaryText}</span>{" "}
              {closeChar}
            </>
          )}
        </span>
      </button>

      {open && (
        <div className="ml-4 pl-2 border-l border-surface-800/80">
          {entries.length === 0 ? (
            <div className="text-surface-500 pl-3 text-[11px]">(empty)</div>
          ) : (
            entries.map(([k, v]) => (
              <JsonTreeNode
                key={k}
                name={isArray ? null : k}
                value={v}
                depth={depth + 1}
                expandSignal={expandSignal}
                forceOpen={forceOpen}
                highlightedValue={highlightedValue}
                onProbeRoute={onProbeRoute}
              />
            ))
          )}
        </div>
      )}

      {open && <div className="pl-3 text-surface-400">{closeChar}</div>}
    </div>
  );
}

const DEFAULT_TRACE_PATHS = [
  "/api/metadata",
  "/nextdeploy-metadata.json",
  "/api/health",
  "/routes/static",
  "/routes/isr",
  "/routes/ssr?trace=metadata",
  "/routes/dynamic/cloudflare",
  "/api/routes/items/meta-check",
];

function entryToBrowserTrace(
  entry: PerformanceResourceTiming,
  meta: BuildMetadataSnapshot,
  origin: string,
): NetworkTraceEntry | null {
  try {
    const parsed = new URL(entry.name);
    if (parsed.origin !== origin) return null;
    const url = `${parsed.pathname}${parsed.search}`;
    const latencyMs = Math.max(1, Math.round(entry.duration));
    const bytes =
      entry.transferSize > 0 ? entry.transferSize : entry.encodedBodySize;
    const status = entry.responseStatus || 200;
    return {
      id: `browser:${url}`,
      source: "browser",
      method: "GET",
      url,
      status,
      ok: status >= 200 && status < 400,
      latencyMs,
      bytes,
      protocol: entry.nextHopProtocol || null,
      contentType: entry.initiatorType || null,
      poweredBy: null,
      cfRay: null,
      cacheControl:
        entry.transferSize === 0 && entry.decodedBodySize > 0
          ? "memory/disk cache"
          : null,
      classification: classifyPath(url, meta),
      tracedAt: new Date().toISOString(),
    };
  } catch {
    return null;
  }
}

export default function LiveMetadataExplorer() {
  const [snapshot, setSnapshot] = useState<BuildMetadataSnapshot>(
    initialSnapshot as BuildMetadataSnapshot,
  );
  const [section, setSection] = useState<JsonSection>("all");
  const [searchQuery, setSearchQuery] = useState("");
  const [expandSignal, setExpandSignal] = useState({ version: 0, open: true });
  const [copied, setCopied] = useState(false);

  const [networkTab, setNetworkTab] = useState<NetworkTab>("traces");
  const [traces, setTraces] = useState<NetworkTraceEntry[]>([]);
  const [browserEntries, setBrowserEntries] = useState<NetworkTraceEntry[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [inFlightUrl, setInFlightUrl] = useState<string | null>(null);
  const [tracingSuite, setTracingSuite] = useState(false);
  const [customPath, setCustomPath] = useState("/routes/streaming");

  const traceUrl = useCallback(
    async (
      rawPath: string,
      activeMeta: BuildMetadataSnapshot,
      focusSelection = false,
    ): Promise<BuildMetadataSnapshot> => {
      const url = rawPath.startsWith("/") ? rawPath : `/${rawPath}`;
      setInFlightUrl(url);
      const start = performance.now();
      let nextMeta = activeMeta;

      try {
        const res = await fetch(url, { method: "GET", cache: "no-store" });
        const latencyMs = Math.max(1, Math.round(performance.now() - start));
        const text = await res.text();
        const bytes = new TextEncoder().encode(text).length;

        const fullUrl =
          typeof window !== "undefined"
            ? new URL(url, window.location.origin).toString()
            : url;
        const perfEntries =
          typeof performance !== "undefined"
            ? (performance.getEntriesByName(
                fullUrl,
                "resource",
              ) as PerformanceResourceTiming[])
            : [];
        const latestPerf =
          perfEntries.length > 0 ? perfEntries[perfEntries.length - 1] : null;

        const cleanUrl = url.split("?")[0];
        if (res.ok && cleanUrl === "/nextdeploy-metadata.json") {
          try {
            const parsed = JSON.parse(text) as BuildMetadataSnapshot;
            if (parsed && parsed.build_id) {
              nextMeta = { ...parsed, live_edge: activeMeta.live_edge };
              setSnapshot(nextMeta);
            }
          } catch {
            // Keep existing snapshot
          }
        } else if (res.ok && cleanUrl === "/api/metadata") {
          try {
            const parsed = JSON.parse(text) as BuildMetadataSnapshot;
            if (parsed && parsed.build_id) {
              const useParsedBase =
                Date.parse(parsed.synced_at || "") >=
                Date.parse(activeMeta.synced_at || "");
              nextMeta = useParsedBase
                ? parsed
                : { ...activeMeta, live_edge: parsed.live_edge };
              setSnapshot(nextMeta);
            }
          } catch {
            // Keep existing snapshot
          }
        }

        const classification = classifyPath(url, nextMeta);
        const traceId = `probe:${url}`;
        const entry: NetworkTraceEntry = {
          id: traceId,
          source: "probe",
          method: "GET",
          url,
          status: res.status,
          ok: res.ok,
          latencyMs,
          bytes,
          protocol: latestPerf?.nextHopProtocol || null,
          contentType: res.headers.get("content-type"),
          poweredBy: res.headers.get("x-powered-by"),
          cfRay: res.headers.get("cf-ray"),
          cacheControl: res.headers.get("cache-control"),
          classification,
          tracedAt: new Date().toISOString(),
        };

        setTraces((prev) => [entry, ...prev.filter((i) => i.id !== traceId)].slice(0, 14));
        setSelectedId(traceId);
        if (focusSelection) {
          setNetworkTab("traces");
          if (classification.matchedKey) {
            setSection(classification.suggestedSection);
          }
        }
      } catch (err) {
        const latencyMs = Math.max(1, Math.round(performance.now() - start));
        const traceId = `probe:${url}`;
        const entry: NetworkTraceEntry = {
          id: traceId,
          source: "probe",
          method: "GET",
          url,
          status: 0,
          ok: false,
          latencyMs,
          bytes: 0,
          protocol: null,
          contentType: null,
          poweredBy: null,
          cfRay: null,
          cacheControl: err instanceof Error ? err.message : "Network error",
          classification: classifyPath(url, nextMeta),
          tracedAt: new Date().toISOString(),
        };
        setTraces((prev) => [entry, ...prev.filter((i) => i.id !== traceId)].slice(0, 14));
        setSelectedId(traceId);
      } finally {
        setInFlightUrl(null);
      }

      return nextMeta;
    },
    [],
  );

  // Sync live metadata + seed initial traces (strictly 2 requests on mount).
  useEffect(() => {
    let active = true;
    (async () => {
      let current = initialSnapshot as BuildMetadataSnapshot;
      if (!active) return;
      current = await traceUrl("/nextdeploy-metadata.json", current);
      if (!active) return;
      await traceUrl("/api/metadata", current);
    })();
    return () => {
      active = false;
    };
  }, [traceUrl]);

  // Capture real browser network resource timings (zero extra requests).
  useEffect(() => {
    if (typeof window === "undefined" || typeof performance === "undefined") {
      return;
    }
    const origin = window.location.origin;

    const collectResources = () => {
      const raw = performance.getEntriesByType(
        "resource",
      ) as PerformanceResourceTiming[];
      const mapped = new Map<string, NetworkTraceEntry>();
      for (const r of raw) {
        const parsed = entryToBrowserTrace(r, snapshot, origin);
        if (parsed) mapped.set(parsed.id, parsed);
      }
      setBrowserEntries(Array.from(mapped.values()).reverse().slice(0, 24));
    };

    collectResources();

    let observer: PerformanceObserver | null = null;
    try {
      observer = new PerformanceObserver(() => {
        collectResources();
      });
      observer.observe({ type: "resource", buffered: true });
    } catch {
      // Fallback already populated via getEntriesByType
    }

    return () => {
      observer?.disconnect();
    };
  }, [snapshot]);

  const runLiveSuite = async () => {
    if (tracingSuite) return;
    setTracingSuite(true);
    setNetworkTab("traces");
    try {
      let current = snapshot;
      for (const path of DEFAULT_TRACE_PATHS) {
        current = await traceUrl(path, current);
      }
    } finally {
      setTracingSuite(false);
    }
  };

  const activeList = networkTab === "traces" ? traces : browserEntries;
  const maxLatencyMs = useMemo(() => {
    if (activeList.length === 0) return 1;
    return Math.max(1, ...activeList.map((e) => e.latencyMs));
  }, [activeList]);

  const selectedTrace = useMemo(() => {
    const all = [...traces, ...browserEntries];
    return all.find((e) => e.id === selectedId) ?? traces[0] ?? browserEntries[0] ?? null;
  }, [traces, browserEntries, selectedId]);

  const highlightedValue = selectedTrace?.classification.matchedKey ?? null;

  const baseSectionJson = useMemo(() => {
    switch (section) {
      case "route_info":
        return snapshot.route_info;
      case "build_lock":
        return snapshot.build_lock;
      case "detected_features":
        return snapshot.detected_features;
      case "edge_protection":
        return snapshot.edge_protection;
      case "live_edge":
        return snapshot.live_edge ?? { status: "awaiting /api/metadata trace" };
      default:
        return snapshot;
    }
  }, [section, snapshot]);

  const displayedJson = useMemo(() => {
    if (!searchQuery.trim()) return baseSectionJson;
    const filtered = filterJsonValue(baseSectionJson, searchQuery);
    return filtered === FILTER_EMPTY ? null : filtered;
  }, [baseSectionJson, searchQuery]);

  const handleCopyJson = async () => {
    try {
      await navigator.clipboard.writeText(
        JSON.stringify(displayedJson ?? {}, null, 2),
      );
      setCopied(true);
      setTimeout(() => setCopied(false), 1600);
    } catch {
      // Clipboard unavailable
    }
  };

  const handleSelectRow = (entry: NetworkTraceEntry) => {
    setSelectedId(entry.id);
    if (entry.classification.matchedKey) {
      setSection(entry.classification.suggestedSection);
    }
  };

  const pipelineStages = useMemo(() => {
    const t = selectedTrace;
    const isBusy = inFlightUrl !== null;
    const activeUrl = inFlightUrl ?? t?.url ?? "/api/metadata";
    const cls = t?.classification ?? classifyPath(activeUrl, snapshot);
    return [
      {
        num: "01",
        label: "Browser",
        value: `GET ${activeUrl}`,
        sub: t ? `${(t.bytes / 1024).toFixed(1)} KB` : "request",
        icon: Globe,
        active: isBusy || Boolean(t),
      },
      {
        num: "02",
        label: "Edge Guard",
        value: cls.edgeGuard,
        sub: "nextdeploy.yml",
        icon: ShieldCheck,
        active: Boolean(t),
      },
      {
        num: "03",
        label: "proxy.ts",
        value: cls.proxyMatched ? "MATCHED" : "BYPASSED",
        sub: t?.poweredBy ?? "x-powered-by",
        icon: Server,
        active: Boolean(t) && cls.proxyMatched,
      },
      {
        num: "04",
        label: "metadata.json",
        value: cls.bucket,
        sub: cls.detail,
        icon: FileCode2,
        active: Boolean(cls.matchedKey),
      },
      {
        num: "05",
        label: "Edge / R2",
        value: t ? `HTTP ${t.status || "ERR"} · ${t.latencyMs}ms` : "READY",
        sub: t?.cfRay ? `cf-ray ${t.cfRay}` : t?.protocol ?? "verified",
        icon: HardDrive,
        active: Boolean(t?.ok),
      },
    ];
  }, [selectedTrace, inFlightUrl, snapshot]);

  return (
    <div className="space-y-4">
      {/* 1. Live Build Fingerprint Strip */}
      <div className="grid grid-cols-2 sm:grid-cols-5 gap-2.5 font-mono text-xs">
        <div className="p-3 rounded-xl bg-surface-950/90 border border-surface-800">
          <div className="text-[10px] uppercase text-surface-500">Build ID</div>
          <div className="text-brand-300 font-semibold truncate mt-0.5">
            {snapshot.build_id}
          </div>
        </div>
        <div className="p-3 rounded-xl bg-surface-950/90 border border-surface-800">
          <div className="text-[10px] uppercase text-surface-500">
            Git &amp; build.lock
          </div>
          <div className="text-white font-semibold truncate mt-0.5 flex items-center gap-1.5">
            <Lock className="w-3 h-3 text-brand-400 shrink-0" />
            {snapshot.git_commit}
            {snapshot.build_lock?.config_hash
              ? ` · ${snapshot.build_lock.config_hash.slice(0, 8)}`
              : ""}
          </div>
        </div>
        <div className="p-3 rounded-xl bg-surface-950/90 border border-surface-800">
          <div className="text-[10px] uppercase text-surface-500">
            Routes Classified
          </div>
          <div className="text-cyan-300 font-semibold mt-0.5">
            {snapshot.route_info.counts.static_routes} static ·{" "}
            {snapshot.route_info.counts.dynamic_routes} dyn ·{" "}
            {snapshot.route_info.counts.isr_routes} ISR
          </div>
        </div>
        <div className="p-3 rounded-xl bg-surface-950/90 border border-surface-800">
          <div className="text-[10px] uppercase text-surface-500">
            Edge Runtime
          </div>
          <div className="text-emerald-400 font-semibold truncate mt-0.5">
            {snapshot.live_edge?.runtime ?? "syncing..."}
          </div>
        </div>
        <div className="col-span-2 sm:col-span-1 p-3 rounded-xl bg-surface-950/90 border border-surface-800">
          <div className="text-[10px] uppercase text-surface-500">Synced At</div>
          <div className="text-surface-200 truncate mt-0.5">
            {snapshot.synced_at.replace("T", " ").slice(0, 19)}Z
          </div>
        </div>
      </div>

      {/* 2. Live 5-Stage Animated Request Pipeline */}
      <div className="rounded-xl bg-surface-950/95 border border-surface-800 p-3.5 sm:p-4 space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2 font-mono text-[11px]">
          <span className="text-surface-300 font-semibold">
            Request Execution Pipeline{" "}
            <span className="text-surface-500 font-normal">
              (click any JSON route or network row to inspect)
            </span>
          </span>
          {inFlightUrl ? (
            <span className="text-brand-300 inline-flex items-center gap-1.5">
              <RefreshCw className="w-3 h-3 animate-spin" />
              Tracing {inFlightUrl}...
            </span>
          ) : selectedTrace ? (
            <span className="text-emerald-300">
              Mapped: {selectedTrace.url} → {selectedTrace.classification.bucket}
            </span>
          ) : null}
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-5 gap-2 items-stretch">
          {pipelineStages.map((stage, idx) => {
            const Icon = stage.icon;
            return (
              <div key={stage.num} className="relative flex items-center gap-2 min-w-0">
                <div
                  className={`flex-1 min-w-0 p-2.5 rounded-lg border font-mono transition-colors ${
                    stage.active
                      ? "border-emerald-500/40 bg-emerald-500/5"
                      : "border-surface-800 bg-surface-900/60"
                  }`}
                >
                  <div className="flex items-center justify-between gap-1 text-[10px] text-surface-400">
                    <span className="inline-flex items-center gap-1 text-brand-300 font-semibold">
                      <Icon className="w-3 h-3 shrink-0" />
                      {stage.label}
                    </span>
                    <span>{stage.num}</span>
                  </div>
                  <div className="text-[11px] text-white font-semibold truncate mt-1">
                    {stage.value}
                  </div>
                  <div className="text-[10px] text-surface-400 truncate mt-0.5">
                    {stage.sub}
                  </div>
                </div>

                {idx < pipelineStages.length - 1 && (
                  <div className="hidden sm:block w-4 shrink-0 nd-pipe-track">
                    {inFlightUrl && <div className="nd-pipe-beam" />}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {/* 3. Dual Pane: Searchable Structured JSON Tree + Network Request Map */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
        {/* Left: Structured JSON Tree */}
        <div className="lg:col-span-7 rounded-xl bg-surface-950 border border-surface-800 flex flex-col min-w-0">
          <div className="px-3.5 py-2.5 border-b border-surface-800 flex flex-wrap items-center justify-between gap-2">
            <div className="flex flex-wrap items-center gap-1 font-mono text-[11px]">
              {(
                [
                  { id: "all", label: "Full JSON" },
                  { id: "route_info", label: "route_info" },
                  { id: "build_lock", label: "build_lock" },
                  { id: "detected_features", label: "features" },
                  { id: "edge_protection", label: "protection" },
                  { id: "live_edge", label: "live_edge" },
                ] as const
              ).map((tab) => (
                <button
                  key={tab.id}
                  type="button"
                  onClick={() => setSection(tab.id)}
                  className={`px-2 py-1 rounded transition-colors ${
                    section === tab.id
                      ? "bg-brand-500/15 text-brand-300 border border-brand-500/30"
                      : "text-surface-400 hover:text-white border border-transparent"
                  }`}
                >
                  {tab.label}
                </button>
              ))}
            </div>

            <div className="flex items-center gap-1.5 font-mono text-[11px]">
              <button
                type="button"
                onClick={() =>
                  setExpandSignal((s) => ({ version: s.version + 1, open: true }))
                }
                className="px-2 py-1 rounded bg-surface-900 hover:bg-surface-800 text-surface-300 border border-surface-800"
              >
                Expand
              </button>
              <button
                type="button"
                onClick={() =>
                  setExpandSignal((s) => ({ version: s.version + 1, open: false }))
                }
                className="px-2 py-1 rounded bg-surface-900 hover:bg-surface-800 text-surface-300 border border-surface-800"
              >
                Collapse
              </button>
              <button
                type="button"
                onClick={handleCopyJson}
                className="inline-flex items-center gap-1 px-2 py-1 rounded bg-surface-900 hover:bg-surface-800 text-surface-300 border border-surface-800"
              >
                {copied ? (
                  <Check className="w-3 h-3 text-emerald-400" />
                ) : (
                  <Copy className="w-3 h-3" />
                )}
                {copied ? "Copied" : "Copy"}
              </button>
            </div>
          </div>

          {/* Search bar inside JSON tree */}
          <div className="px-3.5 py-2 border-b border-surface-800/80 flex items-center gap-2 font-mono text-[11px]">
            <Search className="w-3.5 h-3.5 text-surface-500 shrink-0" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Filter JSON keys or route paths (e.g. /routes/isr, dynamic, proxy)..."
              aria-label="Filter metadata JSON"
              className="flex-1 min-w-0 bg-transparent text-white placeholder:text-surface-500 focus:outline-none"
            />
            {searchQuery && (
              <button
                type="button"
                onClick={() => setSearchQuery("")}
                aria-label="Clear filter"
                className="text-surface-400 hover:text-white"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          <div className="p-3.5 font-mono text-[11px] sm:text-xs overflow-auto max-h-[430px]">
            <div className="text-[10px] text-surface-500 mb-2">
              Click any green route string (e.g.{" "}
              <span className="text-emerald-300">&quot;/routes/isr&quot;</span>) to
              fire a live browser request and watch it trace through the pipeline.
            </div>
            {displayedJson === null ? (
              <div className="text-xs text-surface-500 py-8 text-center">
                No matching keys or values for &quot;{searchQuery}&quot;
              </div>
            ) : (
              <JsonTreeNode
                name={null}
                value={displayedJson}
                depth={0}
                expandSignal={expandSignal}
                forceOpen={
                  Boolean(searchQuery.trim()) || section === "route_info"
                }
                highlightedValue={highlightedValue}
                onProbeRoute={(path) => {
                  void traceUrl(path, snapshot, true);
                }}
              />
            )}
          </div>
        </div>

        {/* Right: Actual Network Request Map & Browser Resource Waterfall */}
        <div className="lg:col-span-5 rounded-xl bg-surface-950 border border-surface-800 flex flex-col min-w-0">
          <div className="px-3.5 py-2.5 border-b border-surface-800 flex flex-wrap items-center justify-between gap-2">
            <div className="inline-flex items-center gap-1 font-mono text-[11px]">
              <button
                type="button"
                onClick={() => setNetworkTab("traces")}
                className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded transition-colors ${
                  networkTab === "traces"
                    ? "bg-brand-500/15 text-brand-300 border border-brand-500/30"
                    : "text-surface-400 hover:text-white border border-transparent"
                }`}
              >
                <Activity className="w-3 h-3" />
                Live Traces ({traces.length})
              </button>
              <button
                type="button"
                onClick={() => setNetworkTab("browser")}
                className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded transition-colors ${
                  networkTab === "browser"
                    ? "bg-cyan-500/15 text-cyan-300 border border-cyan-500/30"
                    : "text-surface-400 hover:text-white border border-transparent"
                }`}
              >
                <Globe className="w-3 h-3" />
                Browser Waterfall ({browserEntries.length})
              </button>
            </div>

            <button
              type="button"
              onClick={runLiveSuite}
              disabled={tracingSuite}
              className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-brand-500 hover:bg-brand-400 disabled:opacity-60 text-surface-950 text-[11px] font-mono font-semibold transition-colors"
            >
              {tracingSuite ? (
                <RefreshCw className="w-3 h-3 animate-spin" />
              ) : (
                <Play className="w-3 h-3" />
              )}
              {tracingSuite ? "Tracing..." : "Trace Live Network"}
            </button>
          </div>

          <form
            onSubmit={(e) => {
              e.preventDefault();
              void traceUrl(customPath, snapshot, true);
            }}
            className="px-3.5 py-2 border-b border-surface-800/80 flex gap-2 font-mono text-[11px]"
          >
            <input
              type="text"
              value={customPath}
              onChange={(e) => setCustomPath(e.target.value)}
              placeholder="/routes/ssr?test=1"
              aria-label="Route path to trace"
              className="flex-1 min-w-0 px-2.5 py-1 rounded bg-surface-900 border border-surface-800 text-white focus:outline-none focus:border-brand-500"
            />
            <button
              type="submit"
              className="px-2.5 py-1 rounded bg-surface-900 hover:bg-surface-800 border border-surface-700 text-surface-200 shrink-0"
            >
              Fetch
            </button>
          </form>

          <div className="p-3 space-y-2 overflow-y-auto max-h-[375px]">
            {activeList.length === 0 ? (
              <div className="text-xs font-mono text-surface-500 py-8 text-center">
                {networkTab === "traces"
                  ? "Initializing live network requests..."
                  : "No same-origin browser resource timings recorded yet."}
              </div>
            ) : (
              activeList.map((t) => {
                const isSelected = selectedTrace?.id === t.id;
                const barWidth = Math.max(
                  6,
                  Math.min(100, Math.round((t.latencyMs / maxLatencyMs) * 100)),
                );
                return (
                  <button
                    key={`${t.id}-${t.tracedAt}`}
                    type="button"
                    onClick={() => handleSelectRow(t)}
                    className={`w-full text-left p-2.5 rounded-lg border space-y-1.5 font-mono text-[11px] transition-colors ${
                      isSelected
                        ? "border-emerald-500/50 bg-emerald-500/10"
                        : "bg-surface-900/80 border-surface-800 hover:border-surface-700"
                    }`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-1.5 min-w-0">
                        <span
                          className={`px-1.5 py-0.5 rounded text-[10px] font-semibold shrink-0 ${
                            t.ok
                              ? "bg-emerald-500/15 text-emerald-300 border border-emerald-500/30"
                              : "bg-rose-500/15 text-rose-300 border border-rose-500/30"
                          }`}
                        >
                          {t.status || "ERR"}
                        </span>
                        <span className="text-white truncate">{t.url}</span>
                      </div>
                      <span className="text-surface-400 shrink-0">
                        {t.latencyMs}ms · {(t.bytes / 1024).toFixed(1)}KB
                      </span>
                    </div>

                    {/* Latency Waterfall Bar */}
                    <div className="h-1 w-full rounded-full bg-surface-950 overflow-hidden">
                      <div
                        className="h-full rounded-full bg-gradient-to-r from-brand-400 to-cyan-400"
                        style={{ width: `${barWidth}%` }}
                      />
                    </div>

                    <div className="flex flex-wrap items-center gap-1.5 text-[10px]">
                      <span className="px-1.5 py-0.5 rounded bg-brand-500/10 border border-brand-500/30 text-brand-300">
                        {t.classification.bucket}
                      </span>
                      <span className="text-surface-400 truncate">
                        {t.classification.detail}
                      </span>
                    </div>

                    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[10px] text-surface-500">
                      {t.classification.proxyMatched && (
                        <span className="text-cyan-300">proxy.ts</span>
                      )}
                      {t.protocol && <span>{t.protocol}</span>}
                      {t.poweredBy && <span>{t.poweredBy}</span>}
                      {t.cfRay && <span>cf-ray: {t.cfRay}</span>}
                      {t.cacheControl && (
                        <span className="truncate">cache: {t.cacheControl}</span>
                      )}
                    </div>
                  </button>
                );
              })
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
