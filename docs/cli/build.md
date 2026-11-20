# `nextdeploy build`

Execute the Next.js production build and run NextCore route analysis and manifest generation.

---

## Usage

```bash
nextdeploy build [flags]
```

### Flags

| Flag | Type | Description |
| :--- | :--- | :--- |
| `--skip-next-build` | boolean | Skip running `npm run build` and only perform NextCore introspection on existing `.next` build output. |
| `--output-dir` | string | Custom destination for NextDeploy build artifacts (default: `.nextdeploy`). |

---

## Phases

1. **Next.js Standalone Build**: Executes `next build` inside the project to generate client chunks, standalone server artifacts, and page manifests.
2. **NextCore Introspection**: Inspects `.next/server/pages-manifest.json`, `app-paths-manifest.json`, and middleware manifests.
3. **Route Classification**: Classifies every route as Static (`○`), SSG (`●`), Dynamic SSR (`ƒ`), or API (`λ`).
4. **Metadata Generation**: Writes `.nextdeploy/metadata.json` containing the payload required for `nextdeploy ship`.
