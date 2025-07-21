package shared

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"
)

type SecureMessage struct {
	IV         []byte `json:"iv"`
	Ciphertext []byte `json:"ciphertext"`
	Tag        []byte `json:"tag"`
	Sequence   uint64 `json:"sequence"`
	Timestamp  int64  `json:"timestamp"`
}

type MessageHeader struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
}

func EncryptMessage(key []byte, sequence uint64, payload interface{}) ([]byte, error) {
	// Serialize payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Create AES-GCM cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nil, nonce, payloadBytes, nil)

	// Split ciphertext and tag
	tagStart := len(ciphertext) - gcm.Overhead()
	msg := SecureMessage{
		IV:         nonce,
		Ciphertext: ciphertext[:tagStart],
		Tag:        ciphertext[tagStart:],
		Sequence:   sequence,
		Timestamp:  time.Now().Unix(),
	}

	return json.Marshal(msg)
}

func DecryptMessage(key []byte, data []byte) ([]byte, uint64, error) {
	var msg SecureMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, 0, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Verify timestamp (prevent replay)
	if time.Since(time.Unix(msg.Timestamp, 0)) > 30*time.Second {
		return nil, 0, errors.New("message too old")
	}

	// Reconstruct ciphertext
	fullCiphertext := append(msg.Ciphertext, msg.Tag...)

	// Create AES-GCM cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt data
	plaintext, err := gcm.Open(nil, msg.IV, fullCiphertext, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, msg.Sequence, nil
}
