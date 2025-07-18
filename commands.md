
### **NextDeploy Daemon Command Reference**

#### **1. Starting the Daemon**
```bash
# Basic start (foreground)
./nextdeploy-daemon -key-dir ~/.nextdeploy/keys

# With debug logging
./nextdeploy-daemon -debug -key-dir ~/.nextdeploy/keys

# As background daemon
./nextdeploy-daemon --daemon -key-dir ~/.nextdeploy/keys
```

#### **2. Key Management**
```bash
# View current key
cat ~/.nextdeploy/keys/current_key.json

# Manually rotate keys (send USR1 signal)
kill -USR1 $(pgrep nextdeploy-daemon)
```

#### **3. API Testing**
```bash
# Health check (no auth)
curl http://localhost:8080/health

# Get status (requires auth)
API_KEY=$(jq -r '.key_id' ~/.nextdeploy/keys/current_key.json)
curl -H "Authorization: Bearer $API_KEY" http://localhost:8080/status

# Deploy new app (example)
curl -X POST -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"app":"my-app","image":"my-image:latest"}' \
  http://localhost:8080/deploy
```

#### **4. Monitoring**
```bash
# View metrics
curl http://localhost:9090/metrics

# Check running processes
pgrep nextdeploy-daemon

# View logs (if running as daemon)
tail -f /var/log/nextdeploy.log
```

#### **5. Process Management**
```bash
# Graceful stop
kill $(pgrep nextdeploy-daemon)

# Force stop
kill -9 $(pgrep nextdeploy-daemon)

# Reload config (SIGHUP)
kill -HUP $(pgrep nextdeploy-daemon)
```

#### **6. Configuration Flags**
| Flag | Description | Default |
|------|-------------|---------|
| `-key-dir` | Key storage directory | `/var/lib/nextdeploy/keys` |
| `-port` | Main API port | `8080` |
| `-metrics-port` | Metrics port | `9090` |
| `-rotate` | Key rotation interval | `24h` |
| `-debug` | Enable debug logs | `false` |
| `--daemon` | Run in background | `false` |

---

### **Quickstart Example**
1. Start the daemon:
   ```bash
   mkdir -p ~/.nextdeploy/keys
   ./nextdeploy-daemon -key-dir ~/.nextdeploy/keys -debug
   ```

2. In another terminal, test connectivity:
   ```bash
   curl -v http://localhost:8080/health
   ```

3. Make an authenticated request:
   ```bash
   API_KEY=$(jq -r '.key_id' ~/.nextdeploy/keys/current_key.json)
   curl -H "Authorization: Bearer $API_KEY" http://localhost:8080/status
   ```

---

### **Notes**
- Always protect your `current_key.json` file (contains sensitive credentials)
- For production, consider:
  - Setting up systemd service
  - Proper log rotation
  - Firewall rules to restrict API access
# NextDeploy Debugging and Final Setup Guide

Based on your output, let's fix the remaining issues and complete the setup:

## 1. Fix Daemon Connection Issue

The error shows the daemon URL wasn't properly configured. Since you're testing locally:

```bash
# Correct the daemon URL to use localhost
export NEXTDEPLOY_DAEMON_URL="http://localhost:8080"
echo 'export NEXTDEPLOY_DAEMON_URL="http://localhost:8080"' >> ~/.bashrc
source ~/.bashrc
```

## 2. Verify Daemon is Running

```bash
# Check service status
sudo systemctl status nextdeploy

# Check logs
sudo journalctl -u nextdeploy -f
```

## 3. Test CLI Connection

```bash
# Test healthcheck
nextdeploy healthcheck

# With verbose output
nextdeploy --verbose healthcheck
```

## 4. Permission Troubleshooting

If you're still seeing permission errors:

```bash
# Ensure proper permissions on key directory
sudo chmod 700 /var/lib/nextdeploy/keys
sudo chown root:root /var/lib/nextdeploy/keys

# Verify permissions
ls -la /var/lib/nextdeploy/
```

## 5. Alternative Local Testing (Without Systemd)

If you prefer testing without systemd:

```bash
# Stop the service
sudo systemctl stop nextdeploy

# Run manually with debug
sudo /usr/local/bin/nextdeploy-daemon \
    --debug \
    --key-dir ./local-keys \
    --log-file ./daemon.log \
    --host 0.0.0.0 \
    --port 8080
```

## 6. Complete Test Sequence

```bash
# Terminal 1 - Run daemon
sudo /usr/local/bin/nextdeploy-daemon --debug

# Terminal 2 - Test CLI
nextdeploy version
nextdeploy healthcheck
nextdeploy --verbose status

# Terminal 3 - Test API directly
curl http://localhost:8080/health
curl http://localhost:8080/version
```

## Common Solutions to Errors

| Error | Solution |
|-------|----------|
| `Could not resolve host` | Use `localhost` instead of `your-server` |
| `Connection refused` | Ensure daemon is running (`sudo systemctl status nextdeploy`) |
| `Permission denied` on keys | Run `sudo chmod 700 /var/lib/nextdeploy/keys` |
| `401 Unauthorized` | Check if authentication is required in your API |
| Daemon not starting | Check logs with `journalctl -u nextdeploy -f` |

## Final Verification

1. Daemon running:
   ```bash
   ps aux | grep nextdeploy
   ```

2. Port listening:
   ```bash
   netstat -tulnp | grep 8080
   ```

3. CLI working:
   ```bash
   nextdeploy --version
   nextdeploy healthcheck
   ```

4. API accessible:
   ```bash
   curl -v http://localhost:8080/health
   ```

Would you like me to add any specific test scenarios or checks for your particular application?
# **Complete NextDeploy Build & Deployment Guide**

---

