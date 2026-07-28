package config

import (
	"fmt"
	"regexp"
	"strings"
)

// App-name rules. This is the SINGLE source of truth for what a valid app
// identifier looks like — the CLI (config load, pre-upload check) and the daemon
// (validateAppName, sanitizeAppName) all route through it. Before this existed
// there were two regexes and several raw usages, so `app.name` could be accepted
// by the CLI, baked into an artifact, and then rejected 500 lines deep in the
// daemon with a message that named neither the value nor the fix.
const (
	AppNameMinLen = 3
	AppNameMaxLen = 63
)

// appNameRe is deliberately lowercase-only: the name becomes a DNS label, a
// systemd unit name, and a directory on a case-insensitive-capable filesystem.
var appNameRe = regexp.MustCompile(`^[a-z0-9-]+$`)

// ValidateAppName reports whether name is usable as an app identifier. The error
// is UI: it names the offending value, states the rule, explains the slug-vs-
// domain distinction, and gives the concrete fix.
func ValidateAppName(name string) error {
	if name == "" {
		return appNameError("", "app.name is required")
	}
	if !appNameRe.MatchString(name) {
		return appNameError(name, "contains characters outside [a-z0-9-]")
	}
	if len(name) < AppNameMinLen || len(name) > AppNameMaxLen {
		return appNameError(name, fmt.Sprintf("must be %d–%d characters (got %d)", AppNameMinLen, AppNameMaxLen, len(name)))
	}
	return nil
}

// appNameError renders the four-part constraint message — what's wrong, the
// offending value, the rule, the fix — boxed so it can't scroll past unnoticed.
func appNameError(name, why string) error {
	return fmt.Errorf(
		"\n╭─ INVALID app.name ─────────────────────────────────────────────╮\n"+
			"│ value : %q\n"+
			"│ problem: %s\n"+
			"│ rule  : lowercase letters, digits and hyphens (^[a-z0-9-]+$), %d–%d chars\n"+
			"│ note  : this is a SLUG, not a domain — the domain belongs in app.domain.name\n"+
			"│ fix   : set app.name in nextdeploy.yml to e.g. %q, then re-run\n"+
			"╰────────────────────────────────────────────────────────────────╯",
		name, why, AppNameMinLen, AppNameMaxLen, SlugifyAppName(name))
}

// SlugifyAppName derives a valid app identifier from an arbitrary string, so an
// error message can suggest a concrete replacement rather than just a regex.
// It is a suggestion helper only — nothing auto-applies it, because silently
// renaming an app would deploy under a different unit/directory than the user
// asked for.
func SlugifyAppName(s string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
		default:
			// Collapse any run of separators (dots, spaces, underscores) into one
			// hyphen, so "my app.co.uk" → "my-app-co-uk", not "my--app-co-uk".
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > AppNameMaxLen {
		slug = strings.TrimRight(slug[:AppNameMaxLen], "-")
	}
	if len(slug) < AppNameMinLen {
		return "my-app"
	}
	return slug
}
