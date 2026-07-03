package infrasniff

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// Summary is a short human-readable report of what was detected, for printing
// during `nextdeploy init`.
func (r *Result) Summary() string {
	var b strings.Builder
	res := r.Resources()
	if len(res) == 0 && len(r.Secrets) == 0 {
		return "No Cloudflare resources detected — a minimal config was written."
	}
	if src := r.wranglerSource(); src != "" {
		fmt.Fprintf(&b, "Read existing bindings from %s.\n", src)
	}
	if len(res) > 0 {
		names := make([]string, len(res))
		for i, x := range res {
			names[i] = string(x)
		}
		fmt.Fprintf(&b, "Detected resources: %s\n", strings.Join(names, ", "))
	}
	if len(r.Secrets) > 0 {
		fmt.Fprintf(&b, "Server secrets to set: %s\n", strings.Join(r.Secrets, ", "))
	}
	if r.Auth {
		b.WriteString("Auth library detected — added a protection block guarding /app/*.\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (r *Result) wranglerSource() string {
	if r.Wrangler != nil {
		return r.Wrangler.Source
	}
	return ""
}

// SuggestedCloudflareBlock renders a `cloudflare:` YAML block (indented to sit
// under `serverless:`) from the sniff result. Authoritative wrangler bindings
// are emitted with real names/IDs; heuristic-only detections become resource
// declarations + ref bindings the user can complete. Returns "" when nothing
// was detected.
func (r *Result) SuggestedCloudflareBlock() string {
	w := r.Wrangler
	var b strings.Builder
	b.WriteString("    cloudflare:\n")

	bindings := renderBindings(r)
	resources := renderResources(r)

	if bindings != "" {
		b.WriteString("      bindings:\n")
		b.WriteString(bindings)
	}
	if resources != "" {
		b.WriteString("      resources:\n")
		b.WriteString(resources)
	}
	if r.Auth {
		b.WriteString(protectionBlock())
	}

	// Nothing but the header → no detections.
	if bindings == "" && resources == "" && !r.Auth {
		return ""
	}
	_ = w
	return b.String()
}

func renderBindings(r *Result) string {
	w := r.Wrangler
	var b strings.Builder

	// D1
	if w != nil && len(w.D1) > 0 {
		b.WriteString("        d1:\n")
		for _, d := range w.D1 {
			if d.ID != "" {
				fmt.Fprintf(&b, "          - { name: %s, id: %s }\n", d.Name, d.ID)
			} else {
				fmt.Fprintf(&b, "          - { name: %s, ref: %s }\n", d.Name, refName(d.Resource, "app-db"))
			}
		}
	} else if has(r, ResD1) {
		b.WriteString("        d1:\n          - { name: DB, ref: app-db }\n")
	}

	// R2
	if w != nil && len(w.R2) > 0 {
		b.WriteString("        r2:\n")
		for _, x := range w.R2 {
			fmt.Fprintf(&b, "          - { name: %s, bucket: %s }\n", x.Name, refName(x.Resource, "assets"))
		}
	} else if has(r, ResR2) {
		b.WriteString("        r2:\n          - { name: ASSETS }\n")
	}

	// KV — use the explicit id from wrangler when present, otherwise a ref to a
	// provisioned namespace (declared under resources.kv).
	if w != nil && len(w.KV) > 0 {
		b.WriteString("        kv:\n")
		for _, x := range w.KV {
			if x.ID != "" {
				fmt.Fprintf(&b, "          - { name: %s, namespace_id: %s }\n", x.Name, x.ID)
			} else {
				fmt.Fprintf(&b, "          - { name: %s, ref: %s }\n", x.Name, "app-cache")
			}
		}
	} else if has(r, ResKV) {
		b.WriteString("        kv:\n          - { name: CACHE, ref: app-cache }\n")
	}

	// Hyperdrive (BYO Postgres/MySQL)
	if w != nil && len(w.Hyperdrive) > 0 {
		b.WriteString("        hyperdrive:\n")
		for _, x := range w.Hyperdrive {
			fmt.Fprintf(&b, "          - { name: %s, id: %s }\n", x.Name, valueOr(x.ID, "REPLACE_WITH_HYPERDRIVE_ID"))
		}
	} else if has(r, ResHyperdrive) {
		b.WriteString("        hyperdrive:\n          - { name: DB, ref: primary }\n")
	}

	// AI
	if (w != nil && w.AI != "") || has(r, ResAI) {
		name := "AI"
		if w != nil && w.AI != "" {
			name = w.AI
		}
		fmt.Fprintf(&b, "        ai:\n          - { name: %s }\n", name)
	}

	return b.String()
}

func renderResources(r *Result) string {
	w := r.Wrangler
	var b strings.Builder

	// D1 databases to provision (when there's a D1 binding, declare the db).
	if has(r, ResD1) {
		b.WriteString("        d1:\n")
		name := "app-db"
		if w != nil && len(w.D1) > 0 && w.D1[0].Resource != "" {
			name = w.D1[0].Resource
		}
		fmt.Fprintf(&b, "          - { name: %s, migrations_dir: drizzle }\n", name)
	}

	// Hyperdrive config for BYO Postgres.
	if has(r, ResHyperdrive) {
		b.WriteString("        hyperdrive:\n")
		b.WriteString("          - { name: primary, origin_env: DATABASE_URL }\n")
	}

	// KV namespace to provision when a binding used a ref (no explicit id).
	if has(r, ResKV) && (w == nil || !hasKVID(w)) {
		b.WriteString("        kv:\n          - { name: app-cache }\n")
	}

	return b.String()
}

func protectionBlock() string {
	return strings.Join([]string{
		"      protection:",
		"        enabled: true",
		"        auth:",
		"          secret_env: AUTH_SECRET",
		"          protected_paths: [\"/app/*\"]",
		"          login_path: /login",
		"        rate_limit:",
		"          requests_per_minute: 60",
		"",
	}, "\n")
}

// hasKVID reports whether the wrangler config supplies at least one KV
// namespace with an explicit id (so no provisioning ref is needed).
func hasKVID(w *WranglerConfig) bool {
	for _, k := range w.KV {
		if k.ID != "" {
			return true
		}
	}
	return false
}

func has(r *Result, want Resource) bool {
	return slices.Contains(r.Resources(), want)
}

func refName(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

func valueOr(v, def string) string {
	if v != "" {
		return v
	}
	return def
}

// RenderNextDeployYAML composes a complete nextdeploy.yml for the "use my
// existing app" path: app + serverless headers plus the detected cloudflare
// block (or a minimal stub when nothing was detected).
func (r *Result) RenderNextDeployYAML(appName string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Generated by `nextdeploy init` from your existing app.\n")
	fmt.Fprintf(&b, "app:\n  name: %s\n", appName)
	// Domain is an active field, not a commented block — that way setting it is
	// just filling in the value, with no risk of mis-indenting or duplicating
	// keys on uncomment. Empty = deploy on *.workers.dev. The CF pipeline
	// auto-attaches a non-empty value as a Custom Domain on ship. For a custom
	// registrar/DNS mode, replace the line with a block: name/provider/dns/zone.
	b.WriteString(domainFieldYAML)
	b.WriteString("\nserverless:\n  provider: cloudflare\n")

	block := r.SuggestedCloudflareBlock()
	if block == "" {
		b.WriteString("  cloudflare:\n    compatibility_date: \"2025-04-01\"\n    compatibility_flags: [nodejs_compat_v2]\n")
	} else {
		// SuggestedCloudflareBlock is rendered two levels too deep for the
		// `serverless:` parent; dedent it so `cloudflare:` sits at 2 spaces.
		b.WriteString(dedentBlock(block))
	}
	return b.String()
}

// domainFieldYAML is the single active `domain:` line written under `app:`. It
// is valid YAML as-is (empty domain) so users edit the value rather than
// uncommenting and risking indentation/duplicate-key errors.
const domainFieldYAML = "  domain: \"\" # optional custom domain, e.g. example.com — attached automatically on ship\n"

// dedentBlock removes two leading spaces from every line, fixing a block that
// was rendered one nesting level too deep.
func dedentBlock(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = strings.TrimPrefix(ln, "  ")
	}
	return strings.Join(lines, "\n")
}

// SecretsChecklist returns the detected secrets sorted, for a post-init "set
// these with `nextdeploy secrets set`" reminder.
func (r *Result) SecretsChecklist() []string {
	out := append([]string{}, r.Secrets...)
	sort.Strings(out)
	return out
}
