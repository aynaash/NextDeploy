# NextDeploy Daemon Setup Guide

## Table of Contents
1. [Prerequisites](#prerequisites)
2. [Installation](#installation)
3. [Configuration](#configuration)
4. [Systemd Service Setup](#systemd-service-setup)
5. [Verification](#verification)
6. [Troubleshooting](#troubleshooting)
7. [Security Considerations](#security-considerations)

## Prerequisites

- Go compiler (version 1.20 or later recommended)
- Systemd or another init system
- Proper permissions to create system directories
- Basic Linux administration knowledge

## Installation

1. Clone the repository:
```bash
git clone https://github.com/aynaash/nextdeploy.git
cd nextdeploy
```

2. Build the binary:
```bash
go build -o /usr/local/bin/nextdeployd ./daemon
```

3. Create required directories:
```bash
sudo mkdir -p /var/lib/nextdeploy/keys
sudo mkdir -p /var/log/nextdeploy
It looks like you're encountering permission issues because your user's primary group is `hersi` (GID 1000), not `elitebook`. Let me help you fix this properly:

## Correct Solution for NextDeploy Directory Permissions

### 1. First, create the missing log directory:
```bash
sudo mkdir -p /var/log/nextdeploy
```

### 2. Set proper ownership using your actual group (hersi):
```bash
sudo chown -R elitebook:hersi /var/lib/nextdeploy
sudo chown -R elitebook:hersi /var/log/nextdeploy
```

### 3. Set secure permissions:
```bash
sudo chmod 750 /var/lib/nextdeploy
sudo chmod 750 /var/log/nextdeploy
sudo chmod 700 /var/lib/nextdeploy/keys
```

### 4. Verify the permissions:
```bash
ls -ld /var/lib/nextdeploy
ls -ld /var/log/nextdeploy
ls -ld /var/lib/nextdeploy/keys
```

### Alternative: Create Dedicated System User (Recommended for Production)
```bash
sudo useradd -r -s /bin/false -d /var/lib/nextdeploy nextdeploy
sudo chown -R nextdeploy:nextdeploy /var/lib/nextdeploy
sudo chown -R nextdeploy:nextdeploy /var/log/nextdeploy
```

## For the Caddy Directory Access Issue
This is separate from NextDeploy - Caddy web server runs as its own user:
```bash
# To access Caddy's directory temporarily:
sudo ls -l /var/lib/caddy

# Don't change Caddy's permissions unless you know what you're doing
# Caddy should maintain its own permissions as the 'caddy' user
```

## Why This Works
1. Your user is `elitebook` with primary group `hersi`
2. The error occurred because group `elitebook` doesn't exist
3. Using `elitebook:hersi` matches your actual user/group configuration

After making these changes, restart your NextDeploy service:
```bash
sudo systemctl restart nextdeployd
```

Remember:
- User and group names must exist on the system
- Always verify with `id` and `groups` commands
- For production, dedicated system users are more secure than using your personal account
sudo chown -R $USER:$USER /var/lib/nextdeploy /var/log/nextdeploy
```

## Configuration

The daemon accepts the following configuration flags:

| Flag | Default Value | Description |
|------|---------------|-------------|
| `--host` | `0.0.0.0` | Host to bind to |
| `--port` | `8080` | Port to listen on |
| `--key-dir` | `/var/lib/nextdeploy/keys` | Directory for key storage |
| `--rotate-freq` | `24h` | Key rotation frequency |
| `--debug` | `false` | Enable debug mode |
| `--log-format` | `json` | Log format (text/json) |
| `--metrics-port` | `9090` | Metrics server port |
| `--daemonize` | `false` | Run as daemon |
| `--pidfile` | `/var/run/nextdeploy.pid` | PID file location |
| `--log-file` | `/var/log/nextdeployd.log` | Log file location |

## Systemd Service Setup

1. Create a systemd service file at `/etc/systemd/system/nextdeployd.service`:

```ini
[Unit]
Description=NextDeploy Daemon
After=network.target

[Service]
Type=simple
User=root
Group=root
WorkingDirectory=/usr/local/bin
ExecStart=/usr/local/bin/nextdeployd \
  --host=0.0.0.0 \
  --port=8080 \
  --key-dir=/var/lib/nextdeploy/keys \
  --rotate-freq=24h \
  --debug=true \
  --log-format=json \
  --metrics-port=9090 \
  --daemonize=true \
  --pidfile=/var/run/nextdeploy.pid \
  --log-file=/var/log/nextdeployd.log
Restart=on-failure
RestartSec=5s
Environment="NEXTDEPLOY_AGENT_ID=your-agent-id"

[Install]
WantedBy=multi-user.target
```

2. Enable and start the service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable nextdeployd
sudo systemctl start nextdeployd
```

## Verification

1. Check service status:
```bash
systemctl status nextdeployd
```

2. Verify metrics endpoint:
```bash
curl http://localhost:9090/metrics
```

3. Check logs:
```bash
journalctl -u nextdeployd -f
```

## Troubleshooting

### Common Issues

1. **"failed to create key directory" error**
   - Verify directory exists: `ls -ld /var/lib/nextdeploy/keys`
   - Check permissions: `sudo chown -R $USER:$USER /var/lib/nextdeploy`

2. **Service fails to start**
   - Check logs: `journalctl -u nextdeployd -xe`
   - Test manual start: `/usr/local/bin/nextdeployd --debug=true`

3. **Flags not being recognized**
   - Ensure flag declarations exist in code before `flag.Parse()`
   - Rebuild the binary after making changes

## Security Considerations

1. **Key Management**
   - Secure key directory: `chmod 700 /var/lib/nextdeploy/keys`
   - Consider regular key rotation

2. **Network Security**
   - Restrict host binding if not needed publicly
   - Consider adding TLS/HTTPS support

3. **Logging**
   - Implement log rotation for audit logs
   - Secure log directory permissions

4. **Service Account**
   - Consider running as non-root user
   - Create dedicated user: `sudo useradd -r -s /bin/false nextdeploy`

For production deployments, always review and adjust security settings according to your organization's policies.
It appears your NextDeploy daemon is running according to systemd (`Active: active (running)`), but you're still getting connection refused errors when trying to access the endpoints. Let's troubleshoot this step by step:

## Immediate Steps to Diagnose

1. **Check if the process is actually listening**:
```bash
sudo lsof -i :8080
sudo lsof -i :9090
```

2. **Verify binding address**:
```bash
sudo netstat -tulnp | grep nextdeploy
# Or alternatively:
sudo ss -tulnp | grep nextdeploy
```

## Most Likely Causes

1. **Wrong binding address**: The daemon might be binding to 127.0.0.1 instead of 0.0.0.0
2. **Port conflict**: Another service might be using these ports
3. **Firewall blocking**: Local firewall rules might be preventing access

## Solutions to Try

### 1. Check Daemon Configuration

Edit your systemd service to explicitly log the binding address:

```bash
sudo systemctl edit nextdeployd
```

Add these overrides:
```
[Service]
Environment=GODEBUG=http2debug=1
ExecStartPre=/bin/sh -c 'echo "Attempting to bind to: %h:%p"'
```

Then restart:
```bash
sudo systemctl daemon-reload
sudo systemctl restart nextdeployd
```

### 2. Try Connecting to Different Addresses

```bash
# Try IPv4 specifically
curl -v http://127.0.0.1:8080/health

# Try IPv6 specifically
curl -v http://[::1]:8080/health

# Try all interfaces
curl -v http://0.0.0.0:8080/health
```

### 3. Check for Port Conflicts

```bash
# Check what's using ports 8080 and 9090
sudo lsof -i :8080
sudo lsof -i :9090

# Alternatively
sudo netstat -tulnp | grep -E '8080|9090'
```

### 4. Test with Different Ports

Temporarily modify your service file to use different ports (e.g., 18080 and 19090) to rule out port conflicts.

### 5. Verify Firewall Settings

```bash
# Check firewall rules
sudo ufw status
# Or for firewalld
sudo firewall-cmd --list-all

# If using ufw, allow the ports
sudo ufw allow 8080/tcp
sudo ufw allow 9090/tcp
```

### 6. Run in Foreground with Debug

Stop the service and run manually:
```bash
sudo systemctl stop nextdeployd
/usr/local/bin/nextdeployd \
  --host=0.0.0.0 \
  --port=8080 \
  --key-dir=/var/lib/nextdeploy/keys \
  --debug=true
```

## If All Else Fails

1. **Check kernel logs**:
```bash
dmesg | grep -i nextdeploy
```

2. **Strace the process**:
```bash
sudo strace -p $(pgrep nextdeployd) -f -e trace=network
```

3. **Try a different host binding**:
```bash
# Edit service to bind to specific IP
ExecStart=/usr/local/bin/nextdeployd --host=127.0.0.1 ...
```

The key indicators we need:
- Is the process actually bound to the ports?
- Is it binding to the correct interface?
- Are there any errors in the detailed logs when connection attempts are made?
# NextDeploy Daemon Troubleshooting Guide

## Current Issues

1. **Permission denied** for key directory (`/var/lib/nextdeploy/keys`)
2. **Connection refused** when trying to access API endpoints
3. **WebSocket auth key warning** during startup
4. **Obsolete syslog configuration** in systemd service

## Step-by-Step Solutions

### 1. Fix Key Directory Permissions

```bash
# Create directory with proper permissions
sudo mkdir -p /var/lib/nextdeploy/keys
sudo chown -R $USER:$USER /var/lib/nextdeploy
sudo chmod 700 /var/lib/nextdeploy/keys

# Verify permissions
ls -ld /var/lib/nextdeploy/keys
```

### 2. Update Systemd Service File

Edit `/etc/systemd/system/nextdeployd.service`:

```ini
[Unit]
Description=NextDeploy Daemon
After=network.target

[Service]
Type=simple
User=$USER  # Replace with your username
Group=$USER # Replace with your group
WorkingDirectory=/usr/local/bin
ExecStart=/usr/local/bin/nextdeployd \
  --host=0.0.0.0 \
  --port=8080 \
  --key-dir=/var/lib/nextdeploy/keys \
  --rotate-freq=24h \
  --debug=true \
  --log-format=json \
  --metrics-port=9090 \
  --daemonize=true \
  --pidfile=/var/run/nextdeploy.pid \
  --log-file=/var/log/nextdeployd.log
Restart=on-failure
RestartSec=5s
Environment="NEXTDEPLOY_AGENT_ID=your-agent-id"

[Install]
WantedBy=multi-user.target
```

Then reload systemd:

```bash
sudo systemctl daemon-reload
sudo systemctl restart nextdeployd
```

### 3. Verify Daemon Startup

Check the service status:

```bash
systemctl status nextdeployd
```

View logs:

```bash
journalctl -u nextdeployd -f --no-pager
```

### 4. Test API Endpoints

After fixing permissions and restarting:

```bash
# Health check (should work immediately)
curl -v http://localhost:8080/health

# Metrics endpoint
curl -v http://localhost:9090/metrics

# Deploy endpoint (will need proper JWT)
curl -X POST -H "Authorization: Bearer <JWT>" \
  -H "Content-Type: application/json" \
  -d '{"app":"myapp","version":"1.0.0"}' \
  http://localhost:8080/deploy

```

### 5. Resolve WebSocket Auth Warning

This warning indicates the KeyManager isn't properly initialized. Ensure:

1. The key directory exists and is accessible
2. The KeyManager is properly initialized in your code
3. The WSAuthKey is generated on startup

Add debug logging to your KeyManager initialization:

```go
func SetupKeyManager(logger *slog.Logger, keyDir string, rotateFreq time.Duration) (*KeyManager, error) {
    logger.Debug("initializing key manager", 
        "key_dir", keyDir, 
        "rotation_interval", rotateFreq)
    
    km := &KeyManager{
        keyDir:     keyDir,
        rotateFreq: rotateFreq,
        logger:     logger,
    }
    
    if err := km.loadOrGenerateKeys(); err != nil {
        logger.Error("failed to initialize key manager", "error", err)
        return nil, fmt.Errorf("failed to initialize key manager: %w", err)
    }
    
    logger.Info("key manager initialized successfully",
        "current_key_id", km.currentKey.ID,
        "next_rotation", km.nextRotation)
    
    return km, nil
}
```

### 6. Alternative Debugging Steps

If issues persist:

1. Run manually in foreground with debug:
```bash
/usr/local/bin/nextdeployd \
  --host=0.0.0.0 \
  --port=8080 \
  --key-dir=/var/lib/nextdeploy/keys \
  --debug=true
```

2. Check listening ports:
```bash
sudo netstat -tulnp | grep nextdeploy
```

3. Verify binary location:
```bash
which nextdeployd
ls -la /usr/local/bin/nextdeployd
```

## Expected Successful Output

After fixing all issues, you should see:

1. Daemon starts without errors
2. Ports 8080 and 9090 are listening
3. API endpoints respond:
   - `/health` returns 200 OK
   - `/metrics` returns Prometheus metrics
   - Authenticated endpoints work with valid JWT
   # NextDeploy API Routes Documentation

## Overview
The NextDeploy daemon exposes several HTTP endpoints for managing deployments, monitoring, and identity management. All routes are protected by middleware chains except for health checks and metrics.

## API Endpoints

### Application Management

| Endpoint | Method | Middleware | Description |
|----------|--------|------------|-------------|
| `/deploy` | POST | Auth, Logging, Recovery, CORS | Deploy a new application version |
| `/stop` | POST | Auth, Logging, Recovery | Stop a running application |
| `/restart` | POST | Auth, Logging, Recovery | Restart an application |
| `/status` | GET | Auth, Logging, Recovery | Get application status |

### Monitoring

| Endpoint | Method | Middleware | Description |
|----------|--------|------------|-------------|
| `/metrics` | GET | Logging, Recovery | System and application metrics (Prometheus format) |

### Identity Management

| Endpoint | Method | Middleware | Description |
|----------|--------|------------|-------------|
| `/submit-env` | POST | Auth, Logging, Recovery | Submit environment variables |
| `/add-identity` | POST | Auth, Logging, Recovery | Add new identity |
| `/revoke-identity` | POST | Auth, Logging, Recovery | Revoke existing identity |
| `/list-identities` | GET | Auth, Logging, Recovery | List all identities |
| `/public-key` | GET | Auth, Logging, Recovery | Get server public key |

### Infrastructure

| Endpoint | Method | Middleware | Description |
|----------|--------|------------|-------------|
| `/secrets/sync` | POST | Auth, Logging, Recovery | Synchronize secrets |

### Health Checks

| Endpoint | Method | Middleware | Description |
|----------|--------|------------|-------------|
| `/health` | GET | Logging, Recovery | Basic health check |

## Middleware Chain

All routes use a middleware chain that provides:

1. **AuthMiddleware**: JWT-based authentication using the KeyManager
2. **LoggingMiddleware**: Request/response logging
3. **RecoveryMiddleware**: Panic recovery
4. **CORSMiddleware**: CORS headers (only on `/deploy` endpoint)

## Server Configuration

### Main Server
- **Port**: Configured via `--port` flag (default: 8080)
- **Timeouts**:
  - Read: 10 seconds
  - Write: 30 seconds
  - Idle: 60 seconds

### Metrics Server
- **Port**: Configured via `--metrics-port` flag (default: 9090)
- **Timeouts**:
  - Read: 5 seconds
  - Write: 10 seconds
  - Idle: 15 seconds

## Example Usage

```bash
# Deploy application
curl -X POST -H "Authorization: Bearer <JWT>" \
  -H "Content-Type: application/json" \
  -d '{"app":"myapp","version":"1.0.0"}' \
  http://localhost:8080/deploy

# Get system metrics
curl http://localhost:9090/metrics

# Health check
curl http://localhost:8080/health
```

## Security Notes
1. All endpoints except `/health` and `/metrics` require authentication
2. The `/metrics` endpoint is exposed on a separate port with simpler middleware
3. CORS is only enabled for the `/deploy` endpoint by default
It seems the NextDeploy daemon is running but still not listening on the expected ports (8080 and 9090). Let's troubleshoot this systematically:

## Step 1: Verify Service Status
```bash
systemctl status nextdeployd --no-pager -l
```

## Step 2: Check Listening Ports
```bash
sudo ss -tulnp | grep nextdeploy
# Or alternatively:
sudo netstat -tulnp | grep nextdeploy
```

## Step 3: Examine Logs for Errors
```bash
journalctl -u nextdeployd -n 50 --no-pager
```

## Step 4: Check Configuration
The logs show your config is:
```
config:{0.0.0.0 8080 /var/lib/nextdeploy/keys 86400000000000 false json 9090 false /var/run/nextdeploy.pid /var/log/nextdeployd.log}
```

Key issues:
1. `daemonize` is false (should be true for systemd)
2. `debug` is false (should be true for troubleshooting)

## Step 5: Update Systemd Service
Edit your service file:
```bash
sudo nano /etc/systemd/system/nextdeployd.service
```

Update ExecStart line:
```ini
ExecStart=/usr/local/bin/nextdeployd \
  --host=0.0.0.0 \
  --port=8080 \
  --key-dir=/var/lib/nextdeploy/keys \
  --rotate-freq=24h \
  --debug=true \
  --log-format=json \
  --metrics-port=9090 \
  --daemonize=true \
  --pidfile=/var/run/nextdeploy.pid \
  --log-file=/var/log/nextdeployd.log
```

## Step 6: Reload and Restart
```bash
sudo systemctl daemon-reload
sudo systemctl restart nextdeployd
journalctl -u nextdeployd -f
```

## Step 7: Verify Port Binding
```bash
# Check if process is running
ps aux | grep nextdeploy

# Check port binding
sudo lsof -i :8080
sudo lsof -i :9090

# Try connecting locally
curl -v http://127.0.0.1:8080/health
```

## Step 8: Run Manually for Debugging
```bash
sudo systemctl stop nextdeployd
/usr/local/bin/nextdeployd \
  --host=0.0.0.0 \
  --port=8080 \
  --key-dir=/var/lib/nextdeploy/keys \
  --debug=true
```

## Common Solutions
1. If ports are in use:
```bash
sudo kill $(sudo lsof -t -i:8080)
sudo kill $(sudo lsof -t -i:9090)
```

2. If binding fails:
```bash
sudo setcap 'cap_net_bind_service=+ep' /usr/local/bin/nextdeployd
```

3. Check for SELinux/AppArmor:
```bash
sudo dmesg | grep -i denied
```
Based on your output, I can see that the NextDeploy key files exist in `/var/lib/nextdeploy/keys`, but the daemon still isn't binding to the ports. Let's do some targeted troubleshooting:

## Key Findings From Your Output
1. The key files exist:
   - `current_key.json` (ECDH and signing keys)
   - `ws_auth_key.json` (WebSocket auth key)
2. Files appear properly formatted
3. Directory structure is correct

## Immediate Troubleshooting Steps

### 1. Check Key File Permissions
```bash
ls -la /var/lib/nextdeploy/keys/
# Should show:
# -rw------- 1 elitebook hersi  current_key.json
# -rw------- 1 elitebook hersi  ws_auth_key.json
```

### 2. Verify Service User Can Access Keys
```bash
sudo -u elitebook cat /var/lib/nextdeploy/keys/current_key.json
```

### 3. Check For Port Conflicts
```bash
sudo lsof -i :8080 -i :9090
sudo netstat -tulnp | grep -E '8080|9090'
```

### 4. Run in Debug Mode Manually
```bash
sudo systemctl stop nextdeployd
/usr/local/bin/nextdeployd \
  --host=0.0.0.0 \
  --port=8080 \
  --metrics-port=9090 \
  --key-dir=/var/lib/nextdeploy/keys \
  --debug=true \
  --log-format=text
```

### 5. Check for Binding Errors
Look for these specific messages in the debug output:
- "Listening on [address]:[port]"
- "Failed to bind"
- "Key initialization complete"

## Common Solutions

### If You See Permission Errors:
```bash
sudo chmod 600 /var/lib/nextdeploy/keys/*.json
sudo chown elitebook:hersi /var/lib/nextdeploy/keys/*.json
```

### If Ports Are Already In Use:
```bash
sudo kill $(sudo lsof -t -i:8080)
sudo kill $(sudo lsof -t -i:9090)
```

### If Binding to 0.0.0.0 Fails:
Try binding to 127.0.0.1 instead:
```bash
/usr/local/bin/nextdeployd --host=127.0.0.1 --port=8080 --debug=true
```

## Expected Successful Output
When working properly, you should see:
1. "Server listening on [IP]:8080" in logs
2. Active ports when running `ss -tulnp`
3. Successful responses to `curl http://127.0.0.1:8080/health`

Would you like me to see the debug output from manually running the daemon? That would help identify exactly where the startup process is failing.

From your output, I can see that the `nextdeployd` service is already stopped and inactive. Here's what happened:

1. You successfully killed the process with:
```bash
sudo pkill -f nextdeployd
```

2. The systemd status shows:
- Service is `inactive (dead)`
- Was stopped at `17:36:13 EAT`
- Main PID was `1966` (now killed)
- Logs show normal shutdown: "Deactivated successfully"

## Next Steps

### 1. To confirm no remaining processes:
```bash
pgrep -fl nextdeployd  # Should return nothing
ps aux | grep nextdeployd  # Should show no running processes
```

### 2. To restart the service:
```bash
sudo systemctl start nextdeployd
sudo systemctl status nextdeployd  # Verify it's running
```

### 3. If you need to completely reset:

First make sure it's stopped:
```bash
sudo systemctl stop nextdeployd
sudo pkill -9 -f nextdeployd  # Force kill if needed
```

Then clear any residual files:
```bash
sudo rm -f /var/run/nextdeploy.pid
```

Finally start fresh:
```bash
sudo systemctl daemon-reload
sudo systemctl start nextdeployd
```

### 4. To permanently disable (if needed):
```bash
sudo systemctl disable --now nextdeployd
```

The service appears to have been properly terminated already. You can now either:
- Restart it if needed (`sudo systemctl start nextdeployd`)
- Investigate why it wasn't binding to ports (check logs with `journalctl -u nextdeployd`)
- Make configuration changes before restarting
Based on the complete context of your NextDeploy system, there are several potential reasons why your routes might not be working as expected. Let's systematically diagnose and solve this:

---

## **Root Causes & Solutions**

### **1. Authentication Failures (Most Likely)**
#### **Symptoms**:
- `curl http://localhost:8080/deploy` fails with no response or `401 Unauthorized`
- Logs show `"missing authentication headers"` or `"invalid signature"`

#### **Why It Happens**:
- All routes (except `/health` and `/metrics`) require:
  - **JWT** in `Authorization` header (via `AuthMiddleware`)
  - **Ed25519 signature** in `X-Signature` header (via `AuthenticateRequest`)
- Your `curl` test doesn't include these headers.

#### **How to Fix**:
##### **For JWT (AuthMiddleware)**:
```bash
# Get a valid token from your KeyManager first
TOKEN=$(curl -s http://localhost:8080/public-key | jq -r .token)

# Then call /deploy with the token
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/deploy
```

##### **For Ed25519 Signing (Advanced)**:
If using cryptographic auth:
```bash
# 1. Get your private key (usually from /var/lib/nextdeploy/keys/current_key.json)
PRIV_KEY=$(cat /var/lib/nextdeploy/keys/current_key.json | jq -r .private_key)

# 2. Sign the request body (example)
SIGNATURE=$(echo -n "POST /deploy {}" | openssl dgst -sha512 -sign <(echo "$PRIV_KEY") | base64 -w 0)

# 3. Call the API
curl -X POST \
  -H "X-Signature: $SIGNATURE" \
  -H "X-Fingerprint: YOUR_FINGERPRINT" \
  http://localhost:8080/deploy
```

---

### **2. Missing HTTP Method (POST Required)**
#### **Symptoms**:
- `405 Method Not Allowed` errors in logs
- Routes like `/deploy` expect `POST`, but `curl` defaults to `GET`

#### **Fix**:
```bash
curl -X POST http://localhost:8080/deploy
```

---

### **3. Port Conflicts**
#### **Symptoms**:
- `curl` fails immediately with "Connection refused"
- `sudo ss -tulnp | grep 8080` shows no process listening

#### **Why It Happens**:
- Another service (or stale `nextdeployd`) might be using the port.
- The daemon crashed but didn't release the port.

#### **Fix**:
```bash
# Kill any conflicting processes
sudo fuser -k 8080/tcp 9090/tcp

# Restart NextDeploy
sudo systemctl restart nextdeployd
```

---

### **4. Daemon Not Running**
#### **Symptoms**:
- `systemctl status nextdeployd` shows `inactive (dead)`
- No logs in `/var/log/nextdeployd.log`

#### **Fix**:
```bash
# Start the daemon (if disabled)
sudo systemctl enable --now nextdeployd

# Or run manually in foreground for debugging
nextdeployd --debug --daemonize=false
```

---

### **5. WebSocket vs. HTTP Confusion**
#### **Symptoms**:
- Routes like `/ws` work, but HTTP APIs fail
- Logs show WebSocket handshakes but no HTTP traffic

#### **Why It Happens**:
- Your `main.go` registers WebSocket routes **after** calling `SetupServers()`, so they might override HTTP routes.

#### **Fix**:
Move WebSocket setup **before** starting servers:
```go
// In main.go:
http.HandleFunc("/ws", wsHandler)  // Register first
mainServer, metricsServer := core.SetupServers(...)  // Then setup HTTP
```

---

### **6. Key Management Issues**
#### **Symptoms**:
- `"invalid public key encoding"` or `"key not found"` errors
- Empty `/var/lib/nextdeploy/keys/` directory

#### **Fix**:
```bash
# Regenerate keys (if missing)
sudo nextdeployd --key-dir=/var/lib/nextdeploy/keys --rotate-freq=24h
```

---

## **Debugging Checklist**
1. **Check Logs**:
   ```bash
   sudo journalctl -u nextdeployd -f
   tail -f /var/log/nextdeployd.log
   ```

2. **Test Health Endpoint (No Auth)**:
   ```bash
   curl http://localhost:8080/health  # Should return 200
   ```

3. **Verify Listening Ports**:
   ```bash
   sudo lsof -i :8080 -i :9090
   ```

4. **Test with Full Auth**:
   ```bash
   curl -H "Authorization: Bearer $TOKEN" -X POST http://localhost:8080/deploy
   ```

---

## **Summary of Fixes**
| Issue                  | Solution                                  |
|------------------------|-------------------------------------------|
| Missing auth headers   | Add `Authorization` and `X-Signature`     |
| Wrong HTTP method      | Use `-X POST`                             |
| Port conflict          | Kill stale processes, restart daemon      |
| Daemon not running     | Start with `systemctl` or debug mode      |
| WebSocket route clash  | Register HTTP routes first                |
| Key problems           | Regenerate keys in `/var/lib/nextdeploy/keys/` |

If you’re still stuck, share the **logs** and the exact `curl` command you’re using. I can help diagnose further!
