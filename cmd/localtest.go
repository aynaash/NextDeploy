package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var (
	cyan    = color.New(color.FgCyan).SprintFunc()
//	green   = color.New(color.FgGreen).SprintFunc()
//	red     = color.New(color.FgRed).SprintFunc()
//	yellow  = color.New(color.FgYellow).SprintFunc()
	magenta = color.New(color.FgMagenta).SprintFunc()
	blue    = color.New(color.FgBlue).SprintFunc()
)

var testCmd = &cobra.Command{
	Use:   "localtest",
	Short: "🚀 Runs comprehensive health checks on Docker images",
	Long: `🌈 Runs comprehensive health checks on the Docker image tagged with the current Git commit hash,
providing visual feedback with emojis and colored output. Includes multiple validation steps
and detailed logging for troubleshooting.`,
	Run: func(cmd *cobra.Command, args []string) {
		startTime := time.Now()
		printHeader("🚀 Starting Docker Image Health Checks")
		
		// Get the current Git commit hash
		printStep(1, "🔍 Fetching Git commit hash")
		commitHash, err := GetCommitHash()
		if err != nil {
			printFailure("❌ Failed to get Git commit hash", err)
			os.Exit(1)
		}
		printSuccess("✅ Found Git commit hash: %s", magenta(commitHash))

		// Find Docker image
		printStep(2, "🐳 Locating Docker image")
		imageName, err := FindDockerImageByCommitTag(commitHash)
		if err != nil {
			printFailure("❌ Docker image not found", err)
			printTip("💡 Try running 'docker build -t your-image-repo:commit-%s .'", commitHash)
			os.Exit(1)
		}
		printSuccess("✅ Found Docker image: %s", magenta(imageName))

		// Verify image exists
		printStep(3, "🔎 Verifying Docker image")
		if exists, err := dockerImageExists(imageName); err != nil {
			printFailure("❌ Docker image verification failed", err)
			os.Exit(1)
		} else if !exists {
			printError("❌ Docker image not found in local registry")
			os.Exit(1)
		}
		printSuccess("✅ Docker image verified successfully")

		// Run health checks
		printStep(4, "🏥 Running comprehensive health checks")
		runHealthChecks(imageName)

		// Print summary
		duration := time.Since(startTime).Round(time.Millisecond)
		printSuccess("\n🎉 All health checks completed in %s", magenta(duration))
	},
}

func init() {
	rootCmd.AddCommand(testCmd)
}

// FindDockerImageByCommitTag finds a Docker image tagged with the Git commit hash
func FindDockerImageByCommitTag(commitHash string) (string, error) {
	printDebug("🔎 Searching for Docker images matching commit %s", commitHash)
	
	cmd := exec.Command("docker", "images", "--format", "{{.Repository}}:{{.Tag}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to list Docker images: %w (output: %s)", err, string(output))
	}

	images := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, image := range images {
		parts := strings.Split(image, ":")
		if len(parts) != 2 {
			continue
		}
		
		tag := parts[1]
		printDebug("  🔍 Checking image tag: %s", tag)
		
		// Check various common tagging patterns
		if tag == commitHash || 
		   strings.HasPrefix(tag, commitHash+"-") ||
		   tag == "commit-"+commitHash || 
		   tag == "git-"+commitHash ||
		   tag == "rev-"+commitHash ||
		   strings.Contains(tag, commitHash) {
			printDebug("  ✅ Found matching image: %s", image)
			return image, nil
		}
	}

	return "", fmt.Errorf("no Docker image found with tag matching commit %s", commitHash)
}

// GetCommitHash gets the current Git commit hash with enhanced validation
func GetCommitHash() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short=7", "HEAD")
	
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git command failed: %s (error: %w)", strings.TrimSpace(stderr.String()), err)
	}

	hash := strings.TrimSpace(out.String())
	if hash == "" {
		return "", fmt.Errorf("empty commit hash returned")
	}
	
	if len(hash) != 7 {
		printWarning("⚠️  Unexpected commit hash length: %d (expected 7)", len(hash))
	}
	
	return hash, nil
}

func dockerImageExists(imageName string) (bool, error) {
	printDebug("🔍 Verifying image exists: %s", imageName)
	
	cmd := exec.Command("docker", "image", "inspect", imageName)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return false, fmt.Errorf("docker inspect failed: %s", string(exitErr.Stderr))
		}
		return false, err
	}
	return true, nil
}

