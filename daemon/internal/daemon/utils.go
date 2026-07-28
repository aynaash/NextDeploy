package daemon

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"

	"github.com/aynaash/nextdeploy/shared/config"
)

func resolveTool(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		log.Printf("[daemon] Warning: could not resolve tool %s: %v", name, err)
		// Fallback to absolute paths commonly used
		switch name {
		case "tar":
			return "/usr/bin/tar"
		case "find":
			return "/usr/bin/find"
		case "chown":
			return "/usr/bin/chown"
		case "chmod":
			return "/usr/bin/chmod"
		case "systemctl":
			return "/usr/bin/systemctl"
		case "caddy":
			return "/usr/bin/caddy"
		}
		return name
	}
	return path
}

// validateAppName delegates to the shared rule so the daemon and the CLI can
// never disagree about what a valid app identifier is — a divergence that
// previously let a name pass client-side and detonate here.
func validateAppName(name string) error {
	return config.ValidateAppName(name)
}

func validateDomain(domain string) error {
	if domain == "localhost" {
		return nil
	}
	// Basic RFC 1035 label validation
	if matched, _ := regexp.MatchString(`^([a-z0-9]+(-[a-z0-9]+)*\.)+[a-z]{2,}$`, domain); !matched {
		return fmt.Errorf("invalid domain name format")
	}
	return nil
}

func StringArg(args map[string]interface{}, key string) (string, bool) {
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func Coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
