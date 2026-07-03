package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aynaash/nextdeploy/daemon/internal/types"
)

const (
	errMissingKey  = "missing 'key' argument"
	errLoadSecrets = "failed to load secrets: %v"
)

// secretsDir is a var (not const) so tests can point the store at a temp dir.
// #nosec G101
var secretsDir = "/opt/nextdeploy/secrets"

func (ch *CommandHandler) handleSecrets(args map[string]any) types.Response {
	action, ok := StringArg(args, "action")
	if !ok {
		return types.Response{Success: false, Message: "missing 'action' argument"}
	}

	appName, ok := StringArg(args, "appName")
	if !ok {
		return types.Response{Success: false, Message: "missing 'appName' argument"}
	}
	if err := validateAppName(appName); err != nil {
		return types.Response{Success: false, Message: err.Error()}
	}

	switch action {
	case "set":
		return ch.setSecret(appName, args)
	case "get":
		return ch.getSecret(appName, args)
	case "unset":
		return ch.unsetSecret(appName, args)
	case "list":
		return ch.listSecrets(appName)
	default:
		return types.Response{Success: false, Message: fmt.Sprintf("unknown secrets action: %s", action)}
	}
}

func (ch *CommandHandler) renderEnvFile(appName, dir string, extra map[string]string) error {
	secrets, err := ch.loadSecrets(appName)
	if err != nil {
		return fmt.Errorf("load secrets for %s: %w", appName, err)
	}
	for k, v := range extra {
		if v != "" {
			secrets[k] = v
		}
	}
	if len(secrets) == 0 {
		return nil
	}

	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, quoteEnvValue(secrets[k]))
	}
	envPath := filepath.Join(dir, ".env.nextdeploy")
	if err := os.WriteFile(envPath, []byte(b.String()), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", envPath, err)
	}
	_ = exec.Command(resolveTool("chown"), "nextdeploy:nextdeploy", envPath).Run()
	return nil
}

func (ch *CommandHandler) syncAppSecrets(appName string) error {
	currentLink := filepath.Join(appsDir, appName, "current")
	if _, err := os.Stat(currentLink); os.IsNotExist(err) {
		return nil
	}
	releaseDir, err := filepath.EvalSymlinks(currentLink)
	if err != nil {
		releaseDir = currentLink
	}
	if err := ch.renderEnvFile(appName, releaseDir, nil); err != nil {
		return err
	}
	// Units are named nextdeploy-{app}-{releaseID}.service — resolve via
	// findActiveService instead of restarting the bare app name.
	serviceName, err := ch.findActiveService(appName)
	if err != nil {
		return fmt.Errorf("no active service for %s: %w", appName, err)
	}
	log.Printf("[secrets] Updated %s/.env.nextdeploy, restarting %s...", releaseDir, serviceName)
	return ch.processManager.RestartService(serviceName)
}

func (ch *CommandHandler) setSecret(appName string, args map[string]any) types.Response {
	key, ok := StringArg(args, "key")
	if !ok {
		return types.Response{Success: false, Message: errMissingKey}
	}
	value, ok := StringArg(args, "value")
	if !ok {
		return types.Response{Success: false, Message: "missing 'value' argument"}
	}

	secrets, err := ch.loadSecrets(appName)
	if err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf(errLoadSecrets, err)}
	}

	secrets[key] = value

	if err := ch.saveSecrets(appName, secrets); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("failed to save secrets: %v", err)}
	}

	if err := ch.syncAppSecrets(appName); err != nil {
		return types.Response{Success: true, Message: fmt.Sprintf("secret set but sync failed: %v", err)}
	}

	return types.Response{Success: true, Message: fmt.Sprintf("secret %s set successfully", key)}
}

func (ch *CommandHandler) getSecret(appName string, args map[string]any) types.Response {
	key, ok := StringArg(args, "key")
	if !ok {
		return types.Response{Success: false, Message: errMissingKey}
	}

	secrets, err := ch.loadSecrets(appName)
	if err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf(errLoadSecrets, err)}
	}

	value, exists := secrets[key]
	if !exists {
		return types.Response{Success: false, Message: fmt.Sprintf("secret %s not found", key)}
	}

	return types.Response{Success: true, Message: value}
}

func (ch *CommandHandler) unsetSecret(appName string, args map[string]any) types.Response {
	key, ok := StringArg(args, "key")
	if !ok {
		return types.Response{Success: false, Message: errMissingKey}
	}

	secrets, err := ch.loadSecrets(appName)
	if err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf(errLoadSecrets, err)}
	}

	if _, exists := secrets[key]; !exists {
		return types.Response{Success: false, Message: fmt.Sprintf("secret %s not found", key)}
	}

	delete(secrets, key)

	if err := ch.saveSecrets(appName, secrets); err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf("failed to save secrets: %v", err)}
	}

	// Hot-reload application environment
	if err := ch.syncAppSecrets(appName); err != nil {
		return types.Response{Success: true, Message: fmt.Sprintf("secret unset but sync failed: %v", err)}
	}

	return types.Response{Success: true, Message: fmt.Sprintf("secret %s unset successfully", key)}
}

func (ch *CommandHandler) listSecrets(appName string) types.Response {
	secrets, err := ch.loadSecrets(appName)
	if err != nil {
		return types.Response{Success: false, Message: fmt.Sprintf(errLoadSecrets, err)}
	}

	keys := make([]string, 0, len(secrets))
	for k := range secrets {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return types.Response{
		Success: true,
		Message: strings.Join(keys, "\n"),
	}
}

func (ch *CommandHandler) loadSecrets(appName string) (map[string]string, error) {
	path := filepath.Join(secretsDir, fmt.Sprintf("%s.json", appName))
	// #nosec G304
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]string), nil
		}
		return nil, err
	}

	var secrets map[string]string
	if err := json.Unmarshal(data, &secrets); err != nil {
		return nil, err
	}

	return secrets, nil
}

func (ch *CommandHandler) saveSecrets(appName string, secrets map[string]string) error {
	if err := os.MkdirAll(secretsDir, 0o700); err != nil {
		return err
	}

	path := filepath.Join(secretsDir, fmt.Sprintf("%s.json", appName))
	data, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o600)
}

// quoteEnvValue wraps a secret value so that it is safely consumed by
// systemd's EnvironmentFile= parser. systemd recognizes the escapes \\, \",
// \n and \r inside double-quoted values, so we escape those four characters
// and wrap the result in double quotes.
func quoteEnvValue(v string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
	)
	return `"` + r.Replace(v) + `"`
}
