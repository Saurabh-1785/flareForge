package notifier

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestConsoleNotifierSend(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	n := NewConsoleNotifier(logger)

	err := n.Send(Notification{
		VaultID:   1,
		Type:      NotifyCheckInReminder,
		Recipient: "0xOwner",
		Subject:   "Test Subject",
		Message:   "Test Message",
		SentAt:    time.Now(),
		Channel:   "console",
	})

	if err != nil {
		t.Errorf("Send() error: %v", err)
	}
}

func TestConsoleNotifierName(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	n := NewConsoleNotifier(logger)
	if n.Name() != "console" {
		t.Errorf("Name() = %q, want console", n.Name())
	}
}

func TestCheckInReminderMessage(t *testing.T) {
	deadline := time.Now().Add(30 * time.Second)
	msg := CheckInReminderMessage(1, "0xOwner", deadline)

	if msg.VaultID != 1 {
		t.Errorf("VaultID = %d, want 1", msg.VaultID)
	}
	if msg.Type != NotifyCheckInReminder {
		t.Errorf("Type = %s, want %s", msg.Type, NotifyCheckInReminder)
	}
	if msg.Recipient != "0xOwner" {
		t.Errorf("Recipient = %s, want 0xOwner", msg.Recipient)
	}
	if msg.Subject == "" {
		t.Error("Subject should not be empty")
	}
	if msg.Message == "" {
		t.Error("Message should not be empty")
	}
}

func TestCheckInMissedMessage(t *testing.T) {
	msg := CheckInMissedMessage(2, "0xOwner")
	if msg.Type != NotifyCheckInMissed {
		t.Errorf("Type = %s, want %s", msg.Type, NotifyCheckInMissed)
	}
	if msg.VaultID != 2 {
		t.Errorf("VaultID = %d, want 2", msg.VaultID)
	}
}

func TestGraceExpiredMessage(t *testing.T) {
	msg := GraceExpiredMessage(3, "0xOwner")
	if msg.Type != NotifyGraceExpired {
		t.Errorf("Type = %s, want %s", msg.Type, NotifyGraceExpired)
	}
}

func TestDisputeWindowMessage(t *testing.T) {
	deadline := time.Now().Add(5 * time.Minute)
	msg := DisputeWindowMessage(4, "0xOwner", deadline)
	if msg.Type != NotifyDisputeWindowOpen {
		t.Errorf("Type = %s, want %s", msg.Type, NotifyDisputeWindowOpen)
	}
}

func TestFinalOverrideMessage(t *testing.T) {
	deadline := time.Now().Add(5 * time.Minute)
	msg := FinalOverrideMessage(5, "0xOwner", deadline)
	if msg.Type != NotifyFinalOverride {
		t.Errorf("Type = %s, want %s", msg.Type, NotifyFinalOverride)
	}
}

func TestGuardianHaltMessage(t *testing.T) {
	msg := GuardianHaltMessage(6, "0xOwner")
	if msg.Type != NotifyGuardianHalt {
		t.Errorf("Type = %s, want %s", msg.Type, NotifyGuardianHalt)
	}
}

func TestFullyReleasedMessage(t *testing.T) {
	msg := FullyReleasedMessage(7, "0xOwner")
	if msg.Type != NotifyFullyReleased {
		t.Errorf("Type = %s, want %s", msg.Type, NotifyFullyReleased)
	}
}

// TestNotificationTypeConstants verifies all notification types are distinct.
func TestNotificationTypeConstants(t *testing.T) {
	types := []NotificationType{
		NotifyCheckInReminder,
		NotifyCheckInMissed,
		NotifyGraceExpired,
		NotifyDisputeWindowOpen,
		NotifyFinalOverride,
		NotifyGuardianHalt,
		NotifyTranche1Released,
		NotifyFullyReleased,
		NotifyQuorumMet,
	}

	seen := make(map[NotificationType]bool)
	for _, nt := range types {
		if seen[nt] {
			t.Errorf("duplicate notification type: %s", nt)
		}
		seen[nt] = true
	}
}
