To access Docker-related data like the image name from the `nextdeploy.yml` configuration using the Go package I provided, you can directly access the struct fields. Here's how you can do it:

## Accessing Docker Configuration

The Docker configuration is stored in the `Docker` field of the `Config` struct. Here are examples of how to access different Docker-related values:

```go
package main

import (
	"fmt"
	"log"
	"path/to/nextdeploy" // Replace with your actual package path
)

func main() {
	// Load the config
	config, err := nextdeploy.Load("nextdeploy.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Access Docker image name
	imageName := config.Docker.Image
	fmt.Printf("Docker image: %s\n", imageName)

	// Access Docker registry
	registry := config.Docker.Registry
	fmt.Printf("Docker registry: %s\n", registry)

	// Access whether to push after build
	pushImage := config.Docker.Push
	fmt.Printf("Push image after build: %t\n", pushImage)

	// Access build context
	buildContext := config.Docker.Build.Context
	fmt.Printf("Build context: %s\n", buildContext)

	// Access Dockerfile path
	dockerfile := config.Docker.Build.Dockerfile
	fmt.Printf("Dockerfile: %s\n", dockerfile)

	// Access build arguments
	for key, value := range config.Docker.Build.Args {
		fmt.Printf("Build arg: %s=%s\n", key, value)
	}

	// Access no-cache setting
	noCache := config.Docker.Build.NoCache
	fmt.Printf("No cache: %t\n", noCache)
}
```

## Updating Docker Configuration

You can also update these values:

```go
func updateDockerConfig(config *nextdeploy.Config) {
	// Update the image name
	config.Docker.Image = "myorg/myapp:latest"

	// Update the registry
	config.Docker.Registry = "docker.io"

	// Update push setting
	config.Docker.Push = false

	// Update build context
	config.Docker.Build.Context = "./app"

	// Add a new build argument
	config.AddDockerBuildArg("NODE_ENV", "production")

	// Remove a build argument
	config.RemoveDockerBuildArg("SOME_ARG")

	// Enable no-cache builds
	config.Docker.Build.NoCache = true
}
```

## Accessing Container Configuration (Deployment)

If you want to access the container configuration (which is part of the deployment section), you can do it like this:

```go
func accessContainerConfig(config *nextdeploy.Config) {
	// Container name
	containerName := config.Deployment.Container.Name
	fmt.Printf("Container name: %s\n", containerName)

	// Restart policy
	restartPolicy := config.Deployment.Container.Restart
	fmt.Printf("Restart policy: %s\n", restartPolicy)

	// Environment file
	envFile := config.Deployment.Container.EnvFile
	fmt.Printf("Environment file: %s\n", envFile)

	// Volumes
	for _, volume := range config.Deployment.Container.Volumes {
		fmt.Printf("Volume: %s\n", volume)
	}

	// Port mappings
	for _, port := range config.Deployment.Container.Ports {
		fmt.Printf("Port: %s\n", port)
	}

	// Healthcheck configuration
	hc := config.Deployment.Container.Healthcheck
	fmt.Printf("Healthcheck path: %s\n", hc.Path)
	fmt.Printf("Healthcheck interval: %s\n", hc.Interval)
	fmt.Printf("Healthcheck timeout: %s\n", hc.Timeout)
	fmt.Printf("Healthcheck retries: %d\n", hc.Retries)
}
```

## Helper Methods

The package already includes some helper methods for common operations:

```go
// Add a container volume
err := config.AddContainerVolume("/host/path:/container/path")
if err != nil {
    fmt.Println("Failed to add volume:", err)
}

// Remove a container volume
err = config.RemoveContainerVolume("/host/path:/container/path")
if err != nil {
    fmt.Println("Failed to remove volume:", err)
}

// Add a port mapping
err = config.AddContainerPort("8080:80")
if err != nil {
    fmt.Println("Failed to add port:", err)
}

// Remove a port mapping
err = config.RemoveContainerPort("8080:80")
if err != nil {
    fmt.Println("Failed to remove port:", err)
}
```

Remember that all these fields are public (start with uppercase letter) in the Go structs, so you can directly access and modify them. The helper methods are provided for convenience and to handle some common operations with additional checks.
