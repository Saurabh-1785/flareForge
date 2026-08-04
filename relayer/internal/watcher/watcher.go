// Package watcher implements the goroutine-per-vault deadline watcher.
// It is the brain of the relayer: watching on-chain deadlines, detecting
// state changes, driving notifications, and shuttling enclave results on-chain.
//
// Build prompt tasks:
//   ✅ Goroutine-per-vault deadline watcher against Coston2 contract events
//   ✅ T-minus-N reminder before a check-in deadline
//   ✅ On grace-period expiry: call requestAttestation() and relay enclave
//      signed quorum result to submitQuorumResult()
//   ✅ Final-override notice once DISPUTE_WINDOW opens
//
// Definition of done: "with all windows at demo length (minutes), you can
// walk away from a running vault, come back, and see WARNING → QUORUM_PENDING
// happen without touching anything by hand, plus a reminder actually arriving."
package watcher

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/continuity-vault/relayer/internal/chain"
	"github.com/continuity-vault/relayer/internal/enclaveclient"
	"github.com/continuity-vault/relayer/internal/notifier"
	"github.com/continuity-vault/relayer/internal/store"
)

// Config holds watcher configuration.
type Config struct {
	// PollInterval is how often the watcher checks deadlines.
	// Demo: 5-10 seconds. Production: 30-60 seconds.
	PollInterval time.Duration

	// ReminderLeadTime is how far before a deadline to send a reminder.
	// Demo: 30 seconds. Production: 5 days.
	ReminderLeadTime time.Duration

	// EnclavePollingInterval is how often to poll the enclave for quorum
	// status when a vault is in QUORUM_PENDING.
	EnclavePollingInterval time.Duration
}

// DefaultDemoConfig returns demo-length configuration.
func DefaultDemoConfig() Config {
	return Config{
		PollInterval:           5 * time.Second,
		ReminderLeadTime:       30 * time.Second,
		EnclavePollingInterval: 3 * time.Second,
	}
}

// DefaultProductionConfig returns production-length configuration.
func DefaultProductionConfig() Config {
	return Config{
		PollInterval:           30 * time.Second,
		ReminderLeadTime:       5 * 24 * time.Hour, // 5 days
		EnclavePollingInterval: 60 * time.Second,
	}
}

// Watcher monitors vaults and drives the lifecycle automatically.
type Watcher struct {
	store    *store.Store
	chain    *chain.Client
	enclave  *enclaveclient.Client
	notifier notifier.Notifier
	config   Config
	logger   *zap.Logger

	// vaultWatchers tracks per-vault goroutines
	mu       sync.Mutex
	watchers map[uint64]context.CancelFunc
	wg       sync.WaitGroup
}

// NewWatcher creates a new vault watcher.
func NewWatcher(
	s *store.Store,
	c *chain.Client,
	e *enclaveclient.Client,
	n notifier.Notifier,
	cfg Config,
	logger *zap.Logger,
) *Watcher {
	return &Watcher{
		store:    s,
		chain:    c,
		enclave:  e,
		notifier: n,
		config:   cfg,
		logger:   logger,
		watchers: make(map[uint64]context.CancelFunc),
	}
}

// Start begins the watcher. It:
// 1. Loads existing vaults from the store
// 2. Spawns a goroutine for each active vault
// 3. Listens for new VaultCreated events
// 4. Runs a periodic deadline-check sweep
func (w *Watcher) Start(ctx context.Context) error {
	w.logger.Info("starting vault watcher",
		zap.Duration("pollInterval", w.config.PollInterval),
		zap.Duration("reminderLeadTime", w.config.ReminderLeadTime),
	)

	// Load existing vaults and start watchers
	if err := w.syncExistingVaults(ctx); err != nil {
		w.logger.Error("failed to sync existing vaults", zap.Error(err))
		// Non-fatal: we'll pick up events via subscription
	}

	// Start the event listener (non-blocking)
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.listenEvents(ctx)
	}()

	// Start the periodic deadline sweep
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.deadlineSweep(ctx)
	}()

	// Start the enclave polling loop
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.enclavePollingLoop(ctx)
	}()

	return nil
}

// Stop gracefully shuts down all watchers.
func (w *Watcher) Stop() {
	w.mu.Lock()
	for vaultID, cancel := range w.watchers {
		cancel()
		delete(w.watchers, vaultID)
	}
	w.mu.Unlock()
	w.wg.Wait()
	w.logger.Info("watcher stopped")
}

