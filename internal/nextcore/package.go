package nextcore

// TODO(yussuf): Rename plog from "cmd" to "detect" to better reflect this package's context.
//   - Use: logger.PackageLogger("detect", "📦 DETECT")

// TODO(yussuf): Split logger instances (plog, dlog, slog) if not all are used here.
//   - If this file only detects package managers, remove dlog/slog to avoid noise.

// TODO(yussuf): Move indicator definitions into a `var indicators = ...` block outside the function.
//   - Keeps `DetectPackageManager()` smaller and more readable.
//   - Possibly expose as a constant or unexported var for testability or override.

// TODO(yussuf): Add file existence check helper:
//   - func fileExists(path string) bool
//   - Cleans up repeated os.Stat logic and avoids clutter in detection loop.

// TODO(yussuf): Refactor score weighting logic for readability:
//   - Abstract into: `scoreManager(scores map[PackageManager]int, manager PackageManager, weight int)`
//   - Reduces cognitive load while reading scoring rules.

// TODO(yussuf): Avoid hardcoded string weights inside `package.json` check:
//   - Create constants for `"pnpm"` and `"yarn"` match scores.
//   - Improves maintainability if scoring logic changes.

// TODO(yussuf): Move `.env` variable scoring outside the function or document clearly:
//   - Document supported env vars and why they influence scoring.
//   - Possibly expose via optional `WithEnvDetection` flag/config.

// TODO(yussuf): Add tie-breaker logic or warning in case of tied scores:
//   - Currently, first highest wins — add fallback preference ordering or warning log.

// TODO(yussuf): Add `DetectPackageManager()` unit tests:
//   - With fake file systems or using a temp dir with mock files.
//   - Ensure coverage of scoring, env detection, tie-breaker logic.

// TODO(yussuf): Add godoc comment above `DetectPackageManager()`:
//   - // DetectPackageManager inspects a project directory to determine the Node.js package manager used.
//   - Explain heuristics, expected inputs, and output.
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
