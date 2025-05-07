#!/bin/bash
set -e

echo "🔍 Determining package manager..."
if [ -f yarn.lock ]; then
  echo "📦 Using Yarn"
  yarn install --frozen-lockfile
elif [ -f package-lock.json ]; then
  echo "📦 Using npm"
  npm ci
elif [ -f pnpm-lock.yaml ]; then
  echo "📦 Using pnpm"
  corepack enable pnpm
  pnpm install --frozen-lockfile
else
  echo "❌ No lockfile found!"
  exit 1
fi
