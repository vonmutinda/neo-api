# Multi-stage Dockerfile for all neobank binaries.
# Root Dockerfile for Railway (build context = repo root).
# For local docker-compose, see deploy/Dockerfile with context: ..
# Usage: docker build --target api -t neo-api .

# ============================================
# Stage 1: Build all Go binaries
# ============================================
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Only build the API binary for Railway (deploy/Dockerfile builds all for local compose).
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/neo-api ./cmd/api

# ============================================
# Stage 2: Runtime images (one per binary)
# ============================================

FROM alpine:3.19 AS base
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -H neobank
USER neobank

# --- API Server (final stage for Railway; they build the last stage) ---
FROM base AS api
COPY --from=builder /bin/neo-api /usr/local/bin/neo-api
EXPOSE 8080
# Healthcheck disabled temporarily to verify app serves successfully.
# HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
#   CMD ["/bin/sh", "-c", "wget -qO- \"http://localhost:${PORT:-8080}/healthz\" || exit 1"]
ENTRYPOINT ["/usr/local/bin/neo-api"]
