"use client";

import { useState, useCallback } from "react";
import { useParams } from "next/navigation";
import {
  useAccount,
  useReadContract,
  useWriteContract,
  useWaitForTransactionReceipt,
} from "wagmi";
import { parseEther, formatEther } from "viem";
import {
  VAULT_REGISTRY_ADDRESS,
  FXRP_ADDRESS,
  vaultRegistryAbi,
  erc20Abi,
} from "@/lib/contracts";
import {
  getStateName,
  STATE_DISPLAY,
  formatSeconds,
  truncateAddress,
  explorerTxUrl,
  explorerAddressUrl,
} from "@/lib/constants";
import { VaultStateBadge } from "@/components/VaultStateBadge";
import { CountdownTimer } from "@/components/CountdownTimer";
import { StateTimeline } from "@/components/StateTimeline";

export default function VaultDetailPage() {
  const params = useParams();
  const vaultId = BigInt(params.id as string);
  const { address, isConnected } = useAccount();

  const [fundAmount, setFundAmount] = useState("");
  const [legalDocInput, setLegalDocInput] = useState("");
  const [showConfig, setShowConfig] = useState(false);

  // ── Read vault data ──────────────────────────────────────────────
  const { data: stateData, refetch: refetchState } = useReadContract({
    address: VAULT_REGISTRY_ADDRESS,
    abi: vaultRegistryAbi,
    functionName: "getVaultState",
    args: [vaultId],
  });

  const { data: balanceData, refetch: refetchBalance } = useReadContract({
    address: VAULT_REGISTRY_ADDRESS,
    abi: vaultRegistryAbi,
    functionName: "getVaultBalance",
    args: [vaultId],
  });

  const { data: configData } = useReadContract({
    address: VAULT_REGISTRY_ADDRESS,
    abi: vaultRegistryAbi,
    functionName: "getVaultConfig",
    args: [vaultId],
  });

  const { data: timingData, refetch: refetchTiming } = useReadContract({
    address: VAULT_REGISTRY_ADDRESS,
    abi: vaultRegistryAbi,
    functionName: "getVaultTiming",
    args: [vaultId],
  });

  const { data: attestationCount } = useReadContract({
    address: VAULT_REGISTRY_ADDRESS,
    abi: vaultRegistryAbi,
    functionName: "vaultAttestationCount",
    args: [vaultId],
  });

  const { data: quorumThreshold } = useReadContract({
    address: VAULT_REGISTRY_ADDRESS,
    abi: vaultRegistryAbi,
    functionName: "quorumThreshold",
  });

  // ── Transaction hooks ────────────────────────────────────────────
  const { data: txHash, writeContract, isPending, error: txError, reset: resetTx } = useWriteContract();
  const { isLoading: confirming, isSuccess: confirmed } = useWaitForTransactionReceipt({ hash: txHash });

  const { data: approveTxHash, writeContract: writeApprove, isPending: approvePending } = useWriteContract();
  const { isLoading: approveConfirming, isSuccess: approveConfirmed } = useWaitForTransactionReceipt({ hash: approveTxHash });

  // ── Derived state ────────────────────────────────────────────────
  const state = typeof stateData === "number" ? stateData : 0;
  const stateName = getStateName(state);
  const stateDisplay = STATE_DISPLAY[stateName];
  const balance = (balanceData as bigint) ?? 0n;
  const config = configData as readonly [string, `0x${string}`, string, string, `0x${string}`] | undefined;
  const timing = timingData as readonly [bigint, bigint, bigint, bigint, bigint, bigint] | undefined;

  const owner = config?.[0] ?? "";
  const planHash = config?.[1] ?? "0x";
  const fundingAsset = config?.[2] ?? "";
  const guardianKey = config?.[3] ?? "";
  const legalDocHash = config?.[4] ?? "0x" + "0".repeat(64);

  const lastCheckIn = timing ? Number(timing[0]) : 0;
  const windowDeadline = timing ? Number(timing[1]) : 0;
  const checkInInterval = timing ? Number(timing[2]) : 0;
  const graceWindow = timing ? Number(timing[3]) : 0;
  const disputeWindow = timing ? Number(timing[4]) : 0;
  const finalWindow = timing ? Number(timing[5]) : 0;

  const isOwner = address && owner && address.toLowerCase() === owner.toLowerCase();
  const isGuardian = address && guardianKey && address.toLowerCase() === guardianKey.toLowerCase();
  const isLoading = isPending || confirming;
  const attestCount = Number(attestationCount ?? 0n);
  const quorumReq = Number(quorumThreshold ?? 2n);

  const refetchAll = useCallback(() => {
    refetchState();
    refetchBalance();
    refetchTiming();
  }, [refetchState, refetchBalance, refetchTiming]);

  // Refetch after confirmed tx
  if (confirmed) {
    setTimeout(() => {
      refetchAll();
      resetTx();
    }, 2000);
  }

  // ── Actions ──────────────────────────────────────────────────────
  const doCheckIn = () => {
    resetTx();
    writeContract({
      address: VAULT_REGISTRY_ADDRESS,
      abi: vaultRegistryAbi,
      functionName: "checkIn",
      args: [vaultId, "0x" as `0x${string}`],
    });
  };

  const doGuardianHalt = () => {
    resetTx();
    writeContract({
      address: VAULT_REGISTRY_ADDRESS,
      abi: vaultRegistryAbi,
      functionName: "guardianHalt",
      args: [vaultId],
    });
  };

  const doOwnerOverride = () => {
    resetTx();
    writeContract({
      address: VAULT_REGISTRY_ADDRESS,
      abi: vaultRegistryAbi,
      functionName: "ownerOverride",
      args: [vaultId, "0x" as `0x${string}`],
    });
  };

  const doCancelVault = () => {
    if (!confirm("Cancel this vault? Remaining funds will be returned.")) return;
    resetTx();
    writeContract({
      address: VAULT_REGISTRY_ADDRESS,
      abi: vaultRegistryAbi,
      functionName: "cancelVault",
      args: [vaultId],
    });
  };

  const doAnchorLegalDoc = () => {
    if (!legalDocInput) return;
    resetTx();
    const hash = legalDocInput.startsWith("0x") ? legalDocInput : ("0x" + legalDocInput);
    writeContract({
      address: VAULT_REGISTRY_ADDRESS,
      abi: vaultRegistryAbi,
      functionName: "anchorLegalDoc",
      args: [vaultId, hash as `0x${string}`],
    });
  };

  const doApproveAndFund = () => {
    if (!fundAmount) return;
    const amount = parseEther(fundAmount);
    writeApprove({
      address: FXRP_ADDRESS,
      abi: erc20Abi,
      functionName: "approve",
      args: [VAULT_REGISTRY_ADDRESS, amount],
    });
  };

  const doFundVault = () => {
    if (!fundAmount) return;
    resetTx();
    const amount = parseEther(fundAmount);
    writeContract({
      address: VAULT_REGISTRY_ADDRESS,
      abi: vaultRegistryAbi,
      functionName: "fundVault",
      args: [vaultId, amount],
    });
  };

  const doFinalizeDispute = () => {
    resetTx();
    writeContract({
      address: VAULT_REGISTRY_ADDRESS,
      abi: vaultRegistryAbi,
      functionName: "finalizeDisputeWindow",
      args: [vaultId],
    });
  };

  const doFinalizeFinal = () => {
    resetTx();
    writeContract({
      address: VAULT_REGISTRY_ADDRESS,
      abi: vaultRegistryAbi,
      functionName: "finalizeFinalWindow",
      args: [vaultId],
    });
  };

  return (
    <div className="page">
      <div className="container" style={{ maxWidth: 900, margin: "0 auto" }}>
        {/* Header */}
        <div className="animate-slide-up" style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", marginBottom: "var(--space-xl)" }}>
          <div>
            <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.8rem", color: "var(--text-tertiary)" }}>VAULT</span>
            <h1 style={{ fontSize: "2.5rem" }}>#{vaultId.toString()}</h1>
          </div>
          <VaultStateBadge stateIndex={state} size="lg" />
        </div>

        {/* State description */}
        <div className="alert alert-info animate-in" style={{ marginBottom: "var(--space-xl)" }}>
          <span style={{ fontSize: "1.2rem" }}>ℹ️</span>
          <div>
            <strong>{stateDisplay.label}</strong> — {stateDisplay.description}
          </div>
        </div>

        {/* State Timeline */}
        <div className="card animate-in" style={{ marginBottom: "var(--space-xl)" }}>
          <h3 style={{ fontSize: "0.85rem", color: "var(--text-tertiary)", textTransform: "uppercase", letterSpacing: "0.06em", marginBottom: "var(--space-md)" }}>
            Lifecycle Progress
          </h3>
          <StateTimeline currentStateIndex={state} />
        </div>

        {/* Stats Row */}
        <div className="grid-stats animate-in" style={{ marginBottom: "var(--space-xl)" }}>
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
                <span className="stat-unit"> / {quorumReq} required</span>
              </span>
            </div>
          </div>
          <div className="card card-compact">
            <div className="stat">
              <span className="stat-label">Check-in Interval</span>
              <span className="stat-value" style={{ fontSize: "1.1rem" }}>
                {formatSeconds(checkInInterval)}
              </span>
            </div>
          </div>
        </div>

        {/* ── Owner Actions ── */}
        {isConnected && (isOwner || isGuardian) && (
          <div className="card animate-in" style={{ marginBottom: "var(--space-xl)" }}>
            <h3 style={{ marginBottom: "var(--space-lg)" }}>Actions</h3>

            <div className="grid-actions" style={{ marginBottom: "var(--space-lg)" }}>
              {/* Check In — ACTIVE or WARNING */}
              {isOwner && (state === 0 || state === 1) && (
                <button className={`btn ${state === 1 ? "btn-warning" : "btn-success"}`} onClick={doCheckIn} disabled={isLoading}>
                  {state === 1 ? "⚠️ Check In NOW" : "✓ Check In"}
                </button>
              )}

              {/* Owner Override — QUORUM_PENDING */}
              {isOwner && state === 2 && (
                <button className="btn btn-warning" onClick={doOwnerOverride} disabled={isLoading}>
                  🚨 I&apos;m Alive — Override
                </button>
              )}

              {/* Guardian Halt — DISPUTE_WINDOW or FINAL_WINDOW */}
              {isGuardian && (state === 3 || state === 6) && (
                <button className="btn btn-danger" onClick={doGuardianHalt} disabled={isLoading}>
                  🛑 Guardian Halt
                </button>
              )}

              {/* Finalize Dispute — DISPUTE_WINDOW, anyone can call */}
              {state === 3 && (
                <button className="btn btn-secondary" onClick={doFinalizeDispute} disabled={isLoading}>
                  Finalize Dispute Window
                </button>
              )}

              {/* Finalize Final — FINAL_WINDOW, anyone can call */}
              {state === 6 && (
                <button className="btn btn-secondary" onClick={doFinalizeFinal} disabled={isLoading}>
                  Finalize Final Window
                </button>
              )}

              {/* Cancel — ACTIVE only */}
              {isOwner && state === 0 && (
                <button className="btn btn-ghost" onClick={doCancelVault} disabled={isLoading}>
                  Cancel Vault
                </button>
              )}
            </div>

            {/* Fund Vault */}
            {isOwner && state < 7 && (
              <div style={{ borderTop: "1px solid var(--border-subtle)", paddingTop: "var(--space-lg)" }}>
                <h4 style={{ fontSize: "0.9rem", marginBottom: "var(--space-sm)" }}>Fund Vault</h4>
                <div style={{ display: "flex", gap: "var(--space-sm)", alignItems: "flex-start" }}>
                  <input
                    className="form-input"
                    type="number"
                    step="0.0001"
                    min="0"
                    placeholder="Amount (FXRP)"
                    value={fundAmount}
                    onChange={(e) => setFundAmount(e.target.value)}
                    style={{ maxWidth: 200 }}
                  />
                  {!approveConfirmed ? (
                    <button className="btn btn-secondary" onClick={doApproveAndFund} disabled={approvePending || approveConfirming || !fundAmount}>
                      {approvePending || approveConfirming ? (
                        <><span className="tx-spinner" /> Approving…</>
                      ) : (
                        "1. Approve FXRP"
                      )}
                    </button>
                  ) : (
                    <button className="btn btn-primary" onClick={doFundVault} disabled={isLoading || !fundAmount}>
                      2. Fund Vault
                    </button>
                  )}
                </div>
              </div>
            )}

            {/* Anchor Legal Doc */}
            {isOwner && state < 7 && (
              <div style={{ borderTop: "1px solid var(--border-subtle)", paddingTop: "var(--space-lg)", marginTop: "var(--space-lg)" }}>
                <h4 style={{ fontSize: "0.9rem", marginBottom: "var(--space-sm)" }}>Anchor Legal Document</h4>
                <div style={{ display: "flex", gap: "var(--space-sm)" }}>
                  <input
                    className="form-input mono"
                    placeholder="keccak256(will/trust deed)"
                    value={legalDocInput}
                    onChange={(e) => setLegalDocInput(e.target.value)}
                    style={{ flex: 1 }}
                  />
                  <button className="btn btn-secondary" onClick={doAnchorLegalDoc} disabled={isLoading || !legalDocInput}>
                    Anchor
                  </button>
                </div>
                <span className="form-hint">
                  On-chain evidentiary linkage — a court can verify this hash against the original document
                </span>
              </div>
            )}

            {/* Transaction feedback */}
            {isPending && (
              <div className="tx-status pending" style={{ marginTop: "var(--space-md)" }}>
                <span className="tx-spinner" /> Confirm in your wallet…
              </div>
            )}
            {confirming && txHash && (
              <div className="tx-status pending" style={{ marginTop: "var(--space-md)" }}>
                <span className="tx-spinner" /> Confirming on Coston2…
              </div>
            )}
            {confirmed && txHash && (
              <div className="tx-status success" style={{ marginTop: "var(--space-md)" }}>
                ✓ Confirmed —{" "}
                <a href={explorerTxUrl(txHash)} target="_blank" rel="noopener noreferrer" style={{ color: "inherit", textDecoration: "underline" }}>
                  View transaction
                </a>
              </div>
            )}
            {txError && (
              <div className="tx-status error" style={{ marginTop: "var(--space-md)" }}>
                ✕ {txError.message?.slice(0, 150)}
              </div>
            )}
          </div>
        )}

        {/* ── Vault Config (collapsible) ── */}
        <div className="card animate-in">
          <button
            className="collapsible-trigger"
            onClick={() => setShowConfig(!showConfig)}
            aria-expanded={showConfig}
          >
            <span>Vault Configuration</span>
            <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M4 6l4 4 4-4" />
            </svg>
          </button>

          {showConfig && (
            <div className="collapsible-content" style={{ marginTop: "var(--space-md)" }}>
              <div style={{ display: "grid", gap: "var(--space-md)" }}>
                <div className="event-item">
                  <span className="event-item-icon">👤</span>
                  <span className="event-item-content">
                    Owner:{" "}
                    <a href={explorerAddressUrl(owner)} target="_blank" rel="noopener noreferrer" style={{ fontFamily: "var(--font-mono)", fontSize: "0.8rem" }}>
                      {truncateAddress(owner)}
                    </a>
                  </span>
                </div>
                <div className="event-item">
                  <span className="event-item-icon">🛡️</span>
                  <span className="event-item-content">
                    Guardian:{" "}
                    <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.8rem" }}>
                      {truncateAddress(guardianKey)}
                    </span>
                  </span>
                </div>
                <div className="event-item">
                  <span className="event-item-icon">🔒</span>
                  <span className="event-item-content">
                    Plan Commitment:{" "}
                    <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.75rem" }}>
                      {(planHash as string).slice(0, 18)}…{(planHash as string).slice(-8)}
                    </span>
                  </span>
                </div>
                <div className="event-item">
                  <span className="event-item-icon">📄</span>
                  <span className="event-item-content">
                    Legal Doc Hash:{" "}
                    <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.75rem" }}>
                      {legalDocHash === "0x" + "0".repeat(64) ? "Not anchored" : `${(legalDocHash as string).slice(0, 18)}…`}
                    </span>
                  </span>
                </div>
                <div className="event-item">
                  <span className="event-item-icon">💎</span>
                  <span className="event-item-content">
                    Funding Asset:{" "}
                    <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.8rem" }}>
                      {truncateAddress(fundingAsset)}
                    </span>
                  </span>
                </div>

                <hr className="divider" style={{ margin: "var(--space-sm) 0" }} />

                <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "var(--space-sm)" }}>
                  <div className="stat">
                    <span className="stat-label">Check-in Interval</span>
                    <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.85rem" }}>{formatSeconds(checkInInterval)}</span>
                  </div>
                  <div className="stat">
                    <span className="stat-label">Grace Window</span>
                    <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.85rem" }}>{formatSeconds(graceWindow)}</span>
                  </div>
                  <div className="stat">
                    <span className="stat-label">Dispute Window</span>
                    <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.85rem" }}>{formatSeconds(disputeWindow)}</span>
                  </div>
                  <div className="stat">
                    <span className="stat-label">Final Window</span>
                    <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.85rem" }}>{formatSeconds(finalWindow)}</span>
                  </div>
                </div>

                <div className="stat" style={{ marginTop: "var(--space-sm)" }}>
                  <span className="stat-label">Last Check-in</span>
                  <span style={{ fontFamily: "var(--font-mono)", fontSize: "0.85rem" }}>
                    {lastCheckIn > 0 ? new Date(lastCheckIn * 1000).toLocaleString() : "Never"}
                  </span>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
