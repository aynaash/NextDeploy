package nextcore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseMiddlewareAndProxy_SupportsSrcDirectory(t *testing.T) {
	t.Run("src/middleware.ts", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "src", "middleware.ts")
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(`export const config = { matcher: ['/dashboard/:path*'] };`), 0o600); err != nil {
			t.Fatal(err)
		}

		mw, err := ParseMiddleware(dir)
		if err != nil {
			t.Fatalf("ParseMiddleware: %v", err)
		}
		if mw == nil || mw.Path != p {
			t.Fatalf("expected middleware at %s, got %+v", p, mw)
		}
		if len(mw.Matchers) != 1 || mw.Matchers[0].Pathname != "/dashboard/:path*" {
			t.Errorf("unexpected matchers: %+v", mw.Matchers)
		}
	})

	t.Run("src/proxy.ts", func(t *testing.T) {
		dir := t.TempDir()
		p := filepath.Join(dir, "src", "proxy.ts")
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(`export const config = { matcher: ['/app/:path*'] };`), 0o600); err != nil {
			t.Fatal(err)
		}

		px, err := ParseProxy(dir)
		if err != nil {
			t.Fatalf("ParseProxy: %v", err)
		}
		if px == nil || px.Path != p {
			t.Fatalf("expected proxy at %s, got %+v", p, px)
		}
		// ParseMiddleware also falls back to src/proxy.ts when no middleware exists.
		mw, err := ParseMiddleware(dir)
		if err != nil {
			t.Fatalf("ParseMiddleware fallback: %v", err)
		}
		if mw == nil || mw.Path != p {
			t.Fatalf("expected ParseMiddleware fallback to %s, got %+v", p, mw)
		}
	})

	t.Run("root takes precedence over src", func(t *testing.T) {
		dir := t.TempDir()
		rootP := filepath.Join(dir, "proxy.ts")
		srcP := filepath.Join(dir, "src", "proxy.ts")
		if err := os.MkdirAll(filepath.Dir(srcP), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(rootP, []byte(`export default function proxy() {}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(srcP, []byte(`export default function proxy() {}`), 0o600); err != nil {
			t.Fatal(err)
		}

		px, err := ParseProxy(dir)
		if err != nil {
			t.Fatalf("ParseProxy: %v", err)
		}
		if px == nil || px.Path != rootP {
			t.Fatalf("expected root proxy %s to take precedence, got %+v", rootP, px)
		}
	})
}
