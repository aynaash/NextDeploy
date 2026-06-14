package serverless

import (
	"os"
	"path/filepath"

	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/secenv"
)

// secretsPreflight gathers secret-hygiene signals from the working directory +
// config and returns non-fatal warnings (committable .env, plaintext secrets,
// no managed store). Thin adapter over the unit-tested secenv.Preflight.
func secretsPreflight(cfg *config.NextDeployConfig) []string {
	return secretsPreflightDir(cfg, ".")
}

func secretsPreflightDir(cfg *config.NextDeployConfig, dir string) []string {
	in := secenv.PreflightInput{
		UsingDoppler:     os.Getenv("DOPPLER_TOKEN") != "" || os.Getenv("DOPPLER_PROJECT") != "",
		UsingSecretStore: hasSecretsStoreBinding(cfg),
		PlainText:        plainTextValues(cfg),
	}
	if data, err := os.ReadFile(filepath.Join(dir, ".gitignore")); err == nil { // #nosec G304
		in.Gitignore = string(data)
	}
	if _, err := os.Stat(filepath.Join(dir, ".env")); err == nil {
		in.HasEnvFile = true
	}
	return secenv.Preflight(in)
}

func hasSecretsStoreBinding(cfg *config.NextDeployConfig) bool {
	if cfg == nil || cfg.Serverless == nil || cfg.Serverless.Cloudflare == nil || cfg.Serverless.Cloudflare.Bindings == nil {
		return false
	}
	return len(cfg.Serverless.Cloudflare.Bindings.SecretsStore) > 0
}

func plainTextValues(cfg *config.NextDeployConfig) map[string]string {
	out := map[string]string{}
	if cfg == nil || cfg.Serverless == nil || cfg.Serverless.Cloudflare == nil || cfg.Serverless.Cloudflare.Bindings == nil {
		return out
	}
	for _, pt := range cfg.Serverless.Cloudflare.Bindings.PlainText {
		out[pt.Name] = pt.Value
	}
	return out
}
