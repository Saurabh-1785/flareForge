package enclaveclient

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewClient("http://localhost:8090", logger)

	if c.baseURL != "http://localhost:8090" {
		t.Errorf("baseURL = %s, want http://localhost:8090", c.baseURL)
	}
	if c.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

// TestHealthCheck_NoServer tests that health check fails gracefully
// when no enclave server is running.
func TestHealthCheck_NoServer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewClient("http://localhost:19999", logger) // non-existent port

	err := c.HealthCheck()
	if err == nil {
		t.Error("HealthCheck should fail when no server is running")
	}
}

// TestGetQuorumStatus_NoServer tests graceful failure.
func TestGetQuorumStatus_NoServer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewClient("http://localhost:19999", logger)

	_, err := c.GetQuorumStatus(1)
	if err == nil {
		t.Error("GetQuorumStatus should fail when no server is running")
	}
}

// TestGetQuorumResult_NoServer tests graceful failure.
func TestGetQuorumResult_NoServer(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	c := NewClient("http://localhost:19999", logger)

	_, err := c.GetQuorumResult(1)
	if err == nil {
		t.Error("GetQuorumResult should fail when no server is running")
	}
}
