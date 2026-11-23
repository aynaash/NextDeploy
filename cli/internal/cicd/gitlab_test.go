package cicd

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRenderGitLabCIWorkflow_IsValidYAML(t *testing.T) {
	yml := RenderGitLabCIWorkflow()
	var doc map[string]any
	if err := yaml.Unmarshal([]byte(yml), &doc); err != nil {
		t.Fatalf("rendered GitLab CI workflow is not valid YAML: %v", err)
	}

	stages, ok := doc["stages"].([]any)
	if !ok || len(stages) == 0 {
		t.Fatalf("stages missing or empty in GitLab CI workflow")
	}

	deploy, ok := doc["deploy"].(map[string]any)
	if !ok {
		t.Fatalf("deploy job missing in GitLab CI workflow")
	}

	if deploy["stage"] != "deploy" {
		t.Errorf("deploy job stage = %v, want deploy", deploy["stage"])
	}
}

func TestRenderGitLabCIWorkflow_HardeningAndDopplerFirst(t *testing.T) {
	yml := RenderGitLabCIWorkflow()

	for _, want := range []string{
		`CLI_VERSION: "` + CLIVersion + `"`,
		"sha256sum --check --status",
		"nextdeploy_${VERSION}_linux_amd64.tar.gz",
		"nextdeploy target",
		"NEXTDEPLOY_TARGET_TYPE",
		"NEXTDEPLOY_DEPLOYMENT_TYPE",
		"cloudflare",
		"aws",
		"vps",
		"DOPPLER_TOKEN",
		"cli.doppler.com/install.sh",
		"doppler run --",
		"SSH_PRIVATE_KEY",
		"SSH_KNOWN_HOSTS",
		"chmod 700 ~/.ssh",
		"chmod 600 ~/.ssh/id_ed25519",
		"chmod 600 ~/.ssh/id_rsa",
		"chmod 600 ~/.ssh/known_hosts",
		"nextdeploy apply --yes",
		"nextdeploy ship --no-provision --verify",
		"nextdeploy ship --verify",
	} {
		if !strings.Contains(yml, want) {
			t.Errorf("GitLab CI workflow missing %q", want)
		}
	}
}

func TestRenderGitLabCIWorkflow_WithKnownTarget(t *testing.T) {
	cfYml := RenderGitLabCIWorkflow("cloudflare")
	if !strings.Contains(cfYml, `NEXTDEPLOY_TARGET_TYPE: "cloudflare"`) {
		t.Errorf("expected GitLab CI workflow to embed NEXTDEPLOY_TARGET_TYPE: \"cloudflare\"")
	}

	vpsYml := RenderGitLabCIWorkflow("vps")
	if !strings.Contains(vpsYml, `NEXTDEPLOY_TARGET_TYPE: "vps"`) {
		t.Errorf("expected GitLab CI workflow to embed NEXTDEPLOY_TARGET_TYPE: \"vps\"")
	}
}
