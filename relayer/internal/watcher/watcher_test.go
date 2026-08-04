package watcher

import (
	"testing"
	"time"
)

func TestDefaultDemoConfig(t *testing.T) {
	cfg := DefaultDemoConfig()

	if cfg.PollInterval != 5*time.Second {
		t.Errorf("PollInterval = %v, want 5s", cfg.PollInterval)
	}
	if cfg.ReminderLeadTime != 30*time.Second {
		t.Errorf("ReminderLeadTime = %v, want 30s", cfg.ReminderLeadTime)
	}
	if cfg.EnclavePollingInterval != 3*time.Second {
		t.Errorf("EnclavePollingInterval = %v, want 3s", cfg.EnclavePollingInterval)
	}
}

func TestDefaultProductionConfig(t *testing.T) {
	cfg := DefaultProductionConfig()

	if cfg.PollInterval != 30*time.Second {
		t.Errorf("PollInterval = %v, want 30s", cfg.PollInterval)
	}
	if cfg.ReminderLeadTime != 5*24*time.Hour {
		t.Errorf("ReminderLeadTime = %v, want 5 days", cfg.ReminderLeadTime)
	}
	if cfg.EnclavePollingInterval != 60*time.Second {
		t.Errorf("EnclavePollingInterval = %v, want 60s", cfg.EnclavePollingInterval)
	}
}
