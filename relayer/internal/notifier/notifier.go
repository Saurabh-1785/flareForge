// Package notifier sends notifications (reminders, warnings, dispute alerts)
// to vault owners.
//
// Build prompt: "Notification service — Whichever transactional email/SMS API
// you're fastest with — Not a judged detail — pick whatever you can wire in
// an hour."
//
// And from "if you're running out of time": "Cut the notification service's
// actual delivery — a console log of 'reminder would have been sent' is fine
// for a demo; the relayer's *detection* logic is what matters, not the
// delivery channel."
//
// Implementation: Console/log-based notifications for the MVP, with
// a clean interface so email/SMS providers can be plugged in later.
package notifier

import (
	"fmt"
	"time"

	"go.uber.org/zap"
)

// NotificationType identifies the kind of notification.
type NotificationType string

const (
	// NotifyCheckInReminder is a T-minus-N warning before check-in deadline.
	NotifyCheckInReminder NotificationType = "CHECK_IN_REMINDER"

	// NotifyCheckInMissed is sent when check-in was missed (ACTIVE → WARNING).
	NotifyCheckInMissed NotificationType = "CHECK_IN_MISSED"

	// NotifyGraceExpired is sent when grace period expired (WARNING → QUORUM_PENDING).
	NotifyGraceExpired NotificationType = "GRACE_EXPIRED"

	// NotifyDisputeWindowOpen is sent when dispute window opens.
	NotifyDisputeWindowOpen NotificationType = "DISPUTE_WINDOW_OPEN"

	// NotifyFinalOverride is the last chance to use guardian halt key.
	NotifyFinalOverride NotificationType = "FINAL_OVERRIDE_NOTICE"

	// NotifyGuardianHalt is sent when a guardian halt occurred.
	NotifyGuardianHalt NotificationType = "GUARDIAN_HALT"

	// NotifyTranche1Released is sent when tranche 1 has been released.
	NotifyTranche1Released NotificationType = "TRANCHE_1_RELEASED"

	// NotifyFullyReleased is sent when all funds have been released.
	NotifyFullyReleased NotificationType = "FULLY_RELEASED"

	// NotifyQuorumMet is sent when quorum has been met.
	NotifyQuorumMet NotificationType = "QUORUM_MET"
)

// Notification represents a notification to send.
type Notification struct {
	VaultID   uint64
	Type      NotificationType
	Recipient string // email address, phone number, or owner address
	Subject   string
	Message   string
	SentAt    time.Time
	Channel   string // "console", "email", "sms"
}

// Notifier sends notifications through configured channels.
type Notifier interface {
	Send(n Notification) error
	Name() string
}

// ──────────────────────────────────────────────────────────────────────
// Console notifier — MVP implementation
// ──────────────────────────────────────────────────────────────────────

// ConsoleNotifier logs notifications to the console/structured log.
// This is the MVP implementation — the detection logic is what matters,
// not the delivery channel.
type ConsoleNotifier struct {
	logger *zap.Logger
}

// NewConsoleNotifier creates a console-based notifier.
func NewConsoleNotifier(logger *zap.Logger) *ConsoleNotifier {
	return &ConsoleNotifier{logger: logger}
}

// Send logs the notification.
func (c *ConsoleNotifier) Send(n Notification) error {
	c.logger.Info("📧 NOTIFICATION SENT",
		zap.Uint64("vaultId", n.VaultID),
		zap.String("type", string(n.Type)),
		zap.String("recipient", n.Recipient),
		zap.String("channel", "console"),
		zap.String("subject", n.Subject),
		zap.String("message", n.Message),
	)

	// Pretty-print for demo visibility
	fmt.Printf(`
  ┌────────────────────────────────────────────────────┐
  │  📧 NOTIFICATION                                   │
  │  Vault:   #%d                                      
  │  Type:    %s            
  │  To:      %s            
  │  Subject: %s            
  │  Message: %s            
  └────────────────────────────────────────────────────┘
`, n.VaultID, n.Type, n.Recipient, n.Subject, n.Message)

	return nil
}

// Name returns the notifier name.
func (c *ConsoleNotifier) Name() string { return "console" }

// ──────────────────────────────────────────────────────────────────────
// Message templates
// ──────────────────────────────────────────────────────────────────────

