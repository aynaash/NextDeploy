import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendDir = path.resolve(__dirname, "..");

function readJsonSafe(filePath) {
  try {
    if (!fs.existsSync(filePath)) return null;
    return JSON.parse(fs.readFileSync(filePath, "utf8"));
  } catch {
    return null;
  }
}

function readTextSafe(filePath) {
  try {
    if (!fs.existsSync(filePath)) return null;
    return fs.readFileSync(filePath, "utf8").trim();
  } catch {
    return null;
  }
}

function sanitizeFilePath(rawPath) {
  if (!rawPath || typeof rawPath !== "string") return null;
  const normalized = rawPath.replace(/\\/g, "/");
  const normalizedRoot = frontendDir.replace(/\\/g, "/");
  if (normalized.startsWith(normalizedRoot + "/")) {
    return normalized.slice(normalizedRoot.length + 1);
  }
  const idx = normalized.lastIndexOf("/frontend/");
  if (idx !== -1) {
    return normalized.slice(idx + "/frontend/".length);
  }
  return path.basename(normalized);
}

function getGitInfo() {
  try {
    const commit = execFileSync("git", ["rev-parse", "--short", "HEAD"], {
      cwd: frontendDir,
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
    const dirtyOut = execFileSync("git", ["status", "--porcelain"], {
      cwd: frontendDir,
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
    return { commit, dirty: dirtyOut.length > 0 };
  } catch {
    return { commit: "unknown", dirty: false };
  }
}

function parseNextDeployYml(ymlText) {
  const result = {
    app_name: "nextdeploy-frontend",
    domain: "nextdeploy.org",
    provider: "cloudflare",
    compatibility_date: "2025-04-01",
    bindings: ["ASSETS (R2)", "AI", "RATE_LIMIT (KV)"],
    protection: {
      enabled: true,
      public_paths: [],
      protected_paths: [],
      rate_limit_paths: [],
      requests_per_minute: 60,
    },
  };
  if (!ymlText) return result;

  const extractInlineArray = (key) => {
    const re = new RegExp(`${key}:\\s*\\[([^\\]]*)\\]`);
    const m = ymlText.match(re);
    if (!m) return [];
    return m[1]
      .split(",")
      .map((s) => s.trim().replace(/^["']|["']$/g, ""))
      .filter(Boolean);
  };

  const nameMatch = ymlText.match(/^\s*name:\s*["']?([^"'\s#]+)["']?/m);
  if (nameMatch) result.app_name = nameMatch[1];

  const domainMatch = ymlText.match(/^\s*domain:\s*["']?([^"'\s#]+)["']?/m);
  if (domainMatch) result.domain = domainMatch[1];

  const compatMatch = ymlText.match(/compatibility_date:\s*["']?([^"'\s#]+)["']?/);
  if (compatMatch) result.compatibility_date = compatMatch[1];

  const rpmMatch = ymlText.match(/requests_per_minute:\s*(\d+)/);
  if (rpmMatch) result.protection.requests_per_minute = Number(rpmMatch[1]);

  const pub = extractInlineArray("public_paths");
  if (pub.length > 0) result.protection.public_paths = pub;

  const prot = extractInlineArray("protected_paths");
  if (prot.length > 0) result.protection.protected_paths = prot;

  const rlSection = ymlText.split("rate_limit:")[1] || "";
  const rlPathsMatch = rlSection.match(/paths:\s*\[([^\]]*)\]/);
  if (rlPathsMatch) {
    result.protection.rate_limit_paths = rlPathsMatch[1]
      .split(",")
      .map((s) => s.trim().replace(/^["']|["']$/g, ""))
      .filter(Boolean);
  }

  return result;
}

function uniqueSorted(arr) {
  return Array.from(
    new Set(arr.filter((v) => typeof v === "string" && v.length > 0)),
  ).sort((a, b) => a.localeCompare(b));
}

function buildSnapshot() {
  const nextdeployMeta = readJsonSafe(
    path.join(frontendDir, ".nextdeploy", "metadata.json"),
  );
  const buildLock = readJsonSafe(
    path.join(frontendDir, ".nextdeploy", "build.lock"),
  );
  const nextBuildId = readTextSafe(path.join(frontendDir, ".next", "BUILD_ID"));
  const routesManifest = readJsonSafe(
    path.join(frontendDir, ".next", "routes-manifest.json"),
  );
  const prerenderManifest = readJsonSafe(
    path.join(frontendDir, ".next", "prerender-manifest.json"),
  );
  const ymlConfig = parseNextDeployYml(
    readTextSafe(path.join(frontendDir, "nextdeploy.yml")),
  );
  const git = getGitInfo();

  const staticFromManifest = (routesManifest?.staticRoutes ?? []).map(
    (r) => r.page,
  );
  const dynamicFromManifest = (routesManifest?.dynamicRoutes ?? []).map(
    (r) => r.page,
  );

  const staticRoutes = uniqueSorted([
    ...(nextdeployMeta?.route_info?.static_routes ?? []),
    ...staticFromManifest,
  ]);

  const dynamicRoutes = uniqueSorted([
    ...(nextdeployMeta?.route_info?.dynamic_routes ?? []),
    ...dynamicFromManifest,
  ]);

  const apiRoutes = uniqueSorted([
    ...(nextdeployMeta?.route_info?.api_routes ?? []),
    ...staticRoutes.filter((r) => r.startsWith("/api/")),
    ...dynamicRoutes.filter((r) => r.startsWith("/api/")),
  ]);

  const isrMap = new Map();
  for (const entry of nextdeployMeta?.route_info?.isr_detail ?? []) {
    if (entry?.path) {
      isrMap.set(entry.path, {
        path: entry.path,
        revalidate: entry.revalidate ?? null,
        tags: Array.isArray(entry.tags) ? entry.tags : [],
      });
    }
  }
  if (prerenderManifest?.routes) {
    for (const [routePath, info] of Object.entries(prerenderManifest.routes)) {
      const reval =
        typeof info?.initialRevalidateSeconds === "number"
          ? info.initialRevalidateSeconds
          : null;
      isrMap.set(routePath, {
        path: routePath,
        revalidate: reval,
        tags: isrMap.get(routePath)?.tags ?? [routePath],
      });
    }
  }
  const isrDetail = Array.from(isrMap.values()).sort((a, b) =>
    a.path.localeCompare(b.path),
  );

  const proxyObj = nextdeployMeta?.proxy ?? nextdeployMeta?.middleware ?? null;
  const hasProxyFile = fs.existsSync(path.join(frontendDir, "proxy.ts"));

  const detected = nextdeployMeta?.detected_features ?? {};
  const publicDirEntries = (nextdeployMeta?.static_assets?.public_dir ?? []).map(
    (entry) => (typeof entry === "string" ? entry : entry?.public_path),
  );
  const publicAssets = uniqueSorted([
    ...publicDirEntries,
    "/nextdeploy-metadata.json",
  ]);

  const nowIso = new Date().toISOString();

  return {
    schema_version: "1.0",
    source: nextdeployMeta
      ? ".nextdeploy/metadata.json + .next manifests"
      : ".next manifests",
    synced_at: nowIso,
    generated_at: nextdeployMeta?.generated_at ?? nowIso,
    app_name: nextdeployMeta?.app_name ?? ymlConfig.app_name,
    domain: nextdeployMeta?.domain ?? ymlConfig.domain,
    output_mode: nextdeployMeta?.output_mode ?? "standalone",
    package_manager: nextdeployMeta?.package_manager ?? "bun",
    dist_dir: nextdeployMeta?.dist_dir ?? ".next",
    export_dir: nextdeployMeta?.export_dir ?? "out",
    build_id:
      nextBuildId ?? nextdeployMeta?.nextbuildmetadata?.buildId ?? "unknown",
    has_app_router: nextdeployMeta?.nextbuildmetadata?.hasAppRouter ?? true,
    git_commit: nextdeployMeta?.git_commit ?? git.commit,
    git_dirty:
      typeof nextdeployMeta?.git_dirty === "boolean"
        ? nextdeployMeta.git_dirty
        : git.dirty,
    config: nextdeployMeta?.config ?? {
      app_name: ymlConfig.app_name,
      domain: ymlConfig.domain,
      port: 0,
      environment: "",
      target_type: "serverless",
    },
    build_lock: buildLock
      ? {
          git_commit: buildLock.git_commit ?? git.commit,
          git_dirty: Boolean(buildLock.git_dirty),
          generated_at: buildLock.generated_at ?? nowIso,
          metadata_file: buildLock.metadata_file ?? ".nextdeploy/metadata.json",
          config_hash: buildLock.config_hash ?? null,
        }
      : null,
    proxy: proxyObj
      ? {
          file: sanitizeFilePath(proxyObj.path) ?? "proxy.ts",
          runtime: proxyObj.runtime ?? "nodejs",
          matchers: (proxyObj.matchers ?? []).map((m) => m.pathname),
        }
      : hasProxyFile
        ? {
            file: "proxy.ts",
            runtime: "nodejs",
            matchers: ["/((?!_next/static|_next/image|favicon.ico).*)"],
          }
        : null,
    route_info: {
      counts: {
        static_routes: staticRoutes.length,
        dynamic_routes: dynamicRoutes.length,
        isr_routes: isrDetail.length,
        api_routes: apiRoutes.length,
        public_assets: publicAssets.length,
      },
      static_routes: staticRoutes,
      dynamic_routes: dynamicRoutes,
      api_routes: apiRoutes,
      isr_detail: isrDetail,
    },
    public_assets: publicAssets,
    detected_features: {
      HasYouTube: Boolean(detected.HasYouTube),
      HasGoogleFonts: Boolean(detected.HasGoogleFonts),
      HasGoogleAnalytics: Boolean(detected.HasGoogleAnalytics),
      HasStripe: Boolean(detected.HasStripe),
      HasCloudinary: Boolean(detected.HasCloudinary),
      HasServerActions: Boolean(detected.HasServerActions),
      HasI18n: Boolean(detected.HasI18n),
      UserDefinedCSP: Boolean(detected.UserDefinedCSP),
      AllowedOrigins: Array.isArray(detected.AllowedOrigins)
        ? detected.AllowedOrigins
        : [],
    },
    edge_protection: {
      provider: ymlConfig.provider,
      compatibility_date: ymlConfig.compatibility_date,
      bindings: ymlConfig.bindings,
      ...ymlConfig.protection,
    },
  };
}

const snapshot = buildSnapshot();
const formatted = JSON.stringify(snapshot, null, 2) + "\n";

const libOut = path.join(frontendDir, "lib", "build-metadata.json");
const publicOut = path.join(frontendDir, "public", "nextdeploy-metadata.json");

fs.mkdirSync(path.dirname(libOut), { recursive: true });
fs.mkdirSync(path.dirname(publicOut), { recursive: true });
fs.writeFileSync(libOut, formatted, "utf8");
fs.writeFileSync(publicOut, formatted, "utf8");

console.log(
  `Synced metadata snapshot (build_id=${snapshot.build_id}, static=${snapshot.route_info.counts.static_routes}, dynamic=${snapshot.route_info.counts.dynamic_routes}, isr=${snapshot.route_info.counts.isr_routes}, api=${snapshot.route_info.counts.api_routes})`,
);
