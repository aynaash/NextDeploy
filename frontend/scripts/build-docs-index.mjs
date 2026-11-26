// Pre-compiles content/docs/**/*.md into lib/docs-content.generated.json.
//
// Why: on Cloudflare Workers there is no project filesystem, and `gray-matter`
// does `require("fs")` at module load. Reading docs at request time crashed the
// docs route module during evaluation (Context.externalRequire). Bundling the
// parsed docs as JSON keeps the Worker free of fs and frontmatter parsing.
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import matter from "gray-matter";
import * as yaml from "js-yaml";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const frontendDir = path.resolve(__dirname, "..");
const contentRoot = path.join(frontendDir, "content", "docs");
const outFile = path.join(frontendDir, "lib", "docs-content.generated.json");

const docs = {};

function walk(dir, prefix) {
  if (!fs.existsSync(dir)) return;
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      walk(full, [...prefix, entry.name]);
      continue;
    }
    if (!entry.isFile() || !entry.name.endsWith(".md")) continue;

    const slug =
      entry.name === "index.md" ? prefix : [...prefix, entry.name.replace(/\.md$/, "")];
    const key = slug.join("/");
    const isIndex = entry.name === "index.md";
    // Same precedence as the old loader: `<slug>.md` wins over `<slug>/index.md`.
    if (docs[key] && isIndex) continue;

    const parsed = matter(fs.readFileSync(full, "utf8"), {
      engines: { yaml: (input) => yaml.load(input) ?? {} },
    });
    const data = parsed.data ?? {};
    if (!data.title) continue;

    docs[key] = {
      slug,
      meta: {
        title: data.title,
        lede: data.lede,
        status: data.status,
        sourceRef: data.sourceRef,
        related: data.related,
      },
      body: parsed.content,
    };
  }
}

walk(contentRoot, []);

const sorted = Object.fromEntries(Object.entries(docs).sort(([a], [b]) => a.localeCompare(b)));
fs.writeFileSync(outFile, JSON.stringify(sorted) + "\n");
console.log(`[docs-index] wrote ${Object.keys(sorted).length} docs -> ${path.relative(frontendDir, outFile)}`);
