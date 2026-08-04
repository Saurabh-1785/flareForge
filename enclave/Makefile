.PHONY: build test run clean docker-build docker-run

# ──────────────────────────────────────────────────────────────────────
# Vault Enclave — Build Targets
# ──────────────────────────────────────────────────────────────────────

# Build the enclave binary
build:
	CGO_ENABLED=1 go build -o bin/enclave ./cmd/enclave

# Run all tests
test:
	go test -v -count=1 ./...

# Run tests with race detector
test-race:
	CGO_ENABLED=1 go test -v -race -count=1 ./...

# Run specific package tests
test-crypto:
	go test -v ./internal/crypto/...

test-store:
	go test -v ./internal/sealedstore/...

test-quorum:
	go test -v ./internal/quorum/...

test-api:
	go test -v ./internal/api/...

# Run the enclave service locally
run: build
	ENCLAVE_PORT=8090 ENCLAVE_DB_PATH=./data/enclave_sealed.db ./bin/enclave

# Clean build artifacts
clean:
	rm -rf bin/ data/

# Initialize Go modules (first-time setup)
init:
	go mod tidy

# Docker
docker-build:
	docker build -t continuity-vault-enclave .

docker-run:
	docker run -p 8090:8090 -v $$(pwd)/data:/app/data continuity-vault-enclave

# Integration test: full lifecycle via curl
test-integration: run &
	@echo "Waiting for server to start..."
	@sleep 2
	@echo "=== Storing plan ==="
	curl -s -X POST http://localhost:8090/vaults/1/plan \
		-H "Content-Type: application/json" \
		-d '{"planData":{"beneficiaries":[{"identifier":"rBen1","label":"Spouse","splitPercentage":60},{"identifier":"rBen2","label":"Child","splitPercentage":40}],"quorumThreshold":2,"attestationTypes":["DEATH","INCAPACITATION"]},"commitmentHash":"0xtest123"}' | jq .
	@echo "\n=== Submitting attestation 1 ==="
	curl -s -X POST http://localhost:8090/vaults/1/attestations \
		-H "Content-Type: application/json" \
		-d '{"caseId":"case-001","attestationType":"DEATH","attestorAddress":"0xA1","fdcVotingRound":100,"verifiedAt":"2026-08-04T12:00:00Z"}' | jq .
	@echo "\n=== Checking quorum status ==="
	curl -s http://localhost:8090/vaults/1/quorum-status | jq .
	@echo "\n=== Submitting attestation 2 ==="
	curl -s -X POST http://localhost:8090/vaults/1/attestations \
		-H "Content-Type: application/json" \
		-d '{"caseId":"case-001","attestationType":"DEATH","attestorAddress":"0xA2","fdcVotingRound":101,"verifiedAt":"2026-08-04T12:05:00Z"}' | jq .
	@echo "\n=== Checking quorum result ==="
	curl -s http://localhost:8090/vaults/1/quorum-result | jq .
	@echo "\n=== Health check ==="
	curl -s http://localhost:8090/health | jq .
	@echo "\n=== Enclave identity ==="
	curl -s http://localhost:8090/identity | jq .
