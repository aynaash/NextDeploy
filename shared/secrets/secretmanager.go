package secrets

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/aynaash/nextdeploy/shared"
	"github.com/aynaash/nextdeploy/shared/config"
	"github.com/aynaash/nextdeploy/shared/envstore"
)

var (
	SLogs = shared.PackageLogger("Secrets::", "🔐 Secrets Manager::")
)

type SecretManager struct {
	keyPath  string
	envFile  string
	cfg      *config.NextDeployConfig
	secrets  map[string]*Secret
	keyCache map[string][]byte
	manager  *providerManager
	mu       sync.RWMutex
}

// CurrentEncryptionAlg identifies the encryption scheme stored in Secret.Alg.
const CurrentEncryptionAlg = "aes-256-gcm-v1"

// Secret represents a stored secret value and its encryption metadata.
type Secret struct {
	Value       string `json:"value"`
	Version     int    `json:"version"`
	CreatedAt   int64  `json:"created_at"`
	ModifiedAt  int64  `json:"modified_at"`
	IsEncrypted bool   `json:"is_encrypted"`
	Alg         string `json:"alg,omitempty"`
	Salt        string `json:"salt,omitempty"`
	IV          string `json:"iv,omitempty"`
}

type providerManager struct {
	providers map[string]SecretProvider
}

type SecretProvider interface {
	GetSecret(key string) (string, error)
	SetSecret(key, value string) error
	DeleteSecret(key string) error
	ListSecrets() ([]string, error)
	Encrypt(data []byte, key string) ([]byte, error)
	Decrypt(data []byte, key string) ([]byte, error)
	GenerateMasterKey() (string, error)
	DeriveKey(key string) ([]byte, error)
	ValidateSecretFormat(secret string) error
}

type Option func(*SecretManager)

func WithConfig(cfg *config.NextDeployConfig) Option {
	return func(sm *SecretManager) {
		sm.cfg = cfg
	}
}

func WithKeyPath(path string) Option {
	return func(sm *SecretManager) {
		if path != "" {
			sm.keyPath = path
			return
		}

		homedir, err := os.UserHomeDir()
		if err != nil {
			SLogs.Error("Failed to get home directory: %v", err)
			return
		}

		appName := "default"
		if sm.cfg != nil && sm.cfg.App.Name != "" {
			appName = sm.cfg.App.Name
		}

		sm.keyPath = filepath.Join(homedir, ".nextdeploy", appName)
	}
}

func WithProvider(name string, provider SecretProvider) Option {
	return func(sm *SecretManager) {
		sm.ensureProviderManager()
		sm.manager.providers[name] = provider
	}
}

// WithEnvFile configures a dotenv file path to load when initializing SecretManager.
func WithEnvFile(path string) Option {
	return func(sm *SecretManager) {
		sm.envFile = path
	}
}

// LoadEnvFile reads key/value pairs from a dotenv file and loads valid keys
// into the SecretManager without overwriting existing entries.
func (sm *SecretManager) LoadEnvFile(path string) error {
	envData, err := envstore.ReadEnvFile(path)
	if err != nil {
		return err
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now().Unix()
	for k, v := range envData {
		if err := ValidateSecretName(k); err != nil {
			continue
		}
		if _, exists := sm.secrets[k]; exists {
			continue
		}
		sm.secrets[k] = &Secret{
			Value:      v,
			Version:    1,
			CreatedAt:  now,
			ModifiedAt: now,
		}
	}
	return nil
}

// NewSecretManager creates and initializes a SecretManager with the given options.
func NewSecretManager(opts ...Option) (*SecretManager, error) {
	sm := &SecretManager{
		secrets:  make(map[string]*Secret),
		keyCache: make(map[string][]byte),
	}

	for _, opt := range opts {
		opt(sm)
	}

	noConfigProvided := sm.cfg == nil
	if noConfigProvided {
		if err := sm.loadOrDefaultConfig(); err != nil {
			return nil, err
		}
	}

	if sm.envFile != "" {
		if err := sm.LoadEnvFile(sm.envFile); err != nil {
			return nil, err
		}
	} else if noConfigProvided {
		if _, err := os.Stat(".env"); err == nil {
			if loadErr := sm.LoadEnvFile(".env"); loadErr != nil {
				SLogs.Warn("Failed to load .env file: %v", loadErr)
			}
		}
	}

	return sm, nil
}

func (sm *SecretManager) loadOrDefaultConfig() error {
	if _, err := os.Stat(config.ConfigFile); err == nil {
		cfg, loadErr := config.Load()
		if loadErr != nil {
			return fmt.Errorf("%w: %w", ErrConfigNotFound, loadErr)
		}
		sm.cfg = cfg
		return nil
	}
	appName := os.Getenv("NEXTDEPLOY_APP_NAME")
	if appName == "" {
		appName = "default"
	}
	sm.cfg = &config.NextDeployConfig{App: config.AppConfig{Name: appName}}
	return nil
}

func (sm *SecretManager) ensureProviderManager() {
	if sm.manager == nil {
		sm.manager = &providerManager{
			providers: make(map[string]SecretProvider),
		}
	}
}
