# Extracting JavaScript Object to Go Map with Arbitrary Logic

To handle the conversion from a JavaScript object to a Go map with arbitrary logic (including handling functions and more complex cases), here's a more flexible approach:

## Solution with Custom Parsing Logic

```go
package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

func extractNextConfig(jsContent string) (map[string]interface{}, error) {
	// Step 1: Extract the nextConfig object from JS code
	configObj, err := extractJSObject(jsContent, "nextConfig")
	if err != nil {
		return nil, err
	}

	// Step 2: Convert to JSON (handling JS-specific syntax)
	jsonStr := jsToJSON(configObj)

	// Step 3: Parse into Go map
	config := make(map[string]interface{})
	err = json.Unmarshal([]byte(jsonStr), &config)
	if err != nil {
		return nil, err
	}

	// Step 4: Handle special cases (like functions)
	handleSpecialCases(config, jsContent)

	return config, nil
}

func extractJSObject(jsContent, objectName string) (string, error) {
	// Simple regex approach - for production use a proper JS parser
	pattern := regexp.MustCompile(`const\s+` + objectName + `\s*=\s*({[\s\S]*?})\s*;`)
	matches := pattern.FindStringSubmatch(jsContent)
	if len(matches) < 2 {
		return "", fmt.Errorf("could not find %s object", objectName)
	}
	return matches[1], nil
}

func jsToJSON(js string) string {
	// Convert JavaScript object to JSON format
	// This is a simplified version - you'd need more robust handling

	// Remove trailing commas
	re := regexp.MustCompile(`,\s*([}\]])`)
	js = re.ReplaceAllString(js, "$1")

	// Convert single quotes to double quotes
	js = strings.ReplaceAll(js, `'`, `"`)

	// Remove JS comments
	js = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(js, "")
	js = regexp.MustCompile(`//.*`).ReplaceAllString(js, "")

	return js
}

func handleSpecialCases(config map[string]interface{}, jsContent string) {
	// Handle webpack function
	if webpackConfig, exists := config["webpack"]; exists {
		if webpackStr, ok := webpackConfig.(string); ok {
			// Extract function body
			if strings.Contains(webpackStr, "function") || strings.Contains(webpackStr, "=>") {
				config["webpack"] = map[string]interface{}{
					"__type__": "function",
					"body":     extractFunctionBody(webpackStr),
				}
			}
		}
	}

	// Add any other special case handling here
}

func extractFunctionBody(funcStr string) string {
	// Extract the body of a function
	// This is simplified - would need better parsing for production
	start := strings.Index(funcStr, "{")
	end := strings.LastIndex(funcStr, "}")
	if start == -1 || end == -1 {
		return funcStr
	}
	return strings.TrimSpace(funcStr[start+1 : end])
}

func main() {
	jsContent := `const nextConfig = {
		reactStrictMode: true,
		output: 'standalone',
		pageExtensions: ['js', 'jsx', 'md', 'mdx', 'ts', 'tsx'],
		experimental: {
			mdxRs: true,
		},
		eslint: {
			ignoreDuringBuilds: true,
		},
		typescript: {
			ignoreBuildErrors: true,
		},
		images: {
			unoptimized: true,
			remotePatterns: [
				{
					protocol: 'https',
					hostname: 'avatars.githubusercontent.com',
				},
				{
					protocol: 'https',
					hostname: 'lh3.googleusercontent.com',
				},
				{
					protocol: 'https',
					hostname: 'randomuser.me',
				},
			],
		},
		webpack: (config, { webpack }) => {
			config.plugins.push(
				new webpack.IgnorePlugin({
					resourceRegExp: /^pg-native$|^cloudflare:sockets$/,
				})
			);
			return config;
		},
	};`

	config, err := extractNextConfig(jsContent)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Full Config: %+v\n", config)

	// Access nested values
	if images, ok := config["images"].(map[string]interface{}); ok {
		fmt.Println("Image remote patterns:")
		if patterns, ok := images["remotePatterns"].([]interface{}); ok {
			for _, p := range patterns {
				if pattern, ok := p.(map[string]interface{}); ok {
					fmt.Printf("- %s://%s\n", pattern["protocol"], pattern["hostname"])
				}
			}
		}
	}

	if webpack, ok := config["webpack"].(map[string]interface{}); ok {
		fmt.Printf("Webpack function: %s\n", webpack["body"])
	}
}
```

## Key Features

1. **Flexible Extraction**:
   - Extracts the `nextConfig` object from JavaScript code
   - Handles nested objects and arrays
   - Preserves the complete structure

2. **Special Case Handling**:
   - Converts functions to a special representation with their bodies
   - Can be extended to handle other JavaScript-specific features

3. **Arbitrary Logic**:
   - You can add custom processing for specific fields
   - Modify values during conversion
   - Handle edge cases as needed

## Production Considerations

For a production environment, you might want to:

1. Use a proper JavaScript parser like [otto](https://github.com/robertkrimen/otto) or [goja](https://github.com/dop251/goja) instead of regex
2. Add more robust error handling
3. Implement caching if you're processing many files
4. Add type conversion for specific known fields
5. Handle circular references in the object

Would you like me to elaborate on any specific aspect of this solution?
