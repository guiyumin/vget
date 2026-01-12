.PHONY: build build-ui build-metal build-cuda build-nocgo build-whisper push version patch minor major

BUILD_DIR := ./build
VERSION_FILE := internal/core/version/version.go
UI_DIR := ./ui
SERVER_DIST := ./internal/server/dist

# Get current version from latest git tag (strips 'v' prefix)
CURRENT_VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.0.0")

# Get whisper.cpp module path
WHISPER_PATH := $(shell go list -m -f '{{.Dir}}' github.com/ggerganov/whisper.cpp/bindings/go 2>/dev/null)

build-ui:
	cd $(UI_DIR) && npm install && npm run build
	rm -rf $(SERVER_DIST)/*
	cp -r $(UI_DIR)/dist/* $(SERVER_DIST)/

# Build whisper.cpp static library
build-whisper:
	@if [ -z "$(WHISPER_PATH)" ]; then \
		echo "Error: whisper.cpp module not found. Run 'go mod download' first."; \
		exit 1; \
	fi
	cd "$(WHISPER_PATH)" && make whisper
	@echo "whisper.cpp library built at $(WHISPER_PATH)/libwhisper.a"

# Standard build with CGO for whisper.cpp (CPU only)
# Requires: make build-whisper (run once)
build: build-ui
	CGO_ENABLED=1 \
	C_INCLUDE_PATH="$(WHISPER_PATH)" \
	LIBRARY_PATH="$(WHISPER_PATH)" \
	go build -o $(BUILD_DIR)/vget ./cmd/vget
	CGO_ENABLED=1 \
	C_INCLUDE_PATH="$(WHISPER_PATH)" \
	LIBRARY_PATH="$(WHISPER_PATH)" \
	go build -o $(BUILD_DIR)/vget-server ./cmd/vget-server

# macOS with Metal acceleration (Apple Silicon)
# Requires: WHISPER_METAL=1 make build-whisper (run once)
build-metal: build-ui
	CGO_ENABLED=1 \
	C_INCLUDE_PATH="$(WHISPER_PATH)" \
	LIBRARY_PATH="$(WHISPER_PATH)" \
	go build -tags metal -o $(BUILD_DIR)/vget ./cmd/vget
	CGO_ENABLED=1 \
	C_INCLUDE_PATH="$(WHISPER_PATH)" \
	LIBRARY_PATH="$(WHISPER_PATH)" \
	go build -tags metal -o $(BUILD_DIR)/vget-server ./cmd/vget-server

# Linux with CUDA acceleration (NVIDIA GPU)
# Requires: GGML_CUDA=1 make build-whisper (run once)
build-cuda: build-ui
	CGO_ENABLED=1 \
	C_INCLUDE_PATH="$(WHISPER_PATH)" \
	LIBRARY_PATH="$(WHISPER_PATH)" \
	CGO_CFLAGS="-I/usr/local/cuda/include" \
	CGO_LDFLAGS="-L/usr/local/cuda/lib64" \
	go build -tags cuda -o $(BUILD_DIR)/vget ./cmd/vget
	CGO_ENABLED=1 \
	C_INCLUDE_PATH="$(WHISPER_PATH)" \
	LIBRARY_PATH="$(WHISPER_PATH)" \
	CGO_CFLAGS="-I/usr/local/cuda/include" \
	CGO_LDFLAGS="-L/usr/local/cuda/lib64" \
	go build -tags cuda -o $(BUILD_DIR)/vget-server ./cmd/vget-server

# Build without CGO (uses embedded whisper.cpp binary)
build-nocgo: build-ui
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/vget ./cmd/vget
	CGO_ENABLED=0 go build -o $(BUILD_DIR)/vget-server ./cmd/vget-server

push:
	git push origin main --tags

# Version bump: make version <patch|minor|major>
version:
	@if [ -z "$(filter patch minor major,$(MAKECMDGOALS))" ]; then \
		echo "Usage: make version <patch|minor|major>"; \
		echo "Current version: $(CURRENT_VERSION)"; \
		exit 1; \
	fi

patch minor major: version
	@TYPE=$@ && \
	echo "Current version: $(CURRENT_VERSION)" && \
	NEW_VERSION=$$(echo "$(CURRENT_VERSION)" | awk -F. -v type="$$TYPE" '{ \
		split($$3, parts, "-"); \
		patch = parts[1]; \
		if (index($$3, "-") > 0) { print $$1"."$$2"."patch } \
		else if (type == "major") { print $$1+1".0.0" } \
		else if (type == "minor") { print $$1"."$$2+1".0" } \
		else { print $$1"."$$2"."$$3+1 } \
	}') && \
	BUILD_DATE=$$(date -u +"%Y-%m-%d") && \
	echo "New version: $$NEW_VERSION" && \
	echo "Build date: $$BUILD_DATE" && \
	sed -i '' 's/Version = ".*"/Version = "'$$NEW_VERSION'"/' $(VERSION_FILE) && \
	sed -i '' 's/Date    = ".*"/Date    = "'$$BUILD_DATE'"/' $(VERSION_FILE) && \
	git add $(VERSION_FILE) && \
	git commit -m "chore: bump version to v$$NEW_VERSION" && \
	git tag "v$$NEW_VERSION" && \
	echo "Created tag v$$NEW_VERSION" && \
	echo "Run 'make push' to push changes and trigger release"
