Here's a complete Go implementation that either:
1. Executes `docker info` and processes the output directly, or
2. Accepts existing output as input for processing

```go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type DockerInfo struct {
	Client struct {
		Version    string `json:"version"`
		Context    string `json:"context"`
		DebugMode  bool   `json:"debug_mode"`
		Plugins    []Plugin `json:"plugins"`
	} `json:"client"`
	Server struct {
		Containers struct {
			Total   int `json:"total"`
			Running int `json:"running"`
			Paused  int `json:"paused"`
			Stopped int `json:"stopped"`
		} `json:"containers"`
		Images         int    `json:"images"`
		Version       string `json:"version"`
		StorageDriver string `json:"storage_driver"`
		LoggingDriver string `json:"logging_driver"`
		CgroupDriver  string `json:"cgroup_driver"`
	} `json:"server"`
	Host struct {
		OS           string `json:"os"`
		Kernel       string `json:"kernel"`
		Architecture string `json:"architecture"`
		CPUs         int    `json:"cpus"`
		Memory       string `json:"memory"`
	} `json:"host"`
}

type Plugin struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path,omitempty"`
}

// GetDockerInfo either executes 'docker info' or processes provided input
func GetDockerInfo(input string) (DockerInfo, error) {
	var output string
	var err error

	if input == "" {
		output, err = runDockerInfo()
		if err != nil {
			return DockerInfo{}, err
		}
	} else {
		output = input
	}

	return parseDockerInfo(output)
}

func runDockerInfo() (string, error) {
	cmd := exec.Command("docker", "info")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("failed to run docker info: %v", err)
	}

	return out.String(), nil
}

func parseDockerInfo(output string) (DockerInfo, error) {
	var info DockerInfo
	scanner := bufio.NewScanner(strings.NewReader(output))
	currentSection := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Handle section headers
		if strings.HasSuffix(line, ":") {
			currentSection = strings.TrimSuffix(line, ":")
			continue
		}

		// Parse line content
		switch currentSection {
		case "Client":
			parseClientInfo(line, &info)
		case "Server":
			parseServerInfo(line, &info)
		case "":
			// Handle host info that appears at the end
			parseHostInfo(line, &info)
		}
	}

	return info, nil
}

func parseClientInfo(line string, info *DockerInfo) {
	switch {
	case strings.HasPrefix(line, "Version:"):
		info.Client.Version = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "Context:"):
		info.Client.Context = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "Debug Mode:"):
		info.Client.DebugMode = strings.Contains(strings.ToLower(line), "true")
	case strings.HasPrefix(line, "buildx:"):
		parts := strings.SplitN(line, ":", 2)
		name := strings.TrimSpace(parts[0])
		rest := strings.TrimSpace(parts[1])
		
		plugin := Plugin{Name: name}
		
		// Parse plugin details
		for _, part := range strings.Split(rest, ",") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "Version:") {
				plugin.Version = strings.TrimSpace(strings.SplitN(part, ":", 2)[1])
			} else if strings.HasPrefix(part, "Path:") {
				plugin.Path = strings.TrimSpace(strings.SplitN(part, ":", 2)[1])
			}
		}
		
		info.Client.Plugins = append(info.Client.Plugins, plugin)
	}
}

func parseServerInfo(line string, info *DockerInfo) {
	switch {
	case strings.HasPrefix(line, "Containers:"):
		info.Server.Containers.Total = parseInt(strings.TrimSpace(strings.SplitN(line, ":", 2)[1]))
	case strings.HasPrefix(line, " Running:"):
		info.Server.Containers.Running = parseInt(strings.TrimSpace(strings.SplitN(line, ":", 2)[1]))
	case strings.HasPrefix(line, " Paused:"):
		info.Server.Containers.Paused = parseInt(strings.TrimSpace(strings.SplitN(line, ":", 2)[1]))
	case strings.HasPrefix(line, " Stopped:"):
		info.Server.Containers.Stopped = parseInt(strings.TrimSpace(strings.SplitN(line, ":", 2)[1]))
	case strings.HasPrefix(line, "Images:"):
		info.Server.Images = parseInt(strings.TrimSpace(strings.SplitN(line, ":", 2)[1]))
	case strings.HasPrefix(line, "Server Version:"):
		info.Server.Version = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "Storage Driver:"):
		info.Server.StorageDriver = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "Logging Driver:"):
		info.Server.LoggingDriver = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "Cgroup Driver:"):
		info.Server.CgroupDriver = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	}
}

func parseHostInfo(line string, info *DockerInfo) {
	switch {
	case strings.HasPrefix(line, "Operating System:"):
		info.Host.OS = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "Kernel Version:"):
		info.Host.Kernel = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "Architecture:"):
		info.Host.Architecture = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	case strings.HasPrefix(line, "CPUs:"):
		info.Host.CPUs = parseInt(strings.TrimSpace(strings.SplitN(line, ":", 2)[1]))
	case strings.HasPrefix(line, "Total Memory:"):
		info.Host.Memory = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
	}
}

func parseInt(s string) int {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	if err != nil {
		return 0
	}
	return i
}

func main() {
	// Option 1: Run docker info directly
	info, err := GetDockerInfo("")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Option 2: Process existing output
	// existingOutput := `...` // Your docker info output here
	// info, err := GetDockerInfo(existingOutput)

	// Print as JSON
	jsonData, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(jsonData))

	// Or print formatted text
	fmt.Println("\nFormatted Docker Info:")
	fmt.Printf("Client Version: %s\n", info.Client.Version)
	fmt.Printf("Server Version: %s\n", info.Server.Version)
	fmt.Printf("Host OS: %s\n", info.Host.OS)
	fmt.Printf("Containers: %d total (%d running, %d stopped)\n", 
		info.Server.Containers.Total,
		info.Server.Containers.Running,
		info.Server.Containers.Stopped)
}
```

### Features:

1. **Two modes of operation**:
   - Automatically runs `docker info` when no input is provided
   - Processes existing output when provided as input

2. **Structured parsing**:
   - Extracts client, server, and host information
   - Handles nested structures (like container counts)

3. **Output options**:
   - JSON output for machine readability
   - Formatted text output for human readability

4. **Error handling**:
   - Proper error handling for Docker command execution
   - Graceful handling of parsing errors

### Usage Examples:

1. **Run directly and get JSON output**:
```go
info, err := GetDockerInfo("")
```

2. **Process existing output**:
```go
existingOutput := `...` // Your docker info output
info, err := GetDockerInfo(existingOutput)
```

3. **Custom output formatting**:
```go
// Access parsed fields directly:
fmt.Printf("Docker Version: %s\n", info.Client.Version)
fmt.Printf("Running Containers: %d\n", info.Server.Containers.Running)
```

The code is structured to be easily extended with additional fields from the docker info output as needed.
