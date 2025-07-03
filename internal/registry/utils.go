package registry

import (
	"regexp"
	"strings"
	"fmt"

)

func ExtractECRDetails(ecrURI string) (string, string, error) {
	// Trim whitespace (in case input has leading/trailing spaces)
	trimmedURI := strings.TrimSpace(ecrURI)

	// Regex to validate and extract parts
	// Format: `ACCOUNT.dkr.ecr.REGION.amazonaws.com/REPO_NAME`
	re := regexp.MustCompile(`^([0-9]+)\.dkr\.ecr\.([a-z0-9-]+)\.amazonaws\.com/(.+)$`)
	matches := re.FindStringSubmatch(trimmedURI)

	if len(matches) < 4 {
		return "", "", fmt.Errorf("invalid ECR URI format: %s", trimmedURI)
	}

	// matches[0] = full string
	// matches[1] = account ID (285688593966)
	// matches[2] = region (us-east-1)
	// matches[3] = repo name (hersiyussuf/hersi.dev)
	return matches[2], matches[3], nil
}
