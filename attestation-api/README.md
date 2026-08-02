# Attestation API

> **Layer 2** — not built yet.

A small HTTPS + JSON service where a trustee posts a signed statement referencing a vault's case ID. FDC's `Web2Json` attestation type then verifies this API response on-chain.

This isn't a workaround — it's the same mechanism proposed for the Phase 2 public-record feed, just pointed at a service we control instead of a third-party obituary API.
