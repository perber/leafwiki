# Step 1: Frontend
FROM node:25-alpine AS frontend-build
WORKDIR /app
COPY ./ui/leafwiki-ui/package*.json ./
RUN npm install
COPY ./ui/leafwiki-ui/ ./
RUN VITE_API_URL=/ npm run build

# Step 2: Backend + Build binary
FROM golang:1.26-alpine AS backend-build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-build /app/dist ./internal/http/dist
RUN CGO_ENABLED=0 go build \
	-ldflags="-s -w -X github.com/perber/wiki/internal/http.EmbedFrontend=true -X github.com/perber/wiki/internal/http.Environment=production" \
	-o /out/leafwiki ./cmd/leafwiki/main.go

# Step 3: Final image (small)
FROM alpine:3.23 AS final
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
