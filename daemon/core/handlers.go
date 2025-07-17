package core

import (
	"crypto/ecdh"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"nextdeploy/shared"
	"time"
)

type APIHandler struct {
	keyManager *KeyManager
}

func nextCoreHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	identity, err := AuthenticateRequest(r, &shared.TrustStore{}, shared.RoleAdmin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	fmt.Printf("🧠 Identity authenticated: %s (%s)\n", identity.Fingerprint, identity.Role)

	// TODO: use the identity to give user what to and not do

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}

	defer func() {
		r.Body.Close()
	}()

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	fmt.Println("🧠 Received NextCore data:")
	pretty, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(pretty))

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("NextCore data received"))
}
func HandleDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	identity, err := AuthenticateRequest(r, &shared.TrustStore{}, shared.RoleDeployer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	fmt.Printf("🧠 Identity authenticated: %s (%s)\n", identity.Fingerprint, identity.Role)
	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, DaemonResponse{
			Success: false,
			Message: "Invalid request payload",
		})
		return
	}
	// Add deployment logic here
}
func HandleReadinessCheck(w http.ResponseWriter, r *http.Request) {
	healthMutex.RLock()
	defer healthMutex.RUnlock()

	if r.Method != http.MethodGet {
		http.Error(w, "Only GET allowed", http.StatusMethodNotAllowed)
		return
	}
	identity, err := AuthenticateRequest(r, &shared.TrustStore{}, shared.RoleAdmin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	fmt.Printf("🧠 Identity authenticated: %s (%s)\n", identity.Fingerprint, identity.Role)

	if healthStatus.Status == "healthy" && checkDependencies() {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
		return
	}

	w.WriteHeader(http.StatusServiceUnavailable)
	json.NewEncoder(w).Encode(map[string]string{"status": "not_ready"})
}

// HandleLivenessCheck checks if the service is still running
func HandleLivenessCheck(w http.ResponseWriter, r *http.Request) {
	healthMutex.RLock()
	defer healthMutex.RUnlock()

	if healthStatus.Status != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]string{"status": "unhealthy"})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "alive"})
}

func checkDependencies() bool {
	// Implement checks for database, external services, etc.
	return true
}
func HandleStop(w http.ResponseWriter, r *http.Request) {
	// Add stop logic here
}

func HandleRestart(w http.ResponseWriter, r *http.Request) {
	appName := r.URL.Query().Get("app_name")
	if appName == "" {
		writeJSON(w, http.StatusBadRequest, DaemonResponse{
			Success: false,
			Message: "App name is required",
		})
		return
	}

	status := AppStatus{
		AppName: appName,
		Status:  "running",
	}
	appStatusStore[appName] = status

	writeJSON(w, http.StatusOK, DaemonResponse{
		Success: true,
		Message: "App started successfully",
		Payload: status,
	})
}

