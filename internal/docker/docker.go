package docker

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

const (
	dockerfileName          = "Dockerfile"
	defaultDockerfileContent = `
# ---------- STAGE 1: Build ----------
FROM node:20-alpine AS builder

WORKDIR /app

COPY package.json ./
COPY yarn.lock ./

RUN yarn install

COPY . .

RUN yarn run build

# ---------- STAGE 2: Runtime ----------
FROM node:20-alpine

WORKDIR /app

COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

RUN adduser -D nextjs && chown -R nextjs:nextjs /app
USER nextjs

EXPOSE 3000
ENV PORT=3000

CMD ["node", "server.js"]
	`
)

// DockerfileExists checks if a Dockerfile exists in the current directory
func DockerfileExists() bool {
	path := filepath.Join(".", dockerfileName)
	fmt.Printf("Checking for Dockerfile at: %s\n", path)
	_, err := os.Stat(path)
	exists := !os.IsNotExist(err)
	fmt.Printf("Dockerfile exists: %v\n", exists)
	return exists
}

// GenerateDockerfile creates a new Dockerfile with default or custom content
func GenerateDockerfile(content string) error {
	fmt.Println("Generating Dockerfile...")
	if DockerfileExists() {
		return errors.New("dockerfile already exists")
	}

	if content == "" {
		content = defaultDockerfileContent
		fmt.Println("Using default Dockerfile content")
	} else {
		fmt.Println("Using custom Dockerfile content")
	}

	return WriteDockerfile(content)
}

// WriteDockerfile writes content to Dockerfile
func WriteDockerfile(content string) error {
	fmt.Printf("Writing Dockerfile with content:\n%s\n", content)
	err := os.WriteFile(dockerfileName, []byte(content), 0644)
	if err != nil {
		fmt.Printf("Error writing Dockerfile: %v\n", err)
	} else {
		fmt.Println("Dockerfile written successfully")
	}
	return err
}

var imageNameRegex = regexp.MustCompile(`^([a-z0-9]+(?:[._-][a-z0-9]+)*)(?:/([a-z0-9]+(?:[._-][a-z0-9]+)*))*(?::([a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}))?$`)

// ValidateImageName checks if a Docker image name is valid
func ValidateImageName(name string) error {
	fmt.Printf("Validating image name: %s\n", name)
	
	if len(name) == 0 {
		return errors.New("image name cannot be empty")
	}
	
	if len(name) > 255 {
		return errors.New("image name exceeds 255 characters")
	}

	if name[0] == '.' || name[0] == '_' || name[0] == '-' || name[0] == '/' || name[0] == ':' {
		return errors.New("image name cannot start with a separator")
	}

	if name[len(name)-1] == '.' || name[len(name)-1] == '_' || name[len(name)-1] == '-' || name[len(name)-1] == '/' || name[len(name)-1] == ':' {
		return errors.New("image name cannot end with a separator")
	}

	if matched := regexp.MustCompile(`[._\-]{2,}|/{2,}|:{2,}`).MatchString(name); matched {
		return errors.New("image name cannot contain consecutive separators")
	}

	if parts := regexp.MustCompile(`:`).Split(name, 2); len(parts) == 2 {
		if len(parts[1]) > 128 {
			return errors.New("tag cannot exceed 128 characters")
		}
	}

	if !imageNameRegex.MatchString(name) {
		return errors.New("invalid image name format")
	}

	fmt.Println("Image name validation passed")
	return nil
}

// CheckDockerInstalled verifies Docker is available
func CheckDockerInstalled() error {
	fmt.Println("Checking if Docker is installed...")
	path, err := exec.LookPath("docker")
	if err != nil {
		fmt.Println("Docker not found in PATH")
		return errors.New("docker not found in PATH")
	}
	fmt.Printf("Docker found at: %s\n", path)
	return nil
}

// BuildImage builds a Docker image
func BuildImage(ctx context.Context, imageName string, noCache bool) error {
	fmt.Printf("Building Docker image: %s (no-cache: %v)\n", imageName, noCache)
	
	if err := ValidateImageName(imageName); err != nil {
		return fmt.Errorf("invalid image name: %w", err)
	}

	if err := CheckDockerInstalled(); err != nil {
		return err
	}

	if !DockerfileExists() {
		return errors.New("dockerfile not found in current directory")
	}

	args := []string{"build", "-t", imageName, "."}
	if noCache {
		args = append(args, "--no-cache")
	}

	fmt.Printf("Executing docker command with args: %v\n", args)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error building image: %v\n", err)
	} else {
		fmt.Println("Image built successfully")
	}
	return err
}

// PushImage pushes a Docker image to registry
func PushImage(ctx context.Context, imageName string) error {
	fmt.Printf("Pushing Docker image: %s\n", imageName)
	
	if err := ValidateImageName(imageName); err != nil {
		return fmt.Errorf("invalid image name: %w", err)
	}

	if err := CheckDockerInstalled(); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "docker", "push", imageName)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		fmt.Printf("Error pushing image: %v\n", err)
	} else {
		fmt.Println("Image pushed successfully")
	}
	return err
}
