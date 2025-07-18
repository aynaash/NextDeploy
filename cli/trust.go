package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"nextdeploy/shared"
	"os"
	"path/filepath"
	"time"
)

// FIX: this logic should exist on both daemon and cli sides
func LoadTrustStore(path string) (*shared.TrustStore, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &shared.TrustStore{Keys: []shared.TrustedKey{}}, nil // Initialize if not exists
		}
		return nil, fmt.Errorf("failed to open trust store: %w", err)
	}
	defer file.Close()

	var store shared.TrustStore
	if err := json.NewDecoder(file).Decode(&store); err != nil {
		return nil, fmt.Errorf("failed to decode trust store: %w", err)
	}
	return &store, nil
}

type CLI struct {
	configDir string
	keyPair   *shared.KeyPair
}

func SaveTrustStore(path string, store *shared.TrustStore) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create trust store file: %w", err)
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(store); err != nil {
		return fmt.Errorf("failed to encode trust store: %w", err)
	}
	return nil
}

func (c *CLI) BootstrapTrust(daemonAddr, email string) error {
	// 1. Fetch daemon's public key
	daemonKey, err := c.fetchDaemonPublicKey(daemonAddr)
	if err != nil {
		return fmt.Errorf("failed to fetch daemon key: %v", err)
	}

	// 2. Verify fingerprint with user
	fmt.Printf("Daemon fingerprint: %s\n", daemonKey)
	fmt.Printf("Please verify this fingerprint with the daemon administrator.\n")
	fmt.Printf("Do you want to trust this key? (yes/no): ")
	var answer string
	if _, err := fmt.Scanln(&answer); err != nil {
		return fmt.Errorf("failed to read user input: %v", err)
	}
	if answer != "yes" {
		return errors.New("user declined to trust the daemon public key")
	}
	// Verify the public key matches the expected format
	// 3. Save daemon's fingerprint locally
	if err := c.saveDaemonFingerprint(daemonAddr, daemonKey); err != nil {
		return fmt.Errorf("failed to save daemon fingerprint: %v", err)
	}

	// 4. Send our public key to the daemon to be added as owner
	if err := c.registerCLIIdentity(daemonAddr, email, daemonKey); err != nil {
		return fmt.Errorf("failed to register CLI identity: %v", err)
	}

	return nil
}
func (c *CLI) fetchDaemonPublicKey(daemonAddr string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/public-key", daemonAddr))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	var pubKeyResp shared.PublicKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&pubKeyResp); err != nil {
		return "", err
	}

	// Compute fingerprint
	pubKeyBytes, err := shared.DecodeFromBase64(pubKeyResp.PublicKey)
	if err != nil {
		return "", err
	}
	Fingerprint, err := shared.GenerateFingerprint(pubKeyBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate fingerprint: %w", err)
	}
	return Fingerprint, nil
}

func (c *CLI) saveDaemonFingerprint(daemonAddr string, key string) error {
	trustFile := filepath.Join(c.configDir, "trusted_daemons.json")

	var trusted map[string]string
	if data, err := os.ReadFile(trustFile); err == nil {
		if err := json.Unmarshal(data, &trusted); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if trusted == nil {
		trusted = make(map[string]string)
	}
	//FIX:
	trusted[daemonAddr] = key

	data, err := json.Marshal(trusted)
	if err != nil {
		return err
	}

	return os.WriteFile(trustFile, data, 0600)
}

func (c *CLI) registerCLIIdentity(daemonAddr, email string, daemonKey string) error {
	// Prepare our identity
	pubKey := shared.EncodeToBase64(c.keyPair.ECDHPublic.Bytes())
	signPub := shared.EncodeToBase64(c.keyPair.SignPublic)
	fingerprint, err := shared.GenerateFingerprint(c.keyPair.ECDHPublic.Bytes())
	if err != nil {
		return fmt.Errorf("failed to generate fingerprint: %w", err)
	}

	identity := shared.Identity{
		Fingerprint: fingerprint,
		PublicKey:   pubKey,
		SignPublic:  signPub,
		Role:        shared.RoleOwner,
		Email:       email,
		AddedBy:     "bootstrap",
		CreatedAt:   time.Now(),
	}

	// Create signed request
	body, err := json.Marshal(identity)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/add-identity", daemonAddr), bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	// Sign the request
	message := fmt.Sprintf("%s %s %s", req.Method, req.URL.Path, string(body))
	signature, err := shared.Sign([]byte(message), c.keyPair.SignPrivate)
	if err != nil {
		return err
	}

	req.Header.Set("X-Signature", shared.EncodeToBase64(signature))
	req.Header.Set("X-Fingerprint", fingerprint)

	// Send request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("failed to register identity: %s", resp.Status)
	}

	return nil
}

func FetchDaemonPublicKey(daemonAddr string, store *shared.TrustStore) (*shared.TrustedKey, error) {
	resp, err := http.Get(daemonAddr + "/public-key")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch daemon public key: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("daemon returned non-200 status: %s", resp.Status)
	}
	var pubKeyResp shared.PublicKeyResponse
	if err := json.NewDecoder(resp.Body).Decode(&pubKeyResp); err != nil {
		return nil, fmt.Errorf("failed to decode public key response: %w", err)
	}
	// check if we already trust this key
	for _, trustedKey := range store.Keys {
		if trustedKey.KeyID == pubKeyResp.KeyID {
			// verify it matches  out copy
			if trustedKey.PublicKey != pubKeyResp.PublicKey || trustedKey.SignPublic != pubKeyResp.SignPublic {
				return nil, fmt.Errorf("public key mismatch for key ID %s", pubKeyResp.KeyID)
			}
			return &trustedKey, nil
		}
	}
	// new key promot user to verify fingerprint
	decodedPubKey, err := shared.DecodeFromBase64(pubKeyResp.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %w", err)
	}
	fingerprint, _ := shared.GenerateFingerprint(decodedPubKey)
	fmt.Printf("New daemon public key detected:\n")
	fmt.Printf("Key ID: %s\n", pubKeyResp.KeyID)
	fmt.Printf("Fingerprint: %s\n", fingerprint)
	fmt.Printf("Please verify this fingerprint with the daemon administrator.\n")
	fmt.Printf("Do you want to trust this key? (yes/no): ")

	var answer string
	if _, err := fmt.Scanln(&answer); err != nil {
		return nil, fmt.Errorf("failed to read user input: %w", err)
	}

	if answer != "yes" {
		return nil, errors.New("user declined to trust the new daemon public key")
	}
	// add to trust store
	newKey := shared.TrustedKey{
		KeyID:       pubKeyResp.KeyID,
		PublicKey:   pubKeyResp.PublicKey,
		SignPublic:  pubKeyResp.SignPublic,
		Fingerprint: fingerprint,
	}

	store.Keys = append(store.Keys, newKey)
	if err := SaveTrustStore("trusted_keys.json", store); err != nil {
		return nil, fmt.Errorf("failed to save trust store: %w", err)
	}
	fmt.Println("New daemon public key trusted and saved.")
	return &newKey, nil

}
