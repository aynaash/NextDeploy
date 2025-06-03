
package utils


import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
		"bufio"
	"syscall"
	"golang.org/x/term"

)

// Data struct for JSON processing
type Data struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

// ProcessData triples the value if a secret exists
func ProcessData(d Data, secret string) Data {
	if secret != "" {
		d.Value *= 3
	}
	return d
}

// DockerfileExists checks if a Dockerfile exists in the current directory
func DockerfileExists() bool {
	_, err := os.Stat("Dockerfile")
	return err == nil
}

// GetGitCommitHash retrieves the latest Git commit hash
func GetGitCommitHash() (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("Failed to get Git commit hash: %v\n%s", err, out.String())
	}

	return strings.TrimSpace(out.String()), nil
}

// GetSecret retrieves a secret from the environment or Doppler
func GetSecret(secretKey string) string {
	secret := os.Getenv(secretKey)
	if secret != "" {
		return secret
	}

	cmd := exec.Command("doppler", "secrets", "get", secretKey, "--plain")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	if err := cmd.Run(); err == nil {
		return strings.TrimSpace(out.String())
	}

	fmt.Printf("Warning: secret %s not found!\n", secretKey)
	return ""
}

// BuildAndPushDockerImage builds and optionally pushes a Docker image using the latest Git commit hash
func BuildAndPushDockerImage(imageName, registry string, push bool) error {
	commitHash, err := GetGitCommitHash()
	if err != nil {
		return err
	}

	imageTag := fmt.Sprintf("%s/%s:%s", registry, imageName, commitHash)

	buildCmd := exec.Command("docker", "build", "-t", imageTag, ".")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	fmt.Println("Building Docker image:", imageTag)
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("Failed to build Docker image: %v", err)
	}

	if push {
		pushCmd := exec.Command("docker", "push", imageTag)
		pushCmd.Stdout = os.Stdout
		pushCmd.Stderr = os.Stderr

		fmt.Println("Pushing Docker image:", imageTag)
		if err := pushCmd.Run(); err != nil {
			return fmt.Errorf("Failed to push Docker image: %v", err)
		}
	}

	fmt.Println("Docker operation successful")
	return nil
}




func Prompt(label string) string {
	var s string
	r := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprintf(os.Stderr, "%s: ", label)
		s, _ = r.ReadString('\n')
		if s != "" {
			break
		}
	}
	return strings.TrimSpace(s)
}

func PromptPassword(label string) string {
	fmt.Fprintf(os.Stderr, "%s: ", label)
	bytePassword, _ := term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	return strings.TrimSpace(string(bytePassword))
}

func PromptWithDefault(label, defaultValue string) string {
	var s string
	r := bufio.NewReader(os.Stdin)
	fmt.Fprintf(os.Stderr, "%s [%s]: ", label, defaultValue)
	s, _ = r.ReadString('\n')
	s = strings.TrimSpace(s)
	if s == "" {
		return defaultValue
	}
	return s
}

func ASCIIArt(title string) string {
	// Simple ASCII art generator
	return fmt.Sprintf(`
┌───────────────────────────────────────┐
│ %-35s │
└───────────────────────────────────────┘
`, title)
}
