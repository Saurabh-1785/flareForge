.PHONY: build test run clean docker-build docker-run

# ──────────────────────────────────────────────────────────────────────
# Relayer — Build Targets
# ──────────────────────────────────────────────────────────────────────

# Build the relayer binary
build:
	CGO_ENABLED=0 go build -o bin/relayer ./cmd/relayer

# Run all tests
test:
	go test -v -count=1 ./...

# Run the relayer
run: build
	./bin/relayer

# Initialize Go modules (first-time setup)
init:
	go mod tidy

# Clean build artifacts
clean:
	rm -rf bin/

# Docker
docker-build:
	docker build -t continuity-vault-relayer .

docker-run:
	docker run --env-file .env continuity-vault-relayer

# Database setup (requires Postgres running)
db-setup:
	@echo "Creating database..."
	createdb continuity_vault 2>/dev/null || true
	@echo "Database ready. Migrations run automatically on first startup."

# Environment template
env-template:
	@echo "# Continuity Vault Relayer Configuration"
	@echo "RPC_URL=https://coston2-api.flare.network/ext/C/rpc"
	@echo "REGISTRY_ADDRESS=0xYOUR_DEPLOYED_REGISTRY_ADDRESS"
	@echo "RELAYER_PRIVATE_KEY=YOUR_RELAYER_PRIVATE_KEY_HEX"
	@echo "CHAIN_ID=114"
	@echo "ENCLAVE_URL=http://localhost:8090"
	@echo "DATABASE_URL=postgres://localhost:5432/continuity_vault?sslmode=disable"
	@echo "DEMO_MODE=true"
