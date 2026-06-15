// Package telemetry sends a single anonymous "a project shipped" event on a
// successful deploy. It powers the public "apps shipped" counter on the
// NextDeploy site.
//
// It is opt-out and deliberately minimal: it never collects identifying
// information — no project name, domain, repo, file paths, env, or secrets.
// The entire payload is a random per-install UUID, the deploy target, the
// nextdeploy version, and OS/arch.
//
// Disable it with any of:
//   - DO_NOT_TRACK=1            (the cross-tool standard)
//   - NEXTDEPLOY_TELEMETRY=0
//   - nextdeploy telemetry off  (writes a local marker)
package telemetry

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	defaultEndpoint = "https://nextdeploy.org/api/telemetry"
	sendTimeout     = 1500 * time.Millisecond

	disabledMarker = "telemetry-disabled"
	noticeMarker   = "telemetry-notice"
	idFile         = "telemetry-id"
)

// Event is the entire telemetry payload. Anonymous by construction.
type Event struct {
	ID      string `json:"id"`      // random per-install UUID — not identifying
	Event   string `json:"event"`   // e.g. "ship.success"
	Target  string `json:"target"`  // vps | aws | cloudflare | other
	Version string `json:"version"` // nextdeploy version
	OS      string `json:"os"`      // runtime.GOOS
	Arch    string `json:"arch"`    // runtime.GOARCH
	TS      int64  `json:"ts"`      // unix seconds
	Nonce   string `json:"nonce"`   // random per-event — replay protection / dedup
}

// signingKeyB64 is a base64-encoded Ed25519 private key (seed or full key),
// injected at build time via:
//
//	-ldflags "-X github.com/aynaash/nextdeploy/shared/telemetry.signingKeyB64=<base64>"
//
// It is intentionally absent from the source tree (it ships only inside
// official release binaries). Builds without it send unsigned events, which the
// server is expected to reject — so only official builds increment the counter.
var signingKeyB64 string

// canonical is the exact byte sequence that gets signed/verified. It is derived
// from the typed fields (not the raw JSON) so client and server agree
// regardless of JSON formatting/key order.
func canonical(ev *Event) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%d|%s",
		ev.ID, ev.Event, ev.Target, ev.Version, ev.OS, ev.Arch, ev.TS, ev.Nonce)
}

// sign returns an Ed25519 signature over the canonical message, base64-encoded,
// or "" if no signing key is built in.
func sign(ev *Event) string {
	if signingKeyB64 == "" {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(signingKeyB64)
	if err != nil {
		return ""
	}
	var priv ed25519.PrivateKey
	switch len(raw) {
	case ed25519.SeedSize:
		priv = ed25519.NewKeyFromSeed(raw)
	case ed25519.PrivateKeySize:
		priv = ed25519.PrivateKey(raw)
	default:
		return ""
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(priv, []byte(canonical(ev))))
}

// RecordShipSuccess reports one successful deploy. It resolves opt-out, prints
// the one-time disclosure, and sends the event synchronously with a short
// timeout. Every failure is swallowed: telemetry must never affect a deploy.
func RecordShipSuccess(target, version string) {
	if !Enabled() {
		return
	}
	maybePrintNotice()
	send(&Event{
		ID:      installID(),
		Event:   "ship.success",
		Target:  normalizeTarget(target),
		Version: version,
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		TS:      time.Now().Unix(),
		Nonce:   randomID(),
	})
}

// Enabled reports whether telemetry should run. Opt-out model: on unless the
// user turned it off via env or the local marker.
func Enabled() bool {
	switch strings.ToLower(os.Getenv("DO_NOT_TRACK")) {
	case "1", "true", "yes":
		return false
	}
	switch strings.ToLower(os.Getenv("NEXTDEPLOY_TELEMETRY")) {
	case "0", "false", "off", "no":
		return false
	}
	if dir, err := configDir(); err == nil {
		if _, err := os.Stat(filepath.Join(dir, disabledMarker)); err == nil {
			return false
		}
	}
	return true
}

// Disable writes the local opt-out marker.
func Disable() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, disabledMarker), []byte("disabled\n"), 0o600)
}

// Enable removes the local opt-out marker (no-op if absent).
func Enable() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(dir, disabledMarker)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// StatusLine returns a human-readable on/off summary for `telemetry status`.
func StatusLine() string {
	if Enabled() {
		return "telemetry is ON (anonymous). Disable with: nextdeploy telemetry off"
	}
	return "telemetry is OFF."
}

func normalizeTarget(target string) string {
	switch strings.ToLower(strings.TrimSpace(target)) {
	case "cloudflare", "cf":
		return "cloudflare"
	case "aws", "aws_lambda", "lambda":
		return "aws"
	case "vps":
		return "vps"
	default:
		return "other"
	}
}

func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "nextdeploy"), nil
}

// installID returns a stable random ID for this install, creating it on first
// use. If it can't be persisted, a fresh random ID is returned (still anonymous).
func installID() string {
	dir, err := configDir()
	if err != nil {
		return randomID()
	}
	path := filepath.Join(dir, idFile)
	if b, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(b)); id != "" {
			return id
		}
	}
	id := randomID()
	if err := os.MkdirAll(dir, 0o700); err == nil {
		_ = os.WriteFile(path, []byte(id), 0o600)
	}
	return id
}

func randomID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

const noticeText = `
NextDeploy collects anonymous deploy telemetry to power the public "apps
shipped" counter. It records only a random ID, the deploy target, and version —
never your code, project name, domain, or secrets.
Opt out any time:  nextdeploy telemetry off   (or set DO_NOT_TRACK=1)
`

func maybePrintNotice() {
	dir, err := configDir()
	if err != nil {
		return
	}
	marker := filepath.Join(dir, noticeMarker)
	if _, err := os.Stat(marker); err == nil {
		return // already shown
	}
	fmt.Fprint(os.Stderr, noticeText)
	if err := os.MkdirAll(dir, 0o700); err == nil {
		_ = os.WriteFile(marker, []byte(time.Now().UTC().Format(time.RFC3339)), 0o600)
	}
}

func send(ev *Event) {
	body, err := json.Marshal(ev)
	if err != nil {
		return
	}
	endpoint := os.Getenv("NEXTDEPLOY_TELEMETRY_URL")
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	ctx, cancel := context.WithTimeout(context.Background(), sendTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body)) // #nosec G107 -- telemetry endpoint, intentionally overridable via env
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "nextdeploy-telemetry/"+ev.Version)
	if sig := sign(ev); sig != "" {
		req.Header.Set("X-NextDeploy-Signature", "ed25519="+sig)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	_ = resp.Body.Close()
}
