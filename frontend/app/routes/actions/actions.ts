"use server";

import { revalidatePath } from "next/cache";
import { cookies } from "next/headers";

export interface EchoActionState {
  ok: boolean;
  echo?: string;
  sha256?: string;
  timestamp?: string;
  error?: string;
}

export interface CookieActionState {
  ok: boolean;
  cookieValue?: string;
  timestamp?: string;
  error?: string;
}

export interface RevalidateActionState {
  ok: boolean;
  path?: string;
  timestamp?: string;
}

export async function echoAction(
  _prev: EchoActionState,
  formData: FormData,
): Promise<EchoActionState> {
  const raw = String(formData.get("message") ?? "").trim();
  if (!raw) {
    return { ok: false, error: "Message cannot be empty." };
  }
  const bytes = new TextEncoder().encode(raw);
  const hashBuf = await crypto.subtle.digest("SHA-256", bytes);
  const sha256 = Array.from(new Uint8Array(hashBuf))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");

  return {
    ok: true,
    echo: raw,
    sha256,
    timestamp: new Date().toISOString(),
  };
}

export async function setDemoCookieAction(
  _prev: CookieActionState,
  formData: FormData,
): Promise<CookieActionState> {
  const value =
    String(formData.get("value") ?? "").trim() || "nextdeploy-edge-ok";
  const store = await cookies();
  store.set("nextdeploy_route_demo", value, {
    path: "/",
    maxAge: 3600,
    sameSite: "lax",
  });

  return {
    ok: true,
    cookieValue: value,
    timestamp: new Date().toISOString(),
  };
}

export async function revalidateIsrAction(): Promise<RevalidateActionState> {
  revalidatePath("/routes/isr");
  return {
    ok: true,
    path: "/routes/isr",
    timestamp: new Date().toISOString(),
  };
}
