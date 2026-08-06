"use client";

import { useState } from "react";
import { ATTESTATION_TYPES } from "@/lib/constants";
import { ATTESTATION_API_URL } from "@/lib/contracts";

export default function TrusteePage() {
  const [caseId, setCaseId] = useState("");
  const [attestationType, setAttestationType] = useState(ATTESTATION_TYPES[0].value);
  const [attestorAddress, setAttestorAddress] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [result, setResult] = useState<{ success: boolean; message: string } | null>(null);

  const selectedType = ATTESTATION_TYPES.find((t) => t.value === attestationType);
  const isValid = caseId.length > 0 && attestorAddress.length === 42;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setResult(null);

    try {
      const response = await fetch(`${ATTESTATION_API_URL}/attest`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          caseId,
          attestationType,
          attestorAddress,
          vaultId: caseId, // case ID typically maps to vault ID
        }),
      });

      const data = await response.json();

      if (response.ok) {
        setResult({
          success: true,
          message: `Attestation recorded successfully. Total attestations for this case: ${data.totalAttestations}`,
        });
      } else {
        setResult({
          success: false,
          message: data.error || "Failed to submit attestation",
        });
      }
    } catch (err) {
      setResult({
        success: false,
        message: `Connection failed. Make sure the attestation API is running at ${ATTESTATION_API_URL}`,
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div className="page">
      <div className="container trustee-page">
        <div className="page-header animate-slide-up" style={{ textAlign: "center" }}>
          <h1>Trustee Attestation</h1>
          <p style={{ margin: "0 auto" }}>
            You&apos;ve been designated as a trustee for a Continuity Vault. Submit
            your attestation below — no cryptocurrency wallet or technical
            knowledge required.
          </p>
        </div>

        <div className="card animate-in">
          <form onSubmit={handleSubmit}>
            {/* Case ID */}
            <div className="form-group">
              <label className="form-label">Case Identifier</label>
              <input
                className="form-input"
                type="text"
                placeholder="Enter the case ID you were given"
                value={caseId}
                onChange={(e) => setCaseId(e.target.value)}
                required
              />
              <span className="form-hint">
                This was provided by the vault holder when they designated you as a trustee
              </span>
            </div>

            {/* Attestation Type */}
            <div className="form-group">
              <label className="form-label">Attestation Type</label>
              <div style={{ display: "flex", flexDirection: "column", gap: "var(--space-sm)" }}>
                {ATTESTATION_TYPES.map((type) => (
                  <label
                    key={type.value}
                    style={{
                      display: "flex",
                      alignItems: "flex-start",
                      gap: "var(--space-md)",
                      padding: "var(--space-md)",
                      borderRadius: "var(--radius-md)",
                      border: `1px solid ${attestationType === type.value ? "var(--border-accent)" : "var(--border-subtle)"}`,
                      background: attestationType === type.value ? "var(--accent-subtle)" : "transparent",
                      cursor: "pointer",
                      transition: "all var(--transition-fast)",
                    }}
                  >
                    <input
                      type="radio"
                      name="attestationType"
                      value={type.value}
                      checked={attestationType === type.value}
                      onChange={(e) => setAttestationType(e.target.value)}
                      style={{ marginTop: 4 }}
                    />
                    <div>
                      <div style={{ fontWeight: 600, color: "var(--text-primary)", marginBottom: 2 }}>
                        {type.label}
                      </div>
                      <div style={{ fontSize: "0.85rem", color: "var(--text-secondary)" }}>
                        {type.description}
                      </div>
                    </div>
                  </label>
                ))}
              </div>
            </div>

            {/* Attestor Address */}
            <div className="form-group">
              <label className="form-label">Your Identifier</label>
              <input
                className="form-input mono"
                type="text"
                placeholder="0x... your Ethereum address"
                value={attestorAddress}
                onChange={(e) => setAttestorAddress(e.target.value)}
                required
              />
              <span className="form-hint">
                The Ethereum address the vault holder registered you with. Used for deduplication only — no funds or wallet needed.
              </span>
            </div>

            <hr className="divider" />

            {/* Attestation Preview */}
            {isValid && (
              <div className="attestation-preview">
                &ldquo;I, <strong>{attestorAddress.slice(0, 10)}…</strong>, attest that the individual
                identified by case <strong>{caseId}</strong> has{" "}
                <strong>{selectedType?.description?.toLowerCase()}</strong>.
                I understand this attestation will be independently verified through the Flare Data Connector
                and contributes to a multi-signal quorum — it cannot trigger fund release alone.&rdquo;
              </div>
            )}

            {/* Submit */}
            <button
              type="submit"
              className="btn btn-primary btn-lg btn-full"
              disabled={!isValid || isSubmitting}
            >
              {isSubmitting ? (
                <>
                  <span className="tx-spinner" />
                  Submitting…
                </>
              ) : (
                "Submit Attestation"
              )}
            </button>

            {/* Result */}
            {result && (
              <div
                className={`tx-status ${result.success ? "success" : "error"}`}
                style={{ marginTop: "var(--space-md)" }}
              >
                {result.success ? "✓" : "✕"} {result.message}
              </div>
            )}
          </form>
        </div>

        {/* Info box */}
        <div className="alert alert-info animate-in" style={{ marginTop: "var(--space-xl)" }}>
          <span style={{ fontSize: "1.2rem" }}>🔒</span>
          <div>
            <strong>Your privacy is protected.</strong> You see a case ID only — never the
            vault balance, beneficiary list, or plan details. Your attestation is one of
            multiple independent signals required. No single attestation can trigger a release.
          </div>
        </div>
      </div>
    </div>
  );
}
