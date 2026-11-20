# `nextdeploy inspect`

Introspect the Next.js compilation artifact to examine route classifications, dynamic dependencies, ISR revalidation timers, and Server Actions.

---

## Usage

```bash
nextdeploy inspect [flags]
```

### Flags

| Flag | Type | Description |
| :--- | :--- | :--- |
| `--routes` | boolean | Print detailed route classification table. |
| `--actions` | boolean | List detected Server Actions and their unique IDs. |
| `--json` | boolean | Output analysis in structured JSON for automation. |

---

## Output Example

```text
NextCore Introspection Report:
────────────────────────────────────────────────────
Next.js Version: 15.1.0 (App Router)
Output Mode:     standalone

Route Analysis:
  ┌──────────────────────┬─────────┬──────────────┬──────────────┐
  │ Route Path           │ Type    │ Revalidation │ Runtime      │
  ├──────────────────────┼─────────┼──────────────┼──────────────┤
  │ /                    │ SSG (●) │ 60s (ISR)    │ Node.js      │
  │ /about               │ Static○ │ -            │ Prerendered  │
  │ /dashboard           │ Dynamicƒ│ -            │ Node.js      │
  │ /api/health          │ API (λ) │ -            │ Node.js      │
  │ /api/webhooks/stripe │ API (λ) │ -            │ Node.js      │
  └──────────────────────┴─────────┴──────────────┴──────────────┘

Detected Server Actions: 3
  * $ACTION_ID_8f3a92b (app/actions.ts: createPost)
  * $ACTION_ID_99e1201 (app/actions.ts: deletePost)
  * $ACTION_ID_b0337c4 (app/auth/actions.ts: logout)
```
