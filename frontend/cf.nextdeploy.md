# NextDeploy Dogfooding Specification: Shipping `masomolab-web` on Cloudflare Workers

> **Target Project:** `masomolab-web`  
> **Source Configuration:** `wrangler.json` (3-tier environment: Dev, Staging, Production)  
> **Core Stack:** Next.js App Router, Durable Objects (SQLite), Cloudflare Workflows, Queues with DLQ, Vectorize, Workers AI, R2 Storage, KV Rate Limiting, Cron Triggers, Custom Domains.

---

## 1. Executive Summary & Architecture Overview

`masomolab-web` is a modern, enterprise-grade edge application running on Cloudflare Workers. It uses:
1. **Next.js App Router** for the web frontend and API routes.
2. **Durable Objects (`CourseAuthoringDO`)** with SQLite storage for stateful real-time course authoring.
3. **Cloudflare Workflows (`IngestWorkflow`)** for multi-step durable background job execution.
4. **Cloudflare Queues (`INGEST_Q`)** with consumer batches and a dead-letter queue (`DLQ`).
5. **Workers AI + Vectorize (`VECTORIZE`)** for RAG search.
6. **R2 (`MEDIA_BUCKET`)** for media storage and **KV (`RATE_LIMITS_KV`)** for rate limiting.

### The Dogfooding Goal
Eliminate the need for separate OpenNext build hacks and manual `wrangler` scripting by enabling NextDeploy to compile, bind, migrate, and ship `masomolab-web` atomically.

---

## 2. Gap Analysis: `wrangler.json` vs. Current NextDeploy

| Feature | `wrangler.json` (Masomolab) | NextDeploy Current State | Required Delta in NextDeploy |
| :--- | :--- | :--- | :--- |
| **Cloudflare Workflows** | `"workflows": [{ "name": "...", "binding": "INGEST_WORKFLOW", "class_name": "IngestWorkflow" }]` | ❌ Not supported in schema or bindings | Add `CFWorkflowBinding` to `shared/config/types.go` and `cloudflare_bindings.go` |
| **Worker Class Exports** | `CourseAuthoringDO`, `IngestWorkflow` exported from `cf-worker/index.ts` | ❌ Bundles standalone Next.js entrypoint only | Add `extensions: []string` to `nextdeploy.yml` and inject `export * from '...'` in `esbuild` |
| **Queue DLQ** | `"dead_letter_queue": "masomolab-processing-dlq"` | ⚠️ Consumer exists, lacks DLQ field | Add `DeadLetterQueue` to `CFQueueConsumer` |
| **Durable Objects (SQLite)** | `"migrations": [{ "tag": "v1", "new_sqlite_classes": [...] }]` | ✅ Fully supported in `buildMigrations` | No code change needed; wire in YAML |
| **Vectorize & AI** | `"vectorize": [...], "ai": { ... }` | ✅ Fully supported | Wire in YAML |
| **R2 & KV** | `"r2_buckets": [...], "kv_namespaces": [...]` | ✅ Fully supported | Wire in YAML |
| **Cron Triggers** | `"triggers": { "crons": ["0 2 * * *"] }` | ✅ Fully supported | Wire in YAML |
| **Custom Domains** | `"routes": [{ "pattern": "masomolab.com", "custom_domain": true }]` | ✅ Fully supported with apex detection | Wire in YAML |

---

## 3. Required Codebase Changes in NextDeploy

### Step 3.1: Add Workflows & DLQ to Configuration Types
**File:** `shared/config/types.go`

```go
// In CFBindings struct (line ~218):
type CFBindings struct {
	R2             []CFR2Binding         `yaml:"r2,omitempty"`
	D1             []CFD1Binding         `yaml:"d1,omitempty"`
	Hyperdrive     []CFHyperdriveBinding `yaml:"hyperdrive,omitempty"`
	Queues         *CFQueueBindings      `yaml:"queues,omitempty"`
	Vectorize      []CFVectorizeBinding  `yaml:"vectorize,omitempty"`
	AI             []CFAIBinding         `yaml:"ai,omitempty"`
	DurableObjects []CFDOBinding         `yaml:"durable_objects,omitempty"`
	Workflows      []CFWorkflowBinding   `yaml:"workflows,omitempty"` // <--- NEW
	KV             []CFKVBinding         `yaml:"kv,omitempty"`
	PlainText      []CFPlainTextBinding  `yaml:"plain_text,omitempty"`
	SecretsStore   []CFSecretStoreBinding `yaml:"secrets_store,omitempty"`
}

// Add CFWorkflowBinding:
type CFWorkflowBinding struct {
	Name      string `yaml:"name"`       // Cloudflare workflow name (e.g. "ingest-workflow")
	Binding   string `yaml:"binding"`    // JS env variable (e.g. "INGEST_WORKFLOW")
	ClassName string `yaml:"class_name"` // Exported class name (e.g. "IngestWorkflow")
}

// In CFQueueConsumer struct:
type CFQueueConsumer struct {
	Queue           string `yaml:"queue"`
	MaxBatchSize    int    `yaml:"max_batch_size,omitempty"`
	MaxBatchTimeout int    `yaml:"max_batch_timeout,omitempty"`
	MaxRetries      int    `yaml:"max_retries,omitempty"`
	DeadLetterQueue string `yaml:"dead_letter_queue,omitempty"` // <--- NEW
}

// In CFBlock struct (line ~85):
type CFBlock struct {
	CompatibilityDate  string        `yaml:"compatibility_date,omitempty"`
	CompatibilityFlags []string      `yaml:"compatibility_flags,omitempty"`
	Extensions         []string      `yaml:"extensions,omitempty"` // <--- NEW: Companion TS/JS files to re-export
	Bindings           CFBindings    `yaml:"bindings,omitempty"`
	Migrations         []CFMigration `yaml:"migrations,omitempty"`
	Triggers           *CFTriggers   `yaml:"triggers,omitempty"`
	// ...
}
```

