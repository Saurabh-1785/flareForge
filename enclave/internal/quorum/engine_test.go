package quorum

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	enclavecrypto "github.com/continuity-vault/enclave/internal/crypto"
	"github.com/continuity-vault/enclave/internal/sealedstore"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestEngine(t *testing.T) (*Engine, *sealedstore.Store, *enclavecrypto.EnclaveKeys) {
	t.Helper()

	keys, err := enclavecrypto.NewEnclaveKeys()
	if err != nil {
		t.Fatalf("NewEnclaveKeys() error: %v", err)
	}

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error: %v", err)
	}

	store, err := sealedstore.NewStore(db, keys)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	engine, err := NewEngine(db, store, keys)
	if err != nil {
		t.Fatalf("NewEngine() error: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return engine, store, keys
}

func seedPlan(t *testing.T, store *sealedstore.Store, vaultID uint64, threshold uint8) {
	t.Helper()

	plan := sealedstore.SealedPlan{
		Beneficiaries: []sealedstore.Beneficiary{
			{Identifier: "rBeneficiary1", SplitPercentage: 60},
			{Identifier: "rBeneficiary2", SplitPercentage: 40},
		},
		QuorumThreshold:  threshold,
		AttestationTypes: []string{"DEATH", "INCAPACITATION"},
	}
	data, _ := json.Marshal(plan)

	err := store.StorePlan(vaultID, sealedstore.PlanSubmission{
		PlanData:       data,
		CommitmentHash: "0xtest",
	})
	if err != nil {
		t.Fatalf("StorePlan() error: %v", err)
	}
}

func TestSubmitAttestation_SingleAttestation(t *testing.T) {
	engine, store, _ := setupTestEngine(t)
	seedPlan(t, store, 1, 2)

	status, err := engine.SubmitAttestation(AttestationRef{
		VaultID:         1,
		CaseID:          "case-001",
		AttestationType: "DEATH",
		AttestorAddress: "0xAttestor1",
		FdcVotingRound:  100,
		VerifiedAt:      time.Now(),
	})
	if err != nil {
		t.Fatalf("SubmitAttestation() error: %v", err)
	}

	if status.QuorumMet {
		t.Error("quorum should not be met with 1 of 2 attestations")
	}
	if status.CurrentCount != 1 {
		t.Errorf("CurrentCount = %d, want 1", status.CurrentCount)
	}
	if status.RequiredCount != 2 {
		t.Errorf("RequiredCount = %d, want 2", status.RequiredCount)
	}
}

func TestSubmitAttestation_QuorumMet(t *testing.T) {
	engine, store, _ := setupTestEngine(t)
	seedPlan(t, store, 1, 2)

	// First attestation
	_, err := engine.SubmitAttestation(AttestationRef{
		VaultID:         1,
		CaseID:          "case-001",
		AttestationType: "DEATH",
		AttestorAddress: "0xAttestor1",
		FdcVotingRound:  100,
		VerifiedAt:      time.Now(),
	})
	if err != nil {
		t.Fatalf("first attestation error: %v", err)
	}

	// Second attestation — quorum should be met
	status, err := engine.SubmitAttestation(AttestationRef{
		VaultID:         1,
		CaseID:          "case-001",
		AttestationType: "DEATH",
		AttestorAddress: "0xAttestor2",
		FdcVotingRound:  101,
		VerifiedAt:      time.Now(),
	})
	if err != nil {
		t.Fatalf("second attestation error: %v", err)
	}

	if !status.QuorumMet {
		t.Error("quorum should be met with 2 of 2 attestations")
	}
	if status.CurrentCount != 2 {
		t.Errorf("CurrentCount = %d, want 2", status.CurrentCount)
	}
	if len(status.SignedResult) == 0 {
		t.Error("SignedResult should not be empty when quorum is met")
	}
	if status.EvaluatedAt == nil {
		t.Error("EvaluatedAt should be set when quorum is met")
	}
}

func TestSubmitAttestation_DuplicateAttestor(t *testing.T) {
	engine, store, _ := setupTestEngine(t)
	seedPlan(t, store, 1, 2)

	// First attestation
	_, err := engine.SubmitAttestation(AttestationRef{
		VaultID:         1,
		CaseID:          "case-001",
		AttestationType: "DEATH",
		AttestorAddress: "0xAttestor1",
		FdcVotingRound:  100,
		VerifiedAt:      time.Now(),
	})
	if err != nil {
		t.Fatalf("first attestation error: %v", err)
	}

	// Same attestor again — should be deduped
	status, err := engine.SubmitAttestation(AttestationRef{
		VaultID:         1,
		CaseID:          "case-001",
		AttestationType: "DEATH",
		AttestorAddress: "0xAttestor1",
		FdcVotingRound:  102,
		VerifiedAt:      time.Now(),
	})
	if err != nil {
		t.Fatalf("duplicate attestation error: %v", err)
	}

	// Count should still be 1 (deduped)
	if status.CurrentCount != 1 {
		t.Errorf("CurrentCount = %d, want 1 (dedup)", status.CurrentCount)
	}
}

func TestSubmitAttestation_InvalidType(t *testing.T) {
	engine, store, _ := setupTestEngine(t)
	seedPlan(t, store, 1, 2)

	_, err := engine.SubmitAttestation(AttestationRef{
		VaultID:         1,
		CaseID:          "case-001",
		AttestationType: "ALIEN_ABDUCTION",
		AttestorAddress: "0xAttestor1",
		FdcVotingRound:  100,
		VerifiedAt:      time.Now(),
	})
	if err == nil {
		t.Error("should reject invalid attestation type")
	}
}

func TestSubmitAttestation_NoPlan(t *testing.T) {
	engine, _, _ := setupTestEngine(t)

	_, err := engine.SubmitAttestation(AttestationRef{
		VaultID:         999,
		CaseID:          "case-999",
		AttestationType: "DEATH",
		AttestorAddress: "0xAttestor1",
		FdcVotingRound:  100,
		VerifiedAt:      time.Now(),
	})
	if err == nil {
		t.Error("should fail when no plan exists")
	}
}

func TestGetQuorumStatus(t *testing.T) {
	engine, store, _ := setupTestEngine(t)
	seedPlan(t, store, 1, 2)

	// Before any attestations
	status, err := engine.GetQuorumStatus(1)
	if err != nil {
		t.Fatalf("GetQuorumStatus() error: %v", err)
	}
	if status.QuorumMet {
		t.Error("quorum should not be met initially")
	}

	// Submit two attestations
	engine.SubmitAttestation(AttestationRef{
		VaultID: 1, CaseID: "case-001", AttestationType: "DEATH",
		AttestorAddress: "0xA1", FdcVotingRound: 100, VerifiedAt: time.Now(),
	})
	engine.SubmitAttestation(AttestationRef{
		VaultID: 1, CaseID: "case-001", AttestationType: "DEATH",
		AttestorAddress: "0xA2", FdcVotingRound: 101, VerifiedAt: time.Now(),
	})

	status, err = engine.GetQuorumStatus(1)
	if err != nil {
		t.Fatalf("GetQuorumStatus() after attestations error: %v", err)
	}
	if !status.QuorumMet {
		t.Error("quorum should be met after 2 attestations")
	}
}

func TestResetQuorum(t *testing.T) {
	engine, store, _ := setupTestEngine(t)
	seedPlan(t, store, 1, 2)

	// Submit attestations and reach quorum
	engine.SubmitAttestation(AttestationRef{
		VaultID: 1, CaseID: "case-001", AttestationType: "DEATH",
		AttestorAddress: "0xA1", FdcVotingRound: 100, VerifiedAt: time.Now(),
	})
	engine.SubmitAttestation(AttestationRef{
		VaultID: 1, CaseID: "case-001", AttestationType: "DEATH",
		AttestorAddress: "0xA2", FdcVotingRound: 101, VerifiedAt: time.Now(),
	})

	// Reset (simulates guardian halt)
	err := engine.ResetQuorum(1)
	if err != nil {
		t.Fatalf("ResetQuorum() error: %v", err)
	}

	// Quorum should no longer be met
	status, err := engine.GetQuorumStatus(1)
	if err != nil {
		t.Fatalf("GetQuorumStatus() after reset error: %v", err)
	}
	if status.QuorumMet {
		t.Error("quorum should not be met after reset")
	}
	if status.CurrentCount != 0 {
		t.Errorf("CurrentCount = %d, want 0 after reset", status.CurrentCount)
	}
}

func TestGetPendingQuorumResults(t *testing.T) {
	engine, store, _ := setupTestEngine(t)
	seedPlan(t, store, 1, 1)

	// Submit attestation (quorum = 1, so this meets it)
	engine.SubmitAttestation(AttestationRef{
		VaultID: 1, CaseID: "case-001", AttestationType: "DEATH",
		AttestorAddress: "0xA1", FdcVotingRound: 100, VerifiedAt: time.Now(),
	})

	pending, err := engine.GetPendingQuorumResults()
	if err != nil {
		t.Fatalf("GetPendingQuorumResults() error: %v", err)
	}
	if len(pending) != 1 || pending[0] != 1 {
		t.Errorf("pending = %v, want [1]", pending)
	}

	// Mark as notified
	if err := engine.MarkNotified(1); err != nil {
		t.Fatalf("MarkNotified() error: %v", err)
	}

	pending, err = engine.GetPendingQuorumResults()
	if err != nil {
		t.Fatalf("GetPendingQuorumResults() after notify error: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("pending should be empty after MarkNotified, got %v", pending)
	}
}

func TestGetQuorumResultPayload(t *testing.T) {
	engine, store, _ := setupTestEngine(t)
	seedPlan(t, store, 1, 1)

	engine.SubmitAttestation(AttestationRef{
		VaultID: 1, CaseID: "case-001", AttestationType: "DEATH",
		AttestorAddress: "0xA1", FdcVotingRound: 100, VerifiedAt: time.Now(),
	})

	payload, err := engine.GetQuorumResultPayload(1)
	if err != nil {
		t.Fatalf("GetQuorumResultPayload() error: %v", err)
	}
	if !payload.QuorumMet {
		t.Error("payload.QuorumMet should be true")
	}
	if payload.Signature == "" {
		t.Error("payload.Signature should not be empty")
	}
	if payload.AttestorCount != 1 {
		t.Errorf("AttestorCount = %d, want 1", payload.AttestorCount)
	}
}

func TestMultiVaultIsolation(t *testing.T) {
	engine, store, _ := setupTestEngine(t)
	seedPlan(t, store, 1, 2)
	seedPlan(t, store, 2, 1)

	// Attestation for vault 1
	engine.SubmitAttestation(AttestationRef{
		VaultID: 1, CaseID: "case-001", AttestationType: "DEATH",
		AttestorAddress: "0xA1", FdcVotingRound: 100, VerifiedAt: time.Now(),
	})

	// Attestation for vault 2
	engine.SubmitAttestation(AttestationRef{
		VaultID: 2, CaseID: "case-002", AttestationType: "INCAPACITATION",
		AttestorAddress: "0xA1", FdcVotingRound: 101, VerifiedAt: time.Now(),
	})

	// Vault 1: 1 of 2, not met
	status1, _ := engine.GetQuorumStatus(1)
	if status1.QuorumMet {
		t.Error("vault 1 quorum should not be met (1/2)")
	}

	// Vault 2: 1 of 1, met
	status2, _ := engine.GetQuorumStatus(2)
	if !status2.QuorumMet {
		t.Error("vault 2 quorum should be met (1/1)")
	}
}
