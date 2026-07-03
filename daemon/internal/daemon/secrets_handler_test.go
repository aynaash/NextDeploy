package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withTempSecretsDir points the package-level secrets store at a temp dir for
// the duration of one test. renderEnvFile/loadSecrets/saveSecrets all read it.
func withTempSecretsDir(t *testing.T) string {
	t.Helper()
	orig := secretsDir
	secretsDir = t.TempDir()
	t.Cleanup(func() { secretsDir = orig })
	return secretsDir
}

func TestRenderEnvFileWritesSortedQuotedEnv(t *testing.T) {
	withTempSecretsDir(t)
	ch := &CommandHandler{}

	// Seed the store through the real writer so the test also covers the
	// save→load round-trip.
	if err := ch.saveSecrets("myapp", map[string]string{
		"ZED":          "last",
		"DATABASE_URL": "postgres://u:p@localhost:5432/db",
		"TRICKY":       "he said \"hi\"\nline2\\end",
	}); err != nil {
		t.Fatalf("saveSecrets: %v", err)
	}

	releaseDir := t.TempDir()
	if err := ch.renderEnvFile("myapp", releaseDir, map[string]string{
		"DOPPLER_TOKEN": "dp.st.abc",
		"SKIPPED":       "", // empty extras must not be written
	}); err != nil {
		t.Fatalf("renderEnvFile: %v", err)
	}

	envPath := filepath.Join(releaseDir, ".env.nextdeploy")
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read env file: %v", err)
	}

	want := `DATABASE_URL="postgres://u:p@localhost:5432/db"
DOPPLER_TOKEN="dp.st.abc"
TRICKY="he said \"hi\"\nline2\\end"
ZED="last"
`
	if got := string(data); got != want {
		t.Errorf("env file content mismatch\n got: %q\nwant: %q", got, want)
	}
	if strings.Contains(string(data), "SKIPPED") {
		t.Error("empty extra value was written; renderEnvFile must skip it")
	}

	info, err := os.Stat(envPath)
	if err != nil {
		t.Fatalf("stat env file: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("env file mode = %o, want 0600 (secrets must not be world-readable)", perm)
	}
}

func TestRenderEnvFileEmptyStoreWritesNothing(t *testing.T) {
	withTempSecretsDir(t) // empty store: no <app>.json exists
	ch := &CommandHandler{}

	releaseDir := t.TempDir()
	if err := ch.renderEnvFile("myapp", releaseDir, nil); err != nil {
		t.Fatalf("renderEnvFile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(releaseDir, ".env.nextdeploy")); !os.IsNotExist(err) {
		t.Error("env file was created for an empty secret store; EnvironmentFile=- expects absence")
	}
}

func TestRenderEnvFileEmptyExtraOnlyWritesNothing(t *testing.T) {
	withTempSecretsDir(t)
	ch := &CommandHandler{}

	releaseDir := t.TempDir()
	// Empty store + extras whose values are all empty (the no-Doppler ship
	// path passes DOPPLER_TOKEN:"") → still no file.
	if err := ch.renderEnvFile("myapp", releaseDir, map[string]string{"DOPPLER_TOKEN": ""}); err != nil {
		t.Fatalf("renderEnvFile: %v", err)
	}
	if _, err := os.Stat(filepath.Join(releaseDir, ".env.nextdeploy")); !os.IsNotExist(err) {
		t.Error("env file was created from empty-valued extras only")
	}
}

func TestRenderEnvFileCorruptStoreFails(t *testing.T) {
	dir := withTempSecretsDir(t)
	ch := &CommandHandler{}

	if err := os.WriteFile(filepath.Join(dir, "myapp.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatalf("seed corrupt store: %v", err)
	}
	if err := ch.renderEnvFile("myapp", t.TempDir(), nil); err == nil {
		t.Error("renderEnvFile succeeded on a corrupt store; activation must fail rather than start secretless")
	}
}

func TestSecretsSaveLoadRoundTrip(t *testing.T) {
	withTempSecretsDir(t)
	ch := &CommandHandler{}

	in := map[string]string{"A": "1", "B": "two words", "C": ""}
	if err := ch.saveSecrets("app-1", in); err != nil {
		t.Fatalf("saveSecrets: %v", err)
	}
	out, err := ch.loadSecrets("app-1")
	if err != nil {
		t.Fatalf("loadSecrets: %v", err)
	}
	if len(out) != len(in) {
		t.Fatalf("round-trip size = %d, want %d", len(out), len(in))
	}
	for k, v := range in {
		if out[k] != v {
			t.Errorf("round-trip %s = %q, want %q", k, out[k], v)
		}
	}

	// Unknown app → empty map, not an error (first deploy has no store yet).
	empty, err := ch.loadSecrets("never-seen")
	if err != nil {
		t.Fatalf("loadSecrets(unknown): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("loadSecrets(unknown) = %v, want empty map", empty)
	}
}

func TestQuoteEnvValueSystemdEscaping(t *testing.T) {
	cases := []struct{ in, want string }{
		{`plain`, `"plain"`},
		{``, `""`},
		{`has "quotes"`, `"has \"quotes\""`},
		{"multi\nline", `"multi\nline"`},
		{`back\slash`, `"back\\slash"`},
		{"cr\rhere", `"cr\rhere"`},
	}
	for _, c := range cases {
		if got := quoteEnvValue(c.in); got != c.want {
			t.Errorf("quoteEnvValue(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}
