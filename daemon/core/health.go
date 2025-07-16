package core

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

func ProbeContainerHealth(containerID string) (string, error) {
	inspect, err := dockerCli.ContainerInspect(context.Background(), containerID)
	if err != nil {
		return "error", err
	}
	if inspect.State != nil && inspect.State.Health != nil {
		return inspect.State.Health.Status, nil
	}
	return "unknown", nil
}

func SystemHealth() map[string]string {
	// Use gopsutil here later for RAM, CPU
	return map[string]string{
		"uptime": "12h",
		"cpu":    "8%",
		"mem":    "34%",
	}
}

// HealthStatus represents the health status of the daemon
type HealthStatus struct {
	Status      string            `json:"status"`
	Version     string            `json:"version"`
	Uptime      string            `json:"uptime"`
	Components  map[string]string `json:"components,omitempty"`
	LastChecked time.Time         `json:"last_checked"`
}

var (
	startTime    = time.Now()
	healthStatus HealthStatus
	healthMutex  = &sync.RWMutex{}
)

// Initialize health status
func init() {
	healthStatus = HealthStatus{
		Status:      "healthy",
		Version:     "1.0.0", // Set this dynamically if needed
		Components:  make(map[string]string),
		LastChecked: time.Now(),
	}
}

// HandleHealthCheck handles health check requests
func HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	healthMutex.RLock()
	defer healthMutex.RUnlock()

	// Update uptime
	healthStatus.Uptime = time.Since(startTime).Round(time.Second).String()
	healthStatus.LastChecked = time.Now()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(healthStatus)
}

// SetComponentStatus updates the status of a specific component
func SetComponentStatus(component, status string) {
	healthMutex.Lock()
	defer healthMutex.Unlock()

	healthStatus.Components[component] = status
	healthStatus.LastChecked = time.Now()
}

// SetGlobalStatus updates the overall health status
func SetGlobalStatus(status string) {
	healthMutex.Lock()
	defer healthMutex.Unlock()

	healthStatus.Status = status
	healthStatus.LastChecked = time.Now()
}

// HealthCheckMiddleware provides readiness/liveness checking
func HealthCheckMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		healthMutex.RLock()
		defer healthMutex.RUnlock()

		if healthStatus.Status != "healthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]string{
				"status": "unavailable",
				"reason": "service_unhealthy",
			})
			return
		}
		next(w, r)
	}
}