// CheckInReminderMessage creates a check-in reminder notification.
func CheckInReminderMessage(vaultID uint64, owner string, deadline time.Time) Notification {
	timeLeft := time.Until(deadline).Round(time.Second)
	return Notification{
		VaultID:   vaultID,
		Type:      NotifyCheckInReminder,
		Recipient: owner,
		Subject:   fmt.Sprintf("Continuity Vault #%d — Check-in reminder", vaultID),
		Message: fmt.Sprintf(
			"Your check-in deadline is in %s (at %s). Please check in to keep your vault active.",
			timeLeft, deadline.Format(time.RFC3339),
		),
		SentAt:  time.Now().UTC(),
		Channel: "console",
	}
}

// CheckInMissedMessage creates a check-in missed notification.
func CheckInMissedMessage(vaultID uint64, owner string) Notification {
	return Notification{
		VaultID:   vaultID,
		Type:      NotifyCheckInMissed,
		Recipient: owner,
		Subject:   fmt.Sprintf("⚠️ Continuity Vault #%d — Check-in MISSED", vaultID),
		Message: fmt.Sprintf(
			"Your check-in was missed. Vault #%d has entered WARNING state. "+
				"A grace period is now active — check in before it expires to return to ACTIVE.",
			vaultID,
		),
		SentAt:  time.Now().UTC(),
		Channel: "console",
	}
}

// GraceExpiredMessage creates a grace-expired notification.
func GraceExpiredMessage(vaultID uint64, owner string) Notification {
	return Notification{
		VaultID:   vaultID,
		Type:      NotifyGraceExpired,
		Recipient: owner,
		Subject:   fmt.Sprintf("🚨 Continuity Vault #%d — Grace period expired", vaultID),
		Message: fmt.Sprintf(
			"The grace period for Vault #%d has expired. "+
				"Attestation has been requested. The vault is now in QUORUM_PENDING state.",
			vaultID,
		),
		SentAt:  time.Now().UTC(),
		Channel: "console",
	}
}

// DisputeWindowMessage creates a dispute-window notification.
func DisputeWindowMessage(vaultID uint64, owner string, deadline time.Time) Notification {
	return Notification{
		VaultID:   vaultID,
		Type:      NotifyDisputeWindowOpen,
		Recipient: owner,
		Subject:   fmt.Sprintf("🔴 Continuity Vault #%d — DISPUTE WINDOW OPEN", vaultID),
		Message: fmt.Sprintf(
			"CRITICAL: Quorum has been met for Vault #%d. "+
				"A dispute window is now open until %s. "+
				"Use your guardian halt key NOW if this is a false trigger. "+
				"If no action is taken, funds will begin releasing.",
			vaultID, deadline.Format(time.RFC3339),
		),
		SentAt:  time.Now().UTC(),
		Channel: "console",
	}
}

// FinalOverrideMessage creates a final-override notice.
func FinalOverrideMessage(vaultID uint64, owner string, deadline time.Time) Notification {
	return Notification{
		VaultID:   vaultID,
		Type:      NotifyFinalOverride,
		Recipient: owner,
		Subject:   fmt.Sprintf("🔴 Continuity Vault #%d — FINAL override window", vaultID),
		Message: fmt.Sprintf(
			"LAST CHANCE: Tranche 1 has been released for Vault #%d. "+
				"A final window is open until %s. "+
				"Use your guardian halt key to stop the remaining release.",
			vaultID, deadline.Format(time.RFC3339),
		),
		SentAt:  time.Now().UTC(),
		Channel: "console",
	}
}

// GuardianHaltMessage creates a guardian halt notification.
func GuardianHaltMessage(vaultID uint64, owner string) Notification {
	return Notification{
		VaultID:   vaultID,
		Type:      NotifyGuardianHalt,
		Recipient: owner,
		Subject:   fmt.Sprintf("✅ Continuity Vault #%d — Guardian HALT activated", vaultID),
		Message: fmt.Sprintf(
			"A guardian halt was activated for Vault #%d. "+
				"The vault has returned to ACTIVE state. "+
				"The trigger was a false positive — the safety net worked.",
			vaultID,
		),
		SentAt:  time.Now().UTC(),
		Channel: "console",
	}
}

// FullyReleasedMessage creates a fully-released notification.
func FullyReleasedMessage(vaultID uint64, owner string) Notification {
	return Notification{
		VaultID:   vaultID,
		Type:      NotifyFullyReleased,
		Recipient: owner,
		Subject:   fmt.Sprintf("Continuity Vault #%d — Fully released", vaultID),
		Message: fmt.Sprintf(
			"All funds in Vault #%d have been released to beneficiaries. "+
				"The vault is now closed.",
			vaultID,
		),
		SentAt:  time.Now().UTC(),
		Channel: "console",
	}
}