---

### Step 3.2: Wire Cloudflare Workflows & DLQ in Upload Metadata
**File:** `cli/internal/serverless/cloudflare_bindings.go`

```go
// Inside buildBindings():
for _, wf := range b.Workflows {
	// Emit the Cloudflare Workflow binding format
	out = append(out, map[string]interface{}{
		"type":          "workflow",
		"name":          wf.Binding,
		"workflow_name": wf.Name,
		"class_name":    wf.ClassName,
	})
}

// Inside Queue Consumer configuration:
for _, c := range b.Queues.Consumers {
	consumerParam := workers.ScriptUpdateParamsMetadataBindingsWorkersBindingKindQueueConsumer{
		QueueName: cloudflare.F(c.Queue),
		Type:      cloudflare.F(workers.ScriptUpdateParamsMetadataBindingsWorkersBindingKindQueueConsumerTypeQueueConsumer),
	}
	if c.DeadLetterQueue != "" {
		consumerParam.DeadLetterQueue = cloudflare.F(c.DeadLetterQueue)
	}
	// ...
}
```

---

### Step 3.3: Re-export Worker Classes in `worker.mjs` Bundle
**File:** `cli/internal/serverless/cloudflare_adapter.go`

In `runEsbuildNextServer()` and `runEsbuild()`, inject the extension imports into the entrypoint shim:

```typescript
// Dynamically generated into entry shim before esbuild bundles:
import { CourseAuthoringDO } from "./apps/web/cf-worker/index.ts";
import { IngestWorkflow } from "./apps/web/cf-worker/index.ts";

export { CourseAuthoringDO, IngestWorkflow };
```

This ensures that when Cloudflare's runtime inspects `worker.mjs`, the classes named in `durable_objects` and `workflows` exist at the top-level module scope.

---

## 4. Production `nextdeploy.yml` for `masomolab-web`

Place this file at `apps/web/nextdeploy.yml` (or repo root):

```yaml
app:
  name: masomolab-web
  domain: masomolab.com

serverless:
  provider: cloudflare
  cloudflare:
    compatibility_date: "2026-05-01"
    compatibility_flags:
      - nodejs_compat

    # Re-export DO and Workflow classes directly into the Worker bundle
    extensions:
      - "apps/web/cf-worker/index.ts"

    bindings:
      ai:
        - name: AI

      vectorize:
        - name: VECTORIZE
          index_name: masomolab-rag

      r2:
        - name: MEDIA_BUCKET
          bucket: masomolab-media

      kv:
        - name: RATE_LIMITS_KV
          id: "7b7680994579467191ed43aa86cb2d5a"

      durable_objects:
        - name: COURSE_AUTHORING
          class_name: CourseAuthoringDO

      workflows:
        - name: ingest-workflow
          binding: INGEST_WORKFLOW
          class_name: IngestWorkflow

      queues:
        producers:
          - name: INGEST_Q
            queue: masomolab-processing
        consumers:
          - queue: masomolab-processing
            max_batch_size: 10
            max_batch_timeout: 30
            max_retries: 3
            dead_letter_queue: masomolab-processing-dlq

    migrations:
      - tag: v1
        new_sqlite_classes:
          - CourseAuthoringDO

    triggers:
      crons:
        - "0 2 * * *"

    observability:
      enabled: true
      head_sampling_rate: 1.0
```

---

## 5. Multi-Environment Doppler Configuration

`masomolab-web` uses separate Cloudflare accounts for Non-Prod and Production. Map them cleanly using Doppler environments:

| Doppler Config | Target Worker Name | Custom Domain | Cloudflare Account |
| :--- | :--- | :--- | :--- |
| `dev` | `masomolab-web-dev` | `workers.dev` | Non-prod account |
| `stg` | `masomolab-web-staging` | `masomolab-web-staging.*.workers.dev` | Non-prod account |
| `prd` | `masomolab-web` | `masomolab.com`, `www.masomolab.com` | Production account |

### Deployment Command:
```bash
# Deploy to Production:
doppler run -c prd -- nextdeploy ship

# Deploy to Staging:
doppler run -c stg -- nextdeploy ship --env staging
```

---

## 6. Verification & Rollback Checklist

1. **Pre-flight Planning Check:**
   ```bash
   doppler run -c prd -- nextdeploy plan --target cloudflare
   ```
   *Verify that NextDeploy reconciles all 7 bindings (AI, Vectorize, R2, KV, DO, Workflow, Queue) without diff errors.*

2. **Deploy & Smoke Test:**
   ```bash
   doppler run -c prd -- nextdeploy ship
   ```
   *Verifies 200 OK on `https://masomolab.com`, cache purge success, and custom domain route persistence.*

3. **Instant Rollback (if required):**
   ```bash
   doppler run -c prd -- nextdeploy rollback --steps 1
   ```
   *Rolls back both the Worker script and the R2 asset namespace atomically.*