## **Table of Contents**
1. [System Requirements](#system-requirements)
2. [Building Binaries](#building-binaries)
3. [Server Deployment](#server-deployment)
4. [Developer Setup](#developer-setup)
5. [File Structure & Permissions](#file-structure--permissions)
6. [Running & Testing](#running--testing)
7. [Troubleshooting](#troubleshooting)
8. [Security Recommendations](#security-recommendations)

---

## **1. System Requirements**

### **For Server (Daemon)**
- Linux (x86_64 recommended)
- Docker installed and running
- Root/sudo access
- Ports 8080 (API) and 9090 (metrics) open

### **For Developer Machine (CLI)**
- Linux/macOS/Windows (WSL2 for Windows)
- Go 1.18+ (for development builds)
- Network access to daemon server

---

## **2. Building Binaries**

### **Automated Build Script**
```bash
go run buildhelp.go
```
**Outputs:**
- `bin/nextdeploy-daemon` (Server binary)
- `bin/nextdeploy` (CLI tool)

### **Manual Build Commands**
```bash
# Build daemon (production)
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags "-s -w -X main.Version=$(cat version.txt)" \
    -o bin/nextdeploy-daemon \
    daemon/main.go

# Build CLI (development)
go build -o bin/nextdeploy cli/main.go
```

---

## **3. Server Deployment**

### **A. Installation**
```bash
# Copy binary
sudo cp bin/nextdeploy-daemon /usr/local/bin/

# Create directories
sudo mkdir -p /var/lib/nextdeploy/{keys,logs}
sudo mkdir -p /etc/nextdeploy

# Set permissions
sudo chown -R root:root /var/lib/nextdeploy
sudo chmod 700 /var/lib/nextdeploy/keys
```

### **B. Systemd Service**
```bash
sudo tee /etc/systemd/system/nextdeploy.service <<EOF
[Unit]
Description=NextDeploy Daemon
After=network.target docker.service

[Service]
User=root
ExecStart=/usr/local/bin/nextdeploy-daemon \
    --daemon \
    --key-dir /var/lib/nextdeploy/keys \
    --log-file /var/lib/nextdeploy/logs/daemon.log \
    --host 0.0.0.0 \
    --port 8080
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Enable service
sudo systemctl daemon-reload
sudo systemctl enable --now nextdeploy
```

### **C. Verify Installation**
```bash
sudo systemctl status nextdeploy
curl http://localhost:8080/health
```

---

## **4. Developer Setup**

### **A. Install CLI**
```bash
cp bin/nextdeploy ~/.local/bin/  # Local user
# OR
sudo cp bin/nextdeploy /usr/local/bin/  # System-wide
```

### **B. Configure Connection**
```bash
echo 'export NEXTDEPLOY_DAEMON_URL="http://your-server-ip:8080"' >> ~/.bashrc
source ~/.bashrc
```

### **C. Test CLI**
```bash
nextdeploy version
nextdeploy healthcheck --verbose
```

---

## **5. File Structure & Permissions**

| Path | Permission | Owner | Purpose |
|------|-----------|-------|---------|
| `/usr/local/bin/nextdeploy-daemon` | `755` | `root:root` | Daemon binary |
| `/var/lib/nextdeploy/keys/` | `700` | `root:root` | Encryption keys |
| `/var/lib/nextdeploy/logs/` | `755` | `root:root` | Log directory |
| `/etc/nextdeploy/config.yaml` | `600` | `root:root` | Configuration |
| `~/.local/bin/nextdeploy` | `755` | User | CLI binary |

---

## **6. Running & Testing**

### **A. Development Testing (No Systemd)**
```bash
# Terminal 1: Run daemon
sudo nextdeploy-daemon --debug --key-dir ./test-keys

# Terminal 2: Test CLI
export NEXTDEPLOY_DAEMON_URL="http://localhost:8080"
nextdeploy healthcheck
```

### **B. Production Commands**
```bash
# Start/Stop
sudo systemctl start nextdeploy
sudo systemctl stop nextdeploy

# View logs
sudo journalctl -u nextdeploy -f
```

### **C. API Endpoints**
```
GET /health       # Service health
GET /version      # Version info
GET /metrics      # Prometheus metrics (port 9090)
```

---

## **7. Troubleshooting**

| Error | Solution |
|-------|----------|
| `Permission denied` on keys | `sudo chmod 700 /var/lib/nextdeploy/keys` |
| `Connection refused` | Check if daemon is running (`ps aux | grep nextdeploy`) |
| `401 Unauthorized` | Verify API authentication requirements |
| `Could not resolve host` | Use IP instead of hostname in `NEXTDEPLOY_DAEMON_URL` |
| Docker permission errors | `sudo usermod -aG docker $USER` |

---

## **8. Security Recommendations**

1. **For Production:**
   - Use HTTPS with TLS certificates
   - Restrict firewall to CLI IPs only
   - Rotate keys regularly (`--rotate` flag)
   - Monitor `/var/lib/nextdeploy/logs/`

2. **Development Tips:**
   ```bash
   # Secure local testing
   mkdir -p ./local-keys
   chmod 700 ./local-keys
   nextdeploy-daemon --key-dir ./local-keys --debug
   ```

3. **Backup Critical Files:**
   ```bash
   # Keys and config
   tar czvf nextdeploy-backup.tar.gz /var/lib/nextdeploy/keys /etc/nextdeploy
   ```

---

## **Final Checklist**
- [ ] Daemon running on server (`systemctl status nextdeploy`)
- [ ] CLI can connect (`nextdeploy healthcheck`)
- [ ] Key directory secured (`/var/lib/nextdeploy/keys`)
- [ ] Firewall allows traffic on required ports
- [ ] Backup procedure in place

For updates, always rebuild binaries and restart the daemon:
```bash
go run build.go
sudo systemctl restart nextdeploy
```
