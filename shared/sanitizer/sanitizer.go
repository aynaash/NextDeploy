// Package sanitizer provides security-focused sanitization functions
// to prevent common vulnerabilities like command injection, path traversal,
// and other security issues.
package sanitizer

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// DockerImageName sanitizes a Docker image name for use in command execution
func DockerImageName(name string) string {
	// Remove any potentially dangerous characters
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\-\.\/:]`)
	sanitized := reg.ReplaceAllString(name, "")

	// Ensure it's not empty and has reasonable length
	if len(sanitized) == 0 || len(sanitized) > 255 {
		return ""
	}

	return sanitized
}

// ContainerName sanitizes a Docker container name
func ContainerName(name string) string {
	// Container names can only contain [a-zA-Z0-9][a-zA-Z0-9_.-]
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\.\-]`)
	sanitized := reg.ReplaceAllString(name, "")

	// Must start with alphanumeric
	if len(sanitized) > 0 && !unicode.IsLetter(rune(sanitized[0])) && !unicode.IsNumber(rune(sanitized[0])) {
		sanitized = "c" + sanitized
	}

	// Limit length (Docker limit is 64 chars)
	if len(sanitized) > 64 {
		sanitized = sanitized[:64]
	}

	return sanitized
}

// FilePath sanitizes a file path to prevent directory traversal attacks
func FilePath(path string, allowedBaseDir string) (string, error) {
	// Clean the path to remove any ../ or ./
	cleanPath := filepath.Clean(path)

	// Ensure the path is within the allowed base directory
	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", err
	}

	absBaseDir, err := filepath.Abs(allowedBaseDir)
	if err != nil {
		return "", err
	}

	// Check if the path is within the allowed directory
	if !strings.HasPrefix(absPath, absBaseDir) {
		return "", &SecurityError{Message: "path traversal attempt detected"}
	}

	return absPath, nil
}

// CommandArgument sanitizes a single command line argument
func CommandArgument(arg string) string {
	// Remove any characters that could be used for command injection
	reg := regexp.MustCompile(`[;&|$()\` + "`" + `'"\t\n\r]`)
	sanitized := reg.ReplaceAllString(arg, "")

	// Trim whitespace and limit length
	sanitized = strings.TrimSpace(sanitized)
	if len(sanitized) > 1024 {
		sanitized = sanitized[:1024]
	}

	return sanitized
}

// URL sanitizes a URL for HTTP requests
func URL(url string) string {
	// Basic URL validation - you might want to use net/url for more robust validation
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\-\.:/?#@!$&'()*+,;=%]`)
	sanitized := reg.ReplaceAllString(url, "")

	// Ensure it starts with http:// or https://
	if !strings.HasPrefix(sanitized, "http://") && !strings.HasPrefix(sanitized, "https://") {
		return ""
	}

	return sanitized
}

// Password sanitizes a password string (removes dangerous characters)
func Password(password string) string {
	// Remove characters that could be used for command injection
	reg := regexp.MustCompile(`[;&|$()\` + "`" + `'"\t\n\r]`)
	return reg.ReplaceAllString(password, "")
}

// Alphanumeric removes all non-alphanumeric characters
func Alphanumeric(input string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9]`)
	return reg.ReplaceAllString(input, "")
}

// Filename sanitizes a filename to prevent path traversal and other issues
func Filename(filename string) string {
	// Remove directory traversal attempts and other dangerous patterns
	reg := regexp.MustCompile(`\.\./|\.\.\\|/|\\|[:*?"<>|]`)
	sanitized := reg.ReplaceAllString(filename, "_")

	// Remove any other potentially dangerous characters
	reg = regexp.MustCompile(`[^a-zA-Z0-9_\-\.]`)
	sanitized = reg.ReplaceAllString(sanitized, "")

	// Ensure it's not empty and not too long
	if len(sanitized) == 0 || len(sanitized) > 255 {
		return "file"
	}

	return sanitized
}

// ShellCommand sanitizes a shell command for safe execution
func ShellCommand(command string) string {
	// This is a very restrictive sanitizer for shell commands
	// Consider using exec.Command with individual arguments instead
	reg := regexp.MustCompile(`[^a-zA-Z0-9_\-\./ ]`)
	sanitized := reg.ReplaceAllString(command, "")

	// Remove multiple spaces and trim
	sanitized = strings.Join(strings.Fields(sanitized), " ")

	return strings.TrimSpace(sanitized)
}

// SafeExecArgs prepares arguments for exec.Command with validation
func SafeExecArgs(command string, args []string) (string, []string, error) {
	// Validate the base command
	safeCommand := Alphanumeric(command)
	if safeCommand == "" {
		return "", nil, &SecurityError{Message: "invalid command"}
	}

	// Sanitize each argument
	safeArgs := make([]string, len(args))
	for i, arg := range args {
		safeArgs[i] = CommandArgument(arg)
		if safeArgs[i] == "" && arg != "" {
			return "", nil, &SecurityError{Message: "invalid argument detected"}
		}
	}

	return safeCommand, safeArgs, nil
}

// SecurityError represents a security-related error
type SecurityError struct {
	Message string
}

func (e *SecurityError) Error() string {
	return "security error: " + e.Message
}

// IsSecurityError checks if an error is a SecurityError
func IsSecurityError(err error) bool {
	_, ok := err.(*SecurityError)
	return ok
}
