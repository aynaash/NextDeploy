package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/aynaash/nextdeploy/shared/credstore"
)

var (
	// Matches ${VAR} or ${VAR:-default}.
	bracedVarPattern = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)
	// Matches a whole-value $VAR placeholder (used only on selected fields so
	// passwords or arbitrary strings containing $ aren't accidentally expanded).
	bareVarPattern = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)$`)
)

// readDotEnvFile parses a dotenv file if it exists. Missing files return an
// empty map without error so callers can treat .env and .nextdeploy/.env as optional.
func readDotEnvFile(path string) map[string]string {
	out := make(map[string]string)
	// #nosec G304 -- path is strictly ".env" or ".nextdeploy/.env" under cwd.
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		if key == "" {
			continue
		}
		val := strings.TrimSpace(line[idx+1:])
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		} else {
			// Strip inline comments on unquoted values (e.g. VPS_HOST=203.0.113.10 # prod).
			for i := 0; i < len(val); i++ {
				if val[i] == '#' && (i == 0 || val[i-1] == ' ' || val[i-1] == '\t') {
					val = strings.TrimSpace(val[:i])
					break
				}
			}
		}
		out[key] = val
	}
	return out
}

// buildEnvLookup resolves variable names in strict precedence without mutating
// process env: process environment -> <cwd>/.nextdeploy/.env -> <cwd>/.env.
func buildEnvLookup(cwd string) func(string) (string, bool) {
	var once sync.Once
	var nextdeployEnv map[string]string
	var rootEnv map[string]string

	loadFiles := func() {
		once.Do(func() {
			nextdeployEnv = readDotEnvFile(filepath.Join(cwd, ".nextdeploy", ".env"))
			rootEnv = readDotEnvFile(filepath.Join(cwd, ".env"))
		})
	}

	return func(name string) (string, bool) {
		if v, ok := os.LookupEnv(name); ok && v != "" {
			return v, true
		}
		loadFiles()
		if v, ok := nextdeployEnv[name]; ok && v != "" {
			return v, true
		}
		if v, ok := rootEnv[name]; ok && v != "" {
			return v, true
		}
		return "", false
	}
}

// expandConfigVars substitutes ${VAR} and ${VAR:-default} (and full-string $VAR
// when allowBareDollar is true) using lookup. Unset variables without a default
// are preserved verbatim so downstream validation can report the exact missing name.
func expandConfigVars(raw string, allowBareDollar bool, lookup func(string) (string, bool)) string {
	if raw == "" {
		return raw
	}

	if allowBareDollar {
		trimmed := strings.TrimSpace(raw)
		if m := bareVarPattern.FindStringSubmatch(trimmed); len(m) == 2 {
			if val, ok := lookup(m[1]); ok && val != "" {
				return val
			}
			return raw
		}
	}

	matches := bracedVarPattern.FindAllStringSubmatchIndex(raw, -1)
	if len(matches) == 0 {
		return raw
	}

	var b strings.Builder
	last := 0
	for _, m := range matches {
		b.WriteString(raw[last:m[0]])
		varName := raw[m[2]:m[3]]
		hasDefault := m[4] != -1
		if val, ok := lookup(varName); ok && val != "" {
			b.WriteString(val)
		} else if hasDefault {
			b.WriteString(raw[m[4]:m[5]])
		} else {
			// Preserve unresolved ${VAR} so validation names the missing env var.
			b.WriteString(raw[m[0]:m[1]])
		}
		last = m[1]
	}
	b.WriteString(raw[last:])
	return b.String()
}

// ExtractUnresolvedVars returns the variable names referenced by any remaining
// ${VAR}, ${VAR:-...}, or whole-value $VAR placeholders in s.
func ExtractUnresolvedVars(s string) []string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}

	seen := make(map[string]struct{})
	var out []string
	add := func(name string) {
		if _, exists := seen[name]; exists {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}

	if m := bareVarPattern.FindStringSubmatch(trimmed); len(m) == 2 {
		add(m[1])
		return out
	}

	for _, m := range bracedVarPattern.FindAllStringSubmatch(s, -1) {
		if len(m) >= 2 {
			add(m[1])
		}
	}
	return out
}

// ServerCredstoreKey returns the per-server credstore key ("vps-<normalized>")
// for serverName, or "" if serverName normalizes to empty.
func ServerCredstoreKey(serverName string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(serverName)) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_':
			b.WriteRune(r)
			lastDash = false
		case !lastDash && b.Len() > 0:
			b.WriteByte('-')
			lastDash = true
		}
	}
	norm := strings.Trim(b.String(), "-")
	if norm == "" {
		return ""
	}
	return "vps-" + norm
}

func serverEnvSuffix(serverName string) string {
	var b strings.Builder
	lastUnderscore := false
	for _, r := range strings.ToUpper(strings.TrimSpace(serverName)) {
		switch {
		case (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastUnderscore = false
		case !lastUnderscore && b.Len() > 0:
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func hasGenericVPSHostPlaceholder(raw string) bool {
	for _, v := range ExtractUnresolvedVars(raw) {
		if v == "VPS_HOST" || v == "NEXTDEPLOY_VPS_HOST" {
			return true
		}
	}
	return false
}

func needsServerResolution(servers []ServerConfig) bool {
	for _, srv := range servers {
		if strings.TrimSpace(srv.Host) == "" ||
			strings.TrimSpace(srv.Username) == "" ||
			strings.TrimSpace(srv.KeyPath) == "" ||
			srv.KeyPassphrase == "" ||
			len(ExtractUnresolvedVars(srv.Host)) > 0 ||
			len(ExtractUnresolvedVars(srv.Username)) > 0 ||
			len(ExtractUnresolvedVars(srv.KeyPath)) > 0 ||
			len(ExtractUnresolvedVars(srv.KeyPassphrase)) > 0 ||
			len(ExtractUnresolvedVars(srv.Password)) > 0 {
			return true
		}
	}
	return false
}

// resolveServers expands env vars (process env -> .nextdeploy/.env -> .env) and
// resolves missing/placeholder VPS connection fields from the encrypted credstore
// ("vps-<server>" then "vps"). Never fails: unresolved placeholders are left
// intact so ValidateServers can report an actionable error when a VPS command runs.
func resolveServers(cfg *NextDeployConfig) {
	if cfg == nil || len(cfg.Servers) == 0 || !needsServerResolution(cfg.Servers) {
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}
	envLookup := buildEnvLookup(cwd)

	credCache := make(map[string]map[string]string)
	loadCred := func(key string) map[string]string {
		if key == "" {
			return nil
		}
		if cached, ok := credCache[key]; ok {
			return cached
		}
		data, err := credstore.Load(key)
		if err != nil {
			credCache[key] = nil
			return nil
		}
		credCache[key] = data
		return data
	}

	singleServer := len(cfg.Servers) == 1

	for i := range cfg.Servers {
		srv := &cfg.Servers[i]
		normKey := ServerCredstoreKey(srv.Name)
		suffix := serverEnvSuffix(srv.Name)

		// 1. Host
		srv.Host = expandConfigVars(srv.Host, true, envLookup)
		if strings.TrimSpace(srv.Host) == "" {
			if suffix != "" {
				if v, ok := envLookup("VPS_HOST_" + suffix); ok {
					srv.Host = v
				}
			}
			if strings.TrimSpace(srv.Host) == "" && singleServer {
				if v, ok := envLookup("VPS_HOST"); ok {
					srv.Host = v
				}
			}
		}
		if strings.TrimSpace(srv.Host) == "" || len(ExtractUnresolvedVars(srv.Host)) > 0 {
			if stored := loadCred(normKey); stored != nil && stored["host"] != "" {
				srv.Host = stored["host"]
			} else if singleServer || hasGenericVPSHostPlaceholder(srv.Host) {
				if stored := loadCred("vps"); stored != nil && stored["host"] != "" {
					srv.Host = stored["host"]
				}
			}
		}

		// 2. Username
		srv.Username = expandConfigVars(srv.Username, true, envLookup)
		if strings.TrimSpace(srv.Username) == "" {
			candidates := make([]string, 0, 4)
			if suffix != "" {
				candidates = append(candidates, "VPS_USERNAME_"+suffix, "VPS_USER_"+suffix)
			}
			candidates = append(candidates, "VPS_USERNAME", "VPS_USER")
			for _, k := range candidates {
				if v, ok := envLookup(k); ok {
					srv.Username = v
					break
				}
			}
		}
		if strings.TrimSpace(srv.Username) == "" || len(ExtractUnresolvedVars(srv.Username)) > 0 {
			if stored := loadCred(normKey); stored != nil && stored["username"] != "" {
				srv.Username = stored["username"]
			} else if stored := loadCred("vps"); stored != nil && stored["username"] != "" {
				srv.Username = stored["username"]
			}
		}

		// 3. KeyPath
		srv.KeyPath = expandConfigVars(srv.KeyPath, true, envLookup)
		if strings.TrimSpace(srv.KeyPath) == "" {
			candidates := make([]string, 0, 4)
			if suffix != "" {
				candidates = append(candidates, "VPS_KEY_PATH_"+suffix, "VPS_SSH_KEY_PATH_"+suffix)
			}
			candidates = append(candidates, "VPS_KEY_PATH", "VPS_SSH_KEY_PATH")
			for _, k := range candidates {
				if v, ok := envLookup(k); ok {
					srv.KeyPath = v
					break
				}
			}
		}
		if strings.TrimSpace(srv.KeyPath) == "" || len(ExtractUnresolvedVars(srv.KeyPath)) > 0 {
			if stored := loadCred(normKey); stored != nil && stored["key_path"] != "" {
				srv.KeyPath = stored["key_path"]
			} else if stored := loadCred("vps"); stored != nil && stored["key_path"] != "" {
				srv.KeyPath = stored["key_path"]
			}
		}

		// 4. KeyPassphrase (braced expansion only; also checks SSH_KEY_PASSPHRASE)
		srv.KeyPassphrase = expandConfigVars(srv.KeyPassphrase, false, envLookup)
		if srv.KeyPassphrase == "" {
			if v, ok := envLookup("SSH_KEY_PASSPHRASE"); ok {
				srv.KeyPassphrase = v
			}
		}
		if srv.KeyPassphrase == "" || len(ExtractUnresolvedVars(srv.KeyPassphrase)) > 0 {
			if stored := loadCred(normKey); stored != nil && stored["key_passphrase"] != "" {
				srv.KeyPassphrase = stored["key_passphrase"]
			} else if stored := loadCred("vps"); stored != nil && stored["key_passphrase"] != "" {
				srv.KeyPassphrase = stored["key_passphrase"]
			}
		}

		// 5. Password (braced expansion only)
		if srv.Password != "" {
			srv.Password = expandConfigVars(srv.Password, false, envLookup)
		}
	}
}

// ValidateServerConfig verifies that a VPS server configuration has a concrete,
// resolved host and no unresolved environment variable placeholders in host,
// username, or key_path.
func ValidateServerConfig(srv ServerConfig) error {
	name := strings.TrimSpace(srv.Name)
	if name == "" {
		name = "unnamed"
	}

	if strings.TrimSpace(srv.Host) == "" {
		return fmt.Errorf(
			"server %q has no host configured; set servers[].host in nextdeploy.yml, export VPS_HOST=203.0.113.10, add VPS_HOST=... to .nextdeploy/.env or .env, or run 'nextdeploy creds set vps'",
			name,
		)
	}
	if vars := ExtractUnresolvedVars(srv.Host); len(vars) > 0 {
		v := vars[0]
		return fmt.Errorf(
			"server %q host %q references unset environment variable %s; export %s=203.0.113.10, add %s=... to .nextdeploy/.env or .env, or run 'nextdeploy creds set vps'",
			name, srv.Host, v, v, v,
		)
	}
	if vars := ExtractUnresolvedVars(srv.Username); len(vars) > 0 {
		v := vars[0]
		return fmt.Errorf(
			"server %q username %q references unset environment variable %s; export %s=<user>, add %s=... to .nextdeploy/.env or .env, or run 'nextdeploy creds set vps'",
			name, srv.Username, v, v, v,
		)
	}
	if vars := ExtractUnresolvedVars(srv.KeyPath); len(vars) > 0 {
		v := vars[0]
		return fmt.Errorf(
			"server %q key_path %q references unset environment variable %s; export %s=<path>, add %s=... to .nextdeploy/.env or .env, or run 'nextdeploy creds set vps'",
			name, srv.KeyPath, v, v, v,
		)
	}
	return nil
}

// ValidateServers ensures at least one VPS server is configured and every
// configured server passes ValidateServerConfig.
func ValidateServers(servers []ServerConfig) error {
	if len(servers) == 0 {
		return fmt.Errorf("no servers configured in nextdeploy.yml")
	}
	for _, srv := range servers {
		if err := ValidateServerConfig(srv); err != nil {
			return err
		}
	}
	return nil
}
