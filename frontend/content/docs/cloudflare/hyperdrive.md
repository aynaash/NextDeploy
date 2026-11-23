---
title: Cloudflare Hyperdrive & D1 SQL
lede: Ultra-low latency database connectivity from edge Workers to PostgreSQL and Cloudflare D1 distributed SQL.
status: ready
sourceRef: shared/config/types.go, shared/nextcompile/
related:
  - title: Cloudflare Workers
    href: /docs/cloudflare/workers
  - title: Resource Bindings
    href: /docs/cloudflare/bindings
---

## The Edge Database Dilemma

Connecting to centralized relational databases (PostgreSQL, MySQL, Neon, Supabase) from edge compute runtimes normally incurs severe latency penalties due to repeated TCP handshakes and TLS negotiations across long geographical distances.

NextDeploy natively orchestrates two edge database solutions:
1. **Cloudflare Hyperdrive**: Globally distributed connection pooling and query acceleration for existing Postgres/MySQL databases.
2. **Cloudflare D1**: Serverless, SQLite-compatible distributed SQL database running natively inside Cloudflare's edge network.

---

## 1. Cloudflare Hyperdrive (Postgres Acceleration)

Hyperdrive pools database connections across Cloudflare's global edge network, maintaining pre-warmed TCP and TLS sessions directly to your origin database.

### Configuration in `nextdeploy.yml`
```yaml
target:
  type: serverless
  provider: cloudflare

serverless:
  cloudflare:
    resources:
      hyperdrive:
        - name: PROD_DB
          origin:
            host: ep-cool-database.us-east-1.neon.tech
            database: neondb
            user: app_user
            password_secret: NEON_DB_PASSWORD
            scheme: postgres
          caching:
            max_age: 60
            stale_while_revalidate: 15
```

### Usage in Next.js Server Components
NextDeploy automatically binds the Hyperdrive connection string to `process.env.PROD_DB_URL`:

```typescript
// app/users/page.tsx
import { Pool } from 'pg';

const pool = new Pool({
  connectionString: process.env.PROD_DB_URL,
});

export default async function UsersPage() {
  const { rows } = await pool.query('SELECT id, name FROM users LIMIT 10');
  return (
    <ul>
      {rows.map((user) => (
        <li key={user.id}>{user.name}</li>
      ))}
    </ul>
  );
}
```

---

## 2. Cloudflare D1 (Native Distributed SQL)

For applications that need zero-latency edge relational storage:

```yaml
serverless:
  cloudflare:
    resources:
      d1:
        - name: DB
          database_name: my-app-db
```

Query D1 directly using the injected binding or standard ORMs (such as Drizzle ORM):

```typescript
// app/api/todos/route.ts
import { NextResponse } from 'next/server';

export async function GET() {
  const db = process.env.DB; // D1Database binding
  const { results } = await db.prepare('SELECT * FROM todos').all();
  return NextResponse.json(results);
}
```
