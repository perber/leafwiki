# Stage 1: Frontend build
FROM node:25-alpine AS frontend

WORKDIR /ui

COPY ./ui/leafwiki-ui/package.json ./package.json
COPY ./ui/leafwiki-ui/package-lock.json ./package-lock.json
RUN npm install

COPY ./ui/leafwiki-ui/ ./
RUN VITE_API_URL=/ npm run build

# Stage 2: Go backend build
FROM golang:1.26-alpine AS builder

ARG GOOS
ARG GOARCH
ARG CGO_ENABLED=0
ARG OUTPUT=leafwiki

ENV GOOS=${GOOS}
ENV GOARCH=${GOARCH}
ENV CGO_ENABLED=${CGO_ENABLED}

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Copy built frontend
COPY --from=frontend /ui/dist ./internal/http/dist

RUN go build \
  -ldflags="-s -w -X github.com/perber/wiki/internal/http.EmbedFrontend=true -X github.com/perber/wiki/internal/http.Environment=production" \
  -o /out/${OUTPUT} ./cmd/leafwiki/main.go
