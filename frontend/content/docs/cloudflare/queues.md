---
title: Cloudflare Queues & Background Jobs
lede: Asynchronous message queuing, background worker jobs, and reliable event-driven pipelines on Cloudflare Workers.
status: ready
sourceRef: shared/config/types.go, shared/nextcompile/actions.go
related:
  - title: Workers Runtime
    href: /docs/cloudflare/workers
  - title: Resource Bindings
    href: /docs/cloudflare/bindings
---

## Overview

Web applications frequently require offloading heavy or non-critical operations (sending transactional emails, generating invoices, processing analytics events, running AI embeddings) out of the user-facing request path.

Cloudflare Queues provides reliable, at-least-once message delivery between Cloudflare Workers without managing Redis or Kafka clusters.

---

## Configuration in `nextdeploy.yml`

Declare your queues and consumer handlers:

```yaml
target:
  type: serverless
  provider: cloudflare

serverless:
  cloudflare:
    resources:
      queues:
        - name: EMAIL_QUEUE
          queue_name: transactional-emails
          max_retries: 5
          dead_letter_queue: email-dlq
          consumer:
            max_batch_size: 10
            max_batch_timeout: 5 # seconds
```

---

## Publishing Messages from Next.js Server Actions

In Next.js Server Components, Server Actions, or Route Handlers, publish messages directly to the queue:

```typescript
// app/actions/signup.ts
'use server';

export async function registerUser(formData: FormData) {
  const email = formData.get('email') as string;
  
  // Save user to database...

  // Offload email sending to Cloudflare Queue
  const queue = process.env.EMAIL_QUEUE; // Queue binding
  await queue.send({
    type: 'WELCOME_EMAIL',
    recipient: email,
    timestamp: Date.now(),
  });

  return { success: true };
}
```

---

## Processing Messages in the Worker Consumer

NextDeploy's `nextcompile` compiler exposes the `queue(batch, env, ctx)` handler:

```typescript
// worker.ts (or integrated via nextcompile)
export default {
  async queue(batch, env) {
    for (const message of batch.messages) {
      const { type, recipient } = message.body;
      if (type === 'WELCOME_EMAIL') {
        await sendEmail(recipient);
        message.ack(); // Acknowledge successful processing
      }
    }
  },
};
```
Messages that fail processing after `max_retries` are automatically routed to the declared Dead-Letter Queue (DLQ).
