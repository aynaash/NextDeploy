// Segment-trie alternative to the linear regex scan in route_match.mjs.
//
// matchDynamic() walks the whole dynamicTable once per request — O(routes).
// This builds a radix/segment trie keyed on path parts, so matching is
// O(path depth × local branching) instead. For the few-hundred-route apps
// this codebase targets the wall-clock win is microseconds (see the note in
// route_match.mjs), so this is NOT wired into the dispatcher by default — it
// exists as a verified, drop-in-compatible alternative and as a worked
// example of turning a linear scan into a trie. route_trie.test.mjs proves
// it returns the same { entry, params } as matchDynamic for an unambiguous
// route corpus.
//
// The `.dev.mjs` suffix marks it dev-only: it is embedded for tests but never
// extracted into a deployed Worker or counted in the runtime content hash
// (see isDevOnlyRuntimeFile in runtime.go). Promote to a plain `.mjs` and
// import it from dispatcher.mjs to actually adopt it.
//
// Priority at each node mirrors build-time specificity ordering:
//   literal  >  [dyn]  >  [...catchAll]  >  [[...optCatchAll]]
// Depth-first with backtracking returns the first full match in that order,
// which equals the specificity-sorted linear scan's winner. Where two routes
// genuinely overlap (a tie the linear scan resolves by unstable sort), the
// trie's order is the canonical, deterministic tie-break.

const DYN = 1; // [slug]        — one non-empty segment
const CATCH = 2; // [...slug]   — one or more remaining segments
const OPT = 3; // [[...slug]]   — zero or more remaining segments

/** Split a route pattern or pathname into segments, mirroring the regex's
 *  optional trailing slash (`\/?$`) and leading-slash anchor. */
function segments(p) {
  let s = p;
  if (s.length > 1 && s.endsWith("/")) s = s.slice(0, -1); // optional trailing /
  if (s.startsWith("/")) s = s.slice(1);
  return s === "" ? [] : s.split("/");
}

function classify(seg) {
  if (seg.startsWith("[[...") && seg.endsWith("]]")) return OPT;
  if (seg.startsWith("[...") && seg.endsWith("]")) return CATCH;
  if (seg.startsWith("[") && seg.endsWith("]")) return DYN;
  return 0; // literal
}

function newNode() {
  return {
    literals: new Map(), // segment string -> node
    dyn: null, // node reached by consuming one dynamic segment
    catchAll: null, // terminal entry for [...x]
    optCatchAll: null, // terminal entry for [[...x]]
    entry: null, // terminal entry for a route ending exactly here
  };
}

/**
 * Build a trie from a dynamicTable (the array emitted into dispatch.mjs).
 * Only entry.route and entry.paramNames are read; the RegExp is ignored.
 */
export function buildRouteTrie(dynamicTable) {
  const root = newNode();
  for (const entry of dynamicTable) {
    let node = root;
    const segs = segments(entry.route);
    for (let i = 0; i < segs.length; i++) {
      const seg = segs[i];
      const kind = classify(seg);
      const last = i === segs.length - 1;
      if (kind === CATCH) {
        node.catchAll = entry; // catch-alls are always the final segment
        break;
      }
      if (kind === OPT) {
        node.optCatchAll = entry;
        break;
      }
      if (kind === DYN) {
        if (!node.dyn) node.dyn = newNode();
        node = node.dyn;
      } else {
        let next = node.literals.get(seg);
        if (!next) {
          next = newNode();
          node.literals.set(seg, next);
        }
        node = next;
      }
      if (last) node.entry = entry;
    }
    if (segs.length === 0) root.entry = entry; // "/" route (rare for dynamics)
  }
  return root;
}

/**
 * Match a pathname against the trie. Returns { entry, params } like
 * matchDynamic, or null. `params` is built by zipping the positionally
 * collected raw captures against entry.paramNames — identical semantics to
 * route_match.mjs's paramsFromMatch (match[i+1] indexing, decodeURIComponent
 * with raw fallthrough).
 */
export function matchTrie(pathname, root) {
  const segs = segments(pathname);
  const hit = walk(root, segs, 0, []);
  if (!hit) return null;
  return { entry: hit.entry, params: zipParams(hit.captures, hit.entry.paramNames) };
}

// Depth-first, priority-ordered, backtracking. Returns { entry, captures } or null.
function walk(node, segs, idx, captures) {
  if (idx === segs.length) {
    // Exact end wins over an optional catch-all matching zero segments.
    if (node.entry) return { entry: node.entry, captures };
    if (node.optCatchAll) {
      return { entry: node.optCatchAll, captures: [...captures, undefined] };
    }
    return null;
  }

  const seg = segs[idx];

  // 1. literal — most specific
  if (seg.length > 0) {
    const child = node.literals.get(seg);
    if (child) {
      const r = walk(child, segs, idx + 1, captures);
      if (r) return r;
    }
  }

  // 2. single dynamic segment ([^/]+ — non-empty)
  if (node.dyn && seg.length > 0) {
    const r = walk(node.dyn, segs, idx + 1, [...captures, seg]);
    if (r) return r;
  }

  // 3. required catch-all — one or more remaining segments
  if (node.catchAll) {
    const rest = segs.slice(idx).join("/");
    return { entry: node.catchAll, captures: [...captures, rest] };
  }

  // 4. optional catch-all — consumes the remainder (here, one or more)
  if (node.optCatchAll) {
    const rest = segs.slice(idx).join("/");
    return { entry: node.optCatchAll, captures: [...captures, rest] };
  }

  return null;
}

function zipParams(rawValues, names) {
  const out = {};
  for (let i = 0; i < names.length; i++) {
    const raw = rawValues[i];
    if (raw === undefined) continue;
    try {
      out[names[i]] = decodeURIComponent(raw);
    } catch {
      out[names[i]] = raw;
    }
  }
  return out;
}
