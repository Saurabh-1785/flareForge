// Package quorum implements the N-of-M quorum evaluation engine.
// It receives verified FDC attestation references, evaluates them against
// the sealed plan's configured thresholds, and produces signed quorum results.
//
// Architecture ref: "Quorum Engine" in the FCC plane.
// Design Principle #1: Trust-minimize the trigger — multiple independent
// signals must agree before a trigger is treated as real.
//
// Phase 1 (MVP): 2-of-2 simple quorum — two independent attestations required.
// Phase 2: N-of-M weighted quorum with attestor reputation scores.
package quorum

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	enclavecrypto "github.com/continuity-vault/enclave/internal/crypto"
	"github.com/continuity-vault/enclave/internal/sealedstore"
)

// ──────────────────────────────────────────────────────────────────────
// Domain types
// ──────────────────────────────────────────────────────────────────────

// AttestationRef is a reference to a verified FDC attestation, relayed
// to the enclave from the on-chain FdcAttestationVerifier.
type AttestationRef struct {
	// VaultID is the on-chain vault this attestation references.
	VaultID uint64 `json:"vaultId"`

	// CaseID matches the attestation-api's case identifier.
	CaseID string `json:"caseId"`

	// AttestationType is the kind of signal (e.g., "DEATH", "INCAPACITATION").
	AttestationType string `json:"attestationType"`

	// AttestorAddress is the Ethereum address of the attestor.
	AttestorAddress string `json:"attestorAddress"`

	// FdcVotingRound is the FDC round that verified this attestation.
	FdcVotingRound uint64 `json:"fdcVotingRound"`

	// VerifiedAt is the timestamp when the on-chain verification occurred.
	VerifiedAt time.Time `json:"verifiedAt"`
}

// QuorumStatus represents the current quorum evaluation state for a vault.
type QuorumStatus struct {
	VaultID           uint64           `json:"vaultId"`
	QuorumMet         bool             `json:"quorumMet"`
	RequiredCount     uint8            `json:"requiredCount"`
	CurrentCount      int              `json:"currentCount"`
	Attestations      []AttestationRef `json:"attestations"`
	SignedResult      []byte           `json:"signedResult,omitempty"`
	EvaluatedAt       *time.Time       `json:"evaluatedAt,omitempty"`
	AcceptedTypes     []string         `json:"acceptedTypes"`
	EnclaveAddress    string           `json:"enclaveAddress"`
}

// ──────────────────────────────────────────────────────────────────────
// Engine — the core quorum evaluation logic.
// ──────────────────────────────────────────────────────────────────────

// Engine evaluates quorum by comparing received attestations against
// the sealed plan's thresholds. It runs entirely inside the TEE.
type Engine struct {
	mu    sync.RWMutex
	db    *sql.DB
	store *sealedstore.Store
	keys  *enclavecrypto.EnclaveKeys
}

// NewEngine creates a new quorum evaluation engine.
func NewEngine(db *sql.DB, store *sealedstore.Store, keys *enclavecrypto.EnclaveKeys) (*Engine, error) {
	e := &Engine{
		db:    db,
		store: store,
		keys:  keys,
	}

	if err := e.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate quorum engine: %w", err)
	}

	return e, nil
}

