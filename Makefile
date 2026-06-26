.PHONY: help build build-all install test lint fmt clean run
.PHONY: build-linux build-darwin build-windows build-all
.PHONY: dist dist-linux dist-darwin dist-windows dist-tarball dist-zip
.PHONY: clean-all checksums

# Variables
BINARY_NAME=agentbusybox
VERSION=$(shell git describe --tags --abbrev=0 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION)"
GOBUILD_FLAGS=-trimpath
DIST_DIR=dist
CHECKSUM_FILE=$(DIST_DIR)/checksums.txt

# UPX compression (skip for macOS - not supported)
USE_UPX ?= true
ifeq ($(shell which upx 2>/dev/null),)
USE_UPX = false
endif
ifeq ($(USE_UPX),true)
UPX_CMD = upx -9
else
UPX_CMD = @true
endif

# Default target
help:
	@echo "AgentBusyBox Build System"
	@echo ""
	@echo "Build targets:"
	@echo "  build            Build for current platform"
	@echo "  build-linux      Build for Linux (amd64, arm64)"
	@echo "  build-darwin     Build for macOS (amd64, arm64)"
	@echo "  build-windows    Build for Windows (amd64, arm64)"
	@echo "  build-all        Build for all platforms and architectures"
	@echo ""
	@echo "Distribution targets:"
	@echo "  dist             Build all distribution packages"
	@echo "  dist-linux       Build Linux packages (tar.gz)"
	@echo "  dist-darwin      Build macOS packages (tar.gz)"
	@echo "  dist-windows     Build Windows packages (zip)"
	@echo ""
	@echo "Other targets:"
	@echo "  install          Install via go install"
	@echo "  test             Run tests"
	@echo "  lint             Run linter"
	@echo "  fmt              Format code"
	@echo "  clean            Remove build artifacts"
	@echo "  clean-all        Remove everything including dist"
	@echo "  checksums        Generate checksums for all dist files"
	@echo "  run              Build and run"
	@echo "  applets          List all registered applets"
	@echo "  help             Show this help"

# Build for current platform
build:
	@mkdir -p bin
	go build $(GOBUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME) .

# Platform builds
build-linux:
	@echo "Building for Linux..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 go build $(GOBUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 go build $(GOBUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 .
	@echo "Compressing Linux amd64 binary with UPX..."
	$(UPX_CMD) bin/$(BINARY_NAME)-linux-amd64