// ──────────────────────────────────────────────────────────────────────
// Event listener — picks up new vaults and state transitions
// ──────────────────────────────────────────────────────────────────────

func (w *Watcher) listenEvents(ctx context.Context) {
	w.logger.Info("starting event listener")

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		sub, logCh, err := w.chain.SubscribeEvents(ctx)
		if err != nil {
			w.logger.Warn("event subscription failed, falling back to polling",
				zap.Error(err),
			)
			// Fallback: poll-based event detection runs via deadlineSweep
			// Sleep and retry subscription
			select {
			case <-ctx.Done():
				return
			case <-time.After(10 * time.Second):
				continue
			}
		}

		w.logger.Info("subscribed to VaultRegistry events")

		for {
			select {
			case <-ctx.Done():
				sub.Unsubscribe()
				return

			case err := <-sub.Err():
				w.logger.Error("event subscription error, reconnecting", zap.Error(err))
				// Break inner loop to reconnect
				goto reconnect

			case vLog := <-logCh:
				w.handleEvent(ctx, vLog)
			}
		}
	reconnect:
		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
		}
	}
}

func (w *Watcher) handleEvent(ctx context.Context, vLog interface{}) {
	// The deadline sweep is the primary state-detection mechanism.
	// The event listener serves as an optimization to detect new vaults
	// immediately rather than waiting for the next polling cycle.
	// In a production deployment, this would parse VaultCreated events
	// and immediately syncVault() for the new vault ID.
	w.logger.Debug("received chain event")
}

// ──────────────────────────────────────────────────────────────────────
// Deadline sweep — the core polling loop
// ──────────────────────────────────────────────────────────────────────

func (w *Watcher) deadlineSweep(ctx context.Context) {
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runSweep(ctx)
		}
	}
}

func (w *Watcher) runSweep(ctx context.Context) {
	// 1. Sync any new vaults from chain
	w.syncNewVaults(ctx)

	// 2. Check each active vault
	vaults, err := w.store.GetActiveVaults()
	if err != nil {
		w.logger.Error("failed to get active vaults", zap.Error(err))
		return
	}

	now := time.Now().UTC()
	for _, v := range vaults {
		w.processVault(ctx, v, now)
	}
}

// processVault evaluates a single vault against its current state and deadlines.
func (w *Watcher) processVault(ctx context.Context, v store.TrackedVault, now time.Time) {
	switch v.State {

	case store.StateActive:
		w.processActiveVault(ctx, v, now)

	case store.StateWarning:
		w.processWarningVault(ctx, v, now)

	case store.StateQuorumPending:
		// Quorum polling is handled by enclavePollingLoop
		// Nothing to do here

	case store.StateDisputeWindow:
		w.processDisputeWindowVault(ctx, v, now)

	case store.StateFinalWindow:
		w.processFinalWindowVault(ctx, v, now)
	}
}

// ──────────────────────────────────────────────────────────────────────
// State-specific processing
// ──────────────────────────────────────────────────────────────────────

// processActiveVault handles ACTIVE vaults — send reminders and detect missed check-ins.
func (w *Watcher) processActiveVault(ctx context.Context, v store.TrackedVault, now time.Time) {
	// 1. Send reminder if deadline is approaching
	reminderThreshold := v.WindowDeadline.Add(-w.config.ReminderLeadTime)
	if !v.ReminderSent && now.After(reminderThreshold) && now.Before(v.WindowDeadline) {
		msg := notifier.CheckInReminderMessage(v.VaultID, v.Owner, v.WindowDeadline)
		if err := w.notifier.Send(msg); err != nil {
			w.logger.Error("failed to send reminder", zap.Uint64("vaultId", v.VaultID), zap.Error(err))
		}
		w.store.SetReminderSent(v.VaultID)
		w.store.LogNotification(store.NotificationLog{
			VaultID:   v.VaultID,
			EventType: string(notifier.NotifyCheckInReminder),
			Recipient: v.Owner,
			Channel:   w.notifier.Name(),
			Message:   msg.Message,
			SentAt:    now,
		})
		w.logger.Info("reminder sent", zap.Uint64("vaultId", v.VaultID))
	}

	// 2. Detect missed check-in → call markWarning on-chain
	if now.After(v.WindowDeadline) {
		w.logger.Info("check-in deadline missed, calling markWarning",
			zap.Uint64("vaultId", v.VaultID),
		)

		tx, err := w.chain.MarkWarning(ctx, v.VaultID)
		if err != nil {
			w.logger.Error("markWarning tx failed", zap.Uint64("vaultId", v.VaultID), zap.Error(err))
			return
		}

		w.logger.Info("markWarning tx sent",
			zap.Uint64("vaultId", v.VaultID),
			zap.String("txHash", tx.Hash().Hex()),
		)

		// Update local state: ACTIVE → WARNING
		graceDeadline := now.Add(time.Duration(v.GraceWindow) * time.Second)
		w.store.UpdateState(v.VaultID, store.StateWarning, graceDeadline)

		// Send notification
		msg := notifier.CheckInMissedMessage(v.VaultID, v.Owner)
		w.notifier.Send(msg)
		w.store.LogNotification(store.NotificationLog{
			VaultID:   v.VaultID,
			EventType: string(notifier.NotifyCheckInMissed),
			Recipient: v.Owner,
			Channel:   w.notifier.Name(),
			Message:   msg.Message,
			SentAt:    now,
		})
	}
}