// migrate creates the attestation tracking tables.
func (e *Engine) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS vault_attestations (
		id               INTEGER PRIMARY KEY AUTOINCREMENT,
		vault_id         INTEGER NOT NULL,
		case_id          TEXT    NOT NULL,
		attestation_type TEXT    NOT NULL,
		attestor_address TEXT    NOT NULL,
		fdc_voting_round INTEGER NOT NULL,
		verified_at      TEXT    NOT NULL,
		received_at      TEXT    NOT NULL DEFAULT (datetime('now')),
		UNIQUE(vault_id, attestor_address)
	);

	CREATE TABLE IF NOT EXISTS quorum_results (
		vault_id      INTEGER PRIMARY KEY,
		quorum_met    INTEGER NOT NULL DEFAULT 0,
		signed_result BLOB,
		evaluated_at  TEXT,
		notified      INTEGER NOT NULL DEFAULT 0
	);

	CREATE INDEX IF NOT EXISTS idx_attestations_vault 
		ON vault_attestations(vault_id);
	`
	_, err := e.db.Exec(schema)
	return err
}

// SubmitAttestation records a new verified attestation and evaluates quorum.
// Returns the updated QuorumStatus.
func (e *Engine) SubmitAttestation(ref AttestationRef) (*QuorumStatus, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 1. Retrieve the sealed plan to check thresholds and accepted types
	plan, err := e.store.GetPlan(ref.VaultID)
	if err != nil {
		return nil, fmt.Errorf("cannot evaluate quorum without a sealed plan: %w", err)
	}

	// 2. Validate attestation type against plan's accepted types
	if !isTypeAccepted(ref.AttestationType, plan.AttestationTypes) {
		return nil, fmt.Errorf(
			"attestation type %q not accepted by plan (accepted: %v)",
			ref.AttestationType, plan.AttestationTypes,
		)
	}

	// 3. Store the attestation (dedup by vault_id + attestor_address)
	_, err = e.db.Exec(`
		INSERT INTO vault_attestations 
			(vault_id, case_id, attestation_type, attestor_address, fdc_voting_round, verified_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(vault_id, attestor_address) DO NOTHING
	`,
		ref.VaultID,
		ref.CaseID,
		ref.AttestationType,
		ref.AttestorAddress,
		ref.FdcVotingRound,
		ref.VerifiedAt.Format(time.RFC3339),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to record attestation: %w", err)
	}

	// 4. Evaluate quorum
	return e.evaluateQuorum(ref.VaultID, plan)
}

// GetQuorumStatus returns the current quorum state for a vault.
// This is what the relayer polls via GET /vaults/{id}/quorum-status.
func (e *Engine) GetQuorumStatus(vaultID uint64) (*QuorumStatus, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	plan, err := e.store.GetPlan(vaultID)
	if err != nil {
		// No plan yet — return empty status
		return &QuorumStatus{
			VaultID:        vaultID,
			QuorumMet:      false,
			RequiredCount:  0,
			CurrentCount:   0,
			EnclaveAddress: e.keys.SigningAddress(),
		}, nil
	}

	return e.evaluateQuorum(vaultID, plan)
}

// MarkNotified marks a quorum result as having been submitted on-chain.
func (e *Engine) MarkNotified(vaultID uint64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	_, err := e.db.Exec(`
		UPDATE quorum_results SET notified = 1 WHERE vault_id = ?
	`, vaultID)
	return err
}

// ResetQuorum clears attestation data for a vault (e.g., after guardian halt).
func (e *Engine) ResetQuorum(vaultID uint64) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	tx, err := e.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM vault_attestations WHERE vault_id = ?`, vaultID); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM quorum_results WHERE vault_id = ?`, vaultID); err != nil {
		return err
	}

	return tx.Commit()
}

// ──────────────────────────────────────────────────────────────────────
// Internal helpers
// ──────────────────────────────────────────────────────────────────────

// evaluateQuorum checks whether the attestation count meets the plan's threshold.
// If quorum is newly met, it signs a result for on-chain submission.
func (e *Engine) evaluateQuorum(vaultID uint64, plan *sealedstore.SealedPlan) (*QuorumStatus, error) {
	// Fetch all attestations for this vault
	rows, err := e.db.Query(`
		SELECT case_id, attestation_type, attestor_address, fdc_voting_round, verified_at
		FROM vault_attestations WHERE vault_id = ?
		ORDER BY verified_at ASC
	`, vaultID)
	if err != nil {
		return nil, fmt.Errorf("failed to query attestations: %w", err)
	}
	defer rows.Close()

	var attestations []AttestationRef
	for rows.Next() {
		var ref AttestationRef
		var verifiedAtStr string
		if err := rows.Scan(
			&ref.CaseID,
			&ref.AttestationType,
			&ref.AttestorAddress,
			&ref.FdcVotingRound,
			&verifiedAtStr,
		); err != nil {
			return nil, fmt.Errorf("failed to scan attestation: %w", err)
		}
		ref.VaultID = vaultID
		ref.VerifiedAt, _ = time.Parse(time.RFC3339, verifiedAtStr)
		attestations = append(attestations, ref)
	}

	status := &QuorumStatus{
		VaultID:        vaultID,
		RequiredCount:  plan.QuorumThreshold,
		CurrentCount:   len(attestations),
		QuorumMet:      len(attestations) >= int(plan.QuorumThreshold),
		Attestations:   attestations,
		AcceptedTypes:  plan.AttestationTypes,
		EnclaveAddress: e.keys.SigningAddress(),
	}

	// If quorum is met, sign the result
	if status.QuorumMet {
		now := time.Now().UTC()
		status.EvaluatedAt = &now

		// Check if we already have a signed result
		var existingResult []byte
		err := e.db.QueryRow(`
			SELECT signed_result FROM quorum_results WHERE vault_id = ? AND quorum_met = 1
		`, vaultID).Scan(&existingResult)

		if err == sql.ErrNoRows {
			// First time quorum is met — sign the result
			sig, err := e.keys.SignQuorumResult(vaultID, true, uint64(now.Unix()))
			if err != nil {
				return nil, fmt.Errorf("failed to sign quorum result: %w", err)
			}
			status.SignedResult = sig

			// Persist the result
			_, err = e.db.Exec(`
				INSERT INTO quorum_results (vault_id, quorum_met, signed_result, evaluated_at)
				VALUES (?, 1, ?, ?)
				ON CONFLICT(vault_id) DO UPDATE SET
					quorum_met    = 1,
					signed_result = excluded.signed_result,
					evaluated_at  = excluded.evaluated_at
			`, vaultID, sig, now.Format(time.RFC3339))
			if err != nil {
				return nil, fmt.Errorf("failed to persist quorum result: %w", err)
			}
		} else if err == nil {
			status.SignedResult = existingResult
		} else {
			return nil, fmt.Errorf("failed to check existing result: %w", err)
		}
	}

	return status, nil
}

// isTypeAccepted checks if an attestation type is in the plan's accepted list.
func isTypeAccepted(attType string, accepted []string) bool {
	for _, a := range accepted {
		if a == attType {
			return true
		}
	}
	return false
}

// GetPendingQuorumResults returns vault IDs where quorum is met but not yet
// submitted on-chain. Used by the relayer to poll for ready-to-submit results.
func (e *Engine) GetPendingQuorumResults() ([]uint64, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	rows, err := e.db.Query(`
		SELECT vault_id FROM quorum_results 
		WHERE quorum_met = 1 AND notified = 0
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vaultIDs []uint64
	for rows.Next() {
		var id uint64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		vaultIDs = append(vaultIDs, id)
	}
	return vaultIDs, nil
}

