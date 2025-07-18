//go:build ignore
// +build ignore

// internal/server/preparation/manager.go
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
)

type NextBuildMetadata struct {
	BuildID               string      `json:"buildId"`
	BuildManifest         interface{} `json:"buildManifest"`
	AppBuildManifest      interface{} `json:"appBuildManifest"`
	PrerenderManifest     interface{} `json:"prerenderManifest"`
	RoutesManifest        interface{} `json:"routesManifest"`
	ImagesManifest        interface{} `json:"imagesManifest"`
	AppPathRoutesManifest interface{} `json:"appPathRoutesManifest"`
	ReactLoadableManifest interface{} `json:"reactLoadableManifest"`
	Diagnostics           []string    `json:"diagnostics"`
}

func main() {
	projectDir := "." // current directory, can be parameterized
	metadata, err := collectNextBuildMetadata(projectDir)
	if err != nil {
		log.Fatalf("Error collecting Next.js build metadata: %v", err)
	}

	// Print or process the collected metadata
	prettyMetadata, _ := json.MarshalIndent(metadata, "", "  ")
	fmt.Println(string(prettyMetadata))
}

func collectNextBuildMetadata(projectDir string) (*NextBuildMetadata, error) {
	// Run npm build
	cmd := exec.Command("npm", "run", "build")
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("npm build failed: %w", err)
	}

	// Path to .next directory
	nextDir := filepath.Join(projectDir, ".next")

	// Read BUILD_ID
	buildID, err := ioutil.ReadFile(filepath.Join(nextDir, "BUILD_ID"))
	if err != nil {
		return nil, fmt.Errorf("failed to read BUILD_ID: %w", err)
	}

	// Helper function to read and parse JSON files
	readJSON := func(filename string) (interface{}, error) {
		data, err := ioutil.ReadFile(filepath.Join(nextDir, filename))
		if err != nil {
			return nil, err
		}
		var result interface{}
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		return result, nil
	}

	// Collect all manifests
	buildManifest, _ := readJSON("build-manifest.json")
	appBuildManifest, _ := readJSON("app-build-manifest.json")
	prerenderManifest, _ := readJSON("prerender-manifest.json")
	routesManifest, _ := readJSON("routes-manifest.json")
	imagesManifest, _ := readJSON("images-manifest.json")
	appPathRoutesManifest, _ := readJSON("app-path-routes-manifest.json")
	reactLoadableManifest, _ := readJSON("react-loadable-manifest.json")

	// Collect diagnostics files
	var diagnostics []string
	diagnosticsDir := filepath.Join(nextDir, "diagnostics")
	if files, err := ioutil.ReadDir(diagnosticsDir); err == nil {
		for _, file := range files {
			diagnostics = append(diagnostics, file.Name())
		}
	}

	return &NextBuildMetadata{
		BuildID:               string(buildID),
		BuildManifest:         buildManifest,
		AppBuildManifest:      appBuildManifest,
		PrerenderManifest:     prerenderManifest,
		RoutesManifest:        routesManifest,
		ImagesManifest:        imagesManifest,
		AppPathRoutesManifest: appPathRoutesManifest,
		ReactLoadableManifest: reactLoadableManifest,
		Diagnostics:           diagnostics,
	}, nil
}
