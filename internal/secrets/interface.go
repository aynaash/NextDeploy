package secrets
import (
	"fmt"
	"strings"
	"encoding/base64"
)
type NDeploy struct {
    secretManager  *SecretManager
		config         *Config
		doppler        *DopplerManager
}

func New(sm *SecretManager, doppler *DopplerManager, configPath string, masterKey string) (*NDeploy, error) {
    fmt.Println("Creating NDeploy with configPath:", configPath, "and masterKey:", masterKey)
    fmt.Println("The secret manager structure looks like this :%v", &SecretManager{})
    
    cfg, err := sm.LoadConfig()
    if err != nil {
        return nil, err
    }
    
    secretManager, err := NewSecretManager(configPath, masterKey, cfg)
    if err != nil {
        return nil, fmt.Errorf("failed to create secret manager: %w", err)
    }
    
    // Remove this line as it's causing the type mismatch
    // doppler = NewDopplerManager(doppler)
    
    // Instead, just use the doppler manager passed in
    secretManager.doppler = doppler
    
    if secretManager == nil {
        return nil, fmt.Errorf("secret manager is nil")
    }
    if secretManager.doppler == nil {
        return nil, fmt.Errorf("doppler manager is nil")
    }
    
    return &NDeploy{
        secretManager: secretManager,
        config:        cfg,
        doppler:       doppler,  // Store the doppler manager in NDeploy as well
    }, nil
}


func (nd *NDeploy) SyncWithDoppler()error {
	//NOTE: should take the nd.config to push secrets 
	return nd.secretManager.doppler.PushSecrets()
}
func (nd *NDeploy) GetSecret(path string) (string, error) {
    return nd.secretManager.GetSecret(path)
}

func (nd *NDeploy) UpdateSecret(path string, value string, encrypt bool) error {
    return nd.secretManager.UpdateSecret(path, value, encrypt)
}

func (nd *NDeploy) DeleteSecret(path string) error {
    return nd.secretManager.DeleteSecret(path)
}

func (nd *NDeploy) EncryptValue(value string) (string, error) {
    encrypted, err := Encrypt([]byte(value), nd.secretManager.masterKey)
    if err != nil {
        return "", err
    }
    return "enc:" + base64.StdEncoding.EncodeToString(encrypted), nil
}

func (nd *NDeploy) DecryptValue(encryptedValue string) (string, error) {
    if !strings.HasPrefix(encryptedValue, "enc:") {
        return encryptedValue, nil
    }

    data, err := base64.StdEncoding.DecodeString(encryptedValue[4:])
    if err != nil {
        return "", err
    }

    decrypted, err := Decrypt(data, nd.secretManager.masterKey)
    if err != nil {
        return "", err
    }

    return string(decrypted), nil
}
