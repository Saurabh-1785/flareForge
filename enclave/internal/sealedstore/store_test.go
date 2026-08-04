package sealedstore

import (
	"database/sql"
	"encoding/json"
	"testing"

	enclavecrypto "github.com/continuity-vault/enclave/internal/crypto"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestStore(t *testing.T) (*Store, *enclavecrypto.EnclaveKeys) {
	t.Helper()

	keys, err := enclavecrypto.NewEnclaveKeys()
	if err != nil {
		t.Fatalf("NewEnclaveKeys() error: %v", err)
	}

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error: %v", err)
	}

	store, err := NewStore(db, keys)
	if err != nil {
		t.Fatalf("NewStore() error: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return store, keys
}

func testPlanData() json.RawMessage {
	plan := SealedPlan{
		Beneficiaries: []Beneficiary{
			{Identifier: "rBeneficiary1XRP", Label: "Spouse", SplitPercentage: 60},
			{Identifier: "rBeneficiary2XRP", Label: "Child", SplitPercentage: 40},
		},
		QuorumThreshold:  2,
		AttestationTypes: []string{"DEATH", "INCAPACITATION"},
	}
	data, _ := json.Marshal(plan)
	return data
}

func TestStorePlan(t *testing.T) {
	store, _ := setupTestStore(t)

	sub := PlanSubmission{
		PlanData:       testPlanData(),
		CommitmentHash: "0xabc123",
	}

	err := store.StorePlan(1, sub)
	if err != nil {
		t.Fatalf("StorePlan() error: %v", err)
	}

	// Verify we can retrieve it
	plan, err := store.GetPlan(1)
	if err != nil {
		t.Fatalf("GetPlan() error: %v", err)
	}

	if plan.VaultID != 1 {
		t.Errorf("VaultID = %d, want 1", plan.VaultID)
	}
	if plan.CommitmentHash != "0xabc123" {
		t.Errorf("CommitmentHash = %s, want 0xabc123", plan.CommitmentHash)
	}
	if len(plan.Beneficiaries) != 2 {
		t.Errorf("len(Beneficiaries) = %d, want 2", len(plan.Beneficiaries))
	}
	if plan.Beneficiaries[0].SplitPercentage != 60 {
		t.Errorf("Beneficiaries[0].SplitPercentage = %d, want 60", plan.Beneficiaries[0].SplitPercentage)
	}
	if plan.QuorumThreshold != 2 {
		t.Errorf("QuorumThreshold = %d, want 2", plan.QuorumThreshold)
	}
}

func TestStorePlanInvalidSplit(t *testing.T) {
	store, _ := setupTestStore(t)

	badPlan := SealedPlan{
		Beneficiaries: []Beneficiary{
			{Identifier: "rAddr1", SplitPercentage: 50},
			{Identifier: "rAddr2", SplitPercentage: 30}, // Total: 80, not 100
		},
		QuorumThreshold: 2,
	}
	data, _ := json.Marshal(badPlan)

	sub := PlanSubmission{
		PlanData:       data,
		CommitmentHash: "0xbad",
	}

	err := store.StorePlan(2, sub)
	if err == nil {
		t.Error("StorePlan() should fail when splits don't sum to 100")
	}
}

func TestStorePlanZeroQuorum(t *testing.T) {
	store, _ := setupTestStore(t)

	badPlan := SealedPlan{
		Beneficiaries: []Beneficiary{
			{Identifier: "rAddr1", SplitPercentage: 100},
		},
		QuorumThreshold: 0,
	}
	data, _ := json.Marshal(badPlan)

	sub := PlanSubmission{
		PlanData:       data,
		CommitmentHash: "0xbad",
	}

	err := store.StorePlan(3, sub)
	if err == nil {
		t.Error("StorePlan() should fail when quorum threshold is 0")
	}
}

func TestGetPlanNotFound(t *testing.T) {
	store, _ := setupTestStore(t)

	_, err := store.GetPlan(999)
	if err == nil {
		t.Error("GetPlan() should fail for non-existent vault")
	}
}

func TestGetPlanMetadata(t *testing.T) {
	store, _ := setupTestStore(t)

	// Before storing
	meta, err := store.GetPlanMetadata(1)
	if err != nil {
		t.Fatalf("GetPlanMetadata() error: %v", err)
	}
	if meta.HasPlan {
		t.Error("HasPlan should be false before storing")
	}

	// Store a plan
	sub := PlanSubmission{
		PlanData:       testPlanData(),
		CommitmentHash: "0xmeta123",
	}
	if err := store.StorePlan(1, sub); err != nil {
		t.Fatalf("StorePlan() error: %v", err)
	}

	// After storing
	meta, err = store.GetPlanMetadata(1)
	if err != nil {
		t.Fatalf("GetPlanMetadata() error: %v", err)
	}
	if !meta.HasPlan {
		t.Error("HasPlan should be true after storing")
	}
	if meta.CommitmentHash != "0xmeta123" {
		t.Errorf("CommitmentHash = %s, want 0xmeta123", meta.CommitmentHash)
	}
}

func TestDeletePlan(t *testing.T) {
	store, _ := setupTestStore(t)

	sub := PlanSubmission{
		PlanData:       testPlanData(),
		CommitmentHash: "0xdel",
	}
	if err := store.StorePlan(1, sub); err != nil {
		t.Fatalf("StorePlan() error: %v", err)
	}

	if err := store.DeletePlan(1); err != nil {
		t.Fatalf("DeletePlan() error: %v", err)
	}

	_, err := store.GetPlan(1)
	if err == nil {
		t.Error("GetPlan() should fail after deletion")
	}
}

func TestStorePlanUpsert(t *testing.T) {
	store, _ := setupTestStore(t)

	sub1 := PlanSubmission{
		PlanData:       testPlanData(),
		CommitmentHash: "0xv1",
	}
	if err := store.StorePlan(1, sub1); err != nil {
		t.Fatalf("StorePlan() v1 error: %v", err)
	}

	// Update with a new plan
	updatedPlan := SealedPlan{
		Beneficiaries: []Beneficiary{
			{Identifier: "rNewAddr", SplitPercentage: 100},
		},
		QuorumThreshold: 1,
	}
	updatedData, _ := json.Marshal(updatedPlan)
	sub2 := PlanSubmission{
		PlanData:       updatedData,
		CommitmentHash: "0xv2",
	}
	if err := store.StorePlan(1, sub2); err != nil {
		t.Fatalf("StorePlan() v2 error: %v", err)
	}

	plan, err := store.GetPlan(1)
	if err != nil {
		t.Fatalf("GetPlan() error: %v", err)
	}
	if plan.CommitmentHash != "0xv2" {
		t.Errorf("CommitmentHash = %s, want 0xv2", plan.CommitmentHash)
	}
	if len(plan.Beneficiaries) != 1 {
		t.Errorf("len(Beneficiaries) = %d, want 1", len(plan.Beneficiaries))
	}
}
