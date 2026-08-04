// Package store manages the relayer's Postgres state: tracked vaults,
// deadlines, notification history, and processing state.
//
// Build prompt: "Postgres schema: vaults, deadlines, notification log"
//
// The relayer tracks:
// - Which vaults exist and their current on-chain state
// - When each vault's next deadline fires
// - What notifications have been sent (dedup)
// - Which quorum results have been relayed on-chain
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

// VaultState mirrors the on-chain VaultState enum.
type VaultState uint8

const (
	StateActive           VaultState = 0
	StateWarning          VaultState = 1
	StateQuorumPending    VaultState = 2
	StateDisputeWindow    VaultState = 3
	StateSlashingReview   VaultState = 4
	StateTranche1Released VaultState = 5
	StateFinalWindow      VaultState = 6
	StateFullyReleased    VaultState = 7
	StateClosed           VaultState = 8
)

// String returns the human-readable state name.
func (s VaultState) String() string {
	switch s {
	case StateActive:
		return "ACTIVE"
	case StateWarning:
		return "WARNING"
	case StateQuorumPending:
		return "QUORUM_PENDING"
	case StateDisputeWindow:
		return "DISPUTE_WINDOW"
	case StateSlashingReview:
		return "SLASHING_REVIEW"
	case StateTranche1Released:
		return "TRANCHE_1_RELEASED"
	case StateFinalWindow:
		return "FINAL_WINDOW"
	case StateFullyReleased:
		return "FULLY_RELEASED"
	case StateClosed:
		return "CLOSED"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", s)
	}
}

// TrackedVault is a vault the relayer is watching.
type TrackedVault struct {
	VaultID         uint64     `json:"vaultId"`
	Owner           string     `json:"owner"`
	State           VaultState `json:"state"`
	WindowDeadline  time.Time  `json:"windowDeadline"`
	CheckInInterval uint64     `json:"checkInInterval"` // seconds
	GraceWindow     uint64     `json:"graceWindow"`     // seconds
	DisputeWindow   uint64     `json:"disputeWindow"`   // seconds
	FinalWindow     uint64     `json:"finalWindow"`     // seconds
	LastCheckIn     time.Time  `json:"lastCheckIn"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`

	// Notification state
	ReminderSent    bool `json:"reminderSent"`
	WarningSent     bool `json:"warningSent"`
	DisputeSent     bool `json:"disputeSent"`
	QuorumRelayed   bool `json:"quorumRelayed"`
}

// NotificationLog records a sent notification for audit/dedup.
type NotificationLog struct {
	ID        int64      `json:"id"`
	VaultID   uint64     `json:"vaultId"`
	EventType string     `json:"eventType"`
	Recipient string     `json:"recipient"`
	Channel   string     `json:"channel"` // "email", "sms", "console"
	Message   string     `json:"message"`
	SentAt    time.Time  `json:"sentAt"`
}

// Store manages the relayer's persistent state.
type Store struct {
	db     *sql.DB
	logger *zap.Logger
}

// NewStore creates a new store. It accepts either a real Postgres connection
// or a Postgres-compatible connection string. For local dev/testing without
// Postgres, use NewMemoryStore() instead.
func NewStore(db *sql.DB, logger *zap.Logger) (*Store, error) {
	s := &Store{db: db, logger: logger}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate relayer store: %w", err)
	}
	return s, nil
}

// ConnectPostgres opens a Postgres connection.
func ConnectPostgres(connStr string) (*sql.DB, error) {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	return db, nil
}

// migrate creates the required tables.
func (s *Store) migrate() error {
	// Use IF NOT EXISTS for idempotent migrations.
	// This schema works on both Postgres and SQLite (for testing).
	schema := `
	CREATE TABLE IF NOT EXISTS tracked_vaults (
		vault_id          BIGINT PRIMARY KEY,
		owner             TEXT    NOT NULL,
		state             INTEGER NOT NULL DEFAULT 0,
		window_deadline   TIMESTAMP NOT NULL,
		check_in_interval BIGINT  NOT NULL,
		grace_window      BIGINT  NOT NULL,
		dispute_window    BIGINT  NOT NULL,
		final_window      BIGINT  NOT NULL,
		last_check_in     TIMESTAMP NOT NULL,
		reminder_sent     BOOLEAN NOT NULL DEFAULT FALSE,
		warning_sent      BOOLEAN NOT NULL DEFAULT FALSE,
		dispute_sent      BOOLEAN NOT NULL DEFAULT FALSE,
		quorum_relayed    BOOLEAN NOT NULL DEFAULT FALSE,
		created_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at        TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS notification_log (
		id          BIGSERIAL PRIMARY KEY,
		vault_id    BIGINT    NOT NULL,
		event_type  TEXT      NOT NULL,
		recipient   TEXT      NOT NULL DEFAULT '',
		channel     TEXT      NOT NULL DEFAULT 'console',
		message     TEXT      NOT NULL,
		sent_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS relayer_state (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_tracked_vaults_state
		ON tracked_vaults(state);
	CREATE INDEX IF NOT EXISTS idx_tracked_vaults_deadline
		ON tracked_vaults(window_deadline);
	CREATE INDEX IF NOT EXISTS idx_notification_log_vault
		ON notification_log(vault_id);
	`
	_, err := s.db.Exec(schema)
	return err
}

