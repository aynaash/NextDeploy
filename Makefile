
BINARY_NAME=nextdeploy
BIN_DIR=bin
VERSION?=dev

# Default build (Linux, local arch)
build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME)

# Cross compile for all major platforms
build-all:
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY_NAME)-linux-amd64
	GOOS=darwin GOARCH=arm64 go build -o $(BIN_DIR)/$(BINARY_NAME)-darwin-arm64
	GOOS=darwin GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY_NAME)-darwin-amd64
	GOOS=windows GOARCH=amd64 go build -o $(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe

# Clean binaries
clean:
	rm -rf $(BIN_DIR)

# Create a GitHub release with binaries attached
release: build-all
	gh release create $(VERSION) \
		$(BIN_DIR)/$(BINARY_NAME)-linux-amd64 \
		$(BIN_DIR)/$(BINARY_NAME)-darwin-arm64 \
		$(BIN_DIR)/$(BINARY_NAME)-darwin-amd64 \
		$(BIN_DIR)/$(BINARY_NAME)-windows-amd64.exe \
		--title "$(BINARY_NAME) $(VERSION)" \
		--notes "Pre-release $(VERSION). 🚀

This build is not production ready. Please test and report issues." \
		--prerelease
