# Continuity Vault

> Non-custodial digital-estate & business-continuity protocol for XRP / BTC / DOGE holders — built on Flare.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Network: Coston2](https://img.shields.io/badge/Network-Coston2-orange.svg)](https://dev.flare.network/)

## What is Continuity Vault?

Continuity Vault lets crypto holders define a **private, multi-condition inheritance or business-succession plan** — who receives funds, under what evidence, on what schedule — without ever putting that plan on a public chain in plaintext.

A missed check-in **plus** independent confirmation (never either signal alone) opens a public dispute window; if nobody halts it, funds release in tranches to beneficiaries who stay unknown on-chain until release.

### Why Flare?

| Flare Primitive | Role in Continuity Vault |
|---|---|
| **FAssets** | The *only* reason XRP/BTC/DOGE holders can have "if I die, split this" logic — those chains have no native smart contracts. |
| **FDC** | Turns a human confirmation or public record into an on-chain fact, so the trigger is more than a dead man's switch timer. |
| **FCC** | Keeps the plan private until execution — a beneficiary list in plaintext on a public contract is a real-world dealbreaker. |

## Architecture

See [`docs/continuity-vault-architecture.md`](docs/continuity-vault-architecture.md) for the full system design, lifecycle diagrams, and component responsibility table.

## Vault Lifecycle (simplified)

```
ACTIVE → (miss check-in) → WARNING → (grace expires) → QUORUM_PENDING
    → (N-of-M signals agree) → DISPUTE_WINDOW → (window elapses clean)
    → TRANCHE_1_RELEASED → FINAL_WINDOW → FULLY_RELEASED
```

The owner can **self-correct at any point** via guardian halt keys — even after quorum is met and after Tranche 1 releases.

## Repository Structure

```
continuity-vault/
├── contracts/          # Solidity smart contracts (Foundry)
│   ├── src/            # VaultRegistry, FAssetsRouter, interfaces
│   ├── script/         # Forge deploy scripts (Coston2)
│   └── test/           # Foundry test suite
├── enclave/            # Vault Enclave — TEE-backed confidential service (Go)
├── attestation-api/    # Web2Json source for trustee attestations
├── relayer/            # Off-chain watcher + notification service (Go)
├── web/                # Next.js frontend
├── scripts/            # End-to-end lifecycle automation
└── docs/               # Architecture, submission, demo script
```

## Quick Start

### Prerequisites

- [Foundry](https://book.getfoundry.sh/) (Solidity toolchain)
- [Go 1.22+](https://go.dev/)
- [Node.js 20+](https://nodejs.org/)
- A Coston2 wallet funded via the [faucet](https://faucet.flare.network/)

### Build Contracts

```bash
cd contracts
forge build
```

### Run Tests

```bash
cd contracts
forge test -vvv
```

### Deploy to Coston2

```bash
cd contracts
forge script script/Deploy.s.sol --rpc-url $COSTON2_RPC --broadcast
```

## Target Users

1. **Long-term XRP/BTC/DOGE holders** without existing digital-estate tooling — a cohort that skews older and higher-net-worth than the average crypto user.
2. **DAO founders and multisig operators** with key-person risk — the same trigger → quorum → release primitive, different beneficiary type.

## Design Principles

1. **Trust-minimize the trigger**, not just the custody
2. **Optimistic execution** — every trigger is provisional until a dispute window closes
3. **Minimize information exposure** before execution
4. **Owner can self-correct** even mid-trigger
5. **Confidentiality is a means**, not the pitch
6. **One state machine, two markets** — inheritance and business continuity
7. **Settle in the asset people understand** — native XRP, not wrapped tokens

## Network

- **Deployment target:** Coston2 (Flare testnet)
- **Contract addresses:** _TBD — will be populated after deployment_

## Hackathon

Built for [Flare Summer Signal](https://dorahacks.io/hackathon/flaresummersignal/detail) hackathon on DoraHacks.

**Bounty tracks:** Interoperable Asset Products + Private Applications (Flare Confidential Compute)

## License

[MIT](LICENSE)
