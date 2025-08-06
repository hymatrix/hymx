.PHONY: build build-linux build-darwin build-windows build-arm build-all clean

BINARY_NAME=hymx
BUILD_DIR=./build
CMD_DIR=./cmd

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)

# Linux x86_64
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)

# macOS x86_64
build-darwin:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)

# macOS / Linux ARM64
build-arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)

# Windows x86_64
build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)

build-all: build build-linux build-darwin build-arm build-windows

clean:
	rm -rf $(BUILD_DIR)/*
