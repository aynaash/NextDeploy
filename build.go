package main

/*
NextDeploy Build System

This script handles building both the NextDeploy server daemon (nextdeployd) and
the command-line interface (ndctl) with proper versioning and deployment instructions.

USAGE:
  go run build.go [flags]

FLAGS:
  -target string    Build target: 'daemon', 'cli', or 'all' (default "all")
  -output string    Output directory (default "./bin")
  -version string   Override version number (default detects from git/VERSION)

ENVIRONMENT:
  NEXTDEPLOY_VERSION  Set build version
  BUILD_STATIC        Force static linking ("true" or "false")

EXAMPLE:
  # Build both components with version 1.0.0
  NEXTDEPLOY_VERSION=1.0.0 go run build.go

  # Build only the daemon with static linking
  BUILD_STATIC=true go run build.go -target daemon
*/

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	// Parse command-line flags
	target := flag.String("target", "all", "Build target: 'daemon', 'cli', or 'all'")
	outputDir := flag.String("output", "./bin", "Output directory")
	versionOverride := flag.String("version", "", "Override version number")
	flag.Parse()

	// Configuration
	projectRoot := "."
	daemonPath := filepath.Join(projectRoot, "daemon", "main.go")
	cliPath := filepath.Join(projectRoot, "cli", "main.go")

	// Ensure output directory exists
	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		fail("Error creating output directory: %v", err)
	}

	// Get build metadata
	version := getVersion(*versionOverride)
	commit := getGitCommit()
	buildTime := time.Now().Format(time.RFC3339)

	// Build targets configuration
	targets := []struct {
		name        string
		source      string
		output      string
		environment []string
		ldflags     string
	}{
		{
			name:        "nextdeployd (server daemon)",
			source:      daemonPath,
			output:      filepath.Join(*outputDir, "nextdeployd"),
			environment: getDaemonEnv(),
			ldflags: fmt.Sprintf("-s -w -X 'main.Version=%s' -X 'main.Commit=%s' -X 'main.BuildTime=%s'",
				version, commit, buildTime),
		},
		{
			name:   "ndctl (CLI tool)",
			source: cliPath,
			output: filepath.Join(*outputDir, "ndctl"),
			environment: []string{
				"GOOS=" + getLocalOS(),
				"GOARCH=" + getLocalArch(),
			},
			ldflags: fmt.Sprintf("-X 'main.Version=%s' -X 'main.Commit=%s'", version, commit),
		},
	}

	// Execute builds
	for _, t := range targets {
		// Skip if not in target filter
		if *target != "all" && !strings.Contains(strings.ToLower(t.name), *target) {
			continue
		}

		fmt.Printf("\n🚀 Building %s\n", t.name)
		fmt.Printf("   Source: %s\n   Output: %s\n", t.source, t.output)
		fmt.Printf("   Build Flags: %s\n", t.ldflags)

		cmd := exec.Command("go", "build", "-ldflags", t.ldflags, "-o", t.output, t.source)
		cmd.Env = append(os.Environ(), append(t.environment, "GO111MODULE=on")...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fail("Build failed for %s: %v", t.name, err)
		}

		fmt.Printf("✅ Success: %s built successfully\n", filepath.Base(t.output))
	}

	printPostBuildInstructions(version)
}

// ---------------------- BUILD HELPERS ----------------------

func getDaemonEnv() []string {
	env := []string{"GOOS=linux", "GOARCH=amd64"}

	// Static linking by default unless explicitly disabled
	if os.Getenv("BUILD_STATIC") != "false" {
		env = append(env, "CGO_ENABLED=0")
	}
	return env
}

func getVersion(override string) string {
	if override != "" {
		return override
	}
	if version := os.Getenv("NEXTDEPLOY_VERSION"); version != "" {
		return version
	}
	if _, err := os.Stat("VERSION"); err == nil {
		if data, err := os.ReadFile("VERSION"); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return "dev"
}

func getGitCommit() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// ---------------------- SYSTEM HELPERS ----------------------

func getLocalOS() string {
	if runtime := os.Getenv("RUNTIME"); runtime != "" {
		return runtime
	}
	return goEnv("GOHOSTOS")
}

func getLocalArch() string {
	return goEnv("GOHOSTARCH")
}

func goEnv(varName string) string {
	cmd := exec.Command("go", "env", varName)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		fail("go env %s failed: %v", varName, err)
	}
	return strings.TrimSpace(out.String())
}

// ---------------------- OUTPUT HELPERS ----------------------

func printPostBuildInstructions(version string) {
	fmt.Printf(`
📝 NextDeploy %s Deployment Guide

SERVER COMPONENT (nextdeployd):
1. Secure Deployment:
   scp bin/nextdeployd admin@production:/usr/local/bin/
   ssh production sudo chown root:root /usr/local/bin/nextdeployd
   ssh production sudo chmod 755 /usr/local/bin/nextdeployd

2. Configuration:
   ssh production sudo mkdir -p /etc/nextdeploy
   ssh production 'echo "default_config: true" | sudo tee /etc/nextdeploy/config.yaml'

3. Data Directories:
   ssh production sudo mkdir -p /var/lib/nextdeploy/{keys,logs,cache}
   ssh production sudo chmod 700 /var/lib/nextdeploy/keys

4. Systemd Service:
   scp examples/nextdeployd.service admin@production:/tmp/
   ssh production sudo mv /tmp/nextdeployd.service /etc/systemd/system/
   ssh production sudo systemctl daemon-reload
   ssh production sudo systemctl enable --now nextdeployd

CLI TOOL (ndctl):
1. Local Installation:
   install -m 755 bin/ndctl ~/.local/bin/

2. Environment Setup:
   echo 'export NDCTL_ENDPOINT="https://your-server:8080"' >> ~/.bashrc
   echo 'export NDCTL_CACHE_DIR="${XDG_CACHE_HOME:-$HOME/.cache}/ndctl"' >> ~/.bashrc
   source ~/.bashrc

3. Verify Connection:
   ndctl --version
   ndctl ping

🔐 Security Recommendations:
- Rotate /var/lib/nextdeploy/keys/* quarterly
- Set up TLS for production deployments
- Configure firewall to restrict access to port 8080
- Monitor /var/lib/nextdeploy/logs/daemon.log

ℹ️  More documentation at https://docs.nextdeploy.example.com
`, version)
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "❌ "+format+"\n", a...)
	os.Exit(1)
}
