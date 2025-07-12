package nextcore

type NextCorePayload struct {
	AppName          string                 `json:"app_name"`
	NextVersion      string                 `json:"next_version"`
	EnvVariables     map[string]string      `json:"env"`
	StaticRoutes     []string               `json:"static_routes"`
	Dynamic          []string               `json:"dymanic_routes"`
	BuildCommand     string                 `json:"build_command"`
	StartCommand     string                 `json:"start_command"`
	HasImageAssets   bool                   `json:"has_image_assets"`
	CDNEnabled       bool                   `json:"cdn_enabled"`
	Domain           string                 `json:"domain"`
	Port             int                    `json:"port"`
	Middleware       []string               `json:"middleware,omitempty"`
	StaticAssets     []string               `json:"static_assets,omitempty"`
	NextConfig       map[string]interface{} `json:"next_config,omitempty"`
	GitCommit        string                 `json:"git_commit,omitempty"`
	GitDirty         bool                   `json:"git_dirty,omitempty"`
	GeneratedAt      string                 `json:"generated_at,omitempty"`
	MetadataFile     string                 `json:"metadata_file,omitempty"`
	BuildLockFile    string                 `json:"build_lock_file,omitempty"`
	MetadataFilePath string                 `json:"metadata_file,omitempty"`
	AssetsOutputDir  string                 `json:"assets_output_dir,omitempty"`
	PublicDir        string                 `json:"public_dir,omitempty"`
}
