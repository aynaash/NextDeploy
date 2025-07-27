//go:build ignore
// +build ignore

// internal/server/preparation/manager.go
package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/robertkrimen/otto"
)

func main() {
	// Example TypeScript config
	tsConfig := `
		interface AppConfig {
			port: number;
			env: string;
		}

		export default {
			port: 3000,
			env: "development",
			db: {
				host: "localhost",
				port: 27017
			}
		} as AppConfig;
	`

	// Example JavaScript config
	jsConfig := `
		module.exports = {
			apiKey: 'abc123',
			enabled: true,
			maxConnections: 10
		};
	`

	// Invalid config with simple key-value pairs
	invalidConfig := `
		This is not valid JavaScript
		apiKey: '12345'
		secret: 'shhh'
		timeout: 5000
		debug: true
	`

	fmt.Println("Extracting from TypeScript:")
	tsResult, err := extractConfig(tsConfig, ".ts")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		printConfig(tsResult)
	}

	fmt.Println("\nExtracting from JavaScript:")
	jsResult, err := extractConfig(jsConfig, ".js")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		printConfig(jsResult)
	}

	fmt.Println("\nExtracting from Invalid Config:")
	invalidResult, err := extractConfig(invalidConfig, ".js")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		printConfig(invalidResult)
	}
}

func extractConfig(content string, ext string) (map[string]interface{}, error) {
	config := make(map[string]interface{})

	// Handle TypeScript files
	if ext == ".ts" {
		content = transpileTypeScriptConfig(content)
	} else {
		content = strings.TrimSpace(content)
	}

	// Try JavaScript evaluation first
	if err := extractWithOtto(content, config); err == nil && len(config) > 0 {
		return config, nil
	}

	// Fallback to regex parsing
	extractWithRegex(content, config)

	return config, nil
}

func extractWithOtto(content string, config map[string]interface{}) error {
	vm := otto.New()
	if _, err := vm.Run(content); err != nil {
		return err
	}

	// Try different export patterns
	exportPatterns := []string{
		"module.exports",
		"exports",
		"(typeof exports === 'object' && typeof module === 'object') ? module.exports : exports.default || exports",
		"(function() { try { return config || settings || cfg || configuration; } catch(e) { return undefined; } })()",
	}

	for _, pattern := range exportPatterns {
		if value, err := vm.Run(pattern); err == nil && !value.IsUndefined() {
			if exported, err := value.Export(); err == nil {
				if exportedMap, ok := exported.(map[string]interface{}); ok {
					for k, v := range exportedMap {
						config[k] = v
					}
					return nil
				}
			}
		}
	}

	return fmt.Errorf("no export object found")
}

func transpileTypeScriptConfig(content string) string {
	// Remove TypeScript type annotations
	re := regexp.MustCompile(`:\s*\w+\s*([,;}])`)
	content = re.ReplaceAllString(content, "$1")

	// Remove interface/type declarations
	re = regexp.MustCompile(`(?m)^\s*(export\s+)?(interface|type)\s+\w+\s*({[^}]*}|=.*)?\s*$`)
	content = re.ReplaceAllString(content, "")

	// Convert export default to module.exports
	re = regexp.MustCompile(`export\s+default`)
	content = re.ReplaceAllString(content, "module.exports =")

	return strings.TrimSpace(content)
}

func extractWithRegex(content string, config map[string]interface{}) {
	// Match key: value pairs
	re := regexp.MustCompile(`(?m)^\s*(?:export\s+|const\s+|let\s+|var\s+)?(\w+)\s*[:=]\s*([^;\n]+);?$`)
	matches := re.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			key := strings.TrimSpace(match[1])
			value := strings.TrimSpace(match[2])

			// Remove surrounding quotes
			value = strings.Trim(value, `'"`)

			// Type detection
			switch {
			case value == "true":
				config[key] = true
			case value == "false":
				config[key] = false
			case strings.HasPrefix(value, `'`) || strings.HasPrefix(value, `"`):
				config[key] = strings.Trim(value, `'"`)
			default:
				if num, err := strconv.ParseFloat(value, 64); err == nil {
					if strings.Contains(value, ".") {
						config[key] = num
					} else {
						config[key] = int(num)
					}
				} else {
					config[key] = value
				}
			}
		}
	}
}

func printConfig(config map[string]interface{}) {
	for k, v := range config {
		fmt.Printf("  %s: %v (%T)\n", k, v, v)
	}
}
