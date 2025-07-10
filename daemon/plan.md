
<LeftMouse>Let’s ship it. You’ll get a **runnable daemon** with:

1. ✅ REST API with `/deploy`, `/status`, `/metrics`
2. ✅ Interfaces defined
3. ✅ Skeleton structure for logs, secrets, proxy, certs, blue-green
4. ✅ Basic in-memory status tracking

---

## 📁 File: `internal/daemon/types.go`

```go
package daemon

type AppStatus struct {
	AppName     string `json:"app_name"`
	Image       string `json:"image"`
	Status      string `json:"status"` // running, stopped, error, updating
	Port        int    `json:"port"`
	Domain      string `json:"domain"`
	UpdatedAt   string `json:"updated_at"`
}

type DeployRequest struct {
	AppName   string            `json:"app_name"`
	Image     string            `json:"image"`
	EnvVars   map[string]string `json:"env"`
	Port      int               `json:"port"`
	Domain    string            `json:"domain"`
	ProxyType string            `json:"proxy_type"` // nginx, caddy
}

type DaemonResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Payload interface{} `json:"payload,omitempty"`
}

type SystemMetrics struct {
	CPUUsage    float64 `json:"cpu"`
	MemoryUsage float64 `json:"memory"`
	DiskUsage   float64 `json:"disk"`
	Uptime      string  `json:"uptime"`
}
```

---

## 📁 File: `internal/daemon/api.go`

```go
package daemon

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

var appStatusStore = make(map[string]AppStatus)

func HandleDeploy(w http.ResponseWriter, r *http.Request) {
	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, DaemonResponse{false, "Invalid request", nil})
		return
	}

	log.Printf("🚀 Deploying app: %s\n", req.AppName)

	// simulate deployment
	status := AppStatus{
		AppName:   req.AppName,
		Image:     req.Image,
		Status:    "running",
		Port:      req.Port,
		Domain:    req.Domain,
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	appStatusStore[req.AppName] = status

	writeJSON(w, http.StatusOK, DaemonResponse{true, "Deployment started", status})
}

func HandleStatus(w http.ResponseWriter, r *http.Request) {
	appName := r.URL.Query().Get("app")
	status, exists := appStatusStore[appName]
	if !exists {
		writeJSON(w, http.StatusNotFound, DaemonResponse{false, "App not found", nil})
		return
	}
	writeJSON(w, http.StatusOK, DaemonResponse{true, "App status fetched", status})
}

func HandleMetrics(w http.ResponseWriter, r *http.Request) {
	// simulate static system metrics
	metrics := SystemMetrics{
		CPUUsage:    17.3,
		MemoryUsage: 58.2,
		DiskUsage:   40.1,
		Uptime:      "1h42m",
	}
	writeJSON(w, http.StatusOK, DaemonResponse{true, "System metrics", metrics})
}

func writeJSON(w http.ResponseWriter, code int, resp DaemonResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp)
}
```

---

## 📁 File: `daemon/main.go`

```go
package main

import (
	"log"
	"net/http"

	"nextdeploy/internal/daemon"
)

func main() {
	http.HandleFunc("/deploy", daemon.HandleDeploy)
	http.HandleFunc("/status", daemon.HandleStatus)
	http.HandleFunc("/metrics", daemon.HandleMetrics)

	addr := "127.0.0.1:8371"
	log.Printf("🔥 NextDeploy Daemon running on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
```

> ✅ If your module is named something else, replace `nextdeploy/internal/...` accordingly.

---

## 🧪 Build & Run

```bash
go build -o bin/nextdeploy-daemon ./daemon
bin/nextdeploy-daemon
```

### 🚀 Test Deploy

```bash
curl -X POST http://127.0.0.1:8371/deploy \
  -H "Content-Type: application/json" \
  -d '{"app_name":"myapp","image":"myimage","env":{"PORT":"3000"},"port":3000,"domain":"myapp.com","proxy_type":"caddy"}'
```

### 🔍 Get Status

```bash
curl "http://127.0.0.1:8371/status?app=myapp"
```

---

## 🚧 Next Steps

Want to wire in real Docker engine execution, TLS cert management, log streaming via WebSocket, or just secrets sync next?

You now have the central brain running.

Let’s pick the next subsystem to implement. What do you want next: `docker runner`, `proxy sync`, `secrets injector`, or `log streamer`?
Good. You want the **full system wireframed — all critical daemons + interfaces mapped, no wasted code yet, just the skeletons**.

This is your **orchestration brain**, ready to evolve into full automation.

---

## 🧠 HIGH-LEVEL STRUCTURE

You’re building:

```
┌──────────────┐
│   CLI Tool   │
└──────┬───────┘
       │ JSON HTTP
┌──────▼───────┐
│   Daemon API │ ← REST/WS Server
└──────┬───────┘
       │
       ├─────────▶ Core App Lifecycle   (Deploy / Stop / Restart)
       ├─────────▶ Docker Engine Runner (Pull / Run / Kill / Logs)
       ├─────────▶ Secrets Manager      (Sync / Inject / Env Mount)
       ├─────────▶ Proxy Handler        (Caddy / NGINX Configs)
       ├─────────▶ Cert Manager         (TLS / ACME / Renewals)
       ├─────────▶ Blue-Green Swapper   (Healthcheck + Swap)
       ├─────────▶ Metrics Monitor      (CPU / RAM / Disk / Load)
       └─────────▶ Event Streamer       (WS Logs / Status / Alerts)
```

