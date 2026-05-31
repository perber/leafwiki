BINARY_NAME=leafwiki
CMD_DIR=./cmd/leafwiki
SIDECAR_NAME=leafwiki-mcp-stdio
SIDECAR_CMD_DIR=./cmd/leafwiki-mcp-stdio
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

build-sidecar:
	go build -o $(SIDECAR_NAME) $(SIDECAR_CMD_DIR)

run:
	go run $(CMD_DIR)

clean:
	rm -f $(BINARY_NAME) $(SIDECAR_NAME)
	rm -rf $(RELEASE_DIR)

test:
	go test ./...

bench:
	go test -bench=. -benchmem -benchtime=3s ./internal/links/... ./internal/core/revision/...

# Build all platform targets
release: $(PLATFORMS)
	@echo "✅ All builds complete."

# Build for each platform using Docker
$(PLATFORMS):
	@mkdir -p $(RELEASE_DIR)
	@GOOS=$(word 1,$(subst /, ,$@)) ; \
	 GOARCH=$(word 2,$(subst /, ,$@)) ; \
	 EXT=$$( [ "$$GOOS" = "windows" ] && echo ".exe" || echo "" ) ; \
	 for target in "$(BINARY_NAME):$(CMD_DIR)" "$(SIDECAR_NAME):$(SIDECAR_CMD_DIR)"; do \
		NAME=$${target%%:*} ; \
		CMD=$${target#*:} ; \
		OUTPUT=$$NAME-$(VERSION)-$$GOOS-$$GOARCH$$EXT ; \
		IMAGE=leafwiki-builder-$$NAME-$$GOOS-$$GOARCH ; \
		echo "📦 Building $$OUTPUT..." ; \
		docker build -f $(DOCKER_BUILDER) \
			--build-arg GOOS=$$GOOS \
			--build-arg GOARCH=$$GOARCH \
			--build-arg APP_VERSION=$(VERSION) \
			--build-arg OUTPUT=$$NAME \
			--build-arg CMD_PATH=$$CMD \
			-t $$IMAGE . ; \
		ID=$$(docker create $$IMAGE) ; \
		docker cp $$ID:/out/$$NAME $(RELEASE_DIR)/$$OUTPUT ; \
		docker rm $$ID ; \
		echo "✅ Binary done: $(RELEASE_DIR)/$$OUTPUT" ; \
		sha256sum $(RELEASE_DIR)/$$OUTPUT > $(RELEASE_DIR)/$$OUTPUT.sha256 ; \
		zip -j $(RELEASE_DIR)/$$OUTPUT.zip $(RELEASE_DIR)/$$OUTPUT ; \
		tar -czf $(RELEASE_DIR)/$$OUTPUT.tar.gz -C $(RELEASE_DIR) $$OUTPUT ; \
		echo "📦 Compressed: zip and tar.gz" ; \
	 done

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
		--sbom=true \
		--provenance=mode=max \
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

run-proxy-e2e:
	@echo "🔐 Starting proxy auth E2E tests..."
	@./e2e-proxy/run.sh

run-e2e-local:
	@echo "⚡ Starting end-to-end tests (local fast path)..."
	@E2E_RUN_MODE=local ./e2e/run.sh

# Skip the Vite build when dist/ is already up to date; useful when iterating
# on tests without changing the frontend. Pass GREP=<pattern> to filter tests,
# e.g.: make run-e2e-local-fast GREP="shoutout"
run-e2e-local-fast:
	@echo "⚡ Starting end-to-end tests (local, skip UI build)..."
	@E2E_RUN_MODE=local E2E_SKIP_UI_BUILD=1 ./e2e/run.sh $(if $(GREP),--grep "$(GREP)",)

run-e2e-root-dir:
	@echo "🗂️ Starting root-dir storage boundary E2E smoke..."
	@E2E_RUN_MODE=local E2E_ENABLE_SEPARATE_ROOT_DIR=1 ./e2e/run.sh --grep "Separate root dir"

run-e2e-root-dir-stdio:
	@echo "🗂️ Starting root-dir MCP STDIO sidecar E2E smoke..."
	@E2E_RUN_MODE=local E2E_ENABLE_MCP_LOCAL=1 E2E_ENABLE_SEPARATE_ROOT_DIR=1 E2E_MCP_CLIENT_TRANSPORT=stdio ./e2e/run.sh tests/mcp-stdio-disable-auth.spec.ts --grep "mcp stdio sidecar seeds"

help:
	@echo "Available commands:"
	@echo "  make build                – Build binary for current system"
	@echo "  make build-sidecar        – Build optional MCP STDIO sidecar for current system"
	@echo "  make release              – Cross-compile binaries for all platforms (via Docker)"
	@echo "  make clean                – Clean all generated files"
	@echo "  make test                 – Run all Go tests"
	@echo "  make bench                – Run Go benchmarks for links and revision"
	@echo "  make run-e2e              – Run end-to-end tests (using Docker)"
	@echo "  make run-proxy-e2e        – Run reverse-proxy auth E2E tests (Docker + nginx)"
	@echo "  make run-e2e-local        – Run end-to-end tests via local fast path"
	@echo "  make run-e2e-local-fast   – Run E2E tests locally, skip UI build (use when dist/ is current)"
	@echo "                              Optional: GREP=<pattern> to filter tests"
	@echo "  make run-e2e-root-dir     – Run focused E2E smoke for separate root dir storage"
	@echo "  make run-e2e-root-dir-stdio – Run focused MCP STDIO smoke with separate root dir storage"
	@echo "  make run                  – Run development server"
	@echo "  make docker-build-publish – Build and push multi-arch Docker image"
	@echo "  make changelog            – Generate changelog"

.PHONY: all build build-sidecar run clean test bench fmt lint help docker-build-publish changelog run-e2e run-e2e-local run-e2e-local-fast run-e2e-root-dir run-e2e-root-dir-stdio run-proxy-e2e
