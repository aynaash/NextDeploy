"use client";

import { useActionState } from "react";
import Link from "next/link";
import {
  echoAction,
  revalidateIsrAction,
  setDemoCookieAction,
  type CookieActionState,
  type EchoActionState,
  type RevalidateActionState,
} from "./actions";

const initialEcho: EchoActionState = { ok: false };
const initialCookie: CookieActionState = { ok: false };
const initialReval: RevalidateActionState = { ok: false };

export default function ActionsClient({
  currentCookie,
}: {
  currentCookie: string | null;
}) {
  const [echoState, echoFormAction, echoPending] = useActionState(
    echoAction,
    initialEcho,
  );
  const [cookieState, cookieFormAction, cookiePending] = useActionState(
    setDemoCookieAction,
    initialCookie,
  );
  const [revalState, revalFormAction, revalPending] = useActionState(
    revalidateIsrAction,
    initialReval,
  );

  return (
    <div className="grid md:grid-cols-3 gap-4">
      {/* 1. Web Crypto Echo Action */}
      <form
        action={echoFormAction}
        className="rounded-xl border border-surface-800 bg-surface-900/70 p-5 space-y-4 flex flex-col justify-between"
      >
        <div className="space-y-3">
          <div className="text-xs font-mono text-brand-300 font-semibold">
            1. echoAction (Web Crypto SHA-256)
          </div>
          <input
            type="text"
            name="message"
            defaultValue="hello-cloudflare-worker"
            placeholder="Enter message..."
            className="w-full px-3 py-2 rounded-lg bg-surface-950 border border-surface-800 text-xs font-mono text-white focus:outline-none focus:border-brand-500"
          />
          {echoState.error && (
            <p className="text-xs font-mono text-rose-400">{echoState.error}</p>
          )}
          {echoState.ok && (
            <div className="p-3 rounded-lg bg-surface-950 border border-surface-800 text-[11px] font-mono space-y-1 break-all">
              <div className="text-emerald-400">Echo: {echoState.echo}</div>
              <div className="text-surface-400">SHA-256: {echoState.sha256}</div>
              <div className="text-surface-500">{echoState.timestamp}</div>
            </div>
          )}
        </div>
        <button
          type="submit"
          disabled={echoPending}
          className="w-full px-3 py-2 rounded-lg bg-brand-500 hover:bg-brand-400 disabled:opacity-60 text-surface-950 text-xs font-semibold transition-colors"
        >
          {echoPending ? "Hashing on server..." : "Run echoAction"}
        </button>
      </form>

      {/* 2. Cookie Mutation Action */}
      <form
        action={cookieFormAction}
        className="rounded-xl border border-surface-800 bg-surface-900/70 p-5 space-y-4 flex flex-col justify-between"
      >
        <div className="space-y-3">
          <div className="text-xs font-mono text-cyan-300 font-semibold">
            2. setDemoCookieAction (cookies().set)
          </div>
          <input
            type="text"
            name="value"
            defaultValue={currentCookie ?? "cf-worker-verified"}
            placeholder="Cookie value..."
            className="w-full px-3 py-2 rounded-lg bg-surface-950 border border-surface-800 text-xs font-mono text-white focus:outline-none focus:border-cyan-500"
          />
          <div className="text-[11px] font-mono text-surface-400">
            Active cookie:{" "}
            <span className="text-surface-200">
              {cookieState.cookieValue ?? currentCookie ?? "(none)"}
            </span>
          </div>
          {cookieState.ok && (
            <div className="text-[11px] font-mono text-emerald-400">
              Set at {cookieState.timestamp} ·{" "}
              <Link href="/routes/ssr" className="underline">
                Verify on /routes/ssr →
              </Link>
            </div>
          )}
        </div>
        <button
          type="submit"
          disabled={cookiePending}
          className="w-full px-3 py-2 rounded-lg bg-surface-800 hover:bg-surface-700 border border-surface-700 text-white text-xs font-semibold transition-colors"
        >
          {cookiePending ? "Setting cookie..." : "Set Demo Cookie"}
        </button>
      </form>

      {/* 3. RevalidatePath Action */}
      <form
        action={revalFormAction}
        className="rounded-xl border border-surface-800 bg-surface-900/70 p-5 space-y-4 flex flex-col justify-between"
      >
        <div className="space-y-3">
          <div className="text-xs font-mono text-amber-300 font-semibold">
            3. revalidateIsrAction (revalidatePath)
          </div>
          <p className="text-xs text-surface-300 leading-relaxed">
            Invokes <code className="text-brand-300 font-mono">revalidatePath(&quot;/routes/isr&quot;)</code>{" "}
            from a Server Action and propagates{" "}
            <code className="text-brand-300 font-mono">x-nextcompile-revalidated-paths</code>.
          </p>
          {revalState.ok && (
            <div className="p-3 rounded-lg bg-surface-950 border border-surface-800 text-[11px] font-mono text-emerald-400">
              Revalidated {revalState.path} at {revalState.timestamp} ·{" "}
              <Link href="/routes/isr" className="underline text-brand-300">
                Open /routes/isr →
              </Link>
            </div>
          )}
        </div>
        <button
          type="submit"
          disabled={revalPending}
          className="w-full px-3 py-2 rounded-lg bg-surface-800 hover:bg-surface-700 border border-surface-700 text-white text-xs font-semibold transition-colors"
        >
          {revalPending ? "Revalidating..." : "Revalidate /routes/isr"}
        </button>
      </form>
    </div>
  );
}
