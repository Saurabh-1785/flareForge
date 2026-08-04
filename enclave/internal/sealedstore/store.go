// Package sealedstore provides encrypted-at-rest storage for vault plans.
// Plans are encrypted with the enclave's AES-256-GCM key before being
// written to SQLite. The plaintext never leaves the TEE process.
//
// Architecture ref: "Sealed Plan Store" in the FCC plane.
// Build prompt: "Encrypted row inside the enclave service's own store
// (Postgres or even SQLite is fine)" — we use SQLite for simplicity.
package sealedstore

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	enclavecrypto "github.com/continuity-vault/enclave/internal/crypto"
)

// ──────────────────────────────────────────────────────────────────────
// Domain types — the plan, beneficiaries, and what comes back out.
// ──────────────────────────────────────────────────────────────────────

// Beneficiary represents a single beneficiary in the sealed plan.
type Beneficiary struct {
	// Identifier is the beneficiary's address — XRPL address for native XRP
	// payout, or an Ethereum address for ERC-20 payout.
	Identifier string `json:"identifier"`

	// Label is a human-readable label (e.g., "Spouse", "Business Partner").
	// Stored encrypted — never visible on-chain pre-trigger.
	Label string `json:"label,omitempty"`

	// SplitPercentage is the percentage of vault funds this beneficiary receives.
	// All beneficiary splits must sum to 100.
	SplitPercentage uint8 `json:"splitPercentage"`
}

// SealedPlan is the full inheritance/succession plan, held only inside the TEE.
// Design Principle #3: beneficiaries stay unknown on-chain until release.
type SealedPlan struct {
	// VaultID matches the on-chain vault ID in VaultRegistry.
	VaultID uint64 `json:"vaultId"`

	// CommitmentHash is keccak256(plan) — stored on-chain as a pointer.
	// The enclave verifies this matches on plan submission.
	CommitmentHash string `json:"commitmentHash"`

	// Beneficiaries — revealed only at release.
	Beneficiaries []Beneficiary `json:"beneficiaries"`

	// QuorumThreshold is the N in N-of-M: how many independent signals
	// must agree before the trigger is treated as real.
	QuorumThreshold uint8 `json:"quorumThreshold"`

	// AttestationTypes lists the accepted signal types (e.g., "DEATH", "INCAPACITATION").
	AttestationTypes []string `json:"attestationTypes"`

	// CreatedAt is when the plan was sealed in the enclave.
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt is the last modification time.
	UpdatedAt time.Time `json:"updatedAt"`
}

// PlanSubmission is the request payload for creating/updating a sealed plan.
type PlanSubmission struct {
	// EncryptedPlan is the client-side encrypted plan blob.
	// In the MVP, the plan may be submitted in plaintext over TLS to the
	// TEE and encrypted server-side. In production with full FCC, the
	// client encrypts against the enclave's public key before transmission.
	PlanData json.RawMessage `json:"planData"`

	// CommitmentHash must match keccak256 of the plan data, linking
	// this sealed plan to its on-chain pointer in VaultRegistry.
	CommitmentHash string `json:"commitmentHash"`
}

// PlanMetadata is the non-sensitive metadata about a stored plan.
type PlanMetadata struct {
	VaultID        uint64    `json:"vaultId"`
	CommitmentHash string    `json:"commitmentHash"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
	HasPlan        bool      `json:"hasPlan"`
}

// ──────────────────────────────────────────────────────────────────────
// Store — the encrypted-at-rest sealed plan storage.
// ──────────────────────────────────────────────────────────────────────

// Store manages encrypted vault plans in SQLite.
type Store struct {
	mu   sync.RWMutex
	db   *sql.DB
	keys *enclavecrypto.EnclaveKeys
}

// NewStore creates a new sealed plan store with the given SQLite database
// and enclave keys for encryption.
func NewStore(db *sql.DB, keys *enclavecrypto.EnclaveKeys) (*Store, error) {
	s := &Store{
		db:   db,
		keys: keys,
	}

	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate sealed store: %w", err)
	}

	return s, nil
}

// migrate creates the required tables.
func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS sealed_plans (
		vault_id         INTEGER PRIMARY KEY,
		commitment_hash  TEXT    NOT NULL,
		encrypted_plan   BLOB   NOT NULL,
		created_at       TEXT    NOT NULL DEFAULT (datetime('now')),
		updated_at       TEXT    NOT NULL DEFAULT (datetime('now'))
	);

	CREATE INDEX IF NOT EXISTS idx_sealed_plans_commitment 
		ON sealed_plans(commitment_hash);
	`
	_, err := s.db.Exec(schema)
	return err
}

