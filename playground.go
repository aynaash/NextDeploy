//go:build ignore
// +build ignore

// internal/server/preparation/manager.go
package main

import (
	"fmt"
	"nextdeploy/core"
	"nextdeploy/shared"
	"time"
)

func main() {
	// Initialize trust store manager
	tsm := core.NewTrustStoreManager("./truststore.json")

	// Create a new identity to add
	newIdentity := shared.Identity{
		Fingerprint: "SHA256:newuserpubkeyhash",
		PublicKey:   "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEXAMpRZJkX7J6Jw6z8YJY7X9Z1J2K...",
		SignPublic:  "MCowBQYDK2VwAyEAEXAMpRZJkX7J6Jw6z8YJY7X9Z1J2K...",
		Role:        "reader",
		Email:       "reader@example.com",
		AddedBy:     "admin@example.com",
		CreatedAt:   time.Now(),
	}

	// Add the new identity
	fmt.Println("Adding new identity...")
	if err := tsm.AddIdentity(newIdentity); err != nil {
		fmt.Printf("Failed to add identity: %v\n", err)
	} else {
		fmt.Println("Successfully added new identity")
	}

	// Remove an existing identity
	fingerprintToRemove := "SHA256:user2pubkeyhash"
	fmt.Printf("\nRemoving identity with fingerprint %s...\n", fingerprintToRemove)
	if err := tsm.RemoveIdentity(fingerprintToRemove); err != nil {
		fmt.Printf("Failed to remove identity: %v\n", err)
	} else {
		fmt.Println("Successfully removed identity")
	}

	// Display current identities
	fmt.Println("\nCurrent identities in trust store:")
	trustStore := tsm.GetTrustStore()
	for _, identity := range trustStore.Identities {
		fmt.Printf("- %s (%s) - Role: %s\n", identity.Email, identity.Fingerprint, identity.Role)
	}
}