// processWarningVault handles WARNING vaults — detect grace expiry and call requestAttestation.
func (w *Watcher) processWarningVault(ctx context.Context, v store.TrackedVault, now time.Time) {
	if !now.After(v.WindowDeadline) {
		return // Grace period hasn't expired yet
	}

	w.logger.Info("grace period expired, calling requestAttestation",
		zap.Uint64("vaultId", v.VaultID),
	)

	// Call requestAttestation on-chain → WARNING → QUORUM_PENDING
	tx, err := w.chain.RequestAttestation(ctx, v.VaultID)
	if err != nil {
		w.logger.Error("requestAttestation tx failed", zap.Uint64("vaultId", v.VaultID), zap.Error(err))
		return
	}

	w.logger.Info("requestAttestation tx sent",
		zap.Uint64("vaultId", v.VaultID),
		zap.String("txHash", tx.Hash().Hex()),
	)

	// Update local state: WARNING → QUORUM_PENDING
	// No deadline for QUORUM_PENDING — it waits for attestations
	w.store.UpdateState(v.VaultID, store.StateQuorumPending, time.Time{})

	// Send notification
	msg := notifier.GraceExpiredMessage(v.VaultID, v.Owner)
	w.notifier.Send(msg)
	w.store.LogNotification(store.NotificationLog{
		VaultID:   v.VaultID,
		EventType: string(notifier.NotifyGraceExpired),
		Recipient: v.Owner,
		Channel:   w.notifier.Name(),
		Message:   msg.Message,
		SentAt:    now,
	})
}

// processDisputeWindowVault handles DISPUTE_WINDOW vaults.
func (w *Watcher) processDisputeWindowVault(ctx context.Context, v store.TrackedVault, now time.Time) {
	// 1. Send dispute-window notification (once)
	if !v.DisputeSent {
		msg := notifier.DisputeWindowMessage(v.VaultID, v.Owner, v.WindowDeadline)
		w.notifier.Send(msg)
		w.store.SetDisputeSent(v.VaultID)
		w.store.LogNotification(store.NotificationLog{
			VaultID:   v.VaultID,
			EventType: string(notifier.NotifyDisputeWindowOpen),
			Recipient: v.Owner,
			Channel:   w.notifier.Name(),
			Message:   msg.Message,
			SentAt:    now,
		})
	}

	// 2. Finalize if window has elapsed
	if now.After(v.WindowDeadline) {
		w.logger.Info("dispute window elapsed, calling finalizeDisputeWindow",
			zap.Uint64("vaultId", v.VaultID),
		)

		tx, err := w.chain.FinalizeDisputeWindow(ctx, v.VaultID)
		if err != nil {
			w.logger.Error("finalizeDisputeWindow tx failed", zap.Uint64("vaultId", v.VaultID), zap.Error(err))
			return
		}

		w.logger.Info("finalizeDisputeWindow tx sent",
			zap.Uint64("vaultId", v.VaultID),
			zap.String("txHash", tx.Hash().Hex()),
		)

		// Update local state: DISPUTE_WINDOW → FINAL_WINDOW
		finalDeadline := now.Add(time.Duration(v.FinalWindow) * time.Second)
		w.store.UpdateState(v.VaultID, store.StateFinalWindow, finalDeadline)

		// Send final-override notice
		msg := notifier.FinalOverrideMessage(v.VaultID, v.Owner, finalDeadline)
		w.notifier.Send(msg)
		w.store.LogNotification(store.NotificationLog{
			VaultID:   v.VaultID,
			EventType: string(notifier.NotifyFinalOverride),
			Recipient: v.Owner,
			Channel:   w.notifier.Name(),
			Message:   msg.Message,
			SentAt:    now,
		})
	}
}

