package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	enclavecrypto "github.com/continuity-vault/enclave/internal/crypto"
	"github.com/continuity-vault/enclave/internal/sealedstore"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()

	keys, err := enclavecrypto.NewEnclaveKeys()
	if err != nil {
		t.Fatalf("NewEnclaveKeys() error: %v", err)
	}

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open() error: %v", err)
	}

	logger, _ := zap.NewDevelopment()
	server, err := NewServer(db, keys, Config{
		Port:                8080,
		AttestationEvidence: "test-attestation-token",
		Logger:              logger,
	})
	if err != nil {
		t.Fatalf("NewServer() error: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return server
}

func TestHealthEndpoint(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)

	if resp["status"] != "ok" {
		t.Errorf("status = %v, want ok", resp["status"])
	}
	if resp["service"] != "continuity-vault-enclave" {
		t.Errorf("service = %v, want continuity-vault-enclave", resp["service"])
	}
}

func TestIdentityEndpoint(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("GET", "/identity", nil)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var info enclavecrypto.KeyInfo
	json.NewDecoder(w.Body).Decode(&info)

	if info.SigningAddress == "" {
		t.Error("signingAddress should not be empty")
	}
	if info.AttestationEvidence != "test-attestation-token" {
		t.Errorf("attestationEvidence = %s, want test-attestation-token", info.AttestationEvidence)
	}
}

