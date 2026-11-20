# Cumulative Static Chunk Pooling

One of the most frustrating errors in production single-page and server-rendered Next.js applications is:

```
ChunkLoadError: Loading chunk 842 failed.
```

NextDeploy eliminates this error permanently through **Cumulative Static Chunk Pooling**.

---

## Why `ChunkLoadError` Happens

When you run `next build`, Webpack or Turbopack creates content-hashed assets:

- Release 1: `/_next/static/chunks/app/profile-a1b2c3.js`
- Release 2: `/_next/static/chunks/app/profile-d4e5f6.js`

If User A loaded your app during Release 1, their browser has the old manifest in memory. When they navigate to `/profile` five minutes after Release 2 is deployed:

1. Browser requests `/_next/static/chunks/app/profile-a1b2c3.js`.
2. Traditional deployments wipe Release 1 container/directory.
3. Server returns `404 Not Found`.
4. Browser throws fatal `ChunkLoadError`, crashing user state.

---

## NextDeploy's Pool Architecture

Instead of isolating static assets inside each individual release directory, NextDeploy routes all static requests to a shared cumulative directory:

```
/var/www/apps/my-app/
├── releases/
│   ├── 2026-11-20_v1/
│   └── 2026-11-20_v2/
└── shared_static/
    ├── chunks/
    │   ├── app/profile-a1b2c3.js  (retained from v1)
    │   └── app/profile-d4e5f6.js  (added in v2)
    └── css/
        ├── 8f4a.css
        └── c91e.css
```

### 1. Cumulative Asset Merge
During each deployment, `nextdeploy ship` rsyncs all content-hashed static files into `/var/www/apps/my-app/shared_static/`. Existing files are never overwritten or deleted during the deploy.

### 2. Immediate NVMe Serving via Caddy
Caddy maps `/_next/static/*` directly to `shared_static`:

```nginx
handle /_next/static/* {
    root * /var/www/apps/my-app/shared_static
    file_server
    header Cache-Control "public, max-age=31536000, immutable"
}
```

Because these files are immutable and content-hashed, Caddy serves them directly from disk with maximum caching headers.

### 3. Safe Garbage Collection
To prevent unlimited disk growth, `nextdeployd` implements an LRU (Least Recently Used) cleanup policy that runs as a background task. Chunks not accessed for 14 days and older than 10 deployments are safely pruned without impacting active sessions.
