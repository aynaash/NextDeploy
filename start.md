### **NextDeploy System - Command Summary & Insights**  

#### **📌 Core Commands**  

| Command | Description | Usage Examples |  
|---------|------------|----------------|  
| **`build`** | Builds `nextdeployd` (daemon) and/or `nextdeploy` (CLI) | `go run main.go build -target all` (default) <br> `go run main.go build -target daemon` <br> `go run main.go build -target cli -output ./build` |  
| **`run`** | Starts the `nextdeployd` daemon | `go run main.go run` (default: `127.0.0.1:8080`) <br> `go run main.go run -port 9000 -host 0.0.0.0 -debug` |  
| **`dev`** | Sets up a development environment (auto-builds + runs daemon) | `go run main.go dev` |  

---

### **🔧 Key Flags & Configurations**  

#### **Build Flags (`build` command)**  
| Flag | Description | Default |  
|------|-------------|---------|  
| `-target` | Build target (`daemon`, `cli`, `all`) | `all` |  
| `-output` | Output directory for binaries | `./bin` |  
| `-version` | Override version number | (Git/VERSION file) |  

#### **Run Flags (`run` command)**  
| Flag | Description | Default |  
|------|-------------|---------|  
| `-host` | Daemon bind address | `127.0.0.1` |  
| `-port` | Daemon port | `8080` |  
| `-key-dir` | Directory for cryptographic keys | `~/.nextdeploy/keys` |  
| `-log-dir` | Log directory | `~/.nextdeploy/logs` |  
| `-pid-file` | PID file location | `~/.nextdeploy/nextdeployd.pid` |  
| `-debug` | Enable debug mode | `false` |  

---

### **⚡ Environment Variables**  
| Variable | Purpose | Example |  
|----------|---------|---------|  
| `NEXTDEPLOY_VERSION` | Override build version | `NEXTDEPLOY_VERSION=2.0.0 go run main.go build` |  
| `BUILD_STATIC` | Disable static linking | `BUILD_STATIC=false go run main.go build` |  
| `RUNTIME` | Cross-compile for a different OS | `RUNTIME=linux go run main.go build` |  

---

### **📂 Default File Structure**  
```
~/.nextdeploy/
├── keys/          # Secure key storage (700 permissions)
├── logs/          # Log files (nextdeployd.log, development.log)
└── nextdeployd.pid  # PID file for daemon process
```

---

### **🔒 Security & Best Practices**  
✅ **Daemon Deployment**  
- Runs on `127.0.0.1` by default (change to `0.0.0.0` only if needed).  
- Keys directory (`~/.nextdeploy/keys`) is automatically set to `700` permissions.  

✅ **Production Setup**  
```bash
# Deploy the daemon
scp ./bin/nextdeployd user@server:/usr/local/bin/
ssh server "sudo chown root:root /usr/local/bin/nextdeployd && sudo chmod 755 /usr/local/bin/nextdeployd"

# Set up systemd service
scp contrib/nextdeployd.service user@server:/etc/systemd/system/
ssh server "sudo systemctl enable --now nextdeployd"
```

✅ **CLI Setup**  
```bash
# Install CLI locally
install -m 755 ./bin/nextdeploy ~/.local/bin/

# Configure endpoint
echo 'export NEXTDEPLOY_ENDPOINT="http://localhost:8080"' >> ~/.bashrc
source ~/.bashrc

# Test connection
nextdeploy ping
```

---

### **🚀 Development Workflow**  
1. **Start Dev Environment**  
   ```bash
   go run main.go dev
   ```
   - Auto-builds `nextdeployd` + `nextdeploy`.  
   - Creates `~/.nextdeploy/{keys,logs}`.  
   - Runs daemon in **debug mode** (`-debug`).  

2. **Check Logs**  
   ```bash
   tail -f ~/.nextdeploy/logs/development.log
   ```

3. **Stop Daemon**  
   ```bash
   pkill -f nextdeployd  # Or kill $(cat ~/.nextdeploy/nextdeployd.pid)
   ```

---

### **💡 Key Insights**  
✔ **Consistent Naming** – No more confusion between `ndctl` and `nextdeploy`.  
✔ **Secure Defaults** – Runs on `localhost`, strict key permissions.  
✔ **Better Logging** – Structured logs in `~/.nextdeploy/logs/`.  
✔ **Easy Cross-Compilation** – Use `RUNTIME=linux` to build for Linux.  
✔ **Dev vs. Production** – `dev` command simplifies testing.  

---
### **📜 Final Notes**  
- **Always** check logs (`~/.nextdeploy/logs/`) if something fails.  
- For **production**, use **systemd** (`nextdeployd.service`).  
- The CLI (`nextdeploy`) connects to `http://localhost:8080` by default.  

---
Would you like a **cheat sheet** version of this? 🚀
