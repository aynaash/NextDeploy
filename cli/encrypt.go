package main

import (
	"bytes"
	"crypto/ecdh"
	"encoding/json"
	"fmt"
	"net/http"
	"nextdeploy/shared"
	"time"
)

func EncryptAndSendEnv(env *shared.EnvFile, daemonKey *shared.TrustedKey, daemonAddr string) error {
	// Generate ephemeral key pair for this session
	keyPair, err := shared.GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate session key pair: %v", err)
	}

	// Decode daemon's public key
	daemonPubBytes, err := shared.DecodeFromBase64(daemonKey.PublicKey)
	if err != nil {
		return fmt.Errorf("invalid daemon public key: %v", err)
	}

	curve := ecdh.X25519()
	daemonPublic, err := curve.NewPublicKey(daemonPubBytes)
	if err != nil {
		return fmt.Errorf("invalid daemon public key format: %v", err)
	}

	// Derive shared key
	sharedKey, err := shared.DeriveSharedKey(keyPair.ECDHPrivate, daemonPublic)
	if err != nil {
		return fmt.Errorf("failed to derive shared key: %v", err)
	}

	// Encrypt the full env blob
	encryptedBlob, nonce, err := shared.Encrypt(env.Raw, sharedKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt env blob: %v", err)
	}

	// Encrypt individual variables
	encryptedVars := make(map[string]string)
	for key, value := range env.Variables {
		encryptedValue, _, err := shared.Encrypt([]byte(value), sharedKey)
		if err != nil {
			return fmt.Errorf("failed to encrypt variable %s: %v", key, err)
		}
		encryptedVars[key] = shared.EncodeToBase64(encryptedValue)
	}

	// Prepare the payload
	encryptedEnv := shared.EncryptedEnv{
		KeyID:        daemonKey.KeyID,
		EnvBlob:      shared.EncodeToBase64(encryptedBlob),
		Variables:    encryptedVars,
		Nonce:        shared.EncodeToBase64(nonce),
		Timestamp:    time.Now(),
		CLIPublicKey: shared.EncodeToBase64(keyPair.ECDHPublic.Bytes()),
	}

	// Serialize the payload for signing
	payloadBytes, err := json.Marshal(encryptedEnv)
	if err != nil {
		return fmt.Errorf("failed to serialize payload: %v", err)
	}

	// Sign the payload
	signature, err := shared.Sign(payloadBytes, keyPair.SignPrivate)
	if err != nil {
		return fmt.Errorf("failed to sign payload: %v", err)
	}

	// Create the envelope
	envelope := shared.Envelope{
		Payload:   string(payloadBytes),
		Signature: shared.EncodeToBase64(signature),
	}

	// Send to daemon
	envelopeBytes, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to serialize envelope: %v", err)
	}

	resp, err := http.Post(
		fmt.Sprintf("%s/submit-env", daemonAddr),
		"application/json",
		bytes.NewBuffer(envelopeBytes),
	)
	if err != nil {
		return fmt.Errorf("failed to send envelope: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	return nil
}
