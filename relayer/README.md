# Relayer / Event Watcher

> **Layer 4** of the Continuity Vault protocol.

## What This Does

The relayer is the off-chain Go service that **watches deadlines, escalates reminders, and shuttles enclave results on-chain**. It is the automation engine that makes the vault lifecycle work without manual intervention.

In a demo scenario with all windows set to minutes, you can:
1. Create a vault
2. Walk away
3. Come back and see `ACTIVE → WARNING → QUORUM_PENDING` happen automatically
4. See reminders and notifications logged

The key flows the relayer drives:

| Trigger | Relayer Action | On-Chain Call |
|---|---|---|
| Check-in deadline approaching | Send T-minus-N reminder | — |
| Check-in deadline missed | Send notification | `markWarning(vaultId)` |
| Grace period expired | Send notification | `requestAttestation(vaultId)` |
| Enclave reports quorum met | Fetch signed result | `submitQuorumResult(vaultId, true, sig)` |
| Dispute window elapsed | Send final-override notice | `finalizeDisputeWindow(vaultId)` |
| Final window elapsed | — | `finalizeFinalWindow(vaultId)` |
| Guardian halt detected | Reset enclave quorum | `enclave /reset` |

## Architecture

```
                            ┌────────────────────┐
                            │  Coston2 Chain     │
                            │  VaultRegistry     │
                            └─────────┬──────────┘
                                      │ events
                     ┌────────────────┼────────────────────┐
                     │                ▼                    │
                     │        ┌───────────────┐            │
            tx calls │        │  Event        │            │ reads
                     │        │  Subscription │            │
                     │        └───────┬───────┘            │
                     │                │                    │
                     ▼                ▼                    ▼
              ┌──────────────────────────────────────────────┐
              │              RELAYER SERVICE                  │
              │                                              │
              │  ┌──────────────┐  ┌───────────────────┐    │
              │  │   Deadline    │  │   Enclave Poller  │    │
              │  │   Sweep Loop  │  │   (quorum status) │    │
              │  └──────┬───────┘  └──────────┬────────┘    │
              │         │                     │              │
              │         ▼                     ▼              │
              │  ┌──────────────────────────────────────┐    │
              │  │          Watcher Engine               │    │
              │  │  (goroutine-per-vault processing)     │    │
              │  └──────────────────┬───────────────────┘    │
              │                     │                        │
              │  ┌──────────────────┼───────────────────┐    │
              │  │                  │                    │    │
              │  ▼                  ▼                    ▼    │
              │  ┌─────────┐  ┌──────────┐  ┌──────────┐    │
              │  │Postgres │  │Notifier  │  │Chain     │    │
              │  │Store    │  │(console) │  │Client    │    │
              │  └─────────┘  └──────────┘  └──────────┘    │
              └──────────────────────────────────────────────┘
                                      │
                                      │ HTTP
                                      ▼
                            ┌────────────────────┐
                            │  Vault Enclave     │
                            │  (Layer 3)         │
                            └────────────────────┘
```

## Stack

| Component | Technology | Why |
|---|---|---|
| Language | Go 1.22+ | Matches architecture spec; goroutine model ideal for per-vault watchers |
| Chain Client | go-ethereum ethclient | Standard Ethereum interaction library |
| Database | Postgres | "Postgres schema: vaults, deadlines, notification log" — build prompt |
| Notifications | Console/log (MVP) | "a console log of 'reminder would have been sent' is fine" — build prompt |
| Container | Alpine Docker | Minimal, no CGO needed for the relayer |

## Database Schema

The relayer creates three tables on startup:

```sql
-- Tracked vaults with deadline state
tracked_vaults (
  vault_id, owner, state, window_deadline,
  check_in_interval, grace_window, dispute_window, final_window,
  last_check_in, reminder_sent, warning_sent, dispute_sent, quorum_relayed,
  created_at, updated_at
)

-- Notification audit log (dedup + history)
notification_log (
  id, vault_id, event_type, recipient, channel, message, sent_at
)

-- Key-value state (e.g., last processed block)
relayer_state (key, value)
```

## Configuration

| Variable | Default | Description |
|---|---|---|
| `RPC_URL` | `https://coston2-api.flare.network/ext/C/rpc` | Coston2 RPC endpoint |
| `REGISTRY_ADDRESS` | **(required)** | Deployed VaultRegistry address |
| `RELAYER_PRIVATE_KEY` | **(required)** | Hex-encoded private key (no 0x prefix) |
| `CHAIN_ID` | `114` | Chain ID (Coston2 = 114) |
| `ENCLAVE_URL` | `http://localhost:8090` | Vault Enclave API URL |
| `DATABASE_URL` | `postgres://localhost:5432/continuity_vault?sslmode=disable` | Postgres connection string |
| `DEMO_MODE` | `true` | Use short intervals for demo |

## Running Locally

```bash
# 1. Set up Postgres
createdb continuity_vault

# 2. Create .env file
make env-template > .env
# Edit .env with your values

# 3. Initialize dependencies
go mod tidy

# 4. Run
source .env && go run ./cmd/relayer
```

## Running Tests

```bash
go test -v ./...
```

## Docker

```bash
docker build -t continuity-vault-relayer .
docker run --env-file .env continuity-vault-relayer
```

## Demo Mode

With `DEMO_MODE=true`:
- Poll interval: **5 seconds**
- Reminder lead time: **30 seconds** before deadline
- Enclave polling: **3 seconds**

This means with all vault windows set to minutes, the full lifecycle plays out in a few minutes without manual intervention.

## Integration Points

### Layer 1 (VaultRegistry) → Layer 4
- **Events**: `VaultCreated`, `CheckIn`, `StateTransition`, `GuardianHalt`, etc.
- **View calls**: `getVaultState()`, `getVaultTiming()`, `isCheckInMissed()`
- **Write calls**: `markWarning()`, `requestAttestation()`, `submitQuorumResult()`, `finalizeDisputeWindow()`, `finalizeFinalWindow()`

### Layer 3 (Enclave) → Layer 4
- **Poll**: `GET /vaults/{id}/quorum-status` — checks if quorum is met
- **Fetch**: `GET /vaults/{id}/quorum-result` — gets signed result for on-chain submission
- **Reset**: `POST /vaults/{id}/reset` — after guardian halt

### Layer 4 → User
- **Notifications**: Check-in reminders, missed check-in alerts, grace expiry warnings, dispute window notices, final-override alerts