build-darwin:
	@echo "Building for macOS..."
	@mkdir -p bin
	GOOS=darwin GOARCH=amd64 go build $(GOBUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 go build $(GOBUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 .

build-windows:
	@echo "Building for Windows..."
	@mkdir -p bin
	GOOS=windows GOARCH=amd64 go build $(GOBUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe .
	GOOS=windows GOARCH=arm64 go build $(GOBUILD_FLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-arm64.exe .
	@echo "Compressing Windows amd64 binary with UPX..."
	$(UPX_CMD) bin/$(BINARY_NAME)-windows-amd64.exe

# Build all platforms
build-all: build-linux build-darwin build-windows
	@echo ""
	@echo "Build complete! Binaries in bin/"
	@ls -lh bin/

# Install
install:
	go install $(GOBUILD_FLAGS) $(LDFLAGS) .

# Test
test:
	go test -v -race ./...

# Lint
lint:
	golangci-lint run ./...

# Format
fmt:
	gofmt -w .
	goimports -w .

# Clean
clean:
	rm -rf bin/

# Clean all
clean-all: clean
	rm -rf $(DIST_DIR)

# Run
run: build
	./bin/$(BINARY_NAME)

# List applets
applets: build
	./bin/$(BINARY_NAME) --list

# Distribution: tar.gz for Linux and macOS
dist-tarball: build-linux build-darwin
	@echo ""
	@echo "Creating tarball packages..."
	@mkdir -p $(DIST_DIR)/tarball
	@for arch in amd64 arm64; do \
		echo "  Packaging $(BINARY_NAME)-linux-$${arch}.tar.gz..."; \
		cd bin && tar czf ../$(DIST_DIR)/tarball/$(BINARY_NAME)-linux-$${arch}-$(VERSION).tar.gz $(BINARY_NAME)-linux-$${arch} && cd ..; \
	done
	@for arch in amd64 arm64; do \
		echo "  Packaging $(BINARY_NAME)-darwin-$${arch}.tar.gz..."; \
		cd bin && tar czf ../$(DIST_DIR)/tarball/$(BINARY_NAME)-darwin-$${arch}-$(VERSION).tar.gz $(BINARY_NAME)-darwin-$${arch} && cd ..; \
	done

# Distribution: zip for Windows
dist-zip: build-windows
	@echo ""
	@echo "Creating Windows zip packages..."
	@mkdir -p $(DIST_DIR)/zip
	@for arch in amd64 arm64; do \
		echo "  Packaging $(BINARY_NAME)-windows-$${arch}.zip..."; \
		cd bin && zip ../$(DIST_DIR)/zip/$(BINARY_NAME)-windows-$${arch}-$(VERSION).zip $(BINARY_NAME)-windows-$${arch}.exe && cd ..; \
	done

# Platform distributions
dist-linux: build-linux
	@echo ""
	@echo "Creating Linux packages..."
	@mkdir -p $(DIST_DIR)/tarball
	@for arch in amd64 arm64; do \
		echo "  Packaging $(BINARY_NAME)-linux-$${arch}.tar.gz..."; \
		cd bin && tar czf ../$(DIST_DIR)/tarball/$(BINARY_NAME)-linux-$${arch}-$(VERSION).tar.gz $(BINARY_NAME)-linux-$${arch} && cd ..; \
	done
	@echo "Linux packages complete!"

dist-darwin: build-darwin
	@echo ""
	@echo "Creating macOS packages..."
	@mkdir -p $(DIST_DIR)/tarball
	@for arch in amd64 arm64; do \
		echo "  Packaging $(BINARY_NAME)-darwin-$${arch}.tar.gz..."; \
		cd bin && tar czf ../$(DIST_DIR)/tarball/$(BINARY_NAME)-darwin-$${arch}-$(VERSION).tar.gz $(BINARY_NAME)-darwin-$${arch} && cd ..; \
	done
	@echo "macOS packages complete!"

dist-windows: build-windows
	@echo ""
	@echo "Creating Windows packages..."
	@mkdir -p $(DIST_DIR)/zip
	@for arch in amd64 arm64; do \
		echo "  Packaging $(BINARY_NAME)-windows-$${arch}.zip..."; \
		cd bin && zip ../$(DIST_DIR)/zip/$(BINARY_NAME)-windows-$${arch}-$(VERSION).zip $(BINARY_NAME)-windows-$${arch}.exe && cd ..; \
	done
	@echo "Windows packages complete!"

# Generate checksums
checksums:
	@echo "Generating checksums..."
	@mkdir -p $(DIST_DIR)
	@cd $(DIST_DIR) && \
	find . -type f \( -name "*.tar.gz" -o -name "*.zip" \) | sort | \
	while read f; do \
		sha256sum "$$f"; \
	done > checksums.txt
	@echo "Checksums written to $(CHECKSUM_FILE)"
	@cat $(CHECKSUM_FILE)

# Build all distribution packages
dist: dist-linux dist-darwin dist-windows checksums
	@echo ""
	@echo "=========================================="
	@echo "All distribution packages built!"
	@echo ""
	@echo "Location: $(DIST_DIR)/"
	@echo ""
	@ls -lh $(DIST_DIR)/*/* 2>/dev/null || true
	@echo ""
	@echo "Checksums: $(CHECKSUM_FILE)"
	@echo "=========================================="

# Show version
version:
	@echo $(VERSION)
