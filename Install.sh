#!/usr/bin/env bash
set -euo pipefail

REPO="NextDeploy/nextdeploy-daemon"
BIN_NAME="nextdeploy-daemon"
INSTALL_DIR="/usr/local/bin"
SERVICE_NAME="nextdeploy-daemon"
GITHUB_API="https://api.github.com/repos/$REPO/releases/latest"

echo "[+] Installing $BIN_NAME..."

# Ensure curl/jq are available
if ! command -v curl &> /dev/null; then
  echo "[-] curl not found, please install it."
  exit 1
fi
if ! command -v jq &> /dev/null; then
  echo "[-] jq not found, please install it."
  exit 1
fi

# Get latest release download URL
DOWNLOAD_URL=$(curl -sL $GITHUB_API | jq -r '.assets[] | select(.name | test("linux_amd64")) | .browser_download_url')

if [ -z "$DOWNLOAD_URL" ]; then
  echo "[-] Could not find a linux_amd64 binary in the latest release."
  exit 1
fi

# Download binary
curl -L "$DOWNLOAD_URL" -o "/tmp/$BIN_NAME"
chmod +x "/tmp/$BIN_NAME"

# Move to install dir
sudo mv "/tmp/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"

echo "[+] Binary installed to $INSTALL_DIR/$BIN_NAME"

# Create systemd service
SERVICE_FILE="/etc/systemd/system/$SERVICE_NAME.service"
sudo bash -c "cat > $SERVICE_FILE" <<EOF
[Unit]
Description=NextDeploy Daemon
After=network.target

[Service]
ExecStart=$INSTALL_DIR/$BIN_NAME
Restart=always
RestartSec=5
User=root

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd and enable service
sudo systemctl daemon-reload
sudo systemctl enable $SERVICE_NAME
sudo systemctl start $SERVICE_NAME

echo "[+] $SERVICE_NAME installed and running."
