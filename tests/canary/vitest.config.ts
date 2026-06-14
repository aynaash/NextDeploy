import { defineConfig } from "vitest/config";

export default defineConfig({
  test: {
    include: ["./cloudflare_edge.test.ts"],
    // Compiling + serving a Worker bundle can be slow on first request.
    testTimeout: 30_000,
    hookTimeout: 30_000,
  },
});
