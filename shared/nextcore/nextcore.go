package nextcore

import (
	"nextdeploy/shared/config"
	"nextdeploy/shared"
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

	// Get the environment variables
	// TODO: e
	//      -- integrate envstore and secrets manager and encrypt mini doppler extensible
	//      -- version that can be used in production using knowledge learned from bootdev

	return NextCorePayload{
		AppName:     AppName,
		NextVersion: NextJsVersion,
	}, nil
}
func (p *NextCorePayload) CollectMetaForDockerBuild() (metadata NextCorePayload, err error) {
	// This function will collect metadata for Docker build
	// and return a NextCorePayload with the necessary fields filled.
	// For now, we will just return the payload as is.
	AppName := "contextbytes"
	NextVersion := "15.2.0"
	EnvVariables := map[string]string{
		"NODE_ENV":                 "production",
		"NEXT_PUBLIC_API_URL":      "https://api.example.com",
		"NEXT_PUBLIC_ANALYTICS_ID": "UA-123456789-1",
		"NEXT_PUBLIC_CDN_URL":      "https://cdn.example.com",
		"NEXT_PUBLIC_FEATURE_FLAG": "true",
		"NEXT_PUBLIC_CUSTOM_VAR":   "custom_value",
		"NEXT_PUBLIC_ANOTHER_VAR":  "another_value",
	}
	StaticRoutes := []string{
		"/",
		"/about",
		"/contact",
		"/blog",
		"/products",
		"/services",
		"/terms",
		"/privacy",
		"/sitemap.xml",
	}
	Dynamic := []string{
		"/api/data",
		"/api/user",
	}
	BuildCommand := "npm run build"
	StartCommand := "npm start"
	HasImageAssets := true
	CDNEnabled := true
	Domain := "contextbytes.com"
	Port := 3000

	return NextCorePayload{
		AppName:        AppName,
		NextVersion:    NextVersion,
		EnvVariables:   EnvVariables,
		StaticRoutes:   StaticRoutes,
		Dynamic:        Dynamic,
		BuildCommand:   BuildCommand,
		StartCommand:   StartCommand,
		HasImageAssets: HasImageAssets,
		CDNEnabled:     CDNEnabled,
		Domain:         Domain,
		Port:           Port,
	}, err
}
