package main

import (
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"nextdeploy/shared"
	"time"
)

func EncryptWithPublicKey(data []byte, pubKey *ecdh.PublicKey) ([]byte, error) {
	// Generate an ephemeral private key for ECDH
	privKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ephemeral private key: %w", err)
	}

	// Perform ECDH key exchange to get shared secret
	sharedSecret, err := privKey.ECDH(pubKey)
	if err != nil {
		return nil, fmt.Errorf("ECDH key exchange failed: %w", err)
	}

	// Derive an encryption key from the shared secret using SHA-256
	key := sha256.Sum256(sharedSecret)

	// For demonstration - in real use, you'd use this key with an AEAD cipher
	// like AES-GCM or ChaCha20-Poly1305 for actual encryption
	// Here we're just returning the derived key as a placeholder
	// In production, you would:
	// 1. Generate a random nonce
	// 2. Encrypt the data using the key and nonce
	// 3. Return ciphertext + nonce

	return key[:], nil
}

// ParsePublicKeyFromPEM parses a PEM-encoded public key
func ParsePublicKeyFromPEM(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("failed to parse PEM block containing public key")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse DER encoded public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("key is not an RSA public key")
	}

	return rsaPub, nil
}
func EncryptAndSendEnv(env *shared.EnvFile, daemonKey *shared.TrustedKey, addr string) (*bytes.Buffer, error) {
	// Serialize the env file
	envData, err := json.Marshal(env)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal env file: %w", err)
	}

	// Encrypt the data using the daemon's public key
	encryptedData, err := EncryptWithPublicKey(envData, daemonKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt data: %w", err)
	}

	// Create the envelope
	envelope := shared.Envelope{
		Payload:   encryptedData,
		Signature: "",
	}
	envelopeBytes, err := json.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal envelope: %w", err)
	}

	// Create HTTP request
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", addr, bytes.NewBuffer(envelopeBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	// Read and return the response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return bytes.NewBuffer(responseBody), nil
}