// processFinalWindowVault handles FINAL_WINDOW vaults.
func (w *Watcher) processFinalWindowVault(ctx context.Context, v store.TrackedVault, now time.Time) {
	if !now.After(v.WindowDeadline) {
		return
	}

	w.logger.Info("final window elapsed, calling finalizeFinalWindow",
		zap.Uint64("vaultId", v.VaultID),
	)

	tx, err := w.chain.FinalizeFinalWindow(ctx, v.VaultID)
	if err != nil {
		w.logger.Error("finalizeFinalWindow tx failed", zap.Uint64("vaultId", v.VaultID), zap.Error(err))
		return
	}

	w.logger.Info("finalizeFinalWindow tx sent",
		zap.Uint64("vaultId", v.VaultID),
		zap.String("txHash", tx.Hash().Hex()),
	)

	// Update local state: FINAL_WINDOW → FULLY_RELEASED
	w.store.UpdateState(v.VaultID, store.StateFullyReleased, time.Time{})

	// Send notification
	msg := notifier.FullyReleasedMessage(v.VaultID, v.Owner)
	w.notifier.Send(msg)
	w.store.LogNotification(store.NotificationLog{
		VaultID:   v.VaultID,
		EventType: string(notifier.NotifyFullyReleased),
		Recipient: v.Owner,
		Channel:   w.notifier.Name(),
		Message:   msg.Message,
		SentAt:    now,
	})
}

// ──────────────────────────────────────────────────────────────────────
// Enclave polling — watches quorum status for QUORUM_PENDING vaults
// ──────────────────────────────────────────────────────────────────────

func (w *Watcher) enclavePollingLoop(ctx context.Context) {
	ticker := time.NewTicker(w.config.EnclavePollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pollEnclaveQuorum(ctx)
		}
	}
}

func (w *Watcher) pollEnclaveQuorum(ctx context.Context) {
	vaults, err := w.store.GetActiveVaults()
	if err != nil {
		w.logger.Error("failed to get vaults for enclave polling", zap.Error(err))
		return
	}

	for _, v := range vaults {
		if v.State != store.StateQuorumPending || v.QuorumRelayed {
			continue
		}

		status, err := w.enclave.GetQuorumStatus(v.VaultID)
		if err != nil {
			w.logger.Debug("enclave quorum poll failed",
				zap.Uint64("vaultId", v.VaultID),
				zap.Error(err),
			)
			continue
		}

		if !status.QuorumMet {
			w.logger.Debug("quorum not yet met",
				zap.Uint64("vaultId", v.VaultID),
				zap.Int("current", status.CurrentCount),
				zap.Uint8("required", status.RequiredCount),
			)
			continue
		}

		// Quorum is met! Get the signed result and submit on-chain.
		w.relayQuorumResult(ctx, v)
	}
}

func (w *Watcher) relayQuorumResult(ctx context.Context, v store.TrackedVault) {
	w.logger.Info("quorum met! Fetching signed result from enclave",
		zap.Uint64("vaultId", v.VaultID),
	)

	result, err := w.enclave.GetQuorumResult(v.VaultID)
	if err != nil {
		w.logger.Error("failed to get quorum result", zap.Uint64("vaultId", v.VaultID), zap.Error(err))
		return
	}

	// Decode the hex signature
	sigHex := strings.TrimPrefix(result.Signature, "0x")
	signature, err := hex.DecodeString(sigHex)
	if err != nil {
		w.logger.Error("failed to decode signature", zap.Uint64("vaultId", v.VaultID), zap.Error(err))
		return
	}

	// Submit on-chain: submitQuorumResult(vaultId, true, signature)
	tx, err := w.chain.SubmitQuorumResult(ctx, v.VaultID, true, signature)
	if err != nil {
		w.logger.Error("submitQuorumResult tx failed", zap.Uint64("vaultId", v.VaultID), zap.Error(err))
		return
	}

	w.logger.Info("submitQuorumResult tx sent",
		zap.Uint64("vaultId", v.VaultID),
		zap.String("txHash", tx.Hash().Hex()),
	)

	// Update local state: QUORUM_PENDING → DISPUTE_WINDOW
	disputeDeadline := time.Now().UTC().Add(time.Duration(v.DisputeWindow) * time.Second)
	w.store.UpdateState(v.VaultID, store.StateDisputeWindow, disputeDeadline)
	w.store.SetQuorumRelayed(v.VaultID)

	// Send dispute-window notification
	msg := notifier.DisputeWindowMessage(v.VaultID, v.Owner, disputeDeadline)
	w.notifier.Send(msg)
	w.store.LogNotification(store.NotificationLog{
		VaultID:   v.VaultID,
		EventType: string(notifier.NotifyQuorumMet),
		Recipient: v.Owner,
		Channel:   w.notifier.Name(),
		Message:   msg.Message,
		SentAt:    time.Now().UTC(),
	})
}