func runHealthChecks(image string) {
	// Track overall health status
	allChecksPassed := true
	
	// 1. Check if image runs
	printSubStep(1, "🚀 Testing container startup")
	if err := runContainer(image); err != nil {
		printFailure("❌ Container failed to start", err)
		allChecksPassed = false
	} else {
		defer cleanupContainer()
		printSuccess("✅ Container started successfully")
	}

	// 2. Check container logs for errors
	printSubStep(2, "📜 Analyzing container logs")
	if logs, err := getContainerLogs(); err != nil {
		printFailure("❌ Failed to get container logs", err)
		allChecksPassed = false
	} else {
		logStr := string(logs)
		printDebug("Container logs:\n%s", cyan(logStr))
		
		if strings.Contains(strings.ToLower(logStr), "error") {
			printError("❌ Errors detected in container logs!")
			color.New(color.BgRed, color.FgWhite).Println(logStr)
			allChecksPassed = false
		} else {
		printSuccess("✅ No errors found in logs")
		}
	}

	// 3. Check health status
	printSubStep(3, "🩺 Checking container health status")
	if healthy, err := checkContainerHealth(); err != nil {
		printWarning("⚠️  No health check defined for this image")
		printTip("💡 Consider adding HEALTHCHECK to your Dockerfile")
	} else if !healthy {
		printError("❌ Container health check failed!")
		allChecksPassed = false
	} else {
		printSuccess("💚 Container health check passed")
	}

	// 4. HTTP check
	printSubStep(4, "🌐 Testing HTTP endpoint (if applicable)")
	if status, err := checkHttpEndpoint(); err != nil {
		printDebug("ℹ️  HTTP check skipped: %v", err)
		printWarning("⚠️  HTTP check not performed (not a web service?)")
	} else if status >= 400 {
		printError("❌ HTTP check failed with status %d", status)
		allChecksPassed = false
	} else {
		printSuccess("✅ HTTP endpoint responded with status %d", status)
	}

	// Final verdict
	if allChecksPassed {
		printSuccess("\n🌈 All health checks passed successfully!")
	} else {
		printError("\n🔥 Some health checks failed. Please review the issues above.")
	}
}

// Docker operations with improved error handling
func runContainer(image string) error {
	printDebug("🏃 Starting container: %s", image)
	cmd := exec.Command("docker", "run", "--rm", "-d", "--name", "healthcheck_temp", image)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to start container: %w (output: %s)", err, string(output))
	}
	return nil
}

func cleanupContainer() {
	printDebug("🧹 Cleaning up temporary container")
	cmd := exec.Command("docker", "stop", "healthcheck_temp")
	if err := cmd.Run(); err != nil {
		printDebug("⚠️  Failed to clean up container: %v", err)
	}
}

func getContainerLogs() ([]byte, error) {
	cmd := exec.Command("docker", "logs", "healthcheck_temp")
	return cmd.CombinedOutput()
}

func checkContainerHealth() (bool, error) {
	cmd := exec.Command("docker", "inspect", "--format='{{.State.Health.Status}}'", "healthcheck_temp")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("health check failed: %w", err)
	}
	
	status := strings.Trim(strings.TrimSpace(string(output)), "'\"")
	printDebug("Container health status: %s", status)
	
	return status == "healthy", nil
}

func checkHttpEndpoint() (int, error) {
	// Implement your HTTP check logic here
	// For example:
	// resp, err := http.Get("http://localhost:8080/health")
	// if err != nil {
	//   return 0, err
	// }
	// defer resp.Body.Close()
	// return resp.StatusCode, nil
	
	return 0, fmt.Errorf("HTTP check not implemented")
}

// Enhanced output functions
func printHeader(format string, a ...interface{}) {
	color.New(color.Bold, color.FgHiBlue).Printf("\n"+format+"\n", a...)
}

func printStep(num int, text string) {
	color.New(color.Bold).Printf("\n%d. %s\n", num, cyan(text))
}

func printSubStep(num int, text string) {
	fmt.Printf("   %s %s\n", blue(fmt.Sprintf("%d.", num)), text)
}

func printSuccess(format string, a ...interface{}) {
	color.New(color.FgGreen).Printf("     ✅ "+format+"\n", a...)
}

func printError(format string, a ...interface{}) {
	color.New(color.FgRed).Printf("     ❌ "+format+"\n", a...)
}

func printWarning(format string, a ...interface{}) {
	color.New(color.FgYellow).Printf("     ⚠️  "+format+"\n", a...)
}

func printFailure(message string, err error) {
	color.New(color.FgRed).Printf("     ❌ %s\n", message)
	color.New(color.FgHiRed).Printf("       Reason: %v\n", err)
}

func printDebug(format string, a ...interface{}) {
	if os.Getenv("DEBUG") == "true" {
		color.New(color.FgHiBlack).Printf("     [DEBUG] "+format+"\n", a...)
	}
}

func printTip(format string, a ...interface{}) {
	color.New(color.FgHiCyan).Printf("     💡 "+format+"\n", a...)
}
