// Package store tests use SQLite as a stand-in for Postgres since the
// schema is designed to be compatible with both. In CI, you would also
// run against a real Postgres instance.
//
// Note: The store uses $1, $2, ... positional parameters which are Postgres
// syntax. For pure SQLite testing, see store_sqlite_test.go which uses
// a thin compatibility wrapper. For production and integration tests,
// use a real Postgres instance.
package store

import (
	"testing"
	"time"
)

// TestVaultStateString verifies state name mapping.
func TestVaultStateString(t *testing.T) {
	tests := []struct {
		state VaultState
		want  string
	}{
		{StateActive, "ACTIVE"},
		{StateWarning, "WARNING"},
		{StateQuorumPending, "QUORUM_PENDING"},
		{StateDisputeWindow, "DISPUTE_WINDOW"},
		{StateSlashingReview, "SLASHING_REVIEW"},
		{StateTranche1Released, "TRANCHE_1_RELEASED"},
		{StateFinalWindow, "FINAL_WINDOW"},
		{StateFullyReleased, "FULLY_RELEASED"},
		{StateClosed, "CLOSED"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("VaultState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

// TestTrackedVaultFields verifies struct field assignments.
func TestTrackedVaultFields(t *testing.T) {
	now := time.Now().UTC()
	v := TrackedVault{
		VaultID:         1,
		Owner:           "0xOwner",
		State:           StateActive,
		WindowDeadline:  now.Add(1 * time.Hour),
		CheckInInterval: 3600,
		GraceWindow:     600,
		DisputeWindow:   300,
		FinalWindow:     300,
		LastCheckIn:     now,
	}

	if v.VaultID != 1 {
		t.Errorf("VaultID = %d, want 1", v.VaultID)
	}
	if v.State != StateActive {
		t.Errorf("State = %v, want ACTIVE", v.State)
	}
	if v.CheckInInterval != 3600 {
		t.Errorf("CheckInInterval = %d, want 3600", v.CheckInInterval)
	}
}

// TestNotificationLogFields verifies notification log struct.
func TestNotificationLogFields(t *testing.T) {
	now := time.Now().UTC()
	log := NotificationLog{
		VaultID:   1,
		EventType: "CHECK_IN_REMINDER",
		Recipient: "0xOwner",
		Channel:   "console",
		Message:   "Check in now!",
		SentAt:    now,
	}

	if log.VaultID != 1 {
		t.Errorf("VaultID = %d, want 1", log.VaultID)
	}
	if log.EventType != "CHECK_IN_REMINDER" {
		t.Errorf("EventType = %s, want CHECK_IN_REMINDER", log.EventType)
	}
	if log.Channel != "console" {
		t.Errorf("Channel = %s, want console", log.Channel)
	}
}
