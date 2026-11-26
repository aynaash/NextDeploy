export type RouteMode =
  | "SSG"
  | "ISR"
  | "SSR"
  | "STREAM"
  | "ACTION"
  | "API"
  | "EDGE"
  | "META";

export type RouteCategory =
  | "Rendering Modes"
  | "Dynamic Segments"
  | "App Router Architecture"
  | "Control Flow & Boundaries"
  | "Server Actions & API Routes"
  | "Edge Proxy, Protection & Metadata";

export interface RouteSpec {
  id: string;
  category: RouteCategory;
  title: string;
  pattern: string;
  href: string;
  probeUrl: string;
  probeMethod: "GET" | "POST" | "HEAD";
  probeBody?: string;
  probeRedirect?: "follow" | "manual";
  expectedStatus: number[];
  mode: RouteMode;
  description: string;
  cfNotes: string;
}

export const ROUTE_CATEGORIES: RouteCategory[] = [
  "Rendering Modes",
  "Dynamic Segments",
  "App Router Architecture",
  "Control Flow & Boundaries",
  "Server Actions & API Routes",
  "Edge Proxy, Protection & Metadata",
];

export const ROUTES_CATALOG: RouteSpec[] = [
  // Rendering Modes
  {
    id: "static",
    category: "Rendering Modes",
    title: "Static Prerender (SSG)",
    pattern: "/routes/static",
    href: "/routes/static",
    probeUrl: "/routes/static",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "SSG",
    description:
      "Forced static page rendered once at build time via dynamic = \"force-static\".",
    cfNotes: "Served directly from R2 static asset storage without waking Node/Worker SSR.",
  },
  {
    id: "isr",
    category: "Rendering Modes",
    title: "Incremental Static Regeneration (ISR)",
    pattern: "/routes/isr",
    href: "/routes/isr",
    probeUrl: "/routes/isr",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "ISR",
    description:
      "Revalidates every 30 seconds and supports on-demand revalidatePath('/routes/isr').",
    cfNotes: "Tracks R2 revalidation timestamps and refreshes stale HTML via ctx.waitUntil.",
  },
  {
    id: "ssr",
    category: "Rendering Modes",
    title: "Dynamic Server-Side Rendering (SSR)",
    pattern: "/routes/ssr",
    href: "/routes/ssr?source=lab",
    probeUrl: "/routes/ssr?source=probe",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "SSR",
    description:
      "Per-request rendering awaiting searchParams, headers(), cookies(), and Cloudflare env presence.",
    cfNotes: "Executes NextNodeServer inside the Worker with live proxy.ts headers and CF env.",
  },
  {
    id: "streaming",
    category: "Rendering Modes",
    title: "Progressive RSC Streaming & Loading",
    pattern: "/routes/streaming",
    href: "/routes/streaming",
    probeUrl: "/routes/streaming",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "STREAM",
    description:
      "Static shell with three staggered <Suspense> Server Components (120ms, 320ms, 550ms) and loading.tsx.",
    cfNotes: "Streams progressive RSC Flight chunks to HTML via vendored react-server-dom-webpack.",
  },

  // Dynamic Segments
  {
    id: "dynamic-slug-ssg",
    category: "Dynamic Segments",
    title: "Dynamic Segment (Pre-rendered)",
    pattern: "/routes/dynamic/[slug]",
    href: "/routes/dynamic/cloudflare",
    probeUrl: "/routes/dynamic/cloudflare",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "SSG",
    description:
      "Pre-rendered slug from generateStaticParams with per-slug generateMetadata.",
    cfNotes: "Matches pre-built R2 HTML for known params while keeping dynamicParams = true.",
  },
  {
    id: "dynamic-slug-ssr",
    category: "Dynamic Segments",
    title: "Dynamic Segment (On-Demand SSR)",
    pattern: "/routes/dynamic/[slug]",
    href: "/routes/dynamic/edge-worker-live",
    probeUrl: "/routes/dynamic/edge-worker-live",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "SSR",
    description:
      "Unlisted slug rendered on demand at request time because dynamicParams = true.",
    cfNotes: "Falls back cleanly to Worker SSR when a slug is not in R2.",
  },
  {
    id: "nested-dynamic",
    category: "Dynamic Segments",
    title: "Nested Dynamic Segments",
    pattern: "/routes/nested/[category]/[id]",
    href: "/routes/nested/workers/42?env=edge",
    probeUrl: "/routes/nested/workers/42?env=edge",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "SSR",
    description:
      "Multi-parameter route segment resolution with async params and searchParams.",
    cfNotes: "Tests multi-level dynamic trie dispatch inside the Worker.",
  },
  {
    id: "catch-all",
    category: "Dynamic Segments",
    title: "Catch-All Segment",
    pattern: "/routes/catch-all/[...segments]",
    href: "/routes/catch-all/edge/r2/assets/v1",
    probeUrl: "/routes/catch-all/edge/r2/assets/v1",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "SSR",
    description:
      "Captures arbitrary path depth as a string[] array with live breadcrumb navigation.",
    cfNotes: "Verifies wildcard catch-all matching across arbitrary segment counts.",
  },
  {
    id: "optional-catch-all-root",
    category: "Dynamic Segments",
    title: "Optional Catch-All (Root & Nested)",
    pattern: "/routes/optional-catch-all/[[...slug]]",
    href: "/routes/optional-catch-all",
    probeUrl: "/routes/optional-catch-all",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "SSR",
    description:
      "Matches both the bare base path (slug === undefined) and nested segment paths.",
    cfNotes: "Exercises optional catch-all regex/trie handling with zero and N segments.",
  },

  // App Router Architecture
  {
    id: "grouped",
    category: "App Router Architecture",
    title: "Route Group Layout",
    pattern: "/routes/(grouped)/grouped",
    href: "/routes/grouped",
    probeUrl: "/routes/grouped",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "SSG",
    description:
      "Applies a scoped layout from (grouped)/layout.tsx without adding (grouped) to the URL.",
    cfNotes: "Confirms parenthesized route groups strip cleanly from Worker route tables.",
  },
  {
    id: "parallel",
    category: "App Router Architecture",
    title: "Parallel Routes (@metrics + @logs)",
    pattern: "/routes/parallel (@metrics, @logs)",
    href: "/routes/parallel",
    probeUrl: "/routes/parallel",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "SSR",
    description:
      "Renders main children alongside two independent named slots with default.tsx fallbacks.",
    cfNotes: "Exercises multi-slot RSC payload composition in NextNodeServer on Workers.",
  },
  {
    id: "gallery-intercept",
    category: "App Router Architecture",
    title: "Intercepting Route + Modal Slot",
    pattern: "/routes/gallery -> (.)item/[id]",
    href: "/routes/gallery",
    probeUrl: "/routes/gallery/item/worker-01",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "SSR",
    description:
      "Soft navigation opens item/[id] in an @modal overlay; direct visits render the full page.",
    cfNotes: "Verifies both soft RSC interception and hard full-page fallback on Cloudflare.",
  },

  // Control Flow & Boundaries
  {
    id: "not-found",
    category: "Control Flow & Boundaries",
    title: "Scoped notFound() Boundary (404)",
    pattern: "/routes/not-found-demo",
    href: "/routes/not-found-demo",
    probeUrl: "/routes/not-found-demo?trigger=1",
    probeMethod: "GET",
    expectedStatus: [404],
    mode: "SSR",
    description:
      "Invokes notFound() when ?trigger=1 is passed, rendering scoped not-found.tsx with HTTP 404.",
    cfNotes: "Ensures Worker preserves HTTP 404 status code alongside custom RSC boundary UI.",
  },
  {
    id: "error-boundary",
    category: "Control Flow & Boundaries",
    title: "Error Boundary & reset() Recovery",
    pattern: "/routes/error-demo",
    href: "/routes/error-demo",
    probeUrl: "/routes/error-demo",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "SSR",
    description:
      "Client-triggered render throw caught by scoped error.tsx with interactive reset() recovery.",
    cfNotes: "Verifies error boundary hydration and recovery without breaking build prerender.",
  },
  {
    id: "redirect-page",
    category: "Control Flow & Boundaries",
    title: "Server redirect() (307)",
    pattern: "/routes/redirect-demo",
    href: "/routes/redirect-demo",
    probeUrl: "/routes/redirect-demo",
    probeMethod: "GET",
    probeRedirect: "manual",
    expectedStatus: [307, 303, 302, 200],
    mode: "SSR",
    description:
      "Server component calls redirect('/routes/ssr?from=redirect-demo') before rendering.",
    cfNotes: "Confirms Next.js redirect() translates to an edge 307 Location redirect.",
  },

  // Server Actions & API Routes
  {
    id: "server-actions",
    category: "Server Actions & API Routes",
    title: "Server Actions (Web Crypto + Cookies + Revalidate)",
    pattern: "/routes/actions",
    href: "/routes/actions",
    probeUrl: "/routes/actions",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "ACTION",
    description:
      "Three 'use server' actions using crypto.subtle SHA-256, cookies().set(), and revalidatePath().",
    cfNotes: "Exercises Flight reply encoding, bound action dispatch, and Set-Cookie headers.",
  },
  {
    id: "api-methods",
    category: "Server Actions & API Routes",
    title: "API Route Handler (All HTTP Verbs)",
    pattern: "/api/routes/methods",
    href: "/api/routes/methods?probe=1",
    probeUrl: "/api/routes/methods?probe=1",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "API",
    description:
      "Supports GET, POST, PUT, PATCH, DELETE, HEAD, and OPTIONS with JSON body & query echo.",
    cfNotes: "Dispatched through Worker Route Handler bridge under /api/* rate limiting.",
  },
  {
    id: "api-dynamic",
    category: "Server Actions & API Routes",
    title: "Dynamic API Route + CF Bindings Probe",
    pattern: "/api/routes/items/[id]",
    href: "/api/routes/items/cf-edge-42?check=bindings",
    probeUrl: "/api/routes/items/cf-edge-42?check=bindings",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "API",
    description:
      "Dynamic [id] API route inspecting safe boolean availability of ASSETS, RATE_LIMIT, and AI.",
    cfNotes: "Reads globalThis.__nextdeployEnv safely without leaking any secret values.",
  },
  {
    id: "api-stream",
    category: "Server Actions & API Routes",
    title: "Streaming SSE Route Handler",
    pattern: "/api/routes/stream",
    href: "/api/routes/stream",
    probeUrl: "/api/routes/stream",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "STREAM",
    description:
      "ReadableStream Server-Sent Events endpoint emitting 4 timed chunks over text/event-stream.",
    cfNotes: "Streams Web ReadableStream chunks directly through the Worker without buffering.",
  },
  {
    id: "api-revalidate",
    category: "Server Actions & API Routes",
    title: "On-Demand ISR Revalidate API",
    pattern: "/api/routes/revalidate",
    href: "/routes/isr",
    probeUrl: "/api/routes/revalidate",
    probeMethod: "POST",
    expectedStatus: [200],
    mode: "API",
    description:
      "POST handler calling revalidatePath('/routes/isr') and returning JSON confirmation.",
    cfNotes: "Clears stale R2 ISR markers and emits x-nextcompile-revalidated-paths.",
  },

  // Edge Proxy, Protection & Metadata
  {
    id: "edge-health",
    category: "Edge Proxy, Protection & Metadata",
    title: "Edge Health & proxy.ts Header Injection",
    pattern: "/api/health",
    href: "/api/health",
    probeUrl: "/api/health",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "EDGE",
    description:
      "Existing health endpoint verifying proxy.ts x-powered-by injection and edge JSON.",
    cfNotes: "Runs proxy.ts matcher ahead of route dispatch and passes /api/* rate-limit guard.",
  },
  {
    id: "edge-protected-app",
    category: "Edge Proxy, Protection & Metadata",
    title: "Edge Auth Guarded Route (/app)",
    pattern: "/app",
    href: "/app",
    probeUrl: "/app",
    probeMethod: "GET",
    probeRedirect: "manual",
    expectedStatus: [302, 307, 401, 200],
    mode: "EDGE",
    description:
      "Protected path in nextdeploy.yml: Cloudflare edge guard redirects unauthenticated requests to /login.",
    cfNotes: "Enforced at the Worker edge before Next.js compute runs; locally renders Dashboard.",
  },
  {
    id: "meta-sitemap",
    category: "Edge Proxy, Protection & Metadata",
    title: "XML Sitemap & Robots Metadata Routes",
    pattern: "/sitemap.xml",
    href: "/sitemap.xml",
    probeUrl: "/sitemap.xml",
    probeMethod: "GET",
    expectedStatus: [200],
    mode: "META",
    description:
      "Static/revalidated XML sitemap route handler alongside /robots.txt and /manifest.webmanifest.",
    cfNotes: "Served with application/xml content-type and edge cache headers.",
  },
];
