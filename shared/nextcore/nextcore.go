package nextcore

import (
	"encoding/json"
	"fmt"
	"nextdeploy/shared"
	"nextdeploy/shared/config"
	"os"
	"os/exec"
	"path/filepath"
)

const (
	BuildLockFileName = ".nextdeploy/build.lock"
	MetadataFileName  = ".nextdeploy/metadata.json"
	AssetsOutputDir   = ".nextdeploy/assets"
	PublicDir         = "public"
)

var (
	NextCoreLogger = shared.PackageLogger("nextcore", "📦 NEXTCORE")
)

func GenerateMetadata() (NextCorePayload, error) {
	// This function will generate metadata for the Next.js application
	// and return a NextCorePayload with the necessary fields filled.
	// For now, we will just return an empty payload.
	NextCoreLogger.Info("Generating metadata for Next.js application...")
	//Get the app name
	cfg, err := config.Load()
	if err != nil {
		NextCoreLogger.Error("Failed to load configuration: %v", err)
		return NextCorePayload{}, err
	}

	AppName := cfg.App.Name

	// Get the nextjs version
	NextJsVersion, err := GetNextJsVersion("package.json")
	if err != nil {
		NextCoreLogger.Error("Failed to get Next.js version: %v", err)
		return NextCorePayload{}, err
	}
	NextCoreLogger.Info("Next.js version: %s", NextJsVersion)
	// get the build meta data
	NextCoreLogger.Info("Collecting build metadata...")
	buildMeta, err := CollectBuildMetadata()
	NextCoreLogger.Debug("The build metadata looks like this:%v", buildMeta)
	// add config data to the metadata also
	config, err := config.Load()

	return NextCorePayload{
		AppName:           AppName,
		NextVersion:       NextJsVersion,
		NextBuildMetadata: *buildMeta,
		Config:            config,
	}, nil
}

func CollectBuildMetadata() (*NextBuildMetadata, error) {
	projectDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	//TODO: use detected package manager
	NextCoreLogger.Debug("Build the next to generate build metadata")
	cmd := exec.Command("npm", "run", "build")
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
