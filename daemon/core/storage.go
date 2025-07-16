package core

import (
	"encoding/json"
	"fmt"
	"nextdeploy/shared"
	"os"
	"path/filepath"
	"sync"
)

type EnvStorage struct {
	storageDir string
	mu         sync.Mutex
}

func NewEnvStorage(storageDir string) (*EnvStorage, error) {
	if err := os.MkdirAll(storageDir, 0700); err != nil {
		return nil, err
	}
	return &EnvStorage{
		storageDir: storageDir,
	}, nil
}

func (s *EnvStorage) Save(env *shared.EncryptedEnv, plaintext string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// creaet a file name base on timestamp and key id
	filename := filepath.Join(s.storageDir, fmt.Sprintf("%d_%s.env", env.Timestamp.Unix(), env.KeyID))
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create env file: %w", err)
	}
	defer file.Close()
	data := struct {
		Encrypted *shared.EncryptedEnv `json:"encrypted"`
		Plaintext string               `json:"plaintext"`
	}{
		Encrypted: env,
		Plaintext: string(plaintext),
	}

	return json.NewEncoder(file).Encode(data)
}
