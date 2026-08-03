/**
 * Continuity Vault - Attestation API
 *
 * A small Express service where a trustee (or, for the demo, a sandboxed
 * "obituary API" stand-in) posts a signed JSON attestation referencing a
 * vault's case ID.
 *
 * FDC's Web2Json attestation type fetches GET /attestations/:caseId and
 * verifies the response on-chain via Merkle proof.
 *
 * Endpoints:
 *   POST /attest           - Trustee submits an attestation
 *   GET  /attestations/:id - FDC reads this; returns verified JSON
 *   GET  /health           - Health check
 *
 * In-memory store (MVP). Production would use Postgres + signature verification.
 */

const express = require("express");
const { ethers } = require("ethers");

const app = express();
app.use(express.json());

const PORT = process.env.PORT || 3000;

// ─── In-Memory Attestation Store ────────────────────────────────────────────

/**
 * Map<caseId, AttestationRecord>
 *
 * AttestationRecord shape (matches the ABI signature in our FDC Web2Json request):
 * {
 *   caseId: string,
 *   attestationType: string,    // "DEATH", "INCAPACITATION", etc.
 *   attestorAddress: string,    // 0x... Ethereum address
 *   attestedAt: number,         // UNIX timestamp
 *   signature: string,          // EIP-191 signature of the payload
 *   confirmed: boolean          // true once stored
 * }
 */
const attestations = new Map();

// ─── POST /attest ───────────────────────────────────────────────────────────
// Trustee submits a signed attestation for a vault case.

app.post("/attest", (req, res) => {
  const { vaultId, caseId, attestationType, attestorAddress, signature } =
    req.body;

  // Validate required fields
  if (!caseId || !attestationType || !attestorAddress) {
    return res.status(400).json({
      error: "Missing required fields: caseId, attestationType, attestorAddress",
    });
  }

  // Validate attestation type
  const validTypes = ["DEATH", "INCAPACITATION", "KEY_PERSON_DEPARTURE"];
  if (!validTypes.includes(attestationType)) {
    return res.status(400).json({
      error: `Invalid attestationType. Must be one of: ${validTypes.join(", ")}`,
    });
  }

  // Validate Ethereum address format
  if (!ethers.isAddress(attestorAddress)) {
    return res.status(400).json({
      error: "Invalid attestorAddress: must be a valid Ethereum address",
    });
  }

  // Optional: verify EIP-191 signature (production requirement)
  // For MVP/hackathon, we accept the attestation on trust.
  // In Phase 2, this verifies that attestorAddress actually signed the payload.
  if (signature) {
    try {
      const message = JSON.stringify({
        caseId,
        attestationType,
        attestorAddress,
        vaultId,
      });
      const recoveredAddress = ethers.verifyMessage(message, signature);
      if (recoveredAddress.toLowerCase() !== attestorAddress.toLowerCase()) {
        return res.status(403).json({
          error: "Signature does not match attestorAddress",
        });
      }
    } catch (err) {
      // Signature verification failed — log but don't block in MVP
      console.warn(
        `Signature verification failed for case ${caseId}:`,
        err.message
      );
    }
  }

  const record = {
    caseId,
    attestationType,
    attestorAddress,
    attestedAt: Math.floor(Date.now() / 1000),
    confirmed: true,
  };

  // Store by caseId (append to array for multiple attestors)
  if (!attestations.has(caseId)) {
    attestations.set(caseId, []);
  }

  // Dedup: don't allow same attestor twice for same case
  const existing = attestations.get(caseId);
  if (
    existing.some(
      (a) =>
        a.attestorAddress.toLowerCase() === attestorAddress.toLowerCase()
    )
  ) {
    return res.status(409).json({
      error: "Attestor has already submitted for this case",
    });
  }

  existing.push(record);

  console.log(
    `[ATTEST] Case ${caseId} | Type: ${attestationType} | Attestor: ${attestorAddress}`
  );

  res.status(201).json({
    success: true,
    attestation: record,
    totalAttestations: existing.length,
  });
});

// ─── GET /attestations/:caseId ──────────────────────────────────────────────
// FDC Web2Json reads this endpoint. The response schema matches our ABI signature.
//
// JQ transform in FDC request:
//   '{caseId: .caseId, attestationType: .attestationType,
//     attestorAddress: .attestorAddress, attestedAt: .attestedAt,
//     confirmed: .confirmed}'
//
// ABI signature:
//   '{"components": [
//     {"internalType": "string", "name": "caseId", "type": "string"},
//     {"internalType": "string", "name": "attestationType", "type": "string"},
//     {"internalType": "address", "name": "attestorAddress", "type": "address"},
//     {"internalType": "uint256", "name": "attestedAt", "type": "uint256"},
//     {"internalType": "bool", "name": "confirmed", "type": "bool"}
//   ], "name": "task", "type": "tuple"}'

app.get("/attestations/:caseId", (req, res) => {
  const { caseId } = req.params;
  const records = attestations.get(caseId);

  if (!records || records.length === 0) {
    return res.status(404).json({
      error: "No attestations found for this case ID",
      caseId,
    });
  }

  // Return the most recent attestation (FDC Web2Json fetches a single response).
  // For multiple attestors, the relayer submits separate FDC requests per attestor.
  const latest = records[records.length - 1];

  res.json(latest);
});

// ─── GET /attestations/:caseId/all ──────────────────────────────────────────
// Returns all attestations for a case (not used by FDC, but useful for UI/debug).

app.get("/attestations/:caseId/all", (req, res) => {
  const { caseId } = req.params;
  const records = attestations.get(caseId);

  if (!records || records.length === 0) {
    return res.status(404).json({
      error: "No attestations found for this case ID",
      caseId,
    });
  }

  res.json({
    caseId,
    attestations: records,
    count: records.length,
  });
});

// ─── GET /health ────────────────────────────────────────────────────────────

app.get("/health", (_req, res) => {
  res.json({
    status: "ok",
    service: "continuity-vault-attestation-api",
    timestamp: new Date().toISOString(),
    totalCases: attestations.size,
  });
});

// ─── Start Server ───────────────────────────────────────────────────────────

app.listen(PORT, () => {
  console.log(`
  ╔══════════════════════════════════════════════════════╗
  ║  Continuity Vault — Attestation API                 ║
  ║  Listening on http://localhost:${PORT}                 ║
  ║                                                      ║
  ║  POST /attest              Submit attestation        ║
  ║  GET  /attestations/:id    FDC Web2Json endpoint     ║
  ║  GET  /attestations/:id/all All attestations         ║
  ║  GET  /health              Health check              ║
  ╚══════════════════════════════════════════════════════╝
  `);
});

module.exports = app;