// GetSignedResult returns the signed quorum result for a vault, if available.
func (e *Engine) GetSignedResult(vaultID uint64) ([]byte, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var sig []byte
	err := e.db.QueryRow(`
		SELECT signed_result FROM quorum_results 
		WHERE vault_id = ? AND quorum_met = 1
	`, vaultID).Scan(&sig)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no signed result for vault %d", vaultID)
	}
	if err != nil {
		return nil, err
	}
	return sig, nil
}

// ExportQuorumResultJSON returns a JSON-serializable quorum result for
// the relayer to submit to submitQuorumResult() on-chain.
type QuorumResultPayload struct {
	VaultID      uint64 `json:"vaultId"`
	QuorumMet    bool   `json:"quorumMet"`
	Signature    string `json:"signature"`
	EvaluatedAt  string `json:"evaluatedAt"`
	AttestorCount int   `json:"attestorCount"`
}

// GetQuorumResultPayload returns the payload the relayer needs to submit on-chain.
func (e *Engine) GetQuorumResultPayload(vaultID uint64) (*QuorumResultPayload, error) {
	status, err := e.GetQuorumStatus(vaultID)
	if err != nil {
		return nil, err
	}

	if !status.QuorumMet {
		return nil, fmt.Errorf("quorum not met for vault %d", vaultID)
	}

	sig, err := e.GetSignedResult(vaultID)
	if err != nil {
		return nil, err
	}

	evalAt := ""
	if status.EvaluatedAt != nil {
		evalAt = status.EvaluatedAt.Format(time.RFC3339)
	}

	return &QuorumResultPayload{
		VaultID:       vaultID,
		QuorumMet:     true,
		Signature:     fmt.Sprintf("0x%x", sig),
		EvaluatedAt:   evalAt,
		AttestorCount: status.CurrentCount,
	}, nil
}

// Serialize QuorumResultPayload to JSON for API responses.
func (p *QuorumResultPayload) JSON() ([]byte, error) {
	return json.Marshal(p)
}
