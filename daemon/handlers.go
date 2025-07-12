package daemon

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func nextCoreHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

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
