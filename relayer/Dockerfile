# Relayer / Event Watcher — Container Build
#
# Build:   docker build -t continuity-vault-relayer .
# Run:     docker run --env-file .env continuity-vault-relayer

# ──── Stage 1: Build ──────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache gcc musl-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /relayer ./cmd/relayer

# ──── Stage 2: Runtime ────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

# Create non-root user
RUN adduser -D -u 1000 relayer
USER relayer

WORKDIR /app
COPY --from=builder /relayer /app/relayer

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD pgrep relayer || exit 1

ENTRYPOINT ["/app/relayer"]
