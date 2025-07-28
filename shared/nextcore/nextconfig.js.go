package nextcore

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ParseNextConfig evaluates the Next.js config using Node.js
func ParseNextConfig(projectDir string) (*NextConfig, error) {
	configFile, err := findConfigFile(projectDir)
	if err != nil {
		NextCoreLogger.Error("Failed to find Next.js config file:%s", err)
		return nil, err
	}
	if configFile == "" {
		return &NextConfig{}, nil // No config found
	}

	// Get absolute path to our evaluator script
	js := []byte(`
	const path = require('path');
const { transformSync } = require('esbuild');

// Handle both ESM and CJS config files
async function loadConfig(configPath) {
  try {
    // Handle TypeScript files
    if (configPath.endsWith('.ts')) {
      const result = transformSync(require('fs').readFileSync(configPath, 'utf8'), {
        loader: 'ts',
        format: 'cjs'
      });
      const mod = eval(result.code);
      return mod.default || mod;
    }

    // Handle ESM files (.mjs or package.json type: module)
    if (configPath.endsWith('.mjs') || isEsmProject(path.dirname(configPath))) {
      const { default: mod } = await import(configPath);
      return mod;
    }

    // Fallback to CJS
    const mod = require(configPath);
    return mod.default || mod;
  } catch (error) {
    process.exit(1);
  }
}

function isEsmProject(dir) {
  try {
    const pkg = require(path.join(dir, 'package.json'));
    return pkg.type === 'module';
  } catch {
    return false;
  }
}

// Process functions and special cases
function processConfig(config) {
  const result = {};
  
  for (const [key, value] of Object.entries(config)) {
    if (typeof value === 'function') {
      result[key] = {
        __next_core_function__: true,
        source: value.toString()
      };
    } else if (value && typeof value === 'object') {
      result[key] = processConfig(value);
    } else {
      result[key] = value;
    }
  }
  
  return result;
}

// Main execution
async function main() {
  const configPath = path.resolve(process.argv[2]);
  const config = await loadConfig(configPath);
  const processed = processConfig(config);
  console.log(JSON.stringify(processed));
}

main().catch(err => {
  console.error(err);
  process.exit(1);
});
  
	`)
	if err != nil {
		NextCoreLogger.Error("Failed to read evaluator.js:%s", err)
		return nil, fmt.Errorf("failed to read evaluator.js: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		NextCoreLogger.Error("Failed to get current working directory:%s", err)
		return nil, fmt.Errorf("failed to get current working directory: %w", err)
	}
	if cwd == "" {
		return nil, fmt.Errorf("current working directory is empty")
	}

	// create file to save js
	evaluatorPath := filepath.Join(cwd, "evaluator.js")
	if err := os.WriteFile(evaluatorPath, js, 0644); err != nil {
		NextCoreLogger.Error("Failed to write evaluator.js:%s", err)
		return nil, fmt.Errorf("failed to write evaluator.js: %w", err)
	}
	// Ensure Node.js is available
	if _, err := exec.LookPath("node"); err != nil {
		NextCoreLogger.Error("Node.js is not installed or not in PATH:%s", err)
		return nil, fmt.Errorf("Node.js is not installed or not in PATH: %w", err)
	}
	// Ensure the config file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		NextCoreLogger.Error("Next.js config file not found:%s", configFile)
		return nil, fmt.Errorf("Next.js config file not found: %s", configFile)
	}

	// Execute the config evaluator
	cmd := exec.Command("node", evaluatorPath, configFile)
	cmd.Dir = projectDir // Run in project directory for proper resolution

	output, err := cmd.CombinedOutput()
	if err != nil {
		NextCoreLogger.Error("Failed to evaluate Next.js config:%s", err)
		return nil, fmt.Errorf("config evaluation failed: %v\nOutput: %s", err, string(output))
	}

	var config NextConfig
	if err := json.Unmarshal(output, &config); err != nil {
		NextCoreLogger.Error("Failed to parse config JSON:%s", err)
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	return &config, nil
}

func findConfigFile(projectDir string) (string, error) {
	extensions := []string{".js", ".mjs", ".cjs", ".ts"}
	for _, ext := range extensions {
		path := filepath.Join(projectDir, "next.config"+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", nil // No config file found
}
