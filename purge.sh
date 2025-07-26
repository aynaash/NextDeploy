#!/bin/bash
set -eu

BINARIES=("nextdeploy" "nextdeployd")
PATHS=("/usr/local/bin" "/usr/bin" "/bin" "$HOME/bin" "$HOME/.local/bin")

echo "🧹 Starting binary cleanup..."

# Remove from known directories
for bin in "${BINARIES[@]}"; do
  for path in "${PATHS[@]}"; do
    fullpath="${path}/${bin}"
    if [ -f "$fullpath" ]; then
      echo "❌ Removing $fullpath"
      rm -f "$fullpath"
    fi
  done
done

# Kill any running processes
for bin in "${BINARIES[@]}"; do
  echo "🔪 Killing any running '$bin'..."
  pkill -f "$bin" || true
done

# Optional: Locate and nuke rogue binaries (comment out if too aggressive)
echo "🔍 Searching filesystem for stray binaries..."
for bin in "${BINARIES[@]}"; do
  found=$(find / -type f -name "$bin" 2>/dev/null || true)
  if [ -n "$found" ]; then
    echo "⚠️ Found stray binary: $found"
    echo "❌ Deleting..."
    echo "$found" | xargs -r rm -f
  fi
done

echo "🧨 Removing systemd service if it exists..."
if [ -f "/etc/systemd/system/nextdeployd.service" ]; then
  systemctl stop nextdeployd || true
  systemctl disable nextdeployd || true
  rm -f /etc/systemd/system/nextdeployd.service
  systemctl daemon-reload
  echo "✔️ nextdeployd.service removed."
fi
echo "✅ Cleanup complete."
