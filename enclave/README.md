# Vault Enclave — Confidential Compute Reference Service

> **Layer 3** of the Continuity Vault protocol.

## What This Is

The Vault Enclave is the **only component that ever decrypts the full inheritance/succession plan**. It runs inside a Trusted Execution Environment (TEE) — specifically Google Cloud Confidential Space, following the same pattern as [Flare AI Kit](https://github.com/flare-foundation/flare-ai-kit).

This is the component that makes Continuity Vault _actually private_, not just encrypted. The TEE ensures the plan data (who receives funds, how much, under what conditions) is never exposed to anyone — not the service operator, not the infrastructure provider, not an attacker who compromises the host machine.

## Why a TEE, Not Just Encryption

> **Design Principle #5:** Confidentiality is a means, not the pitch.

The plan — who, how much, when — is exactly what regulators, family members, and attackers all want pre-execution. This is the sentence that makes this need a TEE instead of a timelocked contract:

- **A private key in a database** can be exfiltrated by a database breach, a rogue admin, or a legal subpoena.
- **A TEE-held key** cannot be extracted even by the machine's operator. The attestation proof lets anyone verify _what code_ is running inside the enclave, without seeing _what data_ it holds.

The real threat model isn't a contract bug — it's **coercion of a known heir** ("wrench attack"). Trustees see a case ID, not an estate. Beneficiaries stay unknown on-chain until release.

## Hardware & FCC Status

**Current state (as of August 2026):**

At initial FCC rollout, TEE nodes are operated by the Flare Foundation on Google Cloud Confidential Space. **Third-party custom TEE application deployment into Flare's enshrined FCC consensus is a later milestone** — the initial surface is deliberately narrow.

**What this means for Continuity Vault:**

This enclave service is a **reference implementation** that:
1. Runs inside **real isolated hardware** (Google Cloud Confidential Space with AMD SEV-SNP), not a plaintext process labeled "enclave"
2. Produces **real remote attestation proofs** that any verifier can check
3. Follows the **exact same Confidential Space pattern** that Flare AI Kit (the Flare ecosystem's own TEE hackathon reference) already demonstrates

**What changes when FCC opens third-party deployment:**

The migration path is straightforward:
- Replace the standalone Confidential Space VM with an FCC-enshrined enclave node
- The signing key becomes an FCC-registered operator key
- The attestation evidence becomes an FCC consensus attestation instead of a GCP attestation token
- The API surface, quorum logic, and sealed plan format remain identical

This one paragraph is what turns "we mocked the hard part" into "a credible reference implementation with a real migration path."

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                    TEE BOUNDARY                               │
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────────────┐ │
│  │  Enclave     │  │  Sealed     │  │  Quorum Engine       │ │
│  │  Crypto Keys │  │  Plan Store │  │  (N-of-M evaluation) │ │
│  │  (ECDSA +    │  │  (AES-256   │  │                      │ │
│  │   AES-256)   │  │   encrypted │  │  Receives verified   │ │
│  │              │  │   SQLite)   │  │  FDC attestations,   │ │
│  │  Private keys│  │             │  │  checks thresholds,  │ │
│  │  NEVER leave │  │  Plaintext  │  │  signs results       │ │
│  │  this process│  │  NEVER      │  │                      │ │
│  │              │  │  touches    │  │                      │ │
│  └──────┬───────┘  │  disk       │  └──────────┬───────────┘ │
│         │          └──────┬──────┘              │             │
│         └─────────────────┼─────────────────────┘             │
│                           │                                   │
│  ┌────────────────────────┴──────────────────────────────┐   │
│  │                    REST API                            │   │
│  │  POST /vaults/{id}/plan         Store sealed plan     │   │
│  │  POST /vaults/{id}/attestations Submit attestation    │   │
│  │  GET  /vaults/{id}/quorum-status  Quorum status       │   │
│  │  GET  /vaults/{id}/quorum-result  Signed result       │   │
│  │  POST /vaults/{id}/reset        Reset quorum          │   │
│  │  GET  /health                   Health + identity     │   │
│  │  GET  /identity                 Public keys + attest  │   │
│  └───────────────────────────────────────────────────────┘   │
└──────────────────────────────────────────────────────────────┘
                            │
                    TLS / mTLS only
                            │
         ┌──────────────────┼──────────────────┐
         ▼                  ▼                  ▼
    ┌─────────┐      ┌──────────┐      ┌──────────┐
    │ Owner   │      │ Relayer  │      │ Vault    │
    │ Client  │      │ (Go)     │      │ Registry │
    │         │      │ Layer 4  │      │ (on-chain)│
    └─────────┘      └──────────┘      └──────────┘
```

## Data Flow

1. **Owner creates vault on-chain** → VaultRegistry stores `planCommitmentHash` (hash only, never the plan)
2. **Owner seals plan in enclave** → `POST /vaults/{id}/plan` — plan is encrypted with AES-256-GCM and stored in SQLite; the private key exists only in TEE memory
3. **Trustee submits attestation** → goes through `attestation-api/` → FDC verifies via `Web2Json` → verified attestation reference is relayed to enclave
4. **Enclave evaluates quorum** → checks attestation count against plan's configured threshold
5. **Quorum met** → enclave signs a result with its ECDSA key → relayer polls `GET /vaults/{id}/quorum-result` → submits signed result to `submitQuorumResult()` on-chain
6. **On-chain contract verifies signature** → confirms it matches the registered enclave oracle address → opens dispute window

## Stack

| Component | Technology | Why |
|---|---|---|
| Language | Go 1.22+ | Matches architecture spec; good TEE/crypto support |
| API | gorilla/mux REST | Simple, well-tested HTTP routing |
| Storage | SQLite (encrypted at rest) | "Postgres or even SQLite is fine" — build prompt |
| Crypto | go-ethereum secp256k1 + AES-256-GCM | Ethereum-compatible signing + strong encryption |
| Logging | zap | Structured production logging |
| Container | Alpine Docker | Minimal attack surface |
| TEE Platform | Google Cloud Confidential Space | Same as Flare AI Kit and FCC nodes |

## Running Locally

```bash
# From the enclave/ directory
go run ./cmd/enclave

# Or with custom config
ENCLAVE_PORT=8090 ENCLAVE_DB_PATH=./data/sealed.db go run ./cmd/enclave
```

## Running Tests

```bash
# All tests
go test ./...

# Specific packages
go test ./internal/crypto/...
go test ./internal/sealedstore/...
go test ./internal/quorum/...
go test ./internal/api/...

# With verbose output
go test -v ./...
```

## Docker

```bash
# Build
docker build -t continuity-vault-enclave .

# Run locally
docker run -p 8090:8090 continuity-vault-enclave

# Run with persistent storage
docker run -p 8090:8090 -v $(pwd)/data:/app/data continuity-vault-enclave
```

## Deploying to Google Cloud Confidential Space

```bash
# 1. Build and push to Artifact Registry
gcloud builds submit --tag us-docker.pkg.dev/PROJECT_ID/REPO/enclave:latest

# 2. Create Confidential Space VM
gcloud compute instances create enclave-vm \
  --zone=us-central1-a \
  --machine-type=n2d-standard-2 \
  --confidential-compute \
  --maintenance-policy=TERMINATE \
  --image-family=cos-stable \
  --image-project=confidential-space-images \
  --metadata="tee-image-reference=us-docker.pkg.dev/PROJECT_ID/REPO/enclave:latest"
```

## API Examples

```bash
# Store a sealed plan
curl -X POST http://localhost:8090/vaults/1/plan \
  -H "Content-Type: application/json" \
  -d '{
    "planData": {
      "beneficiaries": [
        {"identifier": "rBeneficiary1XRP", "label": "Spouse", "splitPercentage": 60},
        {"identifier": "rBeneficiary2XRP", "label": "Child", "splitPercentage": 40}
      ],
      "quorumThreshold": 2,
      "attestationTypes": ["DEATH", "INCAPACITATION"]
    },
    "commitmentHash": "0xabc123def456789"
  }'

# Submit a verified attestation
curl -X POST http://localhost:8090/vaults/1/attestations \
  -H "Content-Type: application/json" \
  -d '{
    "caseId": "case-001",
    "attestationType": "DEATH",
    "attestorAddress": "0x1234567890abcdef",
    "fdcVotingRound": 100,
    "verifiedAt": "2026-08-04T12:00:00Z"
  }'

