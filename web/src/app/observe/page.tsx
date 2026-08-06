"use client";

import { useState } from "react";
import { useReadContract } from "wagmi";
import { formatEther } from "viem";
import { VAULT_REGISTRY_ADDRESS, vaultRegistryAbi } from "@/lib/contracts";
import {
  getStateName,
  STATE_DISPLAY,
  formatSeconds,
  truncateAddress,
  explorerAddressUrl,
} from "@/lib/constants";
import { VaultStateBadge } from "@/components/VaultStateBadge";
import { CountdownTimer } from "@/components/CountdownTimer";
import { StateTimeline } from "@/components/StateTimeline";

export default function ObservePage() {
  const [inputId, setInputId] = useState("");
  const [vaultId, setVaultId] = useState<bigint | null>(null);

  const handleLookup = (e: React.FormEvent) => {
    e.preventDefault();
    const id = parseInt(inputId);
    if (id > 0) setVaultId(BigInt(id));
  };

  // ── Read vault data (only when vaultId is set) ───────────────────
  const enabled = vaultId !== null;

  const { data: stateData, isError: stateError } = useReadContract({
    address: VAULT_REGISTRY_ADDRESS,
    abi: vaultRegistryAbi,
    functionName: "getVaultState",
    args: vaultId !== null ? [vaultId] : undefined,
    query: { enabled },
  });

  const { data: balanceData } = useReadContract({
    address: VAULT_REGISTRY_ADDRESS,
    abi: vaultRegistryAbi,
    functionName: "getVaultBalance",
    args: vaultId !== null ? [vaultId] : undefined,
    query: { enabled },
  });

  const { data: configData } = useReadContract({
    address: VAULT_REGISTRY_ADDRESS,
    abi: vaultRegistryAbi,
    functionName: "getVaultConfig",
    args: vaultId !== null ? [vaultId] : undefined,
    query: { enabled },
  });

  const { data: timingData } = useReadContract({
    address: VAULT_REGISTRY_ADDRESS,
    abi: vaultRegistryAbi,
    functionName: "getVaultTiming",
    args: vaultId !== null ? [vaultId] : undefined,
    query: { enabled },
  });

  const { data: attestationCount } = useReadContract({
    address: VAULT_REGISTRY_ADDRESS,
    abi: vaultRegistryAbi,
    functionName: "vaultAttestationCount",
    args: vaultId !== null ? [vaultId] : undefined,
    query: { enabled },
  });

  const { data: quorumThreshold } = useReadContract({
    address: VAULT_REGISTRY_ADDRESS,
    abi: vaultRegistryAbi,
    functionName: "quorumThreshold",
    query: { enabled },
  });

  // ── Derived data ─────────────────────────────────────────────────
  const state = typeof stateData === "number" ? stateData : 0;
  const stateName = getStateName(state);
  const stateDisplay = STATE_DISPLAY[stateName];
  const balance = (balanceData as bigint) ?? 0n;
  const config = configData as readonly [string, `0x${string}`, string, string, `0x${string}`] | undefined;
  const timing = timingData as readonly [bigint, bigint, bigint, bigint, bigint, bigint] | undefined;

  const owner = config?.[0] ?? "";
  const guardianKey = config?.[3] ?? "";
  const windowDeadline = timing ? Number(timing[1]) : 0;
  const checkInInterval = timing ? Number(timing[2]) : 0;
  const graceWindow = timing ? Number(timing[3]) : 0;
  const disputeWindow = timing ? Number(timing[4]) : 0;
  const finalWindow = timing ? Number(timing[5]) : 0;
  const attestCount = Number(attestationCount ?? 0n);
  const quorumReq = Number(quorumThreshold ?? 2n);

  const vaultExists = enabled && !stateError && owner !== "" && owner !== "0x0000000000000000000000000000000000000000";

  return (
    <div className="page">
      <div className="container" style={{ maxWidth: 800, margin: "0 auto" }}>
        <div className="page-header animate-slide-up" style={{ textAlign: "center" }}>
          <h1>Observe a Vault</h1>
          <p style={{ margin: "0 auto" }}>
            View the public state of any Continuity Vault. No wallet required.
          </p>
        </div>

        {/* Lookup form */}
        <form onSubmit={handleLookup} style={{ marginBottom: "var(--space-2xl)" }}>
          <div style={{ display: "flex", gap: "var(--space-sm)", maxWidth: 400, margin: "0 auto" }}>
            <input
              className="form-input"
              type="number"
              min="1"
              placeholder="Enter Vault ID"
              value={inputId}
              onChange={(e) => setInputId(e.target.value)}
              style={{ flex: 1 }}
            />
            <button type="submit" className="btn btn-primary" disabled={!inputId}>
              Look Up
            </button>
          </div>
        </form>

        {/* Vault not found */}
        {enabled && !vaultExists && !stateError && (
          <div className="card empty-state animate-in">
            <div className="empty-state-icon">🔍</div>
            <h3>Vault not found</h3>
            <p>No vault exists with ID #{vaultId?.toString()}. Check the number and try again.</p>
          </div>
        )}

        {/* Vault display */}
        {vaultExists && (
          <div className="animate-in">
            {/* Header */}
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "var(--space-xl)" }}>
              <div>
                <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.8rem", color: "var(--text-tertiary)" }}>VAULT</span>
                <h2 style={{ fontSize: "2rem" }}>#{vaultId?.toString()}</h2>
              </div>
              <VaultStateBadge stateIndex={state} size="lg" />
            </div>

            {/* State description */}
            <div className="alert alert-info" style={{ marginBottom: "var(--space-xl)" }}>
              <span style={{ fontSize: "1.2rem" }}>ℹ️</span>
              <div>
                <strong>{stateDisplay.label}</strong> — {stateDisplay.description}
              </div>
            </div>

            {/* Timeline */}
            <div className="card" style={{ marginBottom: "var(--space-xl)" }}>
              <h3 style={{ fontSize: "0.85rem", color: "var(--text-tertiary)", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: "var(--space-md)" }}>
                Lifecycle Progress
              </h3>
              <StateTimeline currentStateIndex={state} />
            </div>

            {/* Stats */}
            <div className="grid-stats" style={{ marginBottom: "var(--space-xl)" }}>
              <div className="card card-compact">
                <div className="stat">
                  <span className="stat-label">Balance</span>
                  <span className="stat-value">
                    {parseFloat(formatEther(balance)).toFixed(4)}
                    <span className="stat-unit"> FXRP</span>
                  </span>
                </div>
              </div>
              <div className="card card-compact">
                <CountdownTimer deadline={windowDeadline} label="Deadline" />
              </div>
              <div className="card card-compact">
                <div className="stat">
                  <span className="stat-label">Attestations</span>
                  <span className="stat-value">
                    {attestCount}
                    <span className="stat-unit"> / {quorumReq}</span>
                  </span>
                </div>
              </div>
            </div>

            {/* Config summary */}
            <div className="card">
              <h3 style={{ fontSize: "0.95rem", marginBottom: "var(--space-md)" }}>Configuration</h3>
              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "var(--space-md)" }}>
                <div className="event-item">
                  <span className="event-item-icon">👤</span>
                  <span className="event-item-content">
                    Owner:{" "}
                    <a href={explorerAddressUrl(owner)} target="_blank" rel="noopener noreferrer" style={{ fontFamily: "var(--font-mono)", fontSize: "0.78rem" }}>
                      {truncateAddress(owner)}
                    </a>
                  </span>
                </div>
                <div className="event-item">
                  <span className="event-item-icon">🛡️</span>
                  <span className="event-item-content">
                    Guardian:{" "}
                    <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.78rem" }}>
                      {truncateAddress(guardianKey)}
                    </span>
                  </span>
                </div>
              </div>

              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr 1fr", gap: "var(--space-md)", marginTop: "var(--space-lg)" }}>
                <div className="stat">
                  <span className="stat-label">Check-in</span>
                  <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.85rem" }}>{formatSeconds(checkInInterval)}</span>
                </div>
                <div className="stat">
                  <span className="stat-label">Grace</span>
                  <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.85rem" }}>{formatSeconds(graceWindow)}</span>
                </div>
                <div className="stat">
                  <span className="stat-label">Dispute</span>
                  <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.85rem" }}>{formatSeconds(disputeWindow)}</span>
                </div>
                <div className="stat">
                  <span className="stat-label">Final</span>
                  <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.85rem" }}>{formatSeconds(finalWindow)}</span>
                </div>
              </div>
            </div>

            {/* Post-release message */}
            {(state === 7 || state === 5) && (
              <div className="alert alert-success" style={{ marginTop: "var(--space-xl)" }}>
                <span style={{ fontSize: "1.2rem" }}>✓</span>
                <div>
                  <strong>Funds have been released.</strong>{" "}
                  {state === 7
                    ? "All funds have been fully released to beneficiaries via FAssets redemption."
                    : "Tranche 1 (50%) has been released. Final window is pending."}
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
