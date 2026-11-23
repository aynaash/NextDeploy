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
	if len(res) == 0 && len(r.Secrets) == 0 && len(r.Warnings) == 0 {
		return "No Cloudflare resources detected — a minimal config was written."
	}
	if src := r.wranglerSource(); src != "" {
		fmt.Fprintf(&b, "Read existing bindings from %s.\n", src)
	}
	for _, w := range r.Warnings {
		fmt.Fprintf(&b, "Warning: %s\n", w)
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
		b.WriteString("Auth library detected — added a disabled protection block (bind RATE_LIMIT KV and set enabled: true to activate).\n")
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
	bindings := renderBindings(r)
	resources := renderResources(r)

	if bindings == "" && resources == "" && !r.Auth {
		return ""
	}

	var b strings.Builder
	b.WriteString("    cloudflare:\n")
	b.WriteString("      compatibility_date: \"2025-04-01\"\n")
	b.WriteString("      compatibility_flags: [nodejs_compat_v2]\n")

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
				fmt.Fprintf(&b, "          - { name: %s, ref: %s }\n", x.Name, kvRefName(x.Name))
			}
		}
	} else if has(r, ResKV) {
		b.WriteString("        kv:\n          - { name: CACHE, ref: app-cache }\n")
	}

	// Hyperdrive (BYO Postgres/MySQL)
	if w != nil && len(w.Hyperdrive) > 0 {
		b.WriteString("        hyperdrive:\n")
		for _, x := range w.Hyperdrive {
			if x.ID != "" {
				fmt.Fprintf(&b, "          - { name: %s, id: %s }\n", x.Name, x.ID)
			} else {
				fmt.Fprintf(&b, "          - { name: %s, ref: %s }\n", x.Name, refName(x.Resource, "primary"))
			}
		}
	} else if has(r, ResHyperdrive) {
		b.WriteString("        hyperdrive:\n          - { name: DB, ref: primary }\n")
	}

	// Vectorize
	if w != nil && len(w.Vectorize) > 0 {
		b.WriteString("        vectorize:\n")
		for _, v := range w.Vectorize {
			fmt.Fprintf(&b, "          - { name: %s, index: %s }\n", v.Name, refName(v.Resource, "app-index"))
		}
	} else if has(r, ResVectorize) {
		b.WriteString("        vectorize:\n          - { name: VECTORIZE, index: app-index }\n")
	}

	// Queues (producers)
	if w != nil && len(w.Queues) > 0 {
		b.WriteString("        queues:\n          producers:\n")
		for _, q := range w.Queues {
			fmt.Fprintf(&b, "            - { name: %s, queue: %s }\n", q.Name, refName(q.Resource, "app-queue"))
		}
	} else if has(r, ResQueue) {
		b.WriteString("        queues:\n          producers:\n            - { name: QUEUE, queue: app-queue }\n")
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

	// D1 databases to provision: only declare databases without an explicit ID,
	// and only attach migrations_dir when wrangler declares it or the directory
	// actually exists on disk (avoiding ENOENT in `plan`/`apply`).
	var d1ToProvision []WranglerBinding
	if w != nil && len(w.D1) > 0 {
		for _, d := range w.D1 {
			if d.ID == "" {
				d1ToProvision = append(d1ToProvision, d)
			}
		}
	} else if has(r, ResD1) {
		d1ToProvision = append(d1ToProvision, WranglerBinding{Resource: "app-db", MigrationsDir: r.MigrationsDir})
	}
	if len(d1ToProvision) > 0 {
		b.WriteString("        d1:\n")
		for _, d := range d1ToProvision {
			migDir := d.MigrationsDir
			if migDir == "" {
				migDir = r.MigrationsDir
			}
			if migDir != "" {
				fmt.Fprintf(&b, "          - { name: %s, migrations_dir: %s }\n", refName(d.Resource, "app-db"), migDir)
			} else {
				fmt.Fprintf(&b, "          - { name: %s }\n", refName(d.Resource, "app-db"))
			}
		}
	}

	// Hyperdrive config for BYO Postgres — skip provisioning if every wrangler
	// entry already carries an explicit ID.
	if w != nil && len(w.Hyperdrive) > 0 {
		var hdNames []string
		for _, h := range w.Hyperdrive {
			if h.ID == "" {
				hdNames = append(hdNames, refName(h.Resource, "primary"))
			}
		}
		if len(hdNames) > 0 {
			b.WriteString("        hyperdrive:\n")
			for _, name := range hdNames {
				fmt.Fprintf(&b, "          - { name: %s, origin_env: DATABASE_URL }\n", name)
			}
		}
	} else if has(r, ResHyperdrive) {
		b.WriteString("        hyperdrive:\n")
		b.WriteString("          - { name: primary, origin_env: DATABASE_URL }\n")
	}

	// KV namespaces to provision — one resource per ID-less KV binding.
	var kvNames []string
	if w != nil && len(w.KV) > 0 {
		for _, k := range w.KV {
			if k.ID == "" {
				kvNames = append(kvNames, kvRefName(k.Name))
			}
		}
	} else if has(r, ResKV) {
		kvNames = append(kvNames, "app-cache")
	}
	if len(kvNames) > 0 {
		b.WriteString("        kv:\n")
		for _, name := range kvNames {
			fmt.Fprintf(&b, "          - { name: %s }\n", name)
		}
	}

	// Queues to provision.
	if w != nil && len(w.Queues) > 0 {
		b.WriteString("        queues:\n")
		for _, q := range w.Queues {
			fmt.Fprintf(&b, "          - { name: %s }\n", refName(q.Resource, "app-queue"))
		}
	} else if has(r, ResQueue) {
		b.WriteString("        queues:\n          - { name: app-queue }\n")
	}

	// Vectorize indexes have immutable dimensions & metric, so emit a commented
	// stub rather than guessing values that would cause immutable drift.
	if has(r, ResVectorize) {
		idxName := "app-index"
		if w != nil && len(w.Vectorize) > 0 && w.Vectorize[0].Resource != "" {
			idxName = w.Vectorize[0].Resource
		}
		fmt.Fprintf(&b, "        # vectorize (fill in immutable dimensions & metric, then uncomment):\n        # vectorize:\n        #   - { name: %s, dimensions: 1536, metric: cosine }\n", idxName)
	}

	return b.String()
}

func protectionBlock() string {
	// Emitted disabled by default on existing apps: enabling rate_limit without
	// a bound RATE_LIMIT KV fails closed with HTTP 503, and enabling HMAC cookie
	// auth can conflict with existing auth libraries (Clerk, NextAuth, Better Auth).
	return strings.Join([]string{
		"      # Auth library detected — bind RATE_LIMIT KV and verify cookie contract before enabling:",
		"      protection:",
		"        enabled: false",
		"        public_paths: [\"/\", \"/login\", \"/api/health\"]",
		"        auth:",
		"          secret_env: AUTH_SECRET",
		"          protected_paths: [\"/app/*\"]",
		"          login_path: /login",
		"        rate_limit:",
		"          kv_binding: RATE_LIMIT",
		"          requests_per_minute: 60",
		"",
	}, "\n")
}

func kvRefName(bindingName string) string {
	slug := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(bindingName), "_", "-"))
	if slug == "" || slug == "cache" {
		return "app-cache"
	}
	return "app-" + slug
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
