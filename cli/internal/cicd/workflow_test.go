package cicd

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRenderDeployWorkflow_TargetAware(t *testing.T) {
	yml := RenderDeployWorkflow()

	for _, want := range []string{
		"id: detect",
		"steps.detect.outputs.type == 'cloudflare'",
		"steps.detect.outputs.type == 'aws'",
		"steps.detect.outputs.type == 'vps'",
		"go install github.com/aynaash/nextdeploy/cli@" + CLIVersion,
		"nextdeploy ship --no-provision --verify", // cloudflare
		"nextdeploy ship --verify",                // aws / vps
		"ln -sf",                                  // `cli` binary → `nextdeploy` on PATH
		"cancel-in-progress: false",
	} {
		if !strings.Contains(yml, want) {
			t.Errorf("workflow missing %q", want)
		}
	}

	if strings.Contains(yml, "@latest") {
		t.Error("CLI must be pinned, not @latest — @latest recompiles every run and picks up breaking releases")
	}
	// Each target's creds must be present so the matching branch can run.
	for _, cred := range []string{"CLOUDFLARE_API_TOKEN", "AWS_ACCESS_KEY_ID", "SSH_PRIVATE_KEY"} {
		if !strings.Contains(yml, cred) {
			t.Errorf("workflow missing %s branch creds", cred)
		}
	}
}

// The workflow lives inside a Go raw string, where a stray edit can break YAML
// indentation without breaking the Go build. Parse it to catch that.
func TestRenderDeployWorkflow_IsValidYAML(t *testing.T) {
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(RenderDeployWorkflow()), &doc); err != nil {
		t.Fatalf("rendered workflow is not valid YAML: %v", err)
	}
	if doc["name"] != "deploy" {
		t.Errorf("name = %v, want deploy", doc["name"])
	}
	jobs, ok := doc["jobs"].(map[string]any)
	if !ok {
		t.Fatalf("jobs block missing or wrong shape: %T", doc["jobs"])
	}
	deploy, ok := jobs["deploy"].(map[string]any)
	if !ok {
		t.Fatalf("deploy job missing: %v", jobs)
	}
	steps, ok := deploy["steps"].([]any)
	if !ok {
		t.Fatalf("steps missing: %T", deploy["steps"])
	}
	// checkout, setup-go, setup-node, deps, CLI, detect, + 3 deploy branches.
	if len(steps) != 9 {
		t.Errorf("want 9 steps, got %d", len(steps))
	}
	if deploy["timeout-minutes"] != 30 {
		t.Errorf("timeout-minutes = %v, want 30", deploy["timeout-minutes"])
	}
}

// App secrets belong in nextdeploy.yml so `secenv` folds them into every
// deploy, not just CI ones. The old scaffold template wired AUTH_SECRET here.
func TestRenderDeployWorkflow_CarriesNoAppSecrets(t *testing.T) {
	yml := RenderDeployWorkflow()
	for _, leaked := range []string{"AUTH_SECRET", "DATABASE_URL", "DOPPLER_TOKEN"} {
		if strings.Contains(yml, leaked) {
			t.Errorf("workflow wires app secret %s — it belongs in nextdeploy.yml", leaked)
		}
	}
}

// `apply` reconciles D1/KV/R2/Vectorize and is Cloudflare-only; AWS provisions
// inside ship, VPS ships over SSH.
func TestRenderDeployWorkflow_ApplyIsCloudflareOnly(t *testing.T) {
	yml := RenderDeployWorkflow()
	if n := strings.Count(yml, "nextdeploy apply"); n != 1 {
		t.Errorf("want exactly one `nextdeploy apply` (the cloudflare branch), got %d", n)
	}
	cf := yml[strings.Index(yml, "Deploy to Cloudflare"):strings.Index(yml, "Deploy to AWS")]
	if !strings.Contains(cf, "nextdeploy apply") {
		t.Error("`nextdeploy apply` is not in the cloudflare branch")
	}
}

func TestRenderPreviewWorkflows(t *testing.T) {
	preview := RenderPreviewWorkflow()
	cleanup := RenderPreviewCleanupWorkflow()

	for _, want := range []string{
		"pull_request:",
		"types: [opened, synchronize, reopened]",
		`--environment "$ENV_NAME"`,
		"NEXTDEPLOY_WORKER_NAME=",
		"CLOUDFLARE_WORKERS_SUBDOMAIN",
		"<!-- nextdeploy-preview -->", // idempotent comment marker
		"updateComment",
		"pull-requests: write",
		"cancel-in-progress: true",
	} {
		if !strings.Contains(preview, want) {
			t.Errorf("preview workflow missing %q", want)
		}
	}

	for _, want := range []string{
		"types: [closed]",
		`nextdeploy destroy --environment "pr-${{ github.event.number }}" --yes --force`,
	} {
		if !strings.Contains(cleanup, want) {
			t.Errorf("cleanup workflow missing %q", want)
		}
	}

	// Both must pin the CLI, like the main workflow.
	for name, yml := range map[string]string{"preview": preview, "cleanup": cleanup} {
		if !strings.Contains(yml, "cli@"+CLIVersion) {
			t.Errorf("%s workflow does not pin the CLI to %s", name, CLIVersion)
		}
		if strings.Contains(yml, "cli@latest") {
			t.Errorf("%s workflow uses @latest", name)
		}
		// go install yields a binary named `cli`; without the symlink every
		// `nextdeploy …` line is "command not found".
		if !strings.Contains(yml, "ln -sf") {
			t.Errorf("%s workflow missing the cli→nextdeploy symlink", name)
		}
	}
}

func TestRenderPreviewWorkflows_AreValidYAML(t *testing.T) {
	for name, yml := range map[string]string{
		"preview": RenderPreviewWorkflow(),
		"cleanup": RenderPreviewCleanupWorkflow(),
	} {
		var doc map[string]any
		if err := yaml.Unmarshal([]byte(yml), &doc); err != nil {
			t.Errorf("%s workflow is not valid YAML: %v", name, err)
		}
	}
}

// A preview must never be able to touch production. The teardown is scoped by
// the same environment mechanism that creates it.
func TestPreviewCleanup_IsEnvironmentScoped(t *testing.T) {
	cleanup := RenderPreviewCleanupWorkflow()
	if strings.Contains(cleanup, "nextdeploy destroy --yes") {
		t.Error("cleanup runs an unscoped destroy — that would target the configured environment")
	}
	if !strings.Contains(cleanup, "--environment") {
		t.Error("cleanup destroy is not environment-scoped")
	}
}
