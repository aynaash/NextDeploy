package shared

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// GenerateCommandID creates a unique ID for command tracking
func GenerateCommandID() string {
	b := make([]byte, 16) // 128-bit ID
	if _, err := rand.Read(b); err != nil {
		// Fallback to timestamp-based ID if crypto fails
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
