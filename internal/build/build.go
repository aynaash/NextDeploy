package build

import (
	"fmt"
	"nextdeploy/internal/git"
	"strings"
	"time"
)

// imageNameBuilder handles the construction of Docker image names with proper formatting
type Builder struct {
	baseName     string
	registry     string
	tag          string
	nameStrategy string
	useTimestamp bool
	branchName   string
}

// newImageNameBuilder creates a new image name builder with default values
func New(baseName, registry string) *Builder {
	return &Builder{
		baseName:     baseName,
		registry:     registry,
		nameStrategy: "commit-hash",
		useTimestamp: true,
	}
}

// WithTag sets the explicit tag for the image
func (b *Builder) WithTag(tag string) *Builder {
	b.tag = tag
	return b
}

// WithNameStrategy sets the naming strategy
func (b *Builder) WithNameStrategy(strategy string) *Builder {
	b.nameStrategy = strategy
	return b
}

// WithTimestamp controls whether to append timestamp
func (b *Builder) WithTimestamp(use bool) *Builder {
	b.useTimestamp = use
	return b
}

// WithBranch sets the branch name for branch-based strategies
func (b *Builder) WithBranch(branch string) *Builder {
	b.branchName = branch
	return b
}

// Build constructs the final image name according to the configured strategy
func (b *Builder) Build() (string, error) {
	// Validate base name
	if strings.TrimSpace(b.baseName) == "" {
		return "", fmt.Errorf("image base name cannot be empty")
	}

	// Sanitize base name
	b.baseName = strings.ToLower(b.baseName)
	b.baseName = strings.ReplaceAll(b.baseName, " ", "-")

	// Get or generate tag
	tag, err := b.generateTag()
	if err != nil {
		return "", fmt.Errorf("failed to generate tag: %w", err)
	}

	// Construct full image name
	imageName := b.baseName
	if b.registry != "" {
		imageName = strings.TrimSuffix(b.registry, "/") + "/" + imageName
	}

	return imageName + ":" + tag, nil
}

// generateTag creates the appropriate tag based on the naming strategy
func (b *Builder) generateTag() (string, error) {
	// Use explicit tag if provided
	if b.tag != "" {
		return b.tag, nil
	}

	// Generate tag based on strategy
	switch b.nameStrategy {
	case "commit-hash":
		return b.generateCommitHashTag()
	case "branch-commit":
		return b.generateBranchCommitTag()
	case "simple":
		return "latest", nil
	case "semver":
		return "", fmt.Errorf("semver strategy requires explicit --tag")
	default:
		return "", fmt.Errorf("unknown naming strategy: %s", b.nameStrategy)
	}
}

// generateCommitHashTag creates a tag with commit hash (and optional timestamp)
func (b *Builder) generateCommitHashTag() (string, error) {
	hash, err := git.GetCommitHash()
	if err != nil {
		return "", fmt.Errorf("failed to get git commit hash: %w", err)
	}

	if !b.useTimestamp {
		return hash, nil
	}

	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s-%s", hash, timestamp), nil
}

// generateBranchCommitTag creates a tag with branch name and commit hash
func (b *Builder) generateBranchCommitTag() (string, error) {
	branch := b.branchName
	if branch == "" {
		var err error
		branch, err = git.GetCurrentBranch()
		if err != nil {
			return "", fmt.Errorf("failed to get git branch: %w", err)
		}
	}

	// Sanitize branch name
	branch = strings.ToLower(branch)
	branch = strings.ReplaceAll(branch, "/", "-")
	branch = strings.ReplaceAll(branch, " ", "-")

	hash, err := git.GetCommitHash()
	if err != nil {
		return "", fmt.Errorf("failed to get git commit hash: %w", err)
	}

	tag := fmt.Sprintf("%s-%s", branch, hash)

	if b.useTimestamp {
		timestamp := time.Now().Format("20060102-150405")
		tag = fmt.Sprintf("%s-%s", tag, timestamp)
	}

	return tag, nil
}
