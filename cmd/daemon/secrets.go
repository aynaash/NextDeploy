package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func SyncSecrets(appName string, secrets map[string]string) error {
	//TODO: write secrets to file or environment
	//  Inject into container at runtime
	return nil
}

func InjectSecrets(app string, secrets map[string]string) error {
	// Write secrets to /etc/nextdeploy/apps/{app}/.env
	dir := fmt.Sprintf("/etc/nextdeploy/apps/%s", app)
	os.MkdirAll(dir, 0700)

	f, err := os.Create(filepath.Join(dir, ".env"))
	if err != nil {
		return err
	}
	defer f.Close()

	for k, v := range secrets {
		f.WriteString(fmt.Sprintf("%s=%s\n", k, v))
	}
	return nil
}

func ConvertSecretsToEnvVars(secrets map[string]string) []string {
	env := []string{}
	for k, v := range secrets {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}
