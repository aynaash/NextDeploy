package registry

import (
	"fmt"
	"nextdeploy/shared/config"
	"nextdeploy/shared/git"
	"regexp"
	"strings"
)

func ExtractECRDetails(ecrURI string) (string, string, string, error) {
	// Trim whitespace (in case input has leading/trailing spaces)
	trimmedURI := strings.TrimSpace(ecrURI)

	// Regex to validate and extract parts
	// Format: `ACCOUNT.dkr.ecr.REGION.amazonaws.com/REPO_NAME`
	re := regexp.MustCompile(`^([0-9]+)\.dkr\.ecr\.([a-z0-9-]+)\.amazonaws\.com/(.+)$`)
	matches := re.FindStringSubmatch(trimmedURI)

	if len(matches) < 4 {
		return "", "", "", fmt.Errorf("invalid ECR URI format: %s", trimmedURI)
	}

	accountID := matches[1]
	region := matches[2]
	repoName := matches[3]

	// matches[0] = full string
	// matches[1] = account ID (285688593966)
	// matches[2] = region (us-east-1)
	// matches[3] = repo name (hersiyussuf/hersi.dev)
	return accountID, region, repoName, nil
}

func GetLatestImageName() string {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load configuration: %v\n", err)
		return ""
	}
	gitCommit, err := git.GetCommitHash()
	if err != nil {
		fmt.Printf("Failed to get git commit hash: %v\n", err)
		return ""
	}
	return fmt.Sprintf("%s:%s", cfg.Docker.Image, gitCommit)
}
