
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

