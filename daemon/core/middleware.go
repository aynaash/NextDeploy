package core

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"nextdeploy/shared" // import your shared package
)

// AuthenticateRequest verifies the signature and checks RBAC permissions
func AuthenticateRequest(r *http.Request, trustStore *shared.TrustStore, requiredRole string) (*shared.Identity, error) {
	// Step 1: Extract and validate headers
	signature := r.Header.Get("X-Signature")
	fingerprint := r.Header.Get("X-Fingerprint")
	if signature == "" || fingerprint == "" {
		return nil, errors.New("missing authentication headers")
	}

	// Step 2: Find identity in trust store
	identity, err := findIdentity(trustStore, fingerprint)
	if err != nil {
		return nil, err
	}

	// Step 3: Verify RBAC permissions
	if err := verifyRBAC(identity.Role, requiredRole); err != nil {
		return nil, err
	}

	// Step 4: Verify cryptographic signature
	if err := verifyRequestSignature(r, identity.SignPublic, signature); err != nil {
		return nil, err
	}

	return identity, nil
}

// Helper function to find identity in trust store
func findIdentity(trustStore *shared.TrustStore, fingerprint string) (*shared.Identity, error) {
	for _, id := range trustStore.Identities {
		if id.Fingerprint == fingerprint {
			// Return a copy of the identity to prevent modification of the original
			identity := id
			return &identity, nil
		}
	}
	return nil, errors.New("unauthorized: unknown identity")
}

// Helper function for RBAC verification
func verifyRBAC(userRole, requiredRole string) error {
	roleHierarchy := map[string]int{
		shared.RoleOwner:    4,
		shared.RoleAdmin:    3,
		shared.RoleDeployer: 2,
		shared.RoleReader:   1,
	}

	userLevel, userOk := roleHierarchy[userRole]
	requiredLevel, requiredOk := roleHierarchy[requiredRole]

	if !userOk || !requiredOk {
		return fmt.Errorf("forbidden: invalid role specified")
	}

	if userLevel < requiredLevel {
		return fmt.Errorf("forbidden: role %s required", requiredRole)
	}

	return nil
}

// Helper function for signature verification
func verifyRequestSignature(r *http.Request, publicKeyB64, signatureB64 string) error {
	// Decode signature
	sigBytes, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return errors.New("invalid signature encoding")
	}

	// Decode public key
	pubKey, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		return errors.New("invalid public key encoding")
	}

	// Reconstruct signed message (method + path + body)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return errors.New("failed to read request body")
	}
	// Restore the body for subsequent reads
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	message := fmt.Sprintf("%s %s %s", r.Method, r.URL.Path, string(body))
	if !ed25519.Verify(ed25519.PublicKey(pubKey), []byte(message), sigBytes) {
		return errors.New("invalid signature")
	}

	return nil
}

// corsMiddleware adds CORS headers to responses
func CORSMiddleware(enabled bool) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if enabled {
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours

				if r.Method == "OPTIONS" {
					w.WriteHeader(http.StatusOK)
					return
				}
			}
			next(w, r)
		}
	}
}

// LoggingMiddleware provides request logging
func LoggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}

		defer func() {
			slog.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"status", sw.status,
				"duration", time.Since(start),
				"user_agent", r.UserAgent())
		}()

		next(sw, r)
	}
}

// statusWriter captures the HTTP status code
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// RecoveryMiddleware handles panics
func RecoveryMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"error", err,
					"path", r.URL.Path,
					"method", r.Method)
				writeError(w, http.StatusInternalServerError, "Internal Server Error")
			}
		}()
		next(w, r)
	}
}

// AuthMiddleware handles authentication
func AuthMiddleware(km *KeyManager) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for health checks
			if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
				next(w, r)
				return
			}

			token := r.Header.Get("Authorization")
			if token == "" {
				writeError(w, http.StatusUnauthorized, "Authorization header required")
				return
			}

			valid, err := km.ValidateToken(token)
			if err != nil {
				slog.Warn("token validation error", "error", err)
				writeError(w, http.StatusInternalServerError, "Internal authentication error")
				return
			}

			if !valid {
				writeError(w, http.StatusUnauthorized, "Invalid authorization token")
				return
			}

			next(w, r)
		}
	}
}

// RateLimitMiddleware provides basic rate limiting
func RateLimitMiddleware(requestsPerMinute int) func(http.HandlerFunc) http.HandlerFunc {
	type client struct {
		limiter  *time.Ticker
		lastSeen time.Time
	}

	var (
		clients = make(map[string]*client)
		mu      sync.Mutex
	)

	// Cleanup old clients
	go func() {
		for {
			time.Sleep(time.Minute)
			mu.Lock()
			for ip, client := range clients {
				if time.Since(client.lastSeen) > 3*time.Minute {
					client.limiter.Stop()
					delete(clients, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr

			mu.Lock()
			c, exists := clients[ip]
			if !exists {
				interval := time.Minute / time.Duration(requestsPerMinute)
				c = &client{
					limiter:  time.NewTicker(interval),
					lastSeen: time.Now(),
				}
				clients[ip] = c
			}
			c.lastSeen = time.Now()
			mu.Unlock()

			select {
			case <-c.limiter.C:
				next(w, r)
			default:
				writeError(w, http.StatusTooManyRequests, "Too many requests")
				return
			}
		}
	}
}

// ChainMiddleware combines multiple middleware functions
func ChainMiddleware(h http.HandlerFunc, middleware ...func(http.HandlerFunc) http.HandlerFunc) http.HandlerFunc {
	for _, m := range middleware {
		h = m(h)
	}
	return h
}
