 #!/usr/bin/env bash
set -euo pipefail

# Config
APP_NAME="nextdeploy-daemon"
INSTALL_DIR="/usr/local/bin"
SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"

echo "📦 Installing $APP_NAME ..."

# Ensure jq is available
if ! command -v jq &>/dev/null; then
  echo "⚠️  jq not found, installing..."
   sudo apt-get update -y && apt-get install -y jq || yum install -y jq
fi

# Get latest release version
VERSION=$(curl -s https://api.github.com/repos/aynaash/NextDeploy/releases/latest | jq -r .tag_name)
OS="linux"
ARCH=$(uname -m)
if [ "$ARCH" = "x86_64" ]; then ARCH="amd64"; fi
if [ "$ARCH" = "aarch64" ]; then ARCH="arm64"; fi

# Download binary
TMP_FILE=$(mktemp)
echo "⬇️  Downloading $APP_NAME $VERSION for $OS-$ARCH ..."
curl -L "https://github.com/aynaash/NextDeploy/releases/download/${VERSION}/${APP_NAME}-${OS}-${ARCH}" -o "$TMP_FILE"
chmod +x "$TMP_FILE"
mv "$TMP_FILE" "${INSTALL_DIR}/${APP_NAME}"

echo "✅ Binary installed at ${INSTALL_DIR}/${APP_NAME}"

# Create systemd service
echo "⚙️  Creating systemd service..."
cat <<EOF | tee "$SERVICE_FILE" > /dev/null
[Unit]
Description=NextDeploy Daemon
After=network.target docker.service
Requires=docker.service

[Service]
ExecStart=${INSTALL_DIR}/${APP_NAME} --config /etc/nextdeploy/config.yml
Restart=always
RestartSec=5
LimitNOFILE=65535
StandardOutput=journal
StandardError=journal
User=root
Environment="ENV=production"

[Install]
WantedBy=multi-user.target
EOF

# Reload and enable service
systemctl daemon-reexec
systemctl daemon-reload
systemctl enable "${APP_NAME}.service"
systemctl start "${APP_NAME}.service"
sudo nextdeployd daemon 

echo "🚀 $APP_NAME service installed and started."
echo "👉 Manage it with:"
echo "   systemctl status $APP_NAME"
echo "   systemctl restart $APP_NAME"
echo "   journalctl -u $APP_NAME -f"
