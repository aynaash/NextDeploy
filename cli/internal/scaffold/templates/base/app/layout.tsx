import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "__APP_NAME__ — NextDeploy Fullstack",
  description: "Production-ready fullstack Next.js app deployed with NextDeploy",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