// ──────────────────────────────────────────────────────────────────────
// Vault CRUD
// ──────────────────────────────────────────────────────────────────────

// UpsertVault creates or updates a tracked vault.
func (s *Store) UpsertVault(v TrackedVault) error {
	_, err := s.db.Exec(`
		INSERT INTO tracked_vaults (
			vault_id, owner, state, window_deadline,
			check_in_interval, grace_window, dispute_window, final_window,
			last_check_in, reminder_sent, warning_sent, dispute_sent, quorum_relayed,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		ON CONFLICT (vault_id) DO UPDATE SET
			owner             = EXCLUDED.owner,
			state             = EXCLUDED.state,
			window_deadline   = EXCLUDED.window_deadline,
			check_in_interval = EXCLUDED.check_in_interval,
			grace_window      = EXCLUDED.grace_window,
			dispute_window    = EXCLUDED.dispute_window,
			final_window      = EXCLUDED.final_window,
			last_check_in     = EXCLUDED.last_check_in,
			reminder_sent     = EXCLUDED.reminder_sent,
			warning_sent      = EXCLUDED.warning_sent,
			dispute_sent      = EXCLUDED.dispute_sent,
			quorum_relayed    = EXCLUDED.quorum_relayed,
			updated_at        = EXCLUDED.updated_at
	`, v.VaultID, v.Owner, v.State, v.WindowDeadline,
		v.CheckInInterval, v.GraceWindow, v.DisputeWindow, v.FinalWindow,
		v.LastCheckIn, v.ReminderSent, v.WarningSent, v.DisputeSent, v.QuorumRelayed,
		time.Now().UTC(),
	)
	return err
}

