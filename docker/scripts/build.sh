#!/bin/bash
set -e
# First build nextjs app

if [ -f yarn.lock]; then
  yarn build
elif [ -f package-lock.json]; then
  npm run build
fi
