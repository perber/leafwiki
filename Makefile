BINARY_NAME=leafwiki
CMD_DIR=./cmd/leafwiki
VERSION ?= $(shell ./scripts/resolve-version.sh)
RELEASE_DIR := releases
DOCKER_BUILDER := Dockerfile.builder
UI_DIR := ui/leafwiki-ui
HTTP_DIST := internal/http/dist
# Single source for the golangci-lint version; the lint-go job in
# .github/workflows/backend.yml reads the same file.
GOLANGCI_LINT_VERSION ?= $(shell cat .golangci-lint-version)
EMBED_LDFLAGS := -X github.com/perber/wiki/internal/http.EmbedFrontend=true -X github.com/perber/wiki/internal/http.Environment=production
LDFLAGS := -X main.Version=$(VERSION) $(EMBED_LDFLAGS)

PLATFORMS := \
  linux/amd64 \
  linux/386 \
  linux/arm64 \
  windows/amd64 \
  darwin/amd64 \
  darwin/arm64

# Release targets other than linux, checked by `make cross` (and in CI) so a
# platform-specific compile error doesn't first show up at the release tag.
CROSS_PLATFORMS ?= windows/amd64 darwin/arm64

all: build

# Build the Vite SPA into internal/http/dist for go:embed.
ui:
	@echo "Building frontend..."
	cd $(UI_DIR) && npm ci --ignore-scripts && VITE_API_URL=/ APP_VERSION=$(VERSION) npm run build
	rm -rf $(HTTP_DIST)
	mkdir -p $(HTTP_DIST)
	cp -R $(UI_DIR)/dist/. $(HTTP_DIST)/
	touch $(HTTP_DIST)/.gitkeep
	@test -f $(HTTP_DIST)/index.html || (echo "Frontend build missing $(HTTP_DIST)/index.html" && exit 1)

# Self-contained binary with embedded UI (matches release/Docker builds).
build: ui
	go build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) $(CMD_DIR)

# API-only binary for local Vite-proxied development (no embedded SPA).
build-api:
	go build -ldflags "-X main.Version=$(VERSION)" -o $(BINARY_NAME) $(CMD_DIR)

run:
	go run -ldflags "-X main.Version=$(VERSION)" $(CMD_DIR)

clean:
	rm -f $(BINARY_NAME)
	rm -rf $(RELEASE_DIR)

test:
	go test ./...

# Built with the toolchain from go.mod, so the result matches CI.
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION) run

# Frontend checks, in two halves so CI can run them as parallel jobs
# (.github/workflows/frontend.yml); ui-check runs both.
ui-deps:
	cd $(UI_DIR) && npm ci --ignore-scripts

ui-lint: ui-deps
	cd $(UI_DIR) && npm run lint -- --max-warnings=0 && npm run format:check

# `npm run build` is the type check (tsc -b).
ui-test: ui-deps
	cd $(UI_DIR) && npm test && npm run build

ui-check: ui-lint ui-test

# The local subset of the pull-request checks. Not included: the linux/386
# tests, govulncheck, Trivy, the install-script test, the e2e lint and the E2E
# suites.
# CI jobs that keep their own command: test-go (go test with coverage for the
# report artifact), lint-go (golangci-lint-action, for PR annotations and its
# cache) and the e2e lint (npm in e2e/, which has no Make target).
ci: verify lint test cross ui-check

bench:
	go test -bench=. -benchmem -benchtime=3s ./internal/links/... ./internal/core/revision/...

# Build and vet (which also type-checks _test.go files) for each of
# CROSS_PLATFORMS, without cgo, like the release builds.
cross:
	@for platform in $(CROSS_PLATFORMS); do \
	  goos=$${platform%/*}; goarch=$${platform#*/}; \
	  echo "Building and vetting for $$goos/$$goarch..."; \
	  CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch go build ./... || exit 1; \
	  CGO_ENABLED=0 GOOS=$$goos GOARCH=$$goarch go vet ./... || exit 1; \
	done

# Small repository checks that need neither Go nor Node; CI runs the same
# target (.github/workflows/verify.yml).
verify:
	./scripts/check-private-references.sh
	./scripts/check-toolchain-versions.sh

# Conventional Commit subjects for BASE..HEAD; CI runs the same script
# (.github/workflows/commit-messages.yml).
BASE ?= origin/main
verify-commits:
	./scripts/check-commit-messages.sh "$(BASE)..HEAD"

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
	 (cd $(RELEASE_DIR) && sha256sum $$OUTPUT > $$OUTPUT.sha256) ; \
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

help:
	@echo "Available commands:"
	@echo "  make build                – Build self-contained binary with embedded UI (needs Node.js)"
	@echo "  make build-api            – Build API-only binary (use with Vite in Dev Setup)"
	@echo "  make ui                   – Build frontend into internal/http/dist for embedding"
	@echo "  make release              – Cross-compile binaries for all platforms (via Docker)"
	@echo "  make clean                – Clean all generated files"
	@echo "  make test                 – Run all Go tests"
	@echo "  make lint                 – Run golangci-lint (version from .golangci-lint-version)"
	@echo "  make ui-check             – Frontend install, lint, format check, tests and build"
	@echo "                              (ui-lint and ui-test run the two halves)"
	@echo "  make ci                   – verify, lint, test, cross and ui-check, like CI"
	@echo "  make verify               – Run small repository checks (private references, toolchain versions)"
	@echo "  make verify-commits       – Check commit subjects since BASE (default origin/main)"
	@echo "  make bench                – Run Go benchmarks for links and revision"
	@echo "  make cross                – Build and vet for windows/amd64 and darwin/arm64"
	@echo "                              Optional: CROSS_PLATFORMS=\"<goos>/<goarch> ...\""
	@echo "  make run-e2e              – Run end-to-end tests (using Docker)"
	@echo "  make run-proxy-e2e        – Run reverse-proxy auth E2E tests (Docker + nginx)"
	@echo "  make run-e2e-local        – Run end-to-end tests via local fast path"
	@echo "  make run-e2e-local-fast   – Run E2E tests locally, skip UI build (use when dist/ is current)"
	@echo "                              Optional: GREP=<pattern> to filter tests"
	@echo "  make run                  – Run development server (API only; use Vite for UI)"
	@echo "  make docker-build-publish – Build and push multi-arch Docker image"
	@echo "  make changelog            – Generate changelog"

.PHONY: all build build-api ui run clean test lint ui-deps ui-lint ui-test ui-check ci bench cross verify verify-commits help docker-build-publish changelog run-e2e run-e2e-local run-e2e-local-fast run-proxy-e2e