func HandleStatus(w http.ResponseWriter, r *http.Request) {
	appName := r.URL.Query().Get("app_name")
	status, exists := appStatusStore[appName]
	if !exists {
		writeJSON(w, http.StatusNotFound, DaemonResponse{
			Success: false,
			Message: "App not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, DaemonResponse{
		Success: true,
		Message: "App status retrieved successfully",
		Payload: status,
	})
}

// FIX: remove writeJSON function and use normal return
func HandleSubmitEnv(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	identity, err := AuthenticateRequest(r, &shared.TrustStore{}, shared.RoleAdmin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	fmt.Printf("🧠 Identity authenticated: %s (%s)\n", identity.Fingerprint, identity.Role)
	var envelope shared.Envelope
	err = json.NewDecoder(r.Body).Decode(&envelope)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, DaemonResponse{
			Success: false,
			Message: "Invalid envelope format",
		})
		return
	}
	_, err = shared.DecodeFromBase64(envelope.Signature)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, DaemonResponse{
			Success: false,
			Message: "Invalid signature format",
		})
		return
	}
	//parse the payload
	var encryptedEnv shared.EncryptedEnv
	if err := json.Unmarshal([]byte(envelope.Payload), &encryptedEnv); err != nil {
		writeJSON(w, http.StatusBadRequest, DaemonResponse{
			Success: false,
			Message: "Invalid payload format",
		})
		return
	}
	km := r.Context().Value("keyManager").(*KeyManager)
	if km == nil {
		http.Error(w, "Key manager not initialized", http.StatusInternalServerError)
		return
	}
	// TODO: Verify properly here
	// if shared.Verify([]byte(envelope.Payload), signature, km.GetCurrentKey().SignPublic) != nil {
	// 	writeJSON(w, http.StatusUnauthorized, DaemonResponse{
	// 		Success: false,
	// 		Message: "Invalid signature",
	// 	})
	// 	return
	// }
	daemonKey := km.GetKey(encryptedEnv.KeyID)
	if daemonKey == nil {
		writeJSON(w, http.StatusNotFound, DaemonResponse{
			Success: false,
			Message: "Daemon key not found",
		})
		return
	}
	cliPubBtes, err := shared.DecodeFromBase64(encryptedEnv.CLIPublicKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, DaemonResponse{
			Success: false,
			Message: "Invalid client public key format",
		})
		return
	}
	curve := ecdh.X25519()
	cliPub, err := curve.NewPublicKey(cliPubBtes)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, DaemonResponse{
			Success: false,
			Message: "Invalid client public key",
		})
		return
	}
	sharedKey, err := shared.DeriveSharedKey(daemonKey.ECDHPrivate, cliPub)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, DaemonResponse{
			Success: false,
			Message: "Failed to compute shared key",
		})
		return
	}
	//Decode nonce
	nonce, err := shared.DecodeFromBase64(encryptedEnv.Nonce)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, DaemonResponse{
			Success: false,
			Message: "Invalid nonce format",
		})
		return
	}
	// env blob
	envblob, err := shared.DecodeFromBase64(encryptedEnv.EnvBlob)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, DaemonResponse{
			Success: false,
			Message: "Invalid environment blob format",
		})
		return
	}
	plaintextBlob, err := shared.Decrypt(envblob, sharedKey, nonce)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, DaemonResponse{
			Success: false,
			Message: "Failed to decrypt environment blob",
		})
		return
	}
	// Decrypt individual variables
	decryptedVariables := make(map[string]string)
	for key, encryptedValue := range encryptedEnv.Variables {
		encryptedBytes, err := shared.DecodeFromBase64(encryptedValue)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, DaemonResponse{
				Success: false,
				Message: fmt.Sprintf("Invalid encrypted variable format for %s", key),
			})
			return
		}
		decryptedValue, err := shared.Decrypt(encryptedBytes, sharedKey, nonce)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, DaemonResponse{
				Success: false,
				Message: fmt.Sprintf("Failed to decrypt variable %s", key),
			})
			return
		}
		decryptedVariables[key] = string(decryptedValue)
		// Log the decrypted values (in production, you'd do something more secure)
		log.Printf("Received new environment:\nFull content:\n%s\n", string(plaintextBlob))
		log.Println("Individual variables:")
		for k, v := range decryptedVariables {
			log.Printf("%s=%s", k, v)
		}

	}
	auditLog, _ := NewAuditLog("./audit_log.json")
	auditLog.AddEntry(shared.AuditLogEntry{
		Action:    "submit_env",
		Actor:     identity.Fingerprint,
		Target:    "environment",
		Timestamp: time.Now(),
		Signature: envelope.Signature,
		IP:        r.RemoteAddr,
	})

}

func HandleAddIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	identity, err := AuthenticateRequest(r, &shared.TrustStore{}, shared.RoleAdmin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if !shared.HasRequiredRole(*identity, shared.RoleAdmin) {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}
	var newIdentity shared.Identity
	if err := json.NewDecoder(r.Body).Decode(&newIdentity); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	// Validate the new identity
	if newIdentity.Fingerprint == "" || newIdentity.Role == "" {
		http.Error(w, "Invalid identity data", http.StatusBadRequest)
		return
	}
	// // Add the new identity to the trust store
	// newIdentity.AddedBy = identity.Fingerprint
	// newIdentity.CreatedAt = time.Now()
	// shared.TrustStore.Identities = append(shared.TrustStore.Identities, newIdentity)
	//
	// if err := shared.SaveTrustStore(&trustStore); err != nil {
	// 	http.Error(w, "Failed to save trust store", http.StatusInternalServerError)
	// 	return
	// }
	tms := NewTrustStoreManager("./truststore.json")
	newId := shared.Identity{
		Fingerprint: "SHA256:newuserpubkeyhash",
		PublicKey:   "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEXAMpRZJkX7J6Jw6z8YJY7X9Z1J2K...",
		SignPublic:  "MCowBQYDK2VwAyEAEXAMpRZJkX7J6Jw6z8YJY7X9Z1J2K...",
		Role:        "reader",
		Email:       "reader@reader.com",
		AddedBy:     "admin@example.com",
		CreatedAt:   time.Now(),
	}
	fmt.Println("Adding new identity...")
	if err := tms.AddIdentity(newId); err != nil {
		http.Error(w, fmt.Sprintf("Failed to add identity: %v", err), http.StatusInternalServerError)
		return
	}
	// Respond with success
	writeJSON(w, http.StatusCreated, DaemonResponse{
		Success: true,
		Message: "Identity added successfully",
		Payload: newIdentity,
	})

	auditLog, _ := NewAuditLog("./audit_log.json")
	auditLog.AddEntry(shared.AuditLogEntry{
		Action:    "add_identity",
		Actor:     identity.Fingerprint,
		Target:    newIdentity.Fingerprint,
		Timestamp: time.Now(),
		Signature: r.Header.Get("X-Signature"),
	})
}

func HandleRevokeIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}
	identity, err := AuthenticateRequest(r, &shared.TrustStore{}, shared.RoleAdmin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if !shared.HasRequiredRole(*identity, shared.RoleAdmin) {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}
	var revokeReq struct {
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&revokeReq); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}
	if revokeReq.Fingerprint == "" {
		http.Error(w, "Fingerprint is required", http.StatusBadRequest)
		return
	}
	tms := NewTrustStoreManager("./truststore.json")
	if err := tms.RemoveIdentity(revokeReq.Fingerprint); err != nil {
		http.Error(w, fmt.Sprintf("Failed to revoke identity: %v", err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, DaemonResponse{
		Success: true,
		Message: "Identity revoked successfully",
	})
	auditLog, _ := NewAuditLog("./audit_log.json")
	auditLog.AddEntry(shared.AuditLogEntry{
		Action:    "revoke_identity",
		Actor:     identity.Fingerprint,
		Target:    revokeReq.Fingerprint,
		Timestamp: time.Now(),
	})

}
func HandleListIdentities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET allowed", http.StatusMethodNotAllowed)
		return
	}
	identity, err := AuthenticateRequest(r, &shared.TrustStore{}, shared.RoleAdmin)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if !shared.HasRequiredRole(*identity, shared.RoleAdmin) {
		http.Error(w, "Insufficient permissions", http.StatusForbidden)
		return
	}
	tms := NewTrustStoreManager("./truststore.json")
	trustStore := tms.GetTrustStore()
	writeJSON(w, http.StatusOK, DaemonResponse{
		Success: true,
		Message: "Identities retrieved successfully",
		Payload: trustStore.Identities,
	})
	auditLog, _ := NewAuditLog("./audit_log.json")
	auditLog.AddEntry(shared.AuditLogEntry{
		Action:    "list_identities",
		Actor:     identity.Fingerprint,
		Timestamp: time.Now(),
	})
}
func HandlePublicKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET allowed", http.StatusMethodNotAllowed)
		return
	}
	km := r.Context().Value("keyManager").(*KeyManager)
	currentKey := km.GetCurrentKey()
	if currentKey == nil {
		http.Error(w, "Failed to retrieve public key", http.StatusInternalServerError)
		return
	}
	response := shared.PublicKeyResponse{
		KeyID:      currentKey.KeyID,
		PublicKey:  shared.EncodeToBase64(currentKey.ECDHPublic.Bytes()),
		SignPublic: shared.EncodeToBase64(currentKey.SignPublic),
	}
	writeJSON(w, http.StatusOK, DaemonResponse{
		Success: true,
		Message: "Public key retrieved successfully",
		Payload: response,
	})
}
func HandleSystemMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := CollectSystemMetrics()
	writeJSON(w, http.StatusOK, DaemonResponse{
		Success: true,
		Message: "System metrics retrieved successfully",
		Payload: metrics,
	})
}

func HandleSecretsSync(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, DaemonResponse{
		Success: true,
		Message: "Secrets synchronized successfully",
	})
}

func HandleProxyConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, DaemonResponse{
		Success: true,
		Message: "Proxy configured successfully",
	})
}

func HandleCertRotate(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, DaemonResponse{
		Success: true,
		Message: "Certificates rotated successfully",
	})
}

func HandleBlueGreenSwap(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, DaemonResponse{
		Success: true,
		Message: "Blue-green deployment swap completed successfully",
	})
}