// GetVault retrieves a tracked vault by ID.
func (s *Store) GetVault(vaultID uint64) (*TrackedVault, error) {
	v := &TrackedVault{}
	err := s.db.QueryRow(`
		SELECT vault_id, owner, state, window_deadline,
			check_in_interval, grace_window, dispute_window, final_window,
			last_check_in, reminder_sent, warning_sent, dispute_sent, quorum_relayed,
			created_at, updated_at
		FROM tracked_vaults WHERE vault_id = $1
	`, vaultID).Scan(
		&v.VaultID, &v.Owner, &v.State, &v.WindowDeadline,
		&v.CheckInInterval, &v.GraceWindow, &v.DisputeWindow, &v.FinalWindow,
		&v.LastCheckIn, &v.ReminderSent, &v.WarningSent, &v.DisputeSent, &v.QuorumRelayed,
		&v.CreatedAt, &v.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

// GetActiveVaults returns all vaults that aren't CLOSED or FULLY_RELEASED.
func (s *Store) GetActiveVaults() ([]TrackedVault, error) {
	rows, err := s.db.Query(`
		SELECT vault_id, owner, state, window_deadline,
			check_in_interval, grace_window, dispute_window, final_window,
			last_check_in, reminder_sent, warning_sent, dispute_sent, quorum_relayed,
			created_at, updated_at
		FROM tracked_vaults
		WHERE state NOT IN ($1, $2)
		ORDER BY window_deadline ASC
	`, StateFullyReleased, StateClosed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vaults []TrackedVault
	for rows.Next() {
		var v TrackedVault
		if err := rows.Scan(
			&v.VaultID, &v.Owner, &v.State, &v.WindowDeadline,
			&v.CheckInInterval, &v.GraceWindow, &v.DisputeWindow, &v.FinalWindow,
			&v.LastCheckIn, &v.ReminderSent, &v.WarningSent, &v.DisputeSent, &v.QuorumRelayed,
			&v.CreatedAt, &v.UpdatedAt,
		); err != nil {
			return nil, err
		}
		vaults = append(vaults, v)
	}
	return vaults, nil
}

// UpdateState updates a vault's state and deadline, resetting notification flags.
func (s *Store) UpdateState(vaultID uint64, state VaultState, deadline time.Time) error {
	_, err := s.db.Exec(`
		UPDATE tracked_vaults
		SET state = $1, window_deadline = $2,
			reminder_sent = FALSE, warning_sent = FALSE, 
			dispute_sent = FALSE, quorum_relayed = FALSE,
			updated_at = $3
		WHERE vault_id = $4
	`, state, deadline, time.Now().UTC(), vaultID)
	return err
}

// SetReminderSent marks the reminder as sent for a vault.
func (s *Store) SetReminderSent(vaultID uint64) error {
	_, err := s.db.Exec(`
		UPDATE tracked_vaults SET reminder_sent = TRUE, updated_at = $1 WHERE vault_id = $2
	`, time.Now().UTC(), vaultID)
	return err
}

// SetWarningSent marks the warning notification as sent.
func (s *Store) SetWarningSent(vaultID uint64) error {
	_, err := s.db.Exec(`
		UPDATE tracked_vaults SET warning_sent = TRUE, updated_at = $1 WHERE vault_id = $2
	`, time.Now().UTC(), vaultID)
	return err
}

// SetDisputeSent marks the dispute-window notification as sent.
func (s *Store) SetDisputeSent(vaultID uint64) error {
	_, err := s.db.Exec(`
		UPDATE tracked_vaults SET dispute_sent = TRUE, updated_at = $1 WHERE vault_id = $2
	`, time.Now().UTC(), vaultID)
	return err
}

// SetQuorumRelayed marks the quorum result as having been submitted on-chain.
func (s *Store) SetQuorumRelayed(vaultID uint64) error {
	_, err := s.db.Exec(`
		UPDATE tracked_vaults SET quorum_relayed = TRUE, updated_at = $1 WHERE vault_id = $2
	`, time.Now().UTC(), vaultID)
	return err
}

// GetVaultsNeedingReminder returns active vaults whose deadline is within
// `reminderWindow` from now, and whose reminder hasn't been sent yet.
func (s *Store) GetVaultsNeedingReminder(reminderWindow time.Duration) ([]TrackedVault, error) {
	deadline := time.Now().UTC().Add(reminderWindow)
	rows, err := s.db.Query(`
		SELECT vault_id, owner, state, window_deadline,
			check_in_interval, grace_window, dispute_window, final_window,
			last_check_in, reminder_sent, warning_sent, dispute_sent, quorum_relayed,
			created_at, updated_at
		FROM tracked_vaults
		WHERE state = $1
			AND reminder_sent = FALSE
			AND window_deadline <= $2
			AND window_deadline > $3
		ORDER BY window_deadline ASC
	`, StateActive, deadline, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vaults []TrackedVault
	for rows.Next() {
		var v TrackedVault
		if err := rows.Scan(
			&v.VaultID, &v.Owner, &v.State, &v.WindowDeadline,
			&v.CheckInInterval, &v.GraceWindow, &v.DisputeWindow, &v.FinalWindow,
			&v.LastCheckIn, &v.ReminderSent, &v.WarningSent, &v.DisputeSent, &v.QuorumRelayed,
			&v.CreatedAt, &v.UpdatedAt,
		); err != nil {
			return nil, err
		}
		vaults = append(vaults, v)
	}
	return vaults, nil
}

// GetVaultsWithExpiredDeadline returns vaults whose deadline has passed.
func (s *Store) GetVaultsWithExpiredDeadline() ([]TrackedVault, error) {
	now := time.Now().UTC()
	rows, err := s.db.Query(`
		SELECT vault_id, owner, state, window_deadline,
			check_in_interval, grace_window, dispute_window, final_window,
			last_check_in, reminder_sent, warning_sent, dispute_sent, quorum_relayed,
			created_at, updated_at
		FROM tracked_vaults
		WHERE window_deadline <= $1
			AND state NOT IN ($2, $3)
		ORDER BY window_deadline ASC
	`, now, StateFullyReleased, StateClosed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var vaults []TrackedVault
	for rows.Next() {
		var v TrackedVault
		if err := rows.Scan(
			&v.VaultID, &v.Owner, &v.State, &v.WindowDeadline,
			&v.CheckInInterval, &v.GraceWindow, &v.DisputeWindow, &v.FinalWindow,
			&v.LastCheckIn, &v.ReminderSent, &v.WarningSent, &v.DisputeSent, &v.QuorumRelayed,
			&v.CreatedAt, &v.UpdatedAt,
		); err != nil {
			return nil, err
		}
		vaults = append(vaults, v)
	}
	return vaults, nil
}

// ──────────────────────────────────────────────────────────────────────
// Notification Log
// ──────────────────────────────────────────────────────────────────────

// LogNotification records a sent notification.
func (s *Store) LogNotification(log NotificationLog) error {
	_, err := s.db.Exec(`
		INSERT INTO notification_log (vault_id, event_type, recipient, channel, message, sent_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, log.VaultID, log.EventType, log.Recipient, log.Channel, log.Message, log.SentAt)
	return err
}

// GetNotifications returns recent notifications for a vault.
func (s *Store) GetNotifications(vaultID uint64, limit int) ([]NotificationLog, error) {
	rows, err := s.db.Query(`
		SELECT id, vault_id, event_type, recipient, channel, message, sent_at
		FROM notification_log
		WHERE vault_id = $1
		ORDER BY sent_at DESC
		LIMIT $2
	`, vaultID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []NotificationLog
	for rows.Next() {
		var l NotificationLog
		if err := rows.Scan(&l.ID, &l.VaultID, &l.EventType, &l.Recipient, &l.Channel, &l.Message, &l.SentAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

// ──────────────────────────────────────────────────────────────────────
// Relayer state (key-value for bookkeeping)
// ──────────────────────────────────────────────────────────────────────

// SetState stores a key-value pair for relayer bookkeeping (e.g., last processed block).
func (s *Store) SetState(key, value string) error {
	_, err := s.db.Exec(`
		INSERT INTO relayer_state (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value
	`, key, value)
	return err
}

// GetState retrieves a relayer state value.
func (s *Store) GetState(key string) (string, error) {
	var value string
	err := s.db.QueryRow(`SELECT value FROM relayer_state WHERE key = $1`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}
