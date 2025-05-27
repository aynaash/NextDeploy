
package secrets

import (
	"fmt"
	"os"
	"path/filepath"
)

func StoreToken(provider, token string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	tokenDir := filepath.Join(home, ".nextdeploy", "tokens")
	if err := os.MkdirAll(tokenDir, 0700); err != nil {
		return err
	}

	tokenFile := filepath.Join(tokenDir, fmt.Sprintf("%s.token", provider))
	return os.WriteFile(tokenFile, []byte(token), 0600)
}
