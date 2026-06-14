package nextcompile

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/aynaash/nextdeploy/shared/protection"
)

// EmitProtectionConfig writes <outDir>/_nextdeploy/protection.json — the policy
// the runtime guard reads. When rt is nil the file holds the JSON literal `null`
// so worker_entry.mjs's static `import protection from "./protection.json"`
// always resolves and the dispatcher treats it as "no guard". Returns the path.
func EmitProtectionConfig(rt *protection.Runtime, outDir string) (string, error) {
	dir := filepath.Join(outDir, "_nextdeploy")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("mkdir _nextdeploy: %w", err)
	}

	data := []byte("null\n")
	if rt != nil {
		j, err := rt.JSON()
		if err != nil {
			return "", fmt.Errorf("marshal protection: %w", err)
		}
		data = append(j, '\n')
	}

	path := filepath.Join(dir, "protection.json")
	if err := os.WriteFile(path, data, 0o640); err != nil {
		return "", fmt.Errorf("write protection.json: %w", err)
	}
	return path, nil
}