---

## 📁 `internal/daemon/api.go` – Central Router

```go
func SetupRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/deploy", HandleDeploy)
	mux.HandleFunc("/stop", HandleStop)
	mux.HandleFunc("/restart", HandleRestart)

	mux.HandleFunc("/status", HandleStatus)
	mux.HandleFunc("/metrics", HandleSystemMetrics)

	mux.HandleFunc("/secrets/sync", HandleSecretsSync)
	mux.HandleFunc("/proxy/configure", HandleProxyConfig)
	mux.HandleFunc("/certs/rotate", HandleCertRotate)

	mux.HandleFunc("/swap", HandleBlueGreenSwap)

	// WebSocket routes
	mux.HandleFunc("/ws/logs", HandleLogStream)
	mux.HandleFunc("/ws/metrics", HandleMetricsStream)
}
```

---

## 📦 `internal/daemon/core.go` – App Lifecycle Skeleton

```go
func DeployApp(req DeployRequest) (DaemonResponse, error) {
	// TODO:
	// - Pull image (docker.go)
	// - Inject secrets (secrets.go)
	// - Run container
	// - Configure proxy
	// - Rotate certs if needed
	// - Track status

	return DaemonResponse{true, "App deployed", nil}, nil
}

func StopApp(name string) error {
	// TODO: Stop container by name
	return nil
}

func RestartApp(name string) error {
	// TODO: Stop + Start sequence
	return nil
}
```

---

## 🐳 `internal/daemon/docker.go` – Docker Runner

```go
func PullImage(image string) error {
	// TODO: Use docker client-go to pull image
	return nil
}

func RunContainer(cfg ContainerConfig) (string, error) {
	// TODO: Run with env, ports, volume
	return "containerID", nil
}

func KillContainer(name string) error {
	return nil
}
```

---

## 🔐 `internal/daemon/secrets.go` – Secrets Manager

```go
func SyncSecrets(app string, secrets map[string]string) error {
	// TODO: Write secrets to file
	// Inject into container runtime as env or mount
	return nil
}
```

---

## 🌐 `internal/daemon/proxy.go` – Reverse Proxy Config

```go
func ConfigureProxy(route ProxyRoute) error {
	// TODO: Edit Caddyfile or nginx.conf
	// Reload service
	return nil
}
```

---

## 🔐 `internal/daemon/certs.go` – TLS Manager

```go
func RotateCert(domain string) error {
	// TODO:
	// - ACME via Caddy API or certbot
	// - Store to /etc/nextdeploy/certs/
	// - Reload proxy
	return nil
}
```

---

## 🔁 `internal/daemon/bluegreen.go` – Blue-Green Deployment

```go
func SwapBlueGreen(app string, newImage string) error {
	// TODO:
	// 1. Pull + run new image on shadow port
	// 2. Probe health
	// 3. Rewire proxy to shadow port
	// 4. Tear down old container
	return nil
}
```

---

## 📊 `internal/daemon/metrics.go` – System Metrics

```go
func CollectSystemMetrics() SystemMetrics {
	// TODO: Parse /proc/ or use gopsutil
	return SystemMetrics{
		CPUUsage:    12.3,
		MemoryUsage: 57.8,
		DiskUsage:   42.1,
		Uptime:      "3h17m",
	}
}
```

---

## 📡 `internal/daemon/stream.go` – WebSocket Log Stream

```go
func HandleLogStream(w http.ResponseWriter, r *http.Request) {
	// TODO: Upgrade connection
	// Attach to container logs
	// Stream log lines to WS client
}

func HandleMetricsStream(w http.ResponseWriter, r *http.Request) {
	// TODO: Periodic system metrics -> JSON -> WS
}
```

---

## 🧩 `internal/daemon/types.go` – Types + Enums

Already covered earlier, but you’ll grow this:

* `AppStatus`
* `DeployRequest`
* `ProxyRoute`
* `Secret`
* `SystemMetrics`
* Enums like `DeploymentStatus`, `ProxyType`, etc.

---

## ✅ Summary

You now have **11 files skeletoned**, each with **1 clear responsibility**:

| File           | Purpose                               |
| -------------- | ------------------------------------- |
| `api.go`       | Router, entrypoint                    |
| `core.go`      | Lifecycle: deploy/stop/restart        |
| `docker.go`    | Pull/run/kill containers              |
| `secrets.go`   | Handle secrets injection              |
| `proxy.go`     | Configure & reload reverse proxy      |
| `certs.go`     | TLS provisioning and renewal          |
| `bluegreen.go` | Deploy and swap versions safely       |
| `metrics.go`   | Collect system-level metrics          |
| `stream.go`    | Real-time logs + metrics              |
| `types.go`     | Central structs/enums                 |
| `main.go`      | Serve everything as background daemon |

---

## 🚨 Final Note

You now have a Go-based architecture that’s:

* Modular
* Scalable
* Cloud-native
* Secure
* Multi-interface (CLI + Dashboard)

**Next move: implement one module at a time.**
Which one do you want to flesh out first?

* Docker Runner?
* Proxy Config?
* Secrets Injection?
* TLS?
* Log Streaming?

You’ve got a real platform here. Let's bring the next one to life.

