# Continuity Vault — Demo Script

> ~4–5 minutes, designed for the Flare Summer Signal submission video.

---

## 1. The Problem (30s)

_XRP/BTC/DOGE holders have no native way to encode "if I die, split this" — their chains have no smart contracts. Show the architecture diagram briefly._

**Show:** `docs/continuity-vault-architecture.md` system diagram for a few seconds.

---

## 2. Why Flare, Specifically (30s)

One sentence each:
- **FAssets** — the only reason XRP holders can have inheritance logic at all
- **FDC** — turns a human confirmation into an on-chain fact
- **FCC** — keeps the plan private until execution

---

## 3. Owner Creates and Funds a Vault (60s)

- Connect wallet (MetaMask on Coston2)
- Create a new vault with demo-length timers (minutes, not days)
- Fund with FXRP
- **Show:** real Coston2 transaction on the block explorer

---

## 4. Normal Check-in (30s)

- Owner performs a signed check-in
- Show the deadline resetting

---

## 5. Missed Check-in → Trigger Flow (60s)

- Let the demo timer elapse
- Show state transitions: `ACTIVE → WARNING → QUORUM_PENDING`
- Trustee submits attestation through the attestation API
- FDC verifies → Quorum Engine evaluates → `DISPUTE_WINDOW` opens

---

## 6. The Safety Net (45s)

**Option A — Guardian Halt:**
- Trigger guardian halt during dispute window
- Show clean abort back to `ACTIVE` (Design Principle #4)

**Option B — Clean Release:**
- Let dispute window elapse
- Show `TRANCHE_1_RELEASED → FINAL_WINDOW → FULLY_RELEASED`
- Show the FAssets redemption transaction

_Show both options in two short takes if time allows._

---

## 7. Roadmap (30s)

- Show Phase 1 / Phase 2 table from the architecture file
- Call out one honest risk from the Honest Risk Ledger
- "Here's what's real, here's what's roadmap, here's the risk we haven't solved"

---

## Notes

- All windows configured to demo length (minutes) for the video
- Full lifecycle should complete in under 10 minutes
- The e2e script (`scripts/e2e-lifecycle.ts`) is this demo's backbone
