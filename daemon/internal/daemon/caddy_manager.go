package daemon

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aynaash/nextdeploy/shared/caddy"
	"github.com/aynaash/nextdeploy/shared/nextcore"
)

const mainCaddyfilePath = "/etc/caddy/Caddyfile"

// appNamePattern bounds what can become a fragment filename. appName flows into
// filepath.Join(configDir, appName+".caddy"); without this guard a value like
// "../../etc/cron.d/x" would let a fragment escape the import directory.
var appNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func sanitizeAppName(appName string) error {
	if !appNamePattern.MatchString(appName) {
		return fmt.Errorf("invalid app name %q: only letters, digits, '-' and '_' are allowed", appName)
	}
	return nil
}

type CaddyManager struct {
	configDir string
}

func NewCaddyManager() *CaddyManager {
	dir := "/etc/caddy/nextdeploy.d"
	// #nosec G:u301
	_ = os.MkdirAll(dir, 0755)
	_ = os.Chmod(dir, 0755)
	return &CaddyManager{
		configDir: dir,
	}
}

func (cm *CaddyManager) GenerateConfig(appName, domain, outputMode string, port int, appDir string, features *nextcore.DetectedFeatures, distDir, exportDir string) error {
	if err := sanitizeAppName(appName); err != nil {
		return err
	}
	caddyConfig := caddy.GenerateCaddyfile(appName, domain, outputMode, port, appDir, features, distDir, exportDir)
	if err := cm.commitFragmentSafely(appName, []byte(caddyConfig)); err != nil {
		return err
	}
	log.Printf("Caddy config generated for %s at %s", appName, filepath.Join(cm.configDir, appName+".caddy"))
	return nil
}

// commitFragmentSafely validates the *resulting* Caddy configuration in a
// sandbox before the fragment is allowed to touch the live import directory.
// Previously a malformed fragment was written straight into configDir and only
// caught by the post-write Validate() — by which point the broken file was
// already live and would also poison the next reload of every other app. Here:
//
//  1. mirror the current live fragments into a temp dir,
//  2. drop in the candidate fragment (replacing this app's, if any),
//  3. validate a candidate main Caddyfile whose import points at the temp dir,
//  4. only on success, atomically rename the fragment into the live dir.
func (cm *CaddyManager) commitFragmentSafely(appName string, content []byte) error {
	sandbox, err := os.MkdirTemp("", "nextdeploy-caddy-*")
	if err != nil {
		return fmt.Errorf("failed to provision caddy validation sandbox: %w", err)
	}
	defer os.RemoveAll(sandbox)

	// Mirror existing live fragments so cross-fragment references and the full
	// import set are validated as a whole, not the candidate in isolation.
	if entries, err := os.ReadDir(cm.configDir); err == nil {
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".caddy") {
				continue
			}
			data, rerr := os.ReadFile(filepath.Join(cm.configDir, e.Name()))
			if rerr != nil {
				continue
			}
			// #nosec G306
			_ = os.WriteFile(filepath.Join(sandbox, e.Name()), data, 0644)
		}
	}

	// Drop in (or overwrite) the candidate fragment.
	// #nosec G306
	if err := os.WriteFile(filepath.Join(sandbox, appName+".caddy"), content, 0644); err != nil {
		return fmt.Errorf("failed to stage candidate caddy fragment: %w", err)
	}

	// Build a candidate main Caddyfile that imports the sandbox instead of the
	// live dir, reusing the real global options (e.g. coraza ordering) so the
	// validation matches what reload will actually parse.
	candidateMain := filepath.Join(sandbox, "Caddyfile")
	var mainContent string
	if existing, err := os.ReadFile(mainCaddyfilePath); err == nil {
		mainContent = strings.ReplaceAll(string(existing), cm.configDir, sandbox)
	} else {
		mainContent = fmt.Sprintf("import %s/*.caddy\n", sandbox)
	}
	// #nosec G306
	if err := os.WriteFile(candidateMain, []byte(mainContent), 0644); err != nil {
		return fmt.Errorf("failed to stage candidate Caddyfile: %w", err)
	}

	caddyPath := resolveTool("caddy")
	// #nosec G204
	cmd := exec.Command(caddyPath, "validate", "--config", candidateMain, "--adapter", "caddyfile")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("caddy pre-flight validation failed for %s (fragment not committed): %v - %s", appName, err, string(out))
	}

	// Validation passed — atomically install the fragment. CreateTemp in the
	// live dir keeps the temp file on the same filesystem so the rename is a
	// true atomic swap (no half-written file is ever visible to a reload).
	finalPath := filepath.Join(cm.configDir, appName+".caddy")
	tmp, err := os.CreateTemp(cm.configDir, "."+appName+".caddy.*")
	if err != nil {
		return fmt.Errorf("failed to create temp caddy fragment: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp caddy fragment: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp caddy fragment: %w", err)
	}
	// #nosec G302
	_ = os.Chmod(tmpPath, 0644)
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to commit validated caddy fragment for %s: %w", appName, err)
	}
	return nil
}

