BINARY_NAME=leafwiki
CMD_DIR=./cmd/leafwiki
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || printf 'v0.1.0')
RELEASE_DIR := releases
DOCKER_BUILDER := Dockerfile.builder

PLATFORMS := \
  linux/amd64 \
  linux/arm64 \
  windows/amd64 \
  darwin/amd64 \
  darwin/arm64

all: build

build:
	go build -o $(BINARY_NAME) $(CMD_DIR)

run:
	go run $(CMD_DIR)

clean:
	rm -f $(BINARY_NAME)
	rm -rf $(RELEASE_DIR)

test:
	go test ./...

# Build all platform targets
release: $(PLATFORMS)
	@echo "✅ All builds complete."

# Build for each platform using Docker
$(PLATFORMS):
	@mkdir -p $(RELEASE_DIR)
	@GOOS=$(word 1,$(subst /, ,$@)) ; \
	 GOARCH=$(word 2,$(subst /, ,$@)) ; \
	 EXT=$$( [ "$$GOOS" = "windows" ] && echo ".exe" || echo "" ) ; \
	 OUTPUT=$(BINARY_NAME)-$(VERSION)-$$GOOS-$$GOARCH$$EXT ; \
	 echo "📦 Building $$OUTPUT..." ; \
	 docker build -f $(DOCKER_BUILDER) \
		--build-arg GOOS=$$GOOS \
		--build-arg GOARCH=$$GOARCH \
		--build-arg APP_VERSION=$(VERSION) \
		--build-arg OUTPUT=$(BINARY_NAME) \
		-t leafwiki-builder-$$GOOS-$$GOARCH . ; \
	 ID=$$(docker create leafwiki-builder-$$GOOS-$$GOARCH) ; \
	 docker cp $$ID:/out/$(BINARY_NAME) $(RELEASE_DIR)/$$OUTPUT ; \
	 docker rm $$ID ; \
	 echo "✅ Binary done: $(RELEASE_DIR)/$$OUTPUT" ; \
	 sha256sum $(RELEASE_DIR)/$$OUTPUT > $(RELEASE_DIR)/$$OUTPUT.sha256 ; \
	 zip -j $(RELEASE_DIR)/$$OUTPUT.zip $(RELEASE_DIR)/$$OUTPUT ; \
	 tar -czf $(RELEASE_DIR)/$$OUTPUT.tar.gz -C $(RELEASE_DIR) $$OUTPUT ; \
	 echo "📦 Compressed: zip and tar.gz"

# Final production Docker image
docker-build-publish:
ifndef REPO_OWNER
	$(error REPO_OWNER is not set. Usage: make docker-build-publish VERSION=vX.Y.Z REPO_OWNER=your_github_username)
endif
	docker buildx build \
		--platform linux/amd64,linux/arm64 \
		--file Dockerfile \
		--target final \
		--build-arg APP_VERSION=$(VERSION) \
		--tag ghcr.io/$(REPO_OWNER)/leafwiki:$(VERSION) \
		--tag ghcr.io/$(REPO_OWNER)/leafwiki:latest \
		--annotation "index:org.opencontainers.image.title=LeafWiki" \
		--annotation "index:org.opencontainers.image.description=LeafWiki – A fast wiki for people who think in folders, not feeds" \
		--push .

# Generate markdown changelog between two tags
changelog:
	@if [ -z "$(PREVIOUS)" ] || [ -z "$(CURRENT)" ]; then \
		echo "Usage: make changelog PREVIOUS=v0.1.0 CURRENT=v0.2.0"; \
		exit 1; \
	fi
	@./scripts/changelog.sh $(PREVIOUS) $(CURRENT)

run-e2e:
	@echo "🚀 Starting end-to-end tests..."
	@./e2e/run.sh

run-e2e-local:
	@echo "⚡ Starting end-to-end tests (local fast path)..."
	@E2E_RUN_MODE=local ./e2e/run.sh

help:
	@echo "Available commands:"
	@echo "  make build      – Build binary for current system"
	@echo "  make release    – Cross-compile binaries for all platforms (via Docker)"
	@echo "  make clean      – Clean all generated files"
	@echo "  make test       – Run all Go tests"
	@echo "  make run-e2e    – Run end-to-end tests (using Docker)"
	@echo "  make run-e2e-local – Run end-to-end tests via local fast path"
	@echo "  make run        – Run development server"
	@echo "  make docker-build-publish    – Build and push multi-arch Docker image"
	@echo "  make changelog – Generate changelog"

.PHONY: all build run clean test fmt lint help docker-build-publish changelog run-e2e run-e2e-local
