import type { ReactNode } from "react";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Intercepting Route Gallery",
  description:
    "App Router intercepting route (.)item/[id] with @modal slot and full-page fallback.",
};

export default function GalleryLayout({
  children,
  modal,
}: {
  children: ReactNode;
  modal: ReactNode;
}) {
  return (
    <>
      {children}
      {modal}
    </>
  );
}
