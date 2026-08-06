"use client";

import { useState } from "react";
import { useAccount, useWriteContract, useWaitForTransactionReceipt } from "wagmi";
import { keccak256, toBytes } from "viem";
import { VAULT_REGISTRY_ADDRESS, FXRP_ADDRESS, vaultRegistryAbi } from "@/lib/contracts";
import { DEMO_PRESETS, formatSeconds, explorerTxUrl } from "@/lib/constants";
import { useRouter } from "next/navigation";

export default function CreateVaultPage() {
  const { isConnected } = useAccount();
  const router = useRouter();

  // Form state
  const [planDescription, setPlanDescription] = useState("");
  const [fundingAsset, setFundingAsset] = useState(FXRP_ADDRESS);
  const [checkInInterval, setCheckInInterval] = useState(DEMO_PRESETS.checkInInterval);
  const [graceWindow, setGraceWindow] = useState(DEMO_PRESETS.graceWindow);
  const [disputeWindow, setDisputeWindow] = useState(DEMO_PRESETS.disputeWindow);
  const [finalWindow, setFinalWindow] = useState(DEMO_PRESETS.finalWindow);
  const [guardianKey, setGuardianKey] = useState("");
  const [useDemo, setUseDemo] = useState(true);

  // Transaction state
  const {
    data: hash,
    writeContract,
    isPending: isWriting,
    error: writeError,
    reset,
  } = useWriteContract();

  const {
    isLoading: isConfirming,
    isSuccess: isConfirmed,
  } = useWaitForTransactionReceipt({ hash });

  // Compute plan commitment hash from description
  const planHash = planDescription
    ? keccak256(toBytes(planDescription))
    : ("0x" + "0".repeat(64)) as `0x${string}`;

  const applyDemoPresets = () => {
    setCheckInInterval(DEMO_PRESETS.checkInInterval);
    setGraceWindow(DEMO_PRESETS.graceWindow);
    setDisputeWindow(DEMO_PRESETS.disputeWindow);
    setFinalWindow(DEMO_PRESETS.finalWindow);
    setUseDemo(true);
  };

  const applyProductionPresets = () => {
    setCheckInInterval(30 * 86400); // 30 days
    setGraceWindow(7 * 86400);      // 7 days
    setDisputeWindow(14 * 86400);    // 14 days
    setFinalWindow(7 * 86400);       // 7 days
    setUseDemo(false);
  };

  const handleSubmit = () => {
    if (!guardianKey || !planDescription) return;

    reset();
    writeContract({
      address: VAULT_REGISTRY_ADDRESS,
      abi: vaultRegistryAbi,
      functionName: "createVault",
      args: [
        planHash,
        fundingAsset,
        BigInt(checkInInterval),
        BigInt(graceWindow),
        BigInt(disputeWindow),
        BigInt(finalWindow),
        guardianKey as `0x${string}`,
      ],
    });
  };

  const isLoading = isWriting || isConfirming;
  const isValid = planDescription.length > 0 && guardianKey.length === 42;

  if (!isConnected) {
    return (
      <div className="page">
        <div className="container" style={{ maxWidth: 640, margin: "0 auto" }}>
          <div className="card empty-state">
            <div className="empty-state-icon">🔗</div>
            <h3>Connect Your Wallet</h3>
            <p>Connect your wallet to create a Continuity Vault.</p>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="page">
      <div className="container" style={{ maxWidth: 720, margin: "0 auto" }}>
        <div className="page-header animate-slide-up">
          <h1>Create Vault</h1>
          <p>
            Set up a private inheritance or business-continuity plan. Your plan is
            hashed — only the commitment goes on-chain.
          </p>
        </div>

        <div className="card animate-in">
          {/* Plan Description → Commitment Hash */}
          <div className="form-group">
            <label className="form-label">Plan Description</label>
            <textarea
              className="form-textarea"
              placeholder="Describe your plan: beneficiaries, split percentages, conditions. This text is hashed — only the keccak256 commitment goes on-chain."
              value={planDescription}
              onChange={(e) => setPlanDescription(e.target.value)}
              rows={4}
            />
            {planDescription && (
              <div className="form-hint" style={{ fontFamily: "var(--font-mono)" }}>
                Commitment hash: {planHash.slice(0, 18)}…{planHash.slice(-8)}
              </div>
            )}
          </div>

          {/* Funding Asset */}
          <div className="form-group">
            <label className="form-label">Funding Asset</label>
            <input
              className="form-input mono"
              value={fundingAsset}
              onChange={(e) => setFundingAsset(e.target.value)}
              placeholder="0x... FXRP token address"
            />
            <span className="form-hint">FXRP ERC-20 address on Coston2</span>
          </div>

          {/* Guardian Halt Key */}
          <div className="form-group">
            <label className="form-label">Guardian Halt Key</label>
            <input
              className="form-input mono"
              value={guardianKey}
              onChange={(e) => setGuardianKey(e.target.value)}
              placeholder="0x... guardian address"
            />
            <span className="form-hint">
              Separate from your owner key — this address can halt a false trigger during the dispute window
            </span>
          </div>

          <hr className="divider" />

          {/* Timing Presets */}
          <div style={{ display: "flex", alignItems: "center", gap: "var(--space-md)", marginBottom: "var(--space-lg)" }}>
            <h3 style={{ fontSize: "1rem", flex: 1 }}>Timing Windows</h3>
            <button
              className={`btn btn-sm ${useDemo ? "btn-primary" : "btn-ghost"}`}
              onClick={applyDemoPresets}
            >
              ⚡ Demo (minutes)
            </button>
            <button
              className={`btn btn-sm ${!useDemo ? "btn-primary" : "btn-ghost"}`}
              onClick={applyProductionPresets}
            >
              🏢 Production (days)
            </button>
          </div>

          <div className="form-row">
            <div className="form-group">
              <label className="form-label">Check-in Interval</label>
              <input
                className="form-input"
                type="number"
                min={1}
                value={checkInInterval}
                onChange={(e) => setCheckInInterval(parseInt(e.target.value) || 0)}
              />
              <span className="form-hint">{formatSeconds(checkInInterval)}</span>
            </div>
            <div className="form-group">
              <label className="form-label">Grace Window</label>
              <input
                className="form-input"
                type="number"
                min={1}
                value={graceWindow}
                onChange={(e) => setGraceWindow(parseInt(e.target.value) || 0)}
              />
              <span className="form-hint">{formatSeconds(graceWindow)}</span>
            </div>
          </div>

          <div className="form-row">
            <div className="form-group">
              <label className="form-label">Dispute Window</label>
              <input
                className="form-input"
                type="number"
                min={1}
                value={disputeWindow}
                onChange={(e) => setDisputeWindow(parseInt(e.target.value) || 0)}
              />
              <span className="form-hint">{formatSeconds(disputeWindow)}</span>
            </div>
            <div className="form-group">
              <label className="form-label">Final Window</label>
              <input
                className="form-input"
                type="number"
                min={1}
                value={finalWindow}
                onChange={(e) => setFinalWindow(parseInt(e.target.value) || 0)}
              />
              <span className="form-hint">{formatSeconds(finalWindow)}</span>
            </div>
          </div>

          {useDemo && (
            <div className="alert alert-info" style={{ marginBottom: "var(--space-lg)" }}>
              ⚡ Demo mode: short timers so the full lifecycle runs in ~8 minutes.
            </div>
          )}

          <hr className="divider" />

          {/* Submit */}
          <button
            className="btn btn-primary btn-lg btn-full"
            onClick={handleSubmit}
            disabled={!isValid || isLoading}
          >
            {isWriting && (
              <>
                <span className="tx-spinner" />
                Confirm in wallet…
              </>
            )}
            {isConfirming && (
              <>
                <span className="tx-spinner" />
                Creating vault on Coston2…
              </>
            )}
            {!isLoading && "Create Vault"}
          </button>

          {isConfirmed && hash && (
            <div className="tx-status success" style={{ marginTop: "var(--space-md)" }}>
              ✓ Vault created! —{" "}
              <a
                href={explorerTxUrl(hash)}
                target="_blank"
                rel="noopener noreferrer"
                style={{ color: "inherit", textDecoration: "underline" }}
              >
                View on explorer
              </a>
              <button
                className="btn btn-primary btn-sm"
                style={{ marginLeft: "var(--space-md)" }}
                onClick={() => router.push("/")}
              >
                Go to Dashboard →
              </button>
            </div>
          )}

          {writeError && (
            <div className="tx-status error" style={{ marginTop: "var(--space-md)" }}>
              ✕ {writeError.message?.slice(0, 150) ?? "Transaction failed"}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
