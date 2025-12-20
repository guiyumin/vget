.PHONY: build build-ui push version patch minor major

BUILD_DIR := ./build
VERSION_FILE := internal/core/version/version.go
UI_DIR := ./ui
SERVER_DIST := ./internal/server/dist

# Get current version from latest git tag (strips 'v' prefix)
CURRENT_VERSION := $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//' || echo "0.0.0")

build-ui:
	cd $(UI_DIR) && npm install && npm run build
	rm -rf $(SERVER_DIST)/*
	cp -r $(UI_DIR)/dist/* $(SERVER_DIST)/

build: build-ui
	go build -o $(BUILD_DIR)/vget ./cmd/vget
	go build -o $(BUILD_DIR)/vget-server ./cmd/vget-server

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
		if (type == "major") { print $$1+1".0.0" } \
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