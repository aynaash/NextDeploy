package nextcore

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func CollectBuildMetadata() (*NextBuildMetadata, error) {
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	NextCoreLogger.Debug("Build the next to generate build metadata")
	PackageManager, err := DetectPackageManager(projectDir)
	if err != nil {
		PackageManager = "npm"
	}
	buildCommand, err := buildCommand(string(PackageManager))
	if err != nil {
		PackageManager = "npm"
	}
	if err := os.MkdirAll(".nextdeploy", 0755); err != nil {
		return nil, fmt.Errorf("failed to create .nextdeploy directory: %w", err)
	}
	cmd := exec.Command("sh", "-c", buildCommand.String())
	cmd.Dir = projectDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("build failed:%w", err)
	}

	nextDir := filepath.Join(projectDir, ".next")
	buildID, err := os.ReadFile(filepath.Join(nextDir, "BUILD_ID"))
	if err != nil {
		return nil, fmt.Errorf("faileed to read BUILD_ID:%s", err)
	}
	readJSON := func(filename string) (interface{}, error) {
		data, err := os.ReadFile(filepath.Join(nextDir, filename))
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
	var diagnostics []string
	diagnosticsDir := filepath.Join(nextDir, "diagnostics")
	if files, err := os.ReadDir(diagnosticsDir); err != nil {
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
