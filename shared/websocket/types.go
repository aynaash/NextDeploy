package websocket

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"encoding/json"
	"github.com/gorilla/websocket"
	"math/rand"

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

func (c *WSClient) SendSecure(msg interface{}) error {
	// 1. Serialize message
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// 2. Encrypt using session key
	iv := make([]byte, 12)
	if _, err := rand.Read(iv); err != nil {
		return err
	}

	aes, err := aes.NewCipher(c.sessionKey)
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(aes)
	if err != nil {
		return err
	}

	ciphertext := gcm.Seal(nil, iv, data, nil)

	// 3. Send secure message
	secureMsg := SecureMessage{
		IV:         iv,
		Ciphertext: ciphertext[:len(ciphertext)-gcm.Overhead()],
		Tag:        ciphertext[len(ciphertext)-gcm.Overhead():],
		Sequence:   atomic.AddUint64(&c.sequence, 1),
	}

	return c.conn.WriteJSON(secureMsg)
}
