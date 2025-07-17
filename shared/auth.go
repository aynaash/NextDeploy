package shared

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// AuthenticateRequest verifies the signature and checks RBAC permissions
func AuthenticateRequest(r *http.Request, trustStore *TrustStore, requiredRole string) (*Identity, error) {
	// Extract signature and fingerprint from headers
	signature := r.Header.Get("X-Signature")
	fingerprint := r.Header.Get("X-Fingerprint")

	if signature == "" || fingerprint == "" {
		return nil, errors.New("missing authentication headers")
	}

	// Find the identity in the trust store
	var identity *Identity
	for _, id := range trustStore.Identities {
		if id.Fingerprint == fingerprint {
			identity = &id
			break
		}
	}

	if identity == nil {
		return nil, errors.New("unauthorized: unknown identity")
	}

	// Verify RBAC role
	if !hasRequiredRole(identity.Role, requiredRole) {
		return nil, fmt.Errorf("forbidden: role %s required", requiredRole)
	}

	// Verify the signature
	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return nil, errors.New("invalid signature encoding")
	}

	pubKey, err := base64.StdEncoding.DecodeString(identity.SignPublic)
	if err != nil {
		return nil, errors.New("invalid public key encoding")
	}

	// Reconstruct the signed message (method + path + body)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, errors.New("failed to read request body")
	}

	// Restore the body for subsequent reads
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	message := fmt.Sprintf("%s %s %s", r.Method, r.URL.Path, string(body))
	if !ed25519.Verify(ed25519.PublicKey(pubKey), []byte(message), sigBytes) {
		return nil, errors.New("invalid signature")
	}

	return identity, nil
}

func hasRequiredRole(userRole, requiredRole string) bool {
	roleHierarchy := map[string]int{
		RoleOwner:    4,
		RoleAdmin:    3,
		RoleDeployer: 2,
		RoleReader:   1,
	}

	return roleHierarchy[userRole] >= roleHierarchy[requiredRole]
}