func (cm *CaddyManager) EnsureMainCaddyfile() error {
	importDirective := fmt.Sprintf("import %s/*.caddy\n", cm.configDir)
	corazaGlobal := "{\n\torder coraza_waf first\n}\n\n"

	content, err := os.ReadFile(mainCaddyfilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return os.WriteFile(mainCaddyfilePath, []byte(corazaGlobal+importDirective), 0600)
		}
		return fmt.Errorf("failed to read main Caddyfile: %w", err)
	}

	contentStr := string(content)

	// Surgically remove the default :80 block if it exists (Welcome to Caddy page)
	if strings.Contains(contentStr, ":80 {") && strings.Contains(contentStr, "root * /usr/share/caddy") {
		// This is a naive but effective way to strip the default block for common Caddy installs
		lines := strings.Split(contentStr, "\n")
		var newLines []string
		inDefaultBlock := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == ":80 {" {
				inDefaultBlock = true
				continue
			}
			if inDefaultBlock {
				if trimmed == "}" {
					inDefaultBlock = false
				}
				continue
			}
			newLines = append(newLines, line)
		}
		contentStr = strings.Join(newLines, "\n")
	}

	newContent := contentStr

	// Ensure global options block is at the top
	if !strings.Contains(newContent, "order coraza_waf") {
		newContent = corazaGlobal + newContent
	}

	// Ensure import directive is present
	if !strings.Contains(newContent, importDirective) {
		if len(newContent) > 0 && newContent[len(newContent)-1] != '\n' {
			newContent += "\n"
		}
		newContent += importDirective
	}

	if newContent != string(content) {
		if err := os.WriteFile(mainCaddyfilePath, []byte(newContent), 0600); err != nil {
			return fmt.Errorf("failed to update main Caddyfile: %w", err)
		}
	}

	return nil
}

func (cm *CaddyManager) Reload() error {
	// Validate config before attempting reload to prevent broken configs
	if err := cm.Validate(); err != nil {
		return fmt.Errorf("config validation failed before reload: %w", err)
	}

	systemctl := resolveTool("systemctl")
	// #nosec G204
	cmd := exec.Command(systemctl, "reload", "caddy")
	output, err := cmd.CombinedOutput()
	if err == nil {
		log.Println("Caddy reloaded successfully via systemctl.")
		return nil
	}

	log.Printf("Warning: systemctl reload caddy failed (%v), falling back to direct caddy reload...", err)
	caddyPath := resolveTool("caddy")
	// #nosec G204
	fallbackCmd := exec.Command(caddyPath, "reload", "--config", mainCaddyfilePath, "--adapter", "caddyfile")
	output, err = fallbackCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("caddy reload failed (systemctl and fallback): %v - %s", err, string(output))
	}

	log.Println("Caddy reloaded successfully via direct fallback command.")
	return nil
}

func (cm *CaddyManager) Validate() error {
	caddyPath := resolveTool("caddy")
	// #nosec G204
	cmd := exec.Command(caddyPath, "validate", "--config", mainCaddyfilePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("caddy validation failed: %v - %s", err, string(output))
	}
	return nil
}

func (cm *CaddyManager) RemoveConfig(appName string) error {
	if err := sanitizeAppName(appName); err != nil {
		return err
	}
	configPath := filepath.Join(cm.configDir, fmt.Sprintf("%s.caddy", appName))
	if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove caddy config for %s: %w", appName, err)
	}
	return nil
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
