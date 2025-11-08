# Step 1: Frontend
FROM node:25-alpine AS frontend-build
WORKDIR /app
COPY ./ui/leafwiki-ui/package*.json ./
RUN npm install
COPY ./ui/leafwiki-ui/ ./
RUN VITE_API_URL=/ npm run build

# Step 2: Backend + Build binary
FROM golang:1.24-alpine AS backend-build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend-build /app/dist ./internal/http/dist
RUN CGO_ENABLED=0 go build \
	-ldflags="-s -w -X github.com/perber/wiki/internal/http.EmbedFrontend=true -X github.com/perber/wiki/internal/http.EnableCors=false -X github.com/perber/wiki/internal/http.Environment=production" \
	-o /out/leafwiki ./cmd/leafwiki/main.go

# Step 3: Final image (small)
FROM alpine:3.20 AS final
WORKDIR /app
COPY --from=backend-build /out/leafwiki /app/leafwiki

EXPOSE 8080

RUN mkdir -p /app/data && chmod 777 /app/data

ENTRYPOINT ["/app/leafwiki"]
