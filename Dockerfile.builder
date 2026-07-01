# Stage 1: Frontend build
FROM node:26-alpine@sha256:144769ec3f32e8ee36b3cfde91e82bee25d9367b20f31a151f3f7eea3a2a8541 AS frontend

WORKDIR /ui
ARG APP_VERSION

COPY ./ui/leafwiki-ui/package.json ./package.json
COPY ./ui/leafwiki-ui/package-lock.json ./package-lock.json
RUN npm ci --ignore-scripts

COPY ./ui/leafwiki-ui/ ./
RUN VITE_API_URL=/ APP_VERSION=${APP_VERSION} npm run build

# Stage 2: Go backend build
FROM golang:1.26-alpine@sha256:3ad57304ad93bbec8548a0437ad9e06a455660655d9af011d58b993f6f615648 AS builder

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
  -o /out/${OUTPUT} ./cmd/leafwiki