func storePlanViaAPI(t *testing.T, server *Server, vaultID uint64) {
	t.Helper()

	plan := sealedstore.SealedPlan{
		Beneficiaries: []sealedstore.Beneficiary{
			{Identifier: "rBen1", Label: "Spouse", SplitPercentage: 60},
			{Identifier: "rBen2", Label: "Child", SplitPercentage: 40},
		},
		QuorumThreshold:  2,
		AttestationTypes: []string{"DEATH", "INCAPACITATION"},
	}
	planData, _ := json.Marshal(plan)

	body, _ := json.Marshal(sealedstore.PlanSubmission{
		PlanData:       planData,
		CommitmentHash: "0xabc123def456",
	})

	req := httptest.NewRequest("POST", fmt.Sprintf("/vaults/%d/plan", vaultID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("store plan: status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestStorePlan(t *testing.T) {
	server := setupTestServer(t)
	storePlanViaAPI(t, server, 1)
}

func TestStorePlanInvalidBody(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest("POST", "/vaults/1/plan", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestStorePlanMissingCommitment(t *testing.T) {
	server := setupTestServer(t)

	body, _ := json.Marshal(map[string]interface{}{
		"planData": map[string]interface{}{},
	})
	req := httptest.NewRequest("POST", "/vaults/1/plan", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetPlanMetadata(t *testing.T) {
	server := setupTestServer(t)

	// Before storing
	req := httptest.NewRequest("GET", "/vaults/1/plan", nil)
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var meta sealedstore.PlanMetadata
	json.NewDecoder(w.Body).Decode(&meta)
	if meta.HasPlan {
		t.Error("hasPlan should be false before storing")
	}

	// Store a plan
	storePlanViaAPI(t, server, 1)

	// After storing
	req = httptest.NewRequest("GET", "/vaults/1/plan", nil)
	w = httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	json.NewDecoder(w.Body).Decode(&meta)
	if !meta.HasPlan {
		t.Error("hasPlan should be true after storing")
	}
}

func TestSubmitAttestationAndQuorum(t *testing.T) {
	server := setupTestServer(t)
	storePlanViaAPI(t, server, 1)

	// Submit first attestation
	att1, _ := json.Marshal(map[string]interface{}{
		"caseId":          "case-test-001",
		"attestationType": "DEATH",
		"attestorAddress": "0xAttestor1",
		"fdcVotingRound":  100,
		"verifiedAt":      "2026-08-04T12:00:00Z",
	})
	req := httptest.NewRequest("POST", "/vaults/1/attestations", bytes.NewReader(att1))
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("first attestation: status = %d, body = %s", w.Code, w.Body.String())
	}

	// Check quorum status — should not be met yet
	req = httptest.NewRequest("GET", "/vaults/1/quorum-status", nil)
	w = httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	var status map[string]interface{}
	json.NewDecoder(w.Body).Decode(&status)
	if status["quorumMet"].(bool) {
		t.Error("quorum should not be met with 1/2 attestations")
	}

	// Submit second attestation — quorum should be met
	att2, _ := json.Marshal(map[string]interface{}{
		"caseId":          "case-test-001",
		"attestationType": "DEATH",
		"attestorAddress": "0xAttestor2",
		"fdcVotingRound":  101,
		"verifiedAt":      "2026-08-04T12:05:00Z",
	})
	req = httptest.NewRequest("POST", "/vaults/1/attestations", bytes.NewReader(att2))
	w = httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("second attestation: status = %d, body = %s", w.Code, w.Body.String())
	}

	// Check quorum status — should be met
	req = httptest.NewRequest("GET", "/vaults/1/quorum-status", nil)
	w = httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	json.NewDecoder(w.Body).Decode(&status)
	if !status["quorumMet"].(bool) {
		t.Error("quorum should be met with 2/2 attestations")
	}

	// Check quorum result endpoint
	req = httptest.NewRequest("GET", "/vaults/1/quorum-result", nil)
	w = httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("quorum result: status = %d, want %d", w.Code, http.StatusOK)
	}

	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if result["signature"] == "" {
		t.Error("signature should not be empty")
	}
}

func TestSubmitAttestationInvalidType(t *testing.T) {
	server := setupTestServer(t)
	storePlanViaAPI(t, server, 1)

	att, _ := json.Marshal(map[string]interface{}{
		"caseId":          "case-001",
		"attestationType": "INVALID_TYPE",
		"attestorAddress": "0xAttestor1",
		"fdcVotingRound":  100,
		"verifiedAt":      "2026-08-04T12:00:00Z",
	})
	req := httptest.NewRequest("POST", "/vaults/1/attestations", bytes.NewReader(att))
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestResetQuorum(t *testing.T) {
	server := setupTestServer(t)
	storePlanViaAPI(t, server, 1)

	// Submit attestation
	att, _ := json.Marshal(map[string]interface{}{
		"caseId":          "case-001",
		"attestationType": "DEATH",
		"attestorAddress": "0xAttestor1",
		"fdcVotingRound":  100,
		"verifiedAt":      "2026-08-04T12:00:00Z",
	})
	req := httptest.NewRequest("POST", "/vaults/1/attestations", bytes.NewReader(att))
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	// Reset
	req = httptest.NewRequest("POST", "/vaults/1/reset", nil)
	w = httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("reset: status = %d, want %d", w.Code, http.StatusOK)
	}

	// Check status — should be empty
	req = httptest.NewRequest("GET", "/vaults/1/quorum-status", nil)
	w = httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)

	var status map[string]interface{}
	json.NewDecoder(w.Body).Decode(&status)
	if status["currentCount"].(float64) != 0 {
		t.Errorf("currentCount = %v, want 0 after reset", status["currentCount"])
	}
}

// TestFullLifecycle drives the complete enclave lifecycle:
// submit plan → submit attestations → reach quorum → verify signed result → reset
func TestFullLifecycle(t *testing.T) {
	server := setupTestServer(t)

	// 1. Store plan with quorum threshold = 2
	storePlanViaAPI(t, server, 1)

	// 2. Submit first attestation
	att1, _ := json.Marshal(map[string]interface{}{
		"caseId":          "case-lifecycle",
		"attestationType": "DEATH",
		"attestorAddress": "0xTrustee1",
		"fdcVotingRound":  200,
		"verifiedAt":      "2026-08-04T10:00:00Z",
	})
	req := httptest.NewRequest("POST", "/vaults/1/attestations", bytes.NewReader(att1))
	w := httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("att1: %d %s", w.Code, w.Body.String())
	}

	// 3. Verify quorum not met
	req = httptest.NewRequest("GET", "/vaults/1/quorum-status", nil)
	w = httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)
	var s1 map[string]interface{}
	json.NewDecoder(w.Body).Decode(&s1)
	if s1["quorumMet"].(bool) {
		t.Fatal("quorum should not be met after 1 attestation")
	}

	// 4. Submit second attestation
	att2, _ := json.Marshal(map[string]interface{}{
		"caseId":          "case-lifecycle",
		"attestationType": "INCAPACITATION",
		"attestorAddress": "0xTrustee2",
		"fdcVotingRound":  201,
		"verifiedAt":      "2026-08-04T10:30:00Z",
	})
	req = httptest.NewRequest("POST", "/vaults/1/attestations", bytes.NewReader(att2))
	w = httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("att2: %d %s", w.Code, w.Body.String())
	}

	// 5. Verify quorum IS met
	req = httptest.NewRequest("GET", "/vaults/1/quorum-status", nil)
	w = httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)
	var s2 map[string]interface{}
	json.NewDecoder(w.Body).Decode(&s2)
	if !s2["quorumMet"].(bool) {
		t.Fatal("quorum should be met after 2 attestations")
	}

	// 6. Get signed result
	req = httptest.NewRequest("GET", "/vaults/1/quorum-result", nil)
	w = httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("result: %d %s", w.Code, w.Body.String())
	}
	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if result["signature"].(string) == "" {
		t.Fatal("should have a signature")
	}
	if result["quorumMet"].(bool) != true {
		t.Fatal("result should show quorumMet=true")
	}

	// 7. Reset (simulates guardian halt)
	req = httptest.NewRequest("POST", "/vaults/1/reset", nil)
	w = httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("reset: %d", w.Code)
	}

	// 8. Verify quorum is reset
	req = httptest.NewRequest("GET", "/vaults/1/quorum-status", nil)
	w = httptest.NewRecorder()
	server.Router().ServeHTTP(w, req)
	var s3 map[string]interface{}
	json.NewDecoder(w.Body).Decode(&s3)
	if s3["quorumMet"].(bool) {
		t.Fatal("quorum should not be met after reset")
	}

	t.Log("✓ Full lifecycle passed: plan → attestations → quorum → signed result → reset")
}
