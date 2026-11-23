import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "NextDeploy - Self-Hosted Next.js Deployment Platform",
    short_name: "NextDeploy",
    description:
      "Deploy Next.js to your own Linux VPS, Cloudflare Workers, or AWS Lambda with zero lock-in, atomic zero-downtime rollouts, and automatic SSL.",
    start_url: "/",
    display: "standalone",
    background_color: "#020617",
    theme_color: "#10b981",
    icons: [
      {
        src: "/favicon-16x16.png",
        sizes: "16x16",
        type: "image/png",
      },
      {
        src: "/favicon-32x32.png",
        sizes: "32x32",
        type: "image/png",
      },
      {
        src: "/android-chrome-192x192.png",
        sizes: "192x192",
        type: "image/png",
      },
      {
        src: "/android-chrome-512x512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "any",
      },
      {
        src: "/maskable-icon-512x512.png",
        sizes: "512x512",
        type: "image/png",
        purpose: "maskable",
      },
    ],
  };
}
