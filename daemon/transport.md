
Alright — here’s how you **turn your Go daemon into a real background process** on the server, like a proper production service.

We’re doing this the **UNIX way**: upload the binary, register it with `systemd`, and get it running in the background with auto-restarts, logging, and persistence.

---

## ✅ Step-by-Step: Run Daemon as Background Process

Let’s assume your compiled binary is called: `nextdeploy-daemon`

---

### 1. 🔨 **Build the Daemon Locally**

```bash
go build -o nextdeploy-daemon ./cmd/daemon
```

---

### 2. 📤 **Copy the Daemon to the Server**

```bash
scp nextdeploy-daemon user@your-server-ip:/usr/local/bin/
```

Or use any file uploader you trust (SCP, rsync, SFTP, etc.).

Once it’s there:

```bash
chmod +x /usr/local/bin/nextdeploy-daemon
```

---

### 3. 📄 **Create systemd Service File**

```bash
sudo nano /etc/systemd/system/nextdeploy-daemon.service
```

Paste this:

```ini
[Unit]
Description=NextDeploy Daemon Service
After=network.target docker.service

[Service]
ExecStart=/usr/local/bin/nextdeploy-daemon
Restart=always
RestartSec=5
User=root
EnvironmentFile=-/etc/nextdeploy/env
WorkingDirectory=/usr/local/bin

[Install]
WantedBy=multi-user.target
```

If you don’t have `/etc/nextdeploy/env` yet, either create it or just remove that line.

---

### 4. 💥 **Enable and Start the Daemon**

```bash
sudo systemctl daemon-reexec     # optional, safest if it's a new service
sudo systemctl daemon-reload     # reload systemd configs
sudo systemctl enable nextdeploy-daemon
sudo systemctl start nextdeploy-daemon
```

---

### 5. ✅ **Check if It’s Running**

```bash
systemctl status nextdeploy-daemon
```

You should see something like:

```
● nextdeploy-daemon.service - NextDeploy Daemon Service
   Loaded: loaded (/etc/systemd/system/nextdeploy-daemon.service; enabled)
   Active: active (running) since ...
   Main PID: 1234 (nextdeploy-daemon)
```

---

### 6. 📜 **View Logs (In Real-Time)**

```bash
journalctl -u nextdeploy-daemon -f
```

This is your **log tail**. Every `log.Println()` from your Go code will show here.

---

## 🧠 Bonus Tips

* 🛠 Want to update it later?

  ```bash
  systemctl stop nextdeploy-daemon
  scp new-binary ...
  systemctl start nextdeploy-daemon
  ```

* 🔐 Want to make it run under non-root?

  * Add a dedicated user:

    ```bash
    sudo useradd --system --no-create-home nextdeploy
    ```
  * Then change `User=root` to `User=nextdeploy` in the service file

* 🧪 Want a fake config to test with?
  Just drop a mock `nextcore.json` in `/etc/nextdeploy/nextcore.json` and log it

---

## 🧱 At This Point You Have:

* A compiled daemon
* A running background process
* Logs and restarts handled
* A persistent runtime on any VPS you control

This makes your platform **feel real**.

Next up, you’ll want to:

* Add config parsing (`nextdeploy.yml`)
* Handle HTTPS certs via Caddy or NGINX
* Auto-pull and run containerized apps

---

You ready to make the daemon read `nextdeploy.yml` next and start pulling images?

Let’s wire it.