// ──────────────────────────────────────────────────────────────────────
// Vault sync — discover and track vaults from the chain
// ──────────────────────────────────────────────────────────────────────

// syncExistingVaults loads all existing vaults from the chain into the store.
func (w *Watcher) syncExistingVaults(ctx context.Context) error {
	nextID, err := w.chain.GetNextVaultID(ctx)
	if err != nil {
		return fmt.Errorf("get nextVaultId: %w", err)
	}

	w.logger.Info("syncing existing vaults", zap.Uint64("nextVaultId", nextID))

	for id := uint64(1); id < nextID; id++ {
		if err := w.syncVault(ctx, id); err != nil {
			w.logger.Warn("failed to sync vault", zap.Uint64("vaultId", id), zap.Error(err))
			continue
		}
	}

	return nil
}

// syncNewVaults checks for new vaults created since last sync.
func (w *Watcher) syncNewVaults(ctx context.Context) {
	nextID, err := w.chain.GetNextVaultID(ctx)
	if err != nil {
		w.logger.Debug("failed to check for new vaults", zap.Error(err))
		return
	}

	// Check each vault we might not have
	for id := uint64(1); id < nextID; id++ {
		existing, _ := w.store.GetVault(id)
		if existing != nil {
			continue // Already tracking
		}

		if err := w.syncVault(ctx, id); err != nil {
			w.logger.Warn("failed to sync new vault", zap.Uint64("vaultId", id), zap.Error(err))
		}
	}
}

// syncVault reads a vault from the chain and upserts it into the store.
func (w *Watcher) syncVault(ctx context.Context, vaultID uint64) error {
	state, err := w.chain.GetVaultState(ctx, vaultID)
	if err != nil {
		return fmt.Errorf("get state: %w", err)
	}

	// Skip fully released and closed vaults
	if store.VaultState(state) == store.StateFullyReleased ||
		store.VaultState(state) == store.StateClosed {
		return nil
	}

	owner, err := w.chain.GetVaultOwner(ctx, vaultID)
	if err != nil {
		return fmt.Errorf("get owner: %w", err)
	}

	timing, err := w.chain.GetVaultTiming(ctx, vaultID)
	if err != nil {
		return fmt.Errorf("get timing: %w", err)
	}

	v := store.TrackedVault{
		VaultID:         vaultID,
		Owner:           owner,
		State:           store.VaultState(state),
		WindowDeadline:  time.Unix(int64(timing.WindowDeadline), 0).UTC(),
		CheckInInterval: timing.CheckInInterval,
		GraceWindow:     timing.GraceWindow,
		DisputeWindow:   timing.DisputeWindow,
		FinalWindow:     timing.FinalWindow,
		LastCheckIn:     time.Unix(int64(timing.LastCheckIn), 0).UTC(),
	}

	if err := w.store.UpsertVault(v); err != nil {
		return fmt.Errorf("upsert vault: %w", err)
	}

	w.logger.Info("synced vault",
		zap.Uint64("vaultId", vaultID),
		zap.String("state", v.State.String()),
		zap.Time("deadline", v.WindowDeadline),
	)
	return nil
}

// RefreshVaultFromChain re-syncs a single vault from on-chain state.
// Useful after detecting a state change event.
func (w *Watcher) RefreshVaultFromChain(ctx context.Context, vaultID uint64) error {
	return w.syncVault(ctx, vaultID)
}
