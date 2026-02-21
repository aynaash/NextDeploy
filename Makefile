
# NextDeploy Build Makefile
.PHONY: help build build-cli build-daemon build-all clean test lint security-scan cross-build install

# Build variables
VERSION ?= $(shell cat version.txt 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILDER ?= $(shell whoami)@$(shell hostname)

# Go build flags
GOFLAGS := -trimpath
LDFLAGS := -s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${BUILD_DATE} -X main.builder=${BUILDER}

# Directories
BIN_DIR := bin
DIST_DIR := dist

# Platform targets for CLI (multiplatform)
CLI_PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64 \
	windows/amd64

# Platform targets for Daemon (Linux only)
DAEMON_PLATFORMS := \
	linux/amd64 \
	linux/arm64

# Default target
help: ## Display this help message
	@echo "NextDeploy Build System"
	@echo "======================="
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# Clean build artifacts
clean: ## Clean build artifacts
	@echo "🧹 Cleaning build artifacts..."
	@rm -rf $(BIN_DIR)/* $(DIST_DIR)/*
	@echo "✅ Clean complete"

# Install dependencies
deps: ## Install build dependencies
	@echo "📦 Installing dependencies..."
	@go mod download
	@go mod verify
	@echo "✅ Dependencies installed"

# Run tests
test: ## Run tests with coverage
	@echo "🧪 Running tests..."
	@go test -race -coverprofile=coverage.out -covermode=atomic -v ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Tests complete - coverage report: coverage.html"

# Run linting
lint: ## Run linting and formatting checks
	@echo "🔍 Running linting..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Installing golangci-lint..."; go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest; }
	@golangci-lint run --timeout=5m
	@echo "📝 Checking formatting..."
	@if [ "$$(gofmt -s -l . | wc -l)" -gt 0 ]; then echo "❌ Files need formatting:"; gofmt -s -l .; exit 1; fi
	@echo "✅ Linting complete"

# Security scanning
security-scan: ## Run security scans
	@echo "🔒 Running security scan..."
	@command -v gosec >/dev/null 2>&1 || { echo "Installing gosec..."; go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest; }
	@gosec ./...
	@command -v govulncheck >/dev/null 2>&1 || { echo "Installing govulncheck..."; go install golang.org/x/vuln/cmd/govulncheck@latest; }
	@govulncheck ./...
	@echo "✅ Security scan complete"

# Build single CLI binary (native platform)
build-cli: ## Build CLI binary for current platform
	@echo "🔨 Building CLI for current platform..."
	@mkdir -p $(BIN_DIR)
	@go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/nextdeploy ./cli
	@echo "✅ CLI built: $(BIN_DIR)/nextdeploy"

# Build single daemon binary (Linux only)
build-daemon: ## Build daemon binary for current platform (Linux)
	@echo "🔨 Building daemon for current platform..."
	@mkdir -p $(BIN_DIR)
	@if [ "$$(go env GOOS)" != "linux" ]; then \
		echo "⚠️  Daemon only supports Linux - building for linux/amd64"; \
		GOOS=linux GOARCH=amd64 go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/nextdeployd ./daemon/cmd/daemon; \
	else \
		go build $(GOFLAGS) -ldflags="$(LDFLAGS)" -o $(BIN_DIR)/nextdeployd ./daemon/cmd/daemon; \
	fi
	@echo "✅ Daemon built: $(BIN_DIR)/nextdeployd"

# Build both binaries
build: build-cli build-daemon ## Build both CLI and daemon

# Cross-compile CLI for all platforms
cross-build-cli: ## Cross-compile CLI for all supported platforms
	@echo "🔨 Cross-compiling CLI for all platforms..."
	@mkdir -p $(DIST_DIR)
	@for platform in $(CLI_PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d/ -f1); \
		GOARCH=$$(echo $$platform | cut -d/ -f2); \
		OUTPUT_NAME="nextdeploy-$$GOOS-$$GOARCH"; \
		if [ "$$GOOS" = "windows" ]; then OUTPUT_NAME="$$OUTPUT_NAME.exe"; fi; \
		echo "Building $$OUTPUT_NAME..."; \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH go build $(GOFLAGS) \
			-ldflags="$(LDFLAGS)" \
			-o $(DIST_DIR)/$$OUTPUT_NAME ./cli; \
		if command -v sha256sum >/dev/null; then \
			cd $(DIST_DIR) && sha256sum $$OUTPUT_NAME > $$OUTPUT_NAME.sha256 && cd ..; \
		fi; \
	done
	@echo "✅ CLI cross-compilation complete"

# Cross-compile daemon for Linux platforms
cross-build-daemon: ## Cross-compile daemon for Linux platforms
	@echo "🔨 Cross-compiling daemon for Linux platforms..."
	@mkdir -p $(DIST_DIR)
	@for platform in $(DAEMON_PLATFORMS); do \
		GOOS=$$(echo $$platform | cut -d/ -f1); \
		GOARCH=$$(echo $$platform | cut -d/ -f2); \
		OUTPUT_NAME="nextdeployd-$$GOOS-$$GOARCH"; \
		echo "Building $$OUTPUT_NAME..."; \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH go build $(GOFLAGS) \
			-ldflags="$(LDFLAGS)" \
			-o $(DIST_DIR)/$$OUTPUT_NAME ./daemon/cmd/daemon; \
		if command -v sha256sum >/dev/null; then \
			cd $(DIST_DIR) && sha256sum $$OUTPUT_NAME > $$OUTPUT_NAME.sha256 && cd ..; \
		fi; \
	done
	@echo "✅ Daemon cross-compilation complete"

# Cross-compile everything
cross-build: cross-build-cli cross-build-daemon ## Cross-compile for all supported platforms

# Build everything (current + cross-platform)
build-all: build cross-build ## Build everything (local + cross-platform)

# Install binaries to system
install: build ## Install binaries to system PATH
	@echo "📦 Installing binaries to system..."
	@sudo cp $(BIN_DIR)/nextdeploy /usr/local/bin/
	@sudo cp $(BIN_DIR)/nextdeployd /usr/local/bin/
	@sudo chmod +x /usr/local/bin/nextdeploy /usr/local/bin/nextdeployd
	@echo "✅ Binaries installed to /usr/local/bin/"

# Development workflow
dev-cli: ## Watch CLI code and rebuild binary on changes
	@command -v air >/dev/null 2>&1 || { echo "Installing air..."; go install github.com/air-verse/air@latest; }
	@air -c .air.cli.toml

dev-daemon: ## Watch daemon code, rebuild and restart on changes 
	@command -v air >/dev/null 2>&1 || { echo "Installing air..."; go install github.com/air-verse/air@latest; }
	@air -c .air.daemon.toml

dev-check: deps lint test security-scan ## Run all development checks

# Release preparation
release-prep: clean dev-check build-all ## Prepare for release

# Show build info
info: ## Show build information
	@echo "Build Information"
	@echo "================="
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Date: $(BUILD_DATE)"
	@echo "Builder: $(BUILDER)"
	@echo "Go Version: $$(go version)"
	@echo "GOOS: $$(go env GOOS)"
	@echo "GOARCH: $$(go env GOARCH)"

# Docker build
docker-build: ## Build Docker image
	@echo "🐳 Building Docker image..."
	@docker build -t nextdeploy:$(VERSION) .
	@docker build -t nextdeploy:latest .
	@echo "✅ Docker image built"

# Docker multi-platform build
docker-buildx: ## Build multi-platform Docker image
	@echo "🐳 Building multi-platform Docker image..."
	@docker buildx build --platform linux/amd64,linux/arm64 -t nextdeploy:$(VERSION) -t nextdeploy:latest .
	@echo "✅ Multi-platform Docker image built"

# List all available targets
list: ## List all make targets
	@$(MAKE) -pRrq -f $(lastword $(MAKEFILE_LIST)) : 2>/dev/null | awk -v RS= -F: '/^# File/,/^# Finished Make data base/ {if ($$1 !~ "^[#.]") {print $$1}}' | sort | egrep -v -e '^[^[:alnum:]]' -e '^$@$$'
