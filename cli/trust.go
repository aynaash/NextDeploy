package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"nextdeploy/shared"
	"os"
)

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
