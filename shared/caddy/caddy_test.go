package caddy

import (
	"strings"
	"testing"
)

const testAppDir = "/opt/nextdeploy/apps/demo/current"

func generate(t *testing.T, mode string) string {
	t.Helper()
	return GenerateCaddyfile("demo", "example.com", mode, 3000, testAppDir, nil, ".next", "out")
}

// siteHeaderIsOneLine guards an assumption the daemon depends on: CaddyManager
// finds collisions by scanning everything before the first "{" (siteAddrs,
// caddy_manager.go:36). A multi-line site header would silently stop those
// checks from seeing anything.
func TestSiteHeaderStaysOnOneLine(t *testing.T) {
	for _, mode := range []string{"standalone", "default", "export"} {
		t.Run(mode, func(t *testing.T) {
			head, _, ok := strings.Cut(generate(t, mode), "{")
			if !ok {
				t.Fatal("no opening brace in the fragment")
			}
			if strings.Contains(strings.TrimSpace(head), "\n") {
				t.Errorf("site header spans multiple lines, which breaks siteAddrs:\n%q", head)
			}
		})
	}
}

func TestPublicAssetsAreServedByCaddyNotProxied(t *testing.T) {
	// Every file under public/ used to wake the Node process for a static read.
	got := generate(t, "standalone")

	if !strings.Contains(got, "root * "+testAppDir+"/public") {
		t.Errorf("public/ is not rooted at the release dir:\n%s", got)
	}
	if !strings.Contains(got, "file_server @publicAsset") {
		t.Errorf("no matched file_server for public assets:\n%s", got)
	}
	// The proxy must still be present and unconditional — it is the fallthrough
	// for everything that is not a real file.
	if !strings.Contains(got, "reverse_proxy localhost:3000") {
		t.Errorf("reverse_proxy missing:\n%s", got)
	}
}

func TestPublicMatcherCannotShadowRoutes(t *testing.T) {
	got := generate(t, "standalone")

	// try_files {path} pins the matcher to an exact hit. Without it Caddy
	// resolves "/" to public/index.html, which Next would never serve.
	if !strings.Contains(got, "try_files {path}") {
		t.Errorf("matcher is not pinned to an exact path — / could resolve to public/index.html:\n%s", got)
	}
	// Trailing-slash requests are route-shaped, never asset-shaped.
	if !strings.Contains(got, "not path */") {
		t.Errorf("trailing-slash requests are not excluded from the file matcher:\n%s", got)
	}
	if strings.Contains(got, "\n\t\tfile_server\n\t\treverse_proxy") {
		t.Error("an unmatched file_server precedes reverse_proxy — it would shadow all routes")
	}
}

func TestPublicHandlerIsWrappedInRoute(t *testing.T) {
	// Caddy executes directives in ITS OWN fixed order, not written order, and
	// reverse_proxy sorts BEFORE file_server. Written bare, the terminal
	// reverse_proxy answers every request and file_server never runs — the
	// handler silently degrades to a no-op that looks correct in the config and
	// in `caddy validate`. route{} preserves written order.
	//
	// This is the single easiest way to regress this feature, and it fails
	// invisibly, so it gets its own test.
	got := generate(t, "standalone")

	routeIdx := strings.Index(got, "route {")
	if routeIdx < 0 {
		t.Fatalf("public handler is not wrapped in route{} — reverse_proxy would shadow file_server:\n%s", got)
	}
	fsIdx := strings.Index(got, "file_server @publicAsset")
	rpIdx := strings.Index(got, "reverse_proxy localhost:")
	if !(routeIdx < fsIdx && fsIdx < rpIdx) {
		t.Errorf("expected route{ file_server @publicAsset ... reverse_proxy }, got:\n%s", got)
	}
}

func TestPublicAssetsAreNotCachedImmutably(t *testing.T) {
	// public/ files are not content-hashed. An immutable max-age would pin a
	// stale favicon or OG image past a deploy with no way to purge it.
	got := generate(t, "standalone")

	idx := strings.Index(got, "@publicAsset")
	if idx < 0 {
		t.Fatal("no public asset matcher")
	}
	tail := got[idx:]
	if strings.Contains(tail, "immutable") {
		t.Errorf("public assets are marked immutable:\n%s", tail)
	}
	if !strings.Contains(tail, `header @publicAsset Cache-Control "public, max-age=0, must-revalidate"`) {
		t.Errorf("public assets do not carry a revalidating Cache-Control:\n%s", tail)
	}
}

func TestHashedStaticAssetsStayImmutableAndShared(t *testing.T) {
	// /_next/static/* IS content-hashed, and is served from shared_static so
	// in-flight clients holding old hashed URLs don't 404 across a cutover.
	got := generate(t, "standalone")

	if !strings.Contains(got, "handle_path /_next/static/*") {
		t.Errorf("hashed asset handler missing:\n%s", got)
	}
	if !strings.Contains(got, "root * /opt/nextdeploy/apps/demo/shared_static") {
		t.Errorf("hashed assets are not served from shared_static:\n%s", got)
	}
	if !strings.Contains(got, `Cache-Control "public, max-age=31536000, immutable"`) {
		t.Errorf("hashed assets lost their immutable caching:\n%s", got)
	}
}

func TestExportModeIsUnaffected(t *testing.T) {
	// A static export has no Node process to offload anything from.
	got := generate(t, "export")

	if strings.Contains(got, "reverse_proxy") {
		t.Errorf("export mode should not proxy:\n%s", got)
	}
	if strings.Contains(got, "@publicAsset") {
		t.Errorf("export mode should not carry the public/ matcher:\n%s", got)
	}
	if !strings.Contains(got, "root * "+testAppDir+"/out") {
		t.Errorf("export mode is not rooted at the export dir:\n%s", got)
	}
}

func TestDomainListCoversApexAndWWW(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"example.com", "example.com, www.example.com"},
		{"www.example.com", "www.example.com, example.com"},
		{"localhost", "localhost"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := GenerateCaddyfile("demo", c.in, "standalone", 3000, testAppDir, nil, ".next", "out")
			if !strings.HasPrefix(got, c.want+" {") {
				t.Errorf("site address = %q, want prefix %q", strings.SplitN(got, "\n", 2)[0], c.want)
			}
		})
	}
}

func TestSchemeAndTrailingSlashAreStrippedFromTheDomain(t *testing.T) {
	// A domain pasted from a browser bar must not become a broken site address.
	for _, in := range []string{"https://example.com", "http://example.com", "example.com/"} {
		got := GenerateCaddyfile("demo", in, "standalone", 3000, testAppDir, nil, ".next", "out")
		if !strings.HasPrefix(got, "example.com, www.example.com {") {
			t.Errorf("domain %q produced site address %q", in, strings.SplitN(got, "\n", 2)[0])
		}
	}
}
