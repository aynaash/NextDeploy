package core

import (
	"crypto/ecdh"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"yourproject/shared"
)

type (
	APIHandler struct {
		keyManager   *KeyManager
		healthStatus *HealthStatus
		healthMutex  *sync.RWMutex
		appStatusStore map[string]shared.AppStatus
	}

	HealthStatus struct {
		Status string `json:"status"`
	}

	DaemonResponse struct {
		Success bool        `json:"success"`
		Message string      `json:"message"`
		Payload interface{} `json:"payload,omitempty"`
	}

	DeployRequest struct {
		// Add your deployment request fields
	}
)

func NewAPIHandler(km *KeyManager) *APIHandler {
	return &APIHandler{
		keyManager:    km,
		healthStatus:  &HealthStatus{Status: "healthy"},
		healthMutex:   &sync.RWMutex{},
		appStatusStore: make(map[string]shared.AppStatus),
	}
}

// Middleware chain for authentication
func (h *APIHandler) Authenticate(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := AuthenticateRequest(r, h.keyManager.TrustStore, shared.RoleDeployer)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, DaemonResponse{
				Success: false,
				Message: err.Error(),
			})
			return
		}

		// Add identity to context for downstream handlers
		ctx := context.WithValue(r.Context(), "identity", identity)
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// Handlers
func (h *APIHandler) HandleDeploy(w http.ResponseWriter, r *http.Request) {
	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	// Add deployment logic here
	h.respondSuccess(w, "Deployment initiated")
}

func (h *APIHandler) HandleReadinessCheck(w http.ResponseWriter, r *http.Request) {
	h.healthMutex.RLock()
	defer h.healthMutex.RUnlock()

	if h.healthStatus.Status == "healthy" && h.checkDependencies() {
		h.respondSuccess(w, "ready")
		return
	}

	h.respondError(w, http.StatusServiceUnavailable, "not_ready")
}

func (h *APIHandler) HandleLivenessCheck(w http.ResponseWriter, r *http.Request) {
	h.healthMutex.RLock()
	defer h.healthMutex.RUnlock()

	if h.healthStatus.Status != "healthy" {
		h.respondError(w, http.StatusServiceUnavailable, "unhealthy")
		return
	}

	h.respondSuccess(w, "alive")
}

func (h *APIHandler) HandleSubmitEnv(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.respondError(w, http.StatusMethodNotAllowed, "Only POST allowed")
		return
	}

	var envelope shared.Envelope
	if err := json.NewDecoder(r.Body).Decode(&envelope); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid envelope format")
		return
	}

	if _, err := shared.DecodeFromBase64(envelope.Signature); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid signature format")
		return
	}

	var encryptedEnv shared.EncryptedEnv
	if err := json.Unmarshal([]byte(envelope.Payload), &encryptedEnv); err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid payload format")
		return
	}

	daemonKey := h.keyManager.GetKey(encryptedEnv.KeyID)
	if daemonKey == nil {
		h.respondError(w, http.StatusNotFound, "Daemon key not found")
		return
	}

	cliPubBytes, err := shared.DecodeFromBase64(encryptedEnv.CLIPublicKey)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid client public key format")
		return
	}

	curve := ecdh.X25519()
	cliPub, err := curve.NewPublicKey(cliPubBytes)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid client public key")
		return
	}

	sharedKey, err := shared.DeriveSharedKey(daemonKey.ECDHPrivate, cliPub)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "Failed to compute shared key")
		return
	}

	nonce, err := shared.DecodeFromBase64(encryptedEnv.Nonce)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid nonce format")
		return
	}

	envblob, err := shared.DecodeFromBase64(encryptedEnv.EnvBlob)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid environment blob format")
		return
	}

	plaintextBlob, err := shared.Decrypt(envblob, sharedKey, nonce)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, "Failed to decrypt environment blob")
		return
	}

	decryptedVariables := make(map[string]string)
	for key, encryptedValue := range encryptedEnv.Variables {
		encryptedBytes, err := shared.DecodeFromBase64(encryptedValue)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid encrypted variable format for %s", key))
			return
		}

		decryptedValue, err := shared.Decrypt(encryptedBytes, sharedKey, nonce)
		if err != nil {
			h.respondError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to decrypt variable %s", key))
			return
		}
		decryptedVariables[key] = string(decryptedValue)
	}

	log.Printf("Received new environment:\nFull content:\n%s\n", string(plaintextBlob))
	log.Println("Individual variables:")
	for k, v := range decryptedVariables {
		log.Printf("%s=%s", k, v)
	}

	h.respondSuccess(w, "Environment processed successfully")
}

// Helper methods
func (h *APIHandler) checkDependencies() bool {
	// Implement checks for database, external services, etc.
	return true
}

func (h *APIHandler) respondSuccess(w http.ResponseWriter, message string, payload ...interface{}) {
	response := DaemonResponse{
		Success: true,
		Message: message,
	}
	
	if len(payload) > 0 {
		response.Payload = payload[0]
	}
	
	writeJSON(w, http.StatusOK, response)
}

func (h *APIHandler) respondError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, DaemonResponse{
		Success: false,
		Message: message,
	})
}

func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}
