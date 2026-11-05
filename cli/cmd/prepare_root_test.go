package cmd

import (
	"os"
	"strings"
	"testing"
)

func TestRootCredentialBlocked(t *testing.T) {
	blocked, reason := rootCredentialBlocked("root", false)
	if !blocked {
		t.Fatal("root login without --allow-root must be blocked")
	}
	if !strings.Contains(reason, "sudo-capable") || !strings.Contains(reason, "--allow-root") {
		t.Errorf("reason should guide the user: %q", reason)
	}

	if blocked, _ := rootCredentialBlocked("root", true); blocked {
		t.Error("--allow-root must override the root block")
	}

	if blocked, _ := rootCredentialBlocked("  root  ", false); !blocked {
		t.Error("username trimming should catch a root login with whitespace")
	}

	for _, u := range []string{"ubuntu", "deploy", "ec2-user", ""} {
		if blocked, _ := rootCredentialBlocked(u, false); blocked {
			t.Errorf("non-root user %q should not be blocked", u)
		}
	}
}

func TestResolveTargetServersRespectsServerSelection(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	if err := os.WriteFile(tmp+"/nextdeploy.yml", []byte(`version: "1"
app:
  name: demo
  port: 3000
repository:
  url: https://example.com/repo.git
  branch: main
servers:
  - name: prod
    host: prod.example.com
    username: deploy
  - name: staging
    host: staging.example.com
    username: deploy
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldDir)

	oldAll := prepareAllServers
	oldServerName := prepareServerName
	defer func() {
		prepareAllServers = oldAll
		prepareServerName = oldServerName
	}()

	prepareAllServers = false
	prepareServerName = "staging"
	targets, err := resolveTargetServers()
	if err != nil {
		t.Fatalf("selecting a named server should work: %v", err)
	}
	if len(targets) != 1 || targets[0].Name != "staging" {
		t.Fatalf("selected target = %#v, want one staging server", targets)
	}

	prepareServerName = ""
	prepareAllServers = true
	if targets, err = resolveTargetServers(); err != nil {
		t.Fatalf("all servers should resolve without error: %v", err)
	} else if len(targets) != 2 {
		t.Fatalf("all targets = %d, want 2", len(targets))
	}

	prepareServerName = "prod"
	prepareAllServers = true
	if _, err = resolveTargetServers(); err == nil {
		t.Fatal("--all and --server should be rejected together")
	}
}
