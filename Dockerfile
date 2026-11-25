# Step 1: Frontend
FROM node:26-alpine@sha256:6bd9ab43c0ec0190ca7b2d193906cb8b7edf6e7f4afa43ddeccd6452710bffb5 AS frontend-build
WORKDIR /app
ARG APP_VERSION
COPY ./ui/leafwiki-ui/package*.json ./
RUN npm ci --ignore-scripts
COPY ./ui/leafwiki-ui/ ./
RUN VITE_API_URL=/ APP_VERSION=${APP_VERSION} npm run build

# Step 2: Backend + Build binary
FROM golang:1.27.4-alpine@sha256:70bd7ddea8b6cbe5d6943cde279d6b67feb10cc924fb9100d1d8757c1575b280 AS backend-build
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
FROM alpine:3.24@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf AS final
WORKDIR /app

# Needed for any outbound HTTPS call (OIDC discovery/JWKS/token exchange,
# SMTP over implicit TLS) to verify the public CA chain. An operator talking
# to a provider behind an internal CA additionally sets SSL_CERT_FILE to a
# full bundle (system CAs + their own root) — Go's crypto/x509 uses *only*
# that file when it's set, so this package is what keeps everything else
# (anyone who doesn't need a custom CA) working without it.
RUN apk add --no-cache ca-certificates

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
