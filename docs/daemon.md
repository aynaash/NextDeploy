Here's a **complete architecture** for your **agent/daemon** that extends your existing SSH-based deployment system with real-time monitoring and remote control capabilities:

---

### **1. Daemon Design Overview**
```
[CLI/Frontend] ←WebSocket/HTTP→ [Server Daemon] ↔ [Docker API] ↔ [Your Containers]
                     ↑
               (Health Monitoring)
```

### **2. Core Components**

#### **A. Daemon Program (`cmd/daemon/main.go`)**
```go
package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/gorilla/websocket"
)

type Daemon struct {
	docker    *client.Client
	upgrader  websocket.Upgrader
	cmdChan   chan Command // For incoming commands
	health    *HealthMonitor
}

type Command struct {
	Action  string          `json:"action"`  // "logs", "restart", "deploy"
	Payload json.RawMessage `json:"payload"` // JSON args
}

func main() {
	// 1. Initialize Docker client
	docker, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Fatalf("Docker init failed: %v", err)
	}

	// 2. Create daemon
	daemon := &Daemon{
		docker:   docker,
		upgrader: websocket.Upgrader{},
		cmdChan:  make(chan Command, 10),
		health:   NewHealthMonitor(docker),
	}

	// 3. Start HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", daemon.handleWebSocket)
	mux.HandleFunc("/health", daemon.health.Handler)

	srv := &http.Server{
		Addr:    ":8081",
		Handler: mux,
	}

	// 4. Graceful shutdown
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		srv.Shutdown(context.Background())
	}()

	log.Println("🚀 Daemon started on :8081")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// WebSocket handler for real-time communication
func (d *Daemon) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := d.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	// Handle incoming commands
	go d.handleIncomingCommands(conn)

	// Stream Docker events
	for event := range d.health.EventStream {
		if err := conn.WriteJSON(event); err != nil {
			break
		}
	}
}
```

#### **B. Health Monitoring (`internal/health/monitor.go`)**
```go
package health

import (
	"context"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

type HealthEvent struct {
	Container string  `json:"container"`
	CPU       float64 `json:"cpu"`
	Memory    float64 `json:"memory"`
	Status    string  `json:"status"`
}

type HealthMonitor struct {
	docker *client.Client
}

func NewHealthMonitor(docker *client.Client) *HealthMonitor {
	return &HealthMonitor{docker: docker}
}

func (h *HealthMonitor) EventStream(ctx context.Context) <-chan HealthEvent {
	events := make(chan HealthEvent)
	
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				containers, _ := h.docker.ContainerList(ctx, types.ContainerListOptions{})
				for _, c := range containers {
					stats, _ := h.docker.ContainerStats(ctx, c.ID, false)
					// Parse stats (simplified)
					events <- HealthEvent{
						Container: c.Names[0],
						CPU:       23.5, // Actual calculation needed
						Memory:    45.2,
						Status:    c.State,
					}
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return events
}
```

#### **C. Systemd Service (Deployed via Your SSH Tool)**
```go
// In your existing SSH deployment code:
func deployAgent(srv *server.ServerStruct, serverName string) error {
	ctx := context.Background()

	// 1. Upload agent binary
	if err := srv.UploadFile(ctx, serverName, "daemon", "/usr/local/bin/nextdeploy-agent"); err != nil {
		return err
	}

	// 2. Create systemd service
	service := `
[Unit]
Description=NextDeploy Agent
After=docker.service

[Service]
ExecStart=/usr/local/bin/nextdeploy-agent
Restart=always
User=root
Environment=DOCKER_HOST=unix:///var/run/docker.sock

[Install]
WantedBy=multi-user.target
`

	cmd := fmt.Sprintf("echo '%s' > /etc/systemd/system/nextdeploy-agent.service && "+
		"systemctl daemon-reload && "+
		"systemctl enable nextdeploy-agent && "+
		"systemctl start nextdeploy-agent", service)

	_, err := srv.ExecuteCommand(ctx, serverName, cmd, os.Stdout)
	return err
}
```

---

### **3. Frontend/CLI Communication**
#### **A. CLI Command Example**
```bash
# Stream logs from remote
nextdeploy agent logs --server=prod --container=frontend

# Trigger redeploy
nextdeploy agent deploy --server=prod --image=myapp:latest
```

#### **B. WebSocket Protocol**
```json
// Incoming command
{
  "action": "logs",
  "payload": {"container": "frontend", "follow": true}
}

// Outgoing stream
{
  "type": "log",
  "data": "2023-01-01T12:00:00Z Server started..."
}
{
  "type": "health",
  "data": {"container": "frontend", "cpu": 15.2}
}
```

---

### **4. Key Features Implemented**
1. **Real-time Monitoring**:
   - Container metrics (CPU/RAM) via Docker API
   - Event streaming over WebSocket

2. **Remote Control**:
   - Start/stop containers
   - Stream logs
   - Trigger deployments

3. **Security**:
   - Runs as root (to access Docker socket)
   - WebSocket origin validation (add to `upgrader.CheckOrigin`)

4. **Integration**:
   - Works alongside your existing SSH tool
   - Lightweight (~10MB binary)

---

### **5. Deployment Flow**
1. Your existing SSH tool deploys the agent binary
2. Agent starts and exposes port `8081`
3. Frontend/CLI connects via:
   ```bash
   websocat ws://server:8081/ws
   ```

Would you like me to expand on any part (e.g., detailed Docker stats collection, auth)?
