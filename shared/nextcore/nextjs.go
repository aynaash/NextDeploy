// NOTE: CROSS COMPILE SAFE
package nextcore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// PackageJSON represents the structure of package.json we care about
type PackageJSON struct {
	Name            string            `json:"name"`
	Version         string            `json:"version"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
	Scripts         map[string]string `json:"scripts"`
	Private         bool              `json:"private"`
}

func GetNextJsVersion(packageJsonPath string) (string, error) {
	data, err := os.ReadFile(packageJsonPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("package.json not found at %s", packageJsonPath)
		}
		return "", fmt.Errorf("error reading package.json: %w", err)
	}
	var pkg PackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", fmt.Errorf("error parsing package.json: %w", err)
	}
	// check at both dev and regular dependencies
	if version, exists := pkg.Dependencies["next"]; exists {
		return version, nil
	}
	if version, exists := pkg.DevDependencies["next"]; exists {
		return version, nil
	}
	return "", fmt.Errorf("Next.js dependency not found in package.json")

}

// ValidateNextJSProject checks if the current or specified directory is a valid Next.js project
func ValidateNextJSProject(cmd *cobra.Command, args []string) error {
	targetDir := "."
	if len(args) > 0 {
		targetDir = args[0]
	}

	absPath, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for directory '%s': %w", targetDir, err)
	}
	err = validateDirectory(absPath)
	if err != nil {
		return fmt.Errorf("failed to validate directory '%s': %w", absPath, err)
	}
	isNextJS, reason, err := IsNextJSProject(absPath)
	if err != nil {
		return fmt.Errorf("failed to validate Next.js project: %w", err)
	}
	if !isNextJS {
		if reason != "" {
			return fmt.Errorf("directory '%s' is not a Next.js project: %s", absPath, reason)
		}
		return fmt.Errorf("directory doesn't appear to be a Next.js project")
	}

	return nil
}

// validateDirectory checks if the path exists and is a directory
func validateDirectory(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory '%s' does not exist", path)
		}
		return fmt.Errorf("failed to access directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("'%s' is not a directory", path)
	}
	return nil
}

// IsNextJSProject performs comprehensive Next.js project validation
// Returns: (isNextJS bool, reason string, err error)
func IsNextJSProject(dir string) (bool, string, error) {
	// Check for package.json first
	pkg, err := readPackageJSON(dir)
	if err != nil {
		return false, "", err
	}
	if pkg == nil {
		return false, "no package.json found", nil
	}

	// Check for Next.js in dependencies
	if hasNextDependency(pkg.Dependencies) || hasNextDependency(pkg.DevDependencies) {
		return true, "", nil
	}

	// Check for Next.js in scripts
	if hasNextScript(pkg.Scripts) {
		return true, "", nil
	}

	// Check for Next.js specific files and directories
	if hasNextJSStructure(dir) {
		return true, "", nil
	}

	return false, "no Next.js dependencies, scripts, or project structure found", nil
}

// readPackageJSON reads and parses package.json
func readPackageJSON(dir string) (*PackageJSON, error) {
	path := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("error reading package.json: %w", err)
	}

	var pkg PackageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, fmt.Errorf("error parsing package.json: %w", err)
	}

	return &pkg, nil
}

// hasNextDependency checks if Next.js is in dependencies
func hasNextDependency(deps map[string]string) bool {
	if deps == nil {
		return false
	}
	_, exists := deps["next"]
	return exists
}

// hasNextScript checks if any script contains "next"
func hasNextScript(scripts map[string]string) bool {
	if scripts == nil {
		return false
	}
	for _, script := range scripts {
		if strings.Contains(strings.ToLower(script), "next") {
			return true
		}
	}
	return false
}

// hasNextJSStructure checks for Next.js specific files and directories
func hasNextJSStructure(dir string) bool {
	nextjsFiles := []string{
		"next.config.js",
		"next.config.mjs",
		"next.config.ts",
		"next-env.d.ts",
	}

	for _, file := range nextjsFiles {
		if _, err := os.Stat(filepath.Join(dir, file)); err == nil {
			return true
		}
	}

	nextjsDirs := []string{
		"pages",
		"src/pages",
		"app",
		"src/app",
		"public",
	}

	// More robust directory checking that handles case differences
	for _, dirPath := range nextjsDirs {
		if entries, err := os.ReadDir(dir); err == nil {
			for _, entry := range entries {
				if strings.EqualFold(entry.Name(), dirPath) && entry.IsDir() {
					return true
				}
			}
		}
	}
	return false
}