# Check quorum status
curl http://localhost:8090/vaults/1/quorum-status

# Get signed quorum result (for on-chain submission)
curl http://localhost:8090/vaults/1/quorum-result

# Health check
curl http://localhost:8090/health

# Enclave identity (public keys + attestation)
curl http://localhost:8090/identity
```

## What's In Scope (Phase 1 / MVP)

- ✅ Real TEE execution (Google Cloud Confidential Space)
- ✅ Real remote attestation evidence
- ✅ AES-256-GCM encrypted plan storage
- ✅ ECDSA secp256k1 signing (Ethereum-compatible)
- ✅ 2-of-2 simple quorum evaluation
- ✅ Attestation type validation against plan config
- ✅ Duplicate attestor rejection
- ✅ Signed quorum result for on-chain verification
- ✅ Quorum reset (for guardian halt flow)

## What's NOT in Scope (Phase 2 / Roadmap)

- ❌ Weighted N-of-M quorum (different attestors carry different weight)
- ❌ Multi-TEE consortium (run across multiple independent TEE operators)
- ❌ Running inside Flare's enshrined FCC consensus (waiting for third-party deployment to open)
- ❌ Client-side X25519 encryption (owner encrypts plan before transmission)
- ❌ IPFS/Arweave sealed blob storage
- ❌ Attestor reputation scoring
