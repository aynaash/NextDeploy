#!/bin/bash

# Exit on error and undefined variables
set -eu

# Configuration
SERVICE_NAME="nextdeployd"
DAEMON_BINARY="nextdeployd"
CLI_BINARY="nextdeploy"
INSTALL_DIR="/usr/local/bin"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
USER="root"
GROUP="root"

echo "🔧 Starting development installation loop for ${SERVICE_NAME}..."

# Step 1: Remove previous daemon installation
echo "1. 🗑  Removing previous daemon binaries, services, and logs..."
if systemctl is-active --quiet "${SERVICE_NAME}"; then
    echo "   → Stopping running service..."
    systemctl stop "${SERVICE_NAME}" || true
fi

if systemctl is-enabled --quiet "${SERVICE_NAME}"; then
    echo "   → Disabling service..."
    systemctl disable "${SERVICE_NAME}" || true
fi

if [ -f "${SERVICE_FILE}" ]; then
    echo "   → Removing service file..."
    rm -f "${SERVICE_FILE}"
fi

if [ -f "${INSTALL_DIR}/${DAEMON_BINARY}" ]; then
    echo "   → Removing daemon binary..."
    rm -f "${INSTALL_DIR}/${DAEMON_BINARY}"
fi

# Step 2: Remove previous CLI binary
echo "2. 🗑  Removing previous CLI binary..."
if [ -f "${INSTALL_DIR}/${CLI_BINARY}" ]; then
    echo "   → Removing CLI binary..."
    rm -f "${INSTALL_DIR}/${CLI_BINARY}"
fi

# Step 3: Install new daemon binary
echo "3. ⬇️  Installing new daemon binary..."
if [ ! -f "./${DAEMON_BINARY}" ]; then
    echo "   ❌ Error: ./${DAEMON_BINARY} not found in current directory!"
    exit 1
fi
cp "./${DAEMON_BINARY}" "${INSTALL_DIR}/"
chmod +x "${INSTALL_DIR}/${DAEMON_BINARY}"
echo "   → Daemon binary installed to ${INSTALL_DIR}/${DAEMON_BINARY}"

# Step 4: Install new CLI binary
echo "4. ⬇️  Installing new CLI binary..."
if [ ! -f "./${CLI_BINARY}" ]; then
    echo "   ❌ Error: ./${CLI_BINARY} not found in current directory!"
    exit 1
fi
cp "./${CLI_BINARY}" "${INSTALL_DIR}/"
chmod +x "${INSTALL_DIR}/${CLI_BINARY}"
echo "   → CLI binary installed to ${INSTALL_DIR}/${CLI_BINARY}"

# Step 5: Set up systemd service
echo "5. ⚙️  Configuring systemd service..."
cat <<EOF | tee "${SERVICE_FILE}" > /dev/null
[Unit]
Description=NextDeploy Daemon
After=network.target

[Service]
Type=simple
User=${USER}
Group=${GROUP}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${DAEMON_BINARY}
Restart=always
StandardOutput=syslog
StandardError=syslog
SyslogIdentifier=${SERVICE_NAME}

[Install]
WantedBy=multi-user.target
EOF

echo "   → Service file created at ${SERVICE_FILE}"

# Step 6: Reload and start service
echo "6. 🔄 Reloading systemd and starting service..."
systemctl daemon-reexec
systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"
systemctl start "${SERVICE_NAME}"

echo "   → Service status:"
systemctl status "${SERVICE_NAME}" --no-pager --lines=0

# Completion message
echo -e "\n✅ Installation complete!"
echo -e "You can now use the CLI with:\n  ${CLI_BINARY} [command]"
echo -e "\nTo check the daemon logs:\n  journalctl -u ${SERVICE_NAME} -f"
