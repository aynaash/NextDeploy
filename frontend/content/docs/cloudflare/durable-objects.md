---
title: Cloudflare Durable Objects
lede: Strongly consistent state, coordinated WebSockets, and embedded SQLite edge actors in Next.js applications.
status: ready
sourceRef: shared/config/types.go, shared/nextcompile/
related:
  - title: Workers Runtime
    href: /docs/cloudflare/workers
  - title: Hyperdrive & D1
    href: /docs/cloudflare/hyperdrive
---

## Overview

Durable Objects provide low-latency, globally coordinated, strongly consistent in-memory state and embedded SQLite storage. They are ideal for real-time collaboration (multiplayer canvases, collaborative text editors, live chat rooms, rate-limiters, and stateful AI agent sessions).

NextDeploy automates the declaration, compilation, and **class migrations** for Durable Objects.

---

## Configuration & Class Migrations

Declare Durable Objects and migration tags in `nextdeploy.yml`:

```yaml
target:
  type: serverless
  provider: cloudflare

serverless:
  cloudflare:
    resources:
      durable_objects:
        - name: CHAT_ROOM
          class_name: ChatRoom
          sqlite: true # Enable embedded SQLite storage engine

    # Durable Object class migrations must be tagged in chronological order
    migrations:
      - tag: v1
        new_classes: ["ChatRoom"]
```

---

## Implementing a Durable Object

Create your stateful actor in TypeScript:

```typescript
// lib/durable-objects/ChatRoom.ts
export class ChatRoom {
  state: DurableObjectState;
  sql: SqlStorage;

  constructor(state: DurableObjectState) {
    this.state = state;
    this.sql = state.storage.sql;
    // Create local embedded SQLite tables
    this.sql.exec(`
      CREATE TABLE IF NOT EXISTS messages (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        user TEXT,
        text TEXT,
        created_at DATETIME DEFAULT CURRENT_TIMESTAMP
      )
    `);
  }

  async fetch(request: Request) {
    const url = new URL(request.url);

    if (request.headers.get('Upgrade') === 'websocket') {
      const pair = new WebSocketPair();
      this.state.acceptWebSocket(pair[1]);
      return new Response(null, { status: 101, webSocket: pair[0] });
    }

    if (request.method === 'POST') {
      const { user, text } = await request.json();
      this.sql.exec('INSERT INTO messages (user, text) VALUES (?, ?)', user, text);
      return Response.json({ success: true });
    }

    const cursor = this.sql.exec('SELECT * FROM messages ORDER BY id DESC LIMIT 50');
    return Response.json(cursor.toArray());
  }
}
```

---

## Accessing Durable Objects from Next.js

Route user requests to the coordinated singleton instance via its unique ID or name:

```typescript
// app/api/chat/route.ts
import { NextRequest, NextResponse } from 'next/server';

export async function GET(req: NextRequest) {
  const namespace = process.env.CHAT_ROOM; // DurableObjectNamespace binding
  const id = namespace.idFromName('room-general');
  const stub = namespace.get(id);

  const res = await stub.fetch(req.url);
  const data = await res.json();
  return NextResponse.json(data);
}
```
