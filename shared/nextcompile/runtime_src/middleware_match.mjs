// nextcompile runtime — middleware/proxy matcher evaluation.
//
// Next's `matcher` config gates which requests run middleware. A request runs
// iff SOME rule matches, where a rule matches iff: the path matches AND every
// `has` condition is present AND every `missing` condition is absent. That
// disjunction-of-conjunctions shape is load-bearing — AND-ing across rules
// would make any two-entry matcher impossible to satisfy.
//
// Conditions are { type: "header"|"cookie"|"query"|"host", key, value? }.
// `value` is optional: empty means "key present with any value"; when set it
// is an exact match. (Next also allows regex values; not supported here.)

import { pathMatches } from "./guard.mjs";

/**
 * Should the middleware/proxy run for this request, given its matcher list?
 *
 * An empty/absent matcher list returns true: Next runs middleware broadly when
 * a matcher is unspecified, and preserving that avoids a behavior regression
 * for apps that rely on it. Rules only ever *restrict*.
 *
 * @param {Request} request
 * @param {Array<{pathname?: string, pattern?: string, has?: Array, missing?: Array}>} matchers
 * @returns {boolean}
 */
export function middlewareMatches(request, matchers) {
  if (!Array.isArray(matchers) || matchers.length === 0) return true;
  let url;
  try {
    url = new URL(request.url);
  } catch {
    return true; // unparseable URL — fail open, same as no matcher
  }
  return matchers.some((rule) => ruleMatches(rule, request, url));
}

function ruleMatches(rule, request, url) {
  if (!rule || typeof rule !== "object") return false;
  if (!pathRuleMatches(rule, url.pathname)) return false;
  for (const cond of rule.has || []) {
    if (!conditionPresent(cond, request, url)) return false;
  }
  for (const cond of rule.missing || []) {
    if (conditionPresent(cond, request, url)) return false;
  }
  return true;
}

// Path leg: prefer the compiled regex `pattern` (Next emits it); fall back to
// the glob `pathname` via guard.mjs's pathMatches. No path constraint → match.
function pathRuleMatches(rule, pathname) {
  if (rule.pattern) {
    try {
      return new RegExp(rule.pattern).test(pathname);
    } catch {
      // Malformed pattern — fall through to pathname rather than throw.
    }
  }
  if (rule.pathname) return pathMatches(pathname, rule.pathname);
  return true;
}

function conditionPresent(cond, request, url) {
  if (!cond || typeof cond !== "object") return false;
  const actual = readCondition(cond, request, url);
  if (actual === null || actual === undefined) return false;
  if (!cond.value) return true; // presence-only
  return actual === cond.value;
}

function readCondition(cond, request, url) {
  switch (cond.type) {
    case "header":
      return request.headers.get(cond.key);
    case "cookie":
      return readCookie(request, cond.key);
    case "query":
      return url.searchParams.get(cond.key);
    case "host":
      return url.hostname; // Next's host condition ignores `key`
    default:
      return null;
  }
}

function readCookie(request, name) {
  const raw = request.headers.get("cookie");
  if (!raw) return null;
  for (const part of raw.split(";")) {
    const idx = part.indexOf("=");
    if (idx === -1) continue;
    if (part.slice(0, idx).trim() === name) {
      return decodeURIComponent(part.slice(idx + 1).trim());
    }
  }
  return null;
}
