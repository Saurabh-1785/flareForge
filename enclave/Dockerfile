# Vault Enclave — Confidential Space Container
#
# Build this image for deployment to Google Cloud Confidential Space.
# The container runs inside a TEE (AMD SEV-SNP), and the attestation
# evidence proves to external verifiers what code is running.
#
# Pattern reference: flare-foundation/flare-ai-kit
#
# Build:   docker build -t continuity-vault-enclave .
# Run:     docker run -p 8090:8090 continuity-vault-enclave
# Deploy:  (see enclave/README.md for Confidential Space instructions)

# ──── Stage 1: Build ──────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -o /enclave ./cmd/enclave

# ──── Stage 2: Runtime ────────────────────────────────────────────────
FROM alpine:3.20

RUN apk add --no-cache ca-certificates sqlite-libs

# Create non-root user for the enclave process
RUN adduser -D -u 1000 enclave
USER enclave

WORKDIR /app
COPY --from=builder /enclave /app/enclave

# Data directory for the sealed plan SQLite database
RUN mkdir -p /app/data
ENV ENCLAVE_DB_PATH=/app/data/enclave_sealed.db
ENV ENCLAVE_PORT=8090

EXPOSE 8090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:8090/health || exit 1

ENTRYPOINT ["/app/enclave"]
