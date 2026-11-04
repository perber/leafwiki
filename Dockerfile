# Step 1: Frontend
FROM node:26-alpine@sha256:c6ad9e9c1d80f6eb607c26b4b0a132ff7a2344fad1c78ea3baa2a7d4c8de6d94 AS frontend-build
WORKDIR /app
ARG APP_VERSION
COPY ./ui/leafwiki-ui/package*.json ./
RUN npm ci --ignore-scripts
COPY ./ui/leafwiki-ui/ ./
RUN VITE_API_URL=/ APP_VERSION=${APP_VERSION} npm run build

# Step 2: Backend + Build binary
FROM golang:1.27.3-alpine@sha256:d200d2dd76b03a99b52651fd1325aff7dcd35422c35c01bac56571b0507f3deb AS backend-build
WORKDIR /app
ARG APP_VERSION
ARG DISABLE_REFRESH_TOKEN_RATE_LIMIT=false
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-build /app/dist ./internal/http/dist
# -tags nomsgpack drops gin's unused MessagePack codec (~5MB with -trimpath).
RUN CGO_ENABLED=0 go build -trimpath -tags nomsgpack \
	-ldflags="-s -w -X github.com/perber/wiki/internal/http.EmbedFrontend=true -X github.com/perber/wiki/internal/http.Environment=production -X main.Version=${APP_VERSION} -X github.com/perber/wiki/internal/wiki/auth.DisableRefreshTokenRateLimit=${DISABLE_REFRESH_TOKEN_RATE_LIMIT}" \
	-o /out/leafwiki ./cmd/leafwiki

# Step 3: Final image (small)
FROM alpine:3.24@sha256:be2656061b536c1883c4034790e21d0a09c5aaaf51968b89368a3c0371ea2c9a AS final
WORKDIR /app
COPY --from=backend-build /out/leafwiki /app/leafwiki

COPY ./docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

EXPOSE 8080

RUN mkdir -p /app/data && chmod 777 /app/data

LABEL org.opencontainers.image.title="LeafWiki" \
      org.opencontainers.image.description="LeafWiki – A fast wiki for people who think in folders, not feeds" \
      org.opencontainers.image.url="https://demo.leafwiki.com" \
      org.opencontainers.image.source="https://github.com/perber/leafwiki" \
      org.opencontainers.image.licenses="MIT"

ENTRYPOINT ["/docker-entrypoint.sh"]
