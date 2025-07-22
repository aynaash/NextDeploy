package websocket

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"github.com/gorilla/websocket"

	"sync"
	"time"
)

type WSClient struct {
	conn        *websocket.Conn
	mu          sync.Mutex
	agentID     string
	privateKey  *ecdsa.PrivateKey
	connected   bool
	pingPeriod  time.Duration
	writeWait   time.Duration
	pongWait    time.Duration
	authKey     *ecdsa.PrivateKey
	sessionKeys map[string]*ecdh.PrivateKey // Session-specific keys
	upgrader    websocket.Upgrader
}

type SecureMessage struct {
	IV         []byte `json:"iv"`         // Initialization vector for AES-GCM
	Ciphertext []byte `json:"ciphertext"` // Encrypted payload
	Tag        []byte `json:"tag"`        // Authentication tag
	Sequence   uint64 `json:"sequence"`   // Prevent replay attacks
}
