#!/bin/bash

# Build and run locally
go run start.go build -target daemon
./bin/nextdeployd \
  -key-dir ~/.nextdeploy/keys \
  -log-file ~/.nextdeploy/logs/daemon.log \
  -pid-file ~/.nextdeploy/nextdeploy.pid \
  -port 8080 \
  -host localhost
