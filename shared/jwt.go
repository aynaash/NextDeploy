package shared

import (
	"crypto/ecdsa"
	"github.com/golang-jwt/jwt/v5"
	"strings"

	"time"
)

// WSClaims represents the custom claims structure for WebSocket JWT tokens
type WSClaims struct {
	ClientID             string `json:"cid"`      // Client identifier
	SessionID            string `json:"sid"`      // Unique session ID
	Scope                string `json:"scope"`    // Authorization scope (e.g., "read:logs", "deploy")
	AgentID              string `json:"agent_id"` // Optional agent identifier
	jwt.RegisteredClaims        // Standard JWT claims
}

// JWTOptions configures token generation options
type JWTOptions struct {
	ExpiresIn time.Duration
	NotBefore time.Duration // Optional delay before token is valid
	Issuer    string        // Token issuer
	Audience  []string      // Intended audience
	Scope     string        // Access scope
	ClientIP  string        // Optional client IP for binding
}

func GenerateWSToken(privateKey *ecdsa.PrivateKey, clientID string, opts JWTOptions) (string, error) {
	if privateKey == nil {
		return "", ErrNilKey
	}

	if clientID == "" {
		return "", ErrEmptyClientID
	}
	now := time.Now()
	claims := WSClaims{
		ClientID:  clientID,
		SessionID: GenerateSessionID(),
		Scope:     opts.Scope,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now.Add(opts.NotBefore)),
			Issuer:    opts.Issuer,
			Audience:  opts.Audience,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = GenerateKeyFingerprint(&privateKey.PublicKey)
	return token.SignedString(privateKey)

}

func GenerateKeyFingerprint(publicKey *ecdsa.PublicKey) string {
	// Generate a unique fingerprint for the public key
	// This is a placeholder; actual implementation may vary based on your requirements
	return "fingerprint-" + publicKey.X.Text(16) + "-" + publicKey.Y.Text(16)
}

// VerifyWSJWT validates and parses a WebSocket JWT token
func VerifyWSJWT(
	tokenString string,
	publicKey *ecdsa.PublicKey,
	expectedAudience string,
) (*WSClaims, error) {
	if publicKey == nil {
		return nil, ErrNilKey
	}

	token, err := jwt.ParseWithClaims(tokenString, &WSClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate algorithm
		if _, ok := token.Method.(*jwt.SigningMethodECDSA); !ok {
			return nil, ErrInvalidSigningMethod
		}

		// Optional: Verify key ID matches expected key
		if kid, ok := token.Header["kid"].(string); ok {
			expectedKid := GenerateKeyFingerprint(publicKey)
			if kid != expectedKid {
				return nil, ErrKeyMismatch
			}
		}

		return publicKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*WSClaims); ok && token.Valid {
		// Validate audience if required
		if expectedAudience != "" {
			if len(claims.Audience) == 0 || claims.Audience[0] != expectedAudience {
				return nil, ErrInvalidAudience
			}

		}
		return claims, nil
	}

	return nil, ErrInvalidToken
}

func GenerateSessionID() string {
	// Generate a unique session ID (e.g., using UUID or random bytes)
	// For simplicity, we'll use a timestamp-based approach here

	return strings.ReplaceAll(time.Now().Format("20060102150405.000"), ".", "")

}

func DeriveSessionKey(claims *WSClaims) (sessionKey []byte, err error) {
	// Derive a session key from the WebSocket JWT claims
	// This is a placeholder function; actual implementation would depend on your security requirements
	// For example, you might use HKDF or another key derivation function based on the JWT claims
	return
}
