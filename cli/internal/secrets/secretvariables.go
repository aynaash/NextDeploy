package secrets

import (
	"os"
	"path/filepath"
)

func (sm *SecretManager) GetAppName() string {
	if sm.cfg != nil && sm.cfg.App.Name != "" {
		return sm.cfg.App.Name
	}
	return "default"
}

func (sm *SecretManager) GetKey() string {
	if sm.cfg == nil && sm.cfg.App.Name == "" {
		return "nokey"
	}
	homedir, err := os.UserHomeDir()
	if err != nil {
		SLogs.Error("Failed to get home directory: %v", err)
		return "nokey"
	}
	return filepath.Join(homedir, ".nextdeploy", sm.cfg.App.Name, "master.key")

}
