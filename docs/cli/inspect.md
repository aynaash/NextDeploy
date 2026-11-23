# `nextdeploy inspect`

Analyze the Next.js standalone build artifact to examine total bundle size, dependency footprints, and identify large dependencies before deployment.

---

## Usage

```bash
nextdeploy inspect
```

---

## Output Example

```text
🔍 Inspecting bundle in /project/.next/standalone...

VPS Standalone Bundle Analysis:
   Total Size:      42.30 MB
   node_modules:    38.10 MB
   Server Code:     4.20 MB

Top Dependencies by Size:
    1. sharp                          14.50 MB
    2. @prisma/client                 8.20 MB
    3. next                           6.10 MB
    4. react-dom                      2.30 MB

Tips to reduce size:
   - Ensure specific large binaries (sharp, prisma) are only installed if needed.
   - Use 'next-bundle-analyzer' to find large client-side chunks.
   - Check for large data files mistakenly included in your source tree.
```

## Deep Explanation

To inspect the bundle analysis logic and thresholds:

```bash
nextdeploy inspect explain
```
