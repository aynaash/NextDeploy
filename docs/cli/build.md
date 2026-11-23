# `nextdeploy build`

Execute the Next.js production build and run NextCore route analysis and manifest generation.

---

## Usage

```bash
nextdeploy build [flags]
```

### Flags

| Flag | Shorthand | Type | Description |
| :--- | :--- | :--- | :--- |
| `--force` | `-f` | boolean | Force a full rebuild even if the git commit hash is unchanged. |

---

## Phases

1. **Next.js Standalone Build**: Executes `next build` inside the project to generate client chunks, standalone server artifacts, and page manifests.
2. **NextCore Introspection**: Inspects `.next/server/pages-manifest.json`, `app-paths-manifest.json`, and middleware manifests.
3. **Route Classification**: Classifies every route as Static (`○`), SSG (`●`), Dynamic SSR (`ƒ`), or API (`λ`).
4. **Metadata Generation**: Writes `.nextdeploy/metadata.json` containing the payload required for `nextdeploy ship`.
