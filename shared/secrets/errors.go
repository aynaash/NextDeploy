package secrets

import "errors"

var (
	ErrSecretNotFound        = errors.New("secret not found")
	ErrDecryptionFailed      = errors.New("decryption failed")
	ErrEncryptionFailed      = errors.New("encryption failed")
	ErrInvalidConfig         = errors.New("invalid configuration")
	ErrInvalidSecretPath     = errors.New("invalid secret path")
	ErrEncryptFailed         = errors.New("encryption failed")
	ErrDecryptFailed         = errors.New("decryption failed")
	ErrDopplerNotConfigured  = errors.New("doppler integration not configured")
	ErrInvalidSecretFormat   = errors.New("invalid secret format")
	ErrUnsupportedPlatform   = errors.New("unsupported platform")
	ErrKeyGenerationFailed   = errors.New("key generation failed")
	ErrSecretAlreadyExists   = errors.New("secret already exists")
	ErrPermissionDenied      = errors.New("permission denied")
	ErrInvalidProvider       = errors.New("invalid secret provider")
	ErrProviderNotConfigured = errors.New("provider not configured")
	ErrConfigNotFound        = errors.New("configuration not found")
)
