# Continuity Vault — Submission

> **Hackathon:** Flare Summer Signal · DoraHacks
> **Deadline:** August 14, 2026
> **Bounty Track(s):** Interoperable Asset Products + Private Applications (Flare Confidential Compute)

---

## Project Description

_Continuity Vault is a non-custodial digital-estate and business-continuity protocol for XRP/BTC/DOGE holders, built on Flare. It lets crypto holders define a private, multi-condition inheritance or succession plan — private until execution, triggered only by independent multi-signal quorum, with a full dispute window for self-correction._

---

## Target Users

1. **Long-term XRP/BTC/DOGE holders** — a cohort without native smart-contract access on their chains, who need inheritance logic that doesn't exist today.
2. **DAO founders / multisig operators** — the same trigger → quorum → release primitive serves key-person business continuity, from the same build.

---

## How the Project Uses Flare

| Flare Primitive | Usage | Load-bearing? |
|---|---|---|
| **FAssets (FXRP)** | Vault funding and native-XRP redemption at release | Yes — without FAssets, XRP holders have no smart-contract access at all |
| **FDC (Web2Json)** | On-chain verification of trustee attestation JSON payloads | Yes — turns a human confirmation into a verifiable on-chain fact |
| **FCC** | Vault Enclave runs inside TEE; sealed plan never exposed on-chain | Yes — beneficiary privacy is a real-world requirement, not a feature |

---

## New Work Log

_Fill in as you build — this is for the "evidence of new work" judging criterion._

| Date | Layer | What was built | Status |
|---|---|---|---|
| Aug 2 | 0 | Repo scaffold, toolchain, docs structure | ✅ |
| | | | |

---

## Contract Addresses (Coston2)

| Contract | Address |
|---|---|
| VaultRegistry | _TBD_ |
| FAssetsRouter | _TBD_ |
| FeeModule (stub) | _TBD_ |

---

## Demo

- **Demo video:** _TBD_
- **Live app:** _TBD_
- **Demo script:** See [`docs/demo-script.md`](demo-script.md)

---

## Roadmap / Next Steps

### Phase 2 (post-hackathon)

| Feature | Description |
|---|---|
| Bonded attestor market | Stake-and-slash economic security for attestors |
| M-of-N weighted quorum | Configurable N-of-M with weighted signal types |
| FBTC / FDOGE support | Extend FAssets Router to BTC and DOGE |
| Protocol-Managed Wallet payout | Direct native XRP via PMW (replaces FAssets redemption) |
| Public-record feed | Web2Json attestation against obituary/court-filing APIs |
| Encrypted blob storage | IPFS/Arweave for sealed plan + legal document storage |
| Compliance hooks | Estate-tax/AML reporting (jurisdiction-dependent) |

### Honest Risks (from architecture doc)

- TEE trust is still a hardware trust root at MVP
- Bonded attestor market has a cold-start problem
- PMW currently covers XRPL only
- Public-record coverage is jurisdiction-dependent
- Large automatic transfers can trigger estate-tax/AML reporting
- Total key loss defeats the safety net entirely

---

## What Pre-existed vs. What's New

| Category | Details |
|---|---|
| **Pre-existing** | Architecture design document, build prompt |
| **Newly built** | All code, contracts, enclave service, frontend, attestation API |
| **Ported** | None |
| **Improved** | _TBD_ |

---

## Team

_TBD_
