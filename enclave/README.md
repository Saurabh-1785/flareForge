# Vault Enclave — Confidential Compute Reference Service

> **Layer 3** — not built yet. This README describes the planned service.

## What This Is

The Vault Enclave is the only component that ever decrypts the full inheritance/succession plan. It runs inside a TEE (Trusted Execution Environment) — specifically Google Cloud Confidential Space, following the same pattern as [Flare AI Kit](https://github.com/flare-foundation/flare-ai-kit).

## Why a TEE, Not Just Encryption

The plan — who receives funds, how much, under what conditions — is exactly what attackers, regulators, and hostile family members all want pre-execution. A TEE ensures this data is never exposed, not even to the service operator. (Design Principle #5: confidentiality is a means, not the pitch.)

## Planned Endpoints

- `POST /vaults/{id}/plan` — accepts encrypted plan blob + commitment hash
- `POST /vaults/{id}/attestations` — accepts verified FDC attestation references
- `GET /vaults/{id}/quorum-status` — polled by the relayer

## Hardware & FCC Status

At initial FCC rollout, TEE nodes are operated by the Flare Foundation on Google Cloud Confidential Space. Third-party custom TEE deployment is a later milestone. This service follows the Confidential Space pattern as a reference implementation with a real migration path to enshrined FCC when that opens.

## Stack

- Go
- gRPC/REST API
- SQLite or Postgres for sealed (encrypted) plan storage
