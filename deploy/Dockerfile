# Multi-stage Dockerfile for all neobank binaries.
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

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/neo-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/neo-ethswitch-gateway ./cmd/ethswitch-gateway
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/neo-recon-worker ./cmd/recon-worker
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/neo-lending-worker ./cmd/lending-worker
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bin/neo-telegram-bot ./cmd/telegram-bot

# ============================================
# Stage 2: Runtime images (one per binary)
# ============================================

FROM alpine:3.19 AS base
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -H neobank
USER neobank

# --- API Server ---
FROM base AS api
COPY --from=builder /bin/neo-api /usr/local/bin/neo-api
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
  CMD ["/bin/sh", "-c", "wget -qO- \"http://localhost:${PORT:-8080}/healthz\" || exit 1"]
ENTRYPOINT ["/usr/local/bin/neo-api"]

# --- EthSwitch Gateway ---
FROM base AS ethswitch-gateway
COPY --from=builder /bin/neo-ethswitch-gateway /usr/local/bin/neo-ethswitch-gateway
ENTRYPOINT ["neo-ethswitch-gateway"]

# --- Recon Worker ---
FROM base AS recon-worker
COPY --from=builder /bin/neo-recon-worker /usr/local/bin/neo-recon-worker
ENTRYPOINT ["neo-recon-worker"]

# --- Lending Worker ---
FROM base AS lending-worker
COPY --from=builder /bin/neo-lending-worker /usr/local/bin/neo-lending-worker
ENTRYPOINT ["neo-lending-worker"]

# --- Telegram Bot ---
FROM base AS telegram-bot
COPY --from=builder /bin/neo-telegram-bot /usr/local/bin/neo-telegram-bot
ENTRYPOINT ["neo-telegram-bot"]
