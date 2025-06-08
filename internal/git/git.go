package git

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// GetCurrentBranch returns the name of the current Git branch in the working directory.
// Returns an error if the directory is not a Git repository or if the command fails.
func GetCurrentBranch() (string, error) {
	// Check if git is installed and available in PATH
	if _, err := exec.LookPath("git"); err != nil {
		return "", errors.New("git command not found")
	}

	// Run git command to get the current branch
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", errors.New("not a git repository or unable to get branch: " + err.Error())
	}

	// Clean up the output (remove newlines and whitespace)
	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "", errors.New("could not determine current branch")
	}

	return branch, nil
}
func GetCommitHash() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--short=7", "HEAD")

	var out bytes.Buffer
	cmd.Stdout = &out

	err := cmd.Run()

	if err != nil {
		return "", fmt.Errorf("git command failed: %w", err)
	}

	return strings.TrimSpace(out.String()), nil
}

func IsDirty() bool {
	cmd := exec.Command("git", "status", "--porcelain")

	var out bytes.Buffer

	cmd.Stdout = &out
	_ = cmd.Run()
	return out.Len() > 0
}
