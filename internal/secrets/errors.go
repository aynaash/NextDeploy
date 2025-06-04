package secrets 
import "errors"

var (
	ErrSecretNotFound = errors.New("secret not found")
	ErrDecryptionFailed = errors.New("decryption failed")
	ErrEncryptionFailed = errors.New("encryption failed")
	ErrInvalidConfig = errors.New("invalid configuration")
	ErrInvalidSecretPath = errors.New("invalid secret path")
)



