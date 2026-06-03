# Step 1: Frontend
FROM node:26-alpine@sha256:7c6af15abe4e3de859690e7db171d0d711bf37d27528eddfe625b2fe89e097f8 AS frontend-build
WORKDIR /app
ARG APP_VERSION
COPY ./ui/leafwiki-ui/package*.json ./
RUN npm ci --ignore-scripts
COPY ./ui/leafwiki-ui/ ./
RUN VITE_API_URL=/ APP_VERSION=${APP_VERSION} npm run build

# Step 2: Backend + Build binary
FROM golang:1.26-alpine@sha256:f23e8b227fb4493eabe03bede4d5a32d04092da71962f1fb79b5f7d1e6c2a17f AS backend-build
WORKDIR /app
ARG DISABLE_REFRESH_TOKEN_RATE_LIMIT=false
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-build /app/dist ./internal/http/dist
RUN CGO_ENABLED=0 go build \
	-ldflags="-s -w -X github.com/perber/wiki/internal/http.EmbedFrontend=true -X github.com/perber/wiki/internal/http.Environment=production -X github.com/perber/wiki/internal/wiki/auth.DisableRefreshTokenRateLimit=${DISABLE_REFRESH_TOKEN_RATE_LIMIT}" \
	-o /out/leafwiki ./cmd/leafwiki/main.go

# Step 3: Final image (small)
FROM alpine:3.23@sha256:5b10f432ef3da1b8d4c7eb6c487f2f5a8f096bc91145e68878dd4a5019afde11 AS final
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