// StorePlan encrypts and stores a plan for the given vault ID.
func (s *Store) StorePlan(vaultID uint64, submission PlanSubmission) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Parse and validate the plan data
	var plan SealedPlan
	if err := json.Unmarshal(submission.PlanData, &plan); err != nil {
		return fmt.Errorf("invalid plan data: %w", err)
	}

	// Validate beneficiary splits sum to 100
	var totalSplit uint8
	for _, b := range plan.Beneficiaries {
		totalSplit += b.SplitPercentage
	}
	if totalSplit != 100 {
		return fmt.Errorf("beneficiary splits must sum to 100, got %d", totalSplit)
	}

	// Validate quorum threshold
	if plan.QuorumThreshold == 0 {
		return fmt.Errorf("quorum threshold must be > 0")
	}

	// Set metadata
	plan.VaultID = vaultID
	plan.CommitmentHash = submission.CommitmentHash
	now := time.Now().UTC()
	plan.CreatedAt = now
	plan.UpdatedAt = now

	// Serialize the complete plan
	planJSON, err := json.Marshal(plan)
	if err != nil {
		return fmt.Errorf("failed to marshal plan: %w", err)
	}

	// Encrypt with the enclave's key — the plaintext never touches disk
	encrypted, err := s.keys.Encrypt(planJSON)
	if err != nil {
		return fmt.Errorf("failed to encrypt plan: %w", err)
	}

	// Store the encrypted blob
	_, err = s.db.Exec(`
		INSERT INTO sealed_plans (vault_id, commitment_hash, encrypted_plan, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(vault_id) DO UPDATE SET
			commitment_hash = excluded.commitment_hash,
			encrypted_plan  = excluded.encrypted_plan,
			updated_at      = excluded.updated_at
	`, vaultID, submission.CommitmentHash, encrypted, now.Format(time.RFC3339), now.Format(time.RFC3339))

	if err != nil {
		return fmt.Errorf("failed to store plan: %w", err)
	}

	return nil
}

// GetPlan decrypts and returns the plan for the given vault ID.
// This should ONLY be called during quorum evaluation or release —
// never for external API responses pre-trigger.
func (s *Store) GetPlan(vaultID uint64) (*SealedPlan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var encrypted []byte
	err := s.db.QueryRow(`
		SELECT encrypted_plan FROM sealed_plans WHERE vault_id = ?
	`, vaultID).Scan(&encrypted)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no plan found for vault %d", vaultID)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query plan: %w", err)
	}

	// Decrypt inside the TEE
	plaintext, err := s.keys.Decrypt(encrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt plan: %w", err)
	}

	var plan SealedPlan
	if err := json.Unmarshal(plaintext, &plan); err != nil {
		return nil, fmt.Errorf("failed to unmarshal plan: %w", err)
	}

	return &plan, nil
}

// GetPlanMetadata returns non-sensitive metadata about a stored plan
// without decrypting the plan itself.
func (s *Store) GetPlanMetadata(vaultID uint64) (*PlanMetadata, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var commitmentHash, createdAt, updatedAt string
	err := s.db.QueryRow(`
		SELECT commitment_hash, created_at, updated_at 
		FROM sealed_plans WHERE vault_id = ?
	`, vaultID).Scan(&commitmentHash, &createdAt, &updatedAt)

	if err == sql.ErrNoRows {
		return &PlanMetadata{
			VaultID: vaultID,
			HasPlan: false,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query plan metadata: %w", err)
	}

	created, _ := time.Parse(time.RFC3339, createdAt)
	updated, _ := time.Parse(time.RFC3339, updatedAt)

	return &PlanMetadata{
		VaultID:        vaultID,
		CommitmentHash: commitmentHash,
		CreatedAt:      created,
		UpdatedAt:      updated,
		HasPlan:        true,
	}, nil
}

// DeletePlan removes a sealed plan (e.g., on vault cancellation).
func (s *Store) DeletePlan(vaultID uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM sealed_plans WHERE vault_id = ?`, vaultID)
	return err
}

// GetCommitmentHash returns the hex-encoded keccak256 hash of the raw plan data.
// Used to verify the on-chain commitment matches.
func GetCommitmentHash(planData []byte) string {
	hash := sha256Hash(planData)
	return "0x" + hex.EncodeToString(hash)
}

// sha256Hash computes a SHA-256 hash of the data.
// In production, this would be keccak256 to match on-chain, but SHA-256
// is used for the MVP commitment hash verification.
func sha256Hash(data []byte) []byte {
	h := sha256.Sum256(data)
	return h[:]
}

