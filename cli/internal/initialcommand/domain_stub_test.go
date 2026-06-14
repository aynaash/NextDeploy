package initialcommand

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/aynaash/nextdeploy/shared/config"
)

func TestInsertDomainStub_AddsAfterName(t *testing.T) {
	in := `# header comment
app:
  name: NextDeploy

serverless:
  provider: cloudflare
`
	out, added := insertDomainStub(in)
	if !added {
		t.Fatal("expected stub to be added")
	}
	// The active domain field lands right after the name line.
	if !strings.Contains(out, "  name: NextDeploy\n  domain:") {
		t.Errorf("domain field not placed after name line:\n%s", out)
	}
	// Existing content preserved.
	for _, frag := range []string{"# header comment", "serverless:", "provider: cloudflare"} {
		if !strings.Contains(out, frag) {
			t.Errorf("lost existing content %q", frag)
		}
	}
	// Still valid YAML, and the empty default leaves domain unset.
	var c config.NextDeployConfig
	if err := yaml.Unmarshal([]byte(out), &c); err != nil {
		t.Fatalf("result is not valid YAML: %v\n%s", err, out)
	}
	if c.App.Domain.Name != "" {
		t.Errorf("default stub should leave domain empty, got %q", c.App.Domain.Name)
	}
}

func TestInsertDomainStub_NoopWhenDomainPresent(t *testing.T) {
	for _, in := range []string{
		"app:\n  name: web\n  domain: example.com\n",
		"app:\n  name: web\n  domain:\n    name: example.com\n",
	} {
		if out, added := insertDomainStub(in); added || out != in {
			t.Errorf("should be a no-op when domain already set:\n%s", in)
		}
	}
}

func TestInsertDomainStub_NoopWithoutAnchor(t *testing.T) {
	in := "serverless:\n  provider: cloudflare\n"
	if out, added := insertDomainStub(in); added || out != in {
		t.Error("should be a no-op when there is no app/name anchor")
	}
}
