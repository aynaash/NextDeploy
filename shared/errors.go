package shared

import (
	"errors"
)

var (
	ErrNilKey                = errors.New("crypto: nil key provided")
	ErrInvalidClientID       = errors.New("auth: invalid client ID")
	ErrInvalidSigningMethod  = errors.New("auth: invalid signing method")
	ErrKeyMismatch           = errors.New("auth: key ID mismatch")
	ErrInvalidAudience       = errors.New("auth: invalid audience")
	ErrInvalidToken          = errors.New("auth: invalid token")
	ErrAuthKeyNotInitialized = errors.New("auth: WebSocket auth key not initialized")
	ErrEmptyClientID         = errors.New("auth: empty client ID")
)
