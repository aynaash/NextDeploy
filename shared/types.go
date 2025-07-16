package shared

import (
	"fmt"
	"strings"
	"time"
)

// EncryptedEnv represents the encrypted environment variables
type EncryptedEnv struct {
	KeyID        string            `json:"key_id"`         // Daemon's key ID used for encryption
	EnvBlob      string            `json:"env_blob"`       // Base64 encoded encrypted full .env content
	Variables    map[string]string `json:"variables"`      // Map of encrypted individual variables
	Nonce        string            `json:"nonce"`          // Base64 encoded nonce used for encryption
	Timestamp    time.Time         `json:"timestamp"`      // When the payload was created
	CLIPublicKey string            `json:"cli_public_key"` // Base64 encoded CLI's ECDH public key
}

type Envelope struct {
	Payload   string `json:"payload"`   // JSON string of EncryptedEnv
	Signature string `json:"signature"` // Base64 encoded signature of the payload
}

// PublicKeyResponse is the response from the daemon's /public-key endpoint
type PublicKeyResponse struct {
	KeyID      string `json:"key_id"`      // Identifier for the key
	PublicKey  string `json:"public_key"`  // Base64 encoded ECDH public key
	SignPublic string `json:"sign_public"` // Base64 encoded Ed25519 public key
}

// TrustedKey represents a trusted daemon public key stored by the CLI
type TrustedKey struct {
	KeyID       string `json:"key_id"`
	PublicKey   string `json:"public_key"`
	SignPublic  string `json:"sign_public"`
	Fingerprint string `json:"fingerprint"`
}

// TrustStore is a collection of trusted keys
type TrustStore struct {
	Keys []TrustedKey `json:"keys"`
}

// EnvFile represents a parsed .env file
type EnvFile struct {
	Variables map[string]string
	Raw       []byte
}

func ParseEnvFile(content []byte) (*EnvFile, error) {
	// Simplified parser
	variables := make(map[string]string)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and components
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			SharedLogger.Error("Invalid line in .env file: %s", line)
			return nil, fmt.Errorf("invalid line in .env file: %s", line)

		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if key == "" || value == "" {
			SharedLogger.Error("Empty key or value in .env file: %s", line)
			return nil, fmt.Errorf("empty key or value in .env file: %s", line)
		}
		variables[key] = value
	}
	return &EnvFile{
		Variables: variables,
		Raw:       content,
	}, nil
}
