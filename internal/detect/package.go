package detect

import (
	"fmt"
	"nextdeploy/internal/logger"
	"os"
	"path/filepath"
	"strings"
)

type PackageManager string

var (
	plog = logger.PackageLogger("cmd", "🚀 CMD")
	dlog = logger.PackageLogger("docker", "🐳 DOCKER")
	slog = logger.PackageLogger("secrets", "🔒 SECRETS")
)

const (
	NPM     PackageManager = "npm"
	Yarn    PackageManager = "yarn"
	PNPM    PackageManager = "pnpm"
	Unknown PackageManager = "unknown"
)

func (pm PackageManager) String() string {
	return string(pm)
}

func DetectPackageManager(projectPath string) (PackageManager, error) {
	plog.Debug("Detecting package manager in %s", projectPath)

	indicators := map[string]struct {
		manager PackageManager
		weight  int
	}{
		"pnpm-lock.yaml":      {PNPM, 100},
		"yarn.lock":           {Yarn, 100},
		"package-lock.json":   {NPM, 100},
		".npmrc":              {NPM, 40},
		".yarnrc":             {Yarn, 40},
		"pnpm-workspace.yaml": {PNPM, 40},
		".yarn":               {Yarn, 30},
		".pnpm-store":         {PNPM, 30},
		"node_modules/.yarn":  {Yarn, 30},
	}

	pkgPath := filepath.Join(projectPath, "package.json")
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		plog.Error("No package.json found in %s", projectPath)
		return Unknown, fmt.Errorf("not a Node.js project")
	}

	scores := make(map[PackageManager]int)
	for filename, data := range indicators {
		if _, err := os.Stat(filepath.Join(projectPath, filename)); err == nil {
			plog.Debug("Found indicator file: %s", filename)
			scores[data.manager] += data.weight
		}
	}

	pkgJson, err := os.ReadFile(pkgPath)
	if err == nil {
		content := string(pkgJson)
		if strings.Contains(content, "pnpm") {
			scores[PNPM] += 20
		}
		if strings.Contains(content, "yarn") {
			scores[Yarn] += 20
		}
		if strings.Contains(content, `"pnpm"`) {
			scores[PNPM] += 50
		}
		if strings.Contains(content, `"yarn"`) {
			scores[Yarn] += 50
		}
	}

	if os.Getenv("PNPM_HOME") != "" {
		scores[PNPM] += 30
	}
	if os.Getenv("YARN_VERSION") != "" {
		scores[Yarn] += 30
	}

	var result PackageManager
	maxScore := 0
	for manager, score := range scores {
		if score > maxScore {
			maxScore = score
			result = manager
		}
	}

	if maxScore == 0 {
		plog.Debug("No clear package manager detected, checking binaries")
		if _, err := os.Stat(filepath.Join(projectPath, "node_modules", ".bin", "pnpm")); err == nil {
			return PNPM, nil
		}
		if _, err := os.Stat(filepath.Join(projectPath, "node_modules", ".bin", "yarn")); err == nil {
			return Yarn, nil
		}
		return NPM, nil
	}

	plog.Info("Detected package manager: %s", result)
	return result, nil
}
