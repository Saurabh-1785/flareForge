# Attestation API

Trustee attestation service for Continuity Vault. This is the off-chain data source that FDC's `Web2Json` attestation type verifies on-chain.

## Quick Start

```bash
cd attestation-api
npm install
npm start
```

Server runs on `http://localhost:3000`.

## Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/attest` | Submit a trustee attestation |
| `GET` | `/attestations/:caseId` | FDC Web2Json reads this endpoint |
| `GET` | `/attestations/:caseId/all` | All attestations for a case |
| `GET` | `/health` | Health check |

## Submit an Attestation

```bash
curl -X POST http://localhost:3000/attest \
  -H 'Content-Type: application/json' \
  -d '{
    "vaultId": 1,
    "caseId": "case-001",
    "attestationType": "DEATH",
    "attestorAddress": "0x70997970C51812dc3A010C7d01b50e0d17dc79C8"
  }'
```

## Read an Attestation (FDC endpoint)

```bash
curl http://localhost:3000/attestations/case-001
```

Response:
```json
{
  "caseId": "case-001",
  "attestationType": "DEATH",
  "attestorAddress": "0x70997970C51812dc3A010C7d01b50e0d17dc79C8",
  "attestedAt": 1700000000,
  "confirmed": true
}
```

## FDC Web2Json Integration

The `GET /attestations/:caseId` response is consumed by FDC's `Web2Json` attestation type with:

- **JQ transform**: `{caseId: .caseId, attestationType: .attestationType, attestorAddress: .attestorAddress, attestedAt: .attestedAt, confirmed: .confirmed}`
- **ABI signature**: matches `FdcAttestationVerifier.AttestationData` struct

The FDC Merkle proof is then submitted to `FdcAttestationVerifier.submitAttestation()` on-chain.
