#!/bin/bash
set -e

echo "🚀 Starting application..."
if [ -f server.js ]; then
  exec node server.js
else
  # Determine package manager for start command
  if [ -f yarn.lock ]; then
    exec yarn start
  elif [ -f package-lock.json ]; then
    exec npm start
  elif [ -f pnpm-lock.yaml ]; then
    exec pnpm start
  else
    echo "❌ No valid start command found!"
    exit 1
  fi
fi
