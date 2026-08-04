package chain

import (
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

// TestABIParsing verifies the VaultRegistry ABI parses correctly.
func TestABIParsing(t *testing.T) {
	parsedABI, err := abi.JSON(strings.NewReader(VaultRegistryABI))
	if err != nil {
		t.Fatalf("ABI parse error: %v", err)
	}

	// Verify events
	events := []string{
		"VaultCreated", "CheckIn", "StateTransition",
		"AttestationRequested", "QuorumResultSubmitted",
		"GuardianHalt", "VaultFullyReleased", "VaultCancelled",
	}
	for _, name := range events {
		if _, ok := parsedABI.Events[name]; !ok {
			t.Errorf("missing event: %s", name)
		}
	}

	// Verify functions
	functions := []string{
		"markWarning", "requestAttestation", "submitQuorumResult",
		"finalizeDisputeWindow", "finalizeFinalWindow",
		"isCheckInMissed", "getVaultState", "getVaultOwner",
		"getVaultTiming", "nextVaultId",
	}
	for _, name := range functions {
		if _, ok := parsedABI.Methods[name]; !ok {
			t.Errorf("missing function: %s", name)
		}
	}
}

// TestPackMarkWarning verifies we can pack the markWarning calldata.
func TestPackMarkWarning(t *testing.T) {
	parsedABI, _ := abi.JSON(strings.NewReader(VaultRegistryABI))

	data, err := parsedABI.Pack("markWarning", big_one())
	if err != nil {
		t.Fatalf("Pack markWarning: %v", err)
	}

	// Should have 4-byte selector + 32-byte uint256
	if len(data) != 36 {
		t.Errorf("packed length = %d, want 36", len(data))
	}
}

// TestPackSubmitQuorumResult verifies we can pack submitQuorumResult.
func TestPackSubmitQuorumResult(t *testing.T) {
	parsedABI, _ := abi.JSON(strings.NewReader(VaultRegistryABI))

	sig := []byte{0x01, 0x02, 0x03}
	data, err := parsedABI.Pack("submitQuorumResult", big_one(), true, sig)
	if err != nil {
		t.Fatalf("Pack submitQuorumResult: %v", err)
	}

	// 4-byte selector + 32 (uint256) + 32 (bool) + 32 (offset) + 32 (length) + 32 (data padded)
	if len(data) < 100 {
		t.Errorf("packed length = %d, seems too short", len(data))
	}
}

// TestPackGetVaultTiming verifies we can pack the getVaultTiming call.
func TestPackGetVaultTiming(t *testing.T) {
	parsedABI, _ := abi.JSON(strings.NewReader(VaultRegistryABI))

	data, err := parsedABI.Pack("getVaultTiming", big_one())
	if err != nil {
		t.Fatalf("Pack getVaultTiming: %v", err)
	}

	if len(data) != 36 {
		t.Errorf("packed length = %d, want 36", len(data))
	}
}

// TestVaultTimingStruct verifies VaultTiming fields.
func TestVaultTimingStruct(t *testing.T) {
	vt := VaultTiming{
		LastCheckIn:     1000,
		WindowDeadline:  2000,
		CheckInInterval: 3600,
		GraceWindow:     600,
		DisputeWindow:   300,
		FinalWindow:     300,
	}

	if vt.CheckInInterval != 3600 {
		t.Errorf("CheckInInterval = %d, want 3600", vt.CheckInInterval)
	}
}

// helper to create big.Int(1)
func big_one() *big.Int {
	return new(big.Int).SetUint64(1)
}
