// Package enclaveclient provides an HTTP client for the Vault Enclave
// (Layer 3). The relayer uses this to:
// - Poll quorum status for watched vaults
// - Retrieve signed quorum results for on-chain submission
// - Reset quorum state after guardian halts
// - Forward verified FDC attestation references
package enclaveclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// QuorumStatus mirrors the enclave's quorum status response.
type QuorumStatus struct {
	VaultID       uint64 `json:"vaultId"`
	QuorumMet     bool   `json:"quorumMet"`
	RequiredCount uint8  `json:"requiredCount"`
	CurrentCount  int    `json:"currentCount"`
	SignedResult  []byte `json:"signedResult,omitempty"`
}

// QuorumResult mirrors the enclave's quorum result response.
type QuorumResult struct {
	VaultID       uint64 `json:"vaultId"`
	QuorumMet     bool   `json:"quorumMet"`
	Signature     string `json:"signature"`
	EvaluatedAt   string `json:"evaluatedAt"`
	AttestorCount int    `json:"attestorCount"`
}

// EnclaveIdentity mirrors the enclave's identity response.
type EnclaveIdentity struct {
	SigningAddress      string `json:"signingAddress"`
	SigningPubKey       string `json:"signingPubKey"`
	EncryptionPubKey    string `json:"encryptionPubKey"`
	AttestationEvidence string `json:"attestationEvidence"`
}

// AttestationRef is a reference to a verified FDC attestation.
type AttestationRef struct {
	VaultID         uint64    `json:"vaultId"`
	CaseID          string    `json:"caseId"`
	AttestationType string    `json:"attestationType"`
	AttestorAddress string    `json:"attestorAddress"`
	FdcVotingRound  uint64    `json:"fdcVotingRound"`
	VerifiedAt      time.Time `json:"verifiedAt"`
}

// Client communicates with the Vault Enclave REST API.
type Client struct {
	baseURL    string
	httpClient *http.Client
	logger     *zap.Logger
}

// NewClient creates a new enclave client.
func NewClient(enclaveURL string, logger *zap.Logger) *Client {
	return &Client{
		baseURL: enclaveURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger,
	}
}

// GetIdentity returns the enclave's public keys and attestation evidence.
func (c *Client) GetIdentity() (*EnclaveIdentity, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/identity")
	if err != nil {
		return nil, fmt.Errorf("GET /identity: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}

	var identity EnclaveIdentity
	if err := json.NewDecoder(resp.Body).Decode(&identity); err != nil {
		return nil, fmt.Errorf("decode identity: %w", err)
	}
	return &identity, nil
}

// GetQuorumStatus polls the quorum status for a vault.
func (c *Client) GetQuorumStatus(vaultID uint64) (*QuorumStatus, error) {
	url := fmt.Sprintf("%s/vaults/%d/quorum-status", c.baseURL, vaultID)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET quorum-status: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}

	var status QuorumStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decode quorum-status: %w", err)
	}
	return &status, nil
}

// GetQuorumResult retrieves the signed quorum result for on-chain submission.
func (c *Client) GetQuorumResult(vaultID uint64) (*QuorumResult, error) {
	url := fmt.Sprintf("%s/vaults/%d/quorum-result", c.baseURL, vaultID)
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET quorum-result: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}

	var result QuorumResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode quorum-result: %w", err)
	}
	return &result, nil
}

// SubmitAttestation forwards a verified FDC attestation reference to the enclave.
func (c *Client) SubmitAttestation(ref AttestationRef) (*QuorumStatus, error) {
	url := fmt.Sprintf("%s/vaults/%d/attestations", c.baseURL, ref.VaultID)

	body, err := json.Marshal(ref)
	if err != nil {
		return nil, fmt.Errorf("marshal attestation: %w", err)
	}

	resp, err := c.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("POST attestations: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.readError(resp)
	}

	var status QuorumStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("decode attestation response: %w", err)
	}
	return &status, nil
}

// ResetQuorum resets quorum data for a vault (after guardian halt).
func (c *Client) ResetQuorum(vaultID uint64) error {
	url := fmt.Sprintf("%s/vaults/%d/reset", c.baseURL, vaultID)

	resp, err := c.httpClient.Post(url, "application/json", nil)
	if err != nil {
		return fmt.Errorf("POST reset: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.readError(resp)
	}
	return nil
}

// HealthCheck verifies the enclave is running.
func (c *Client) HealthCheck() error {
	resp, err := c.httpClient.Get(c.baseURL + "/health")
	if err != nil {
		return fmt.Errorf("GET /health: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("enclave unhealthy: status %d", resp.StatusCode)
	}
	return nil
}

// readError reads an error response body.
func (c *Client) readError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("enclave error (status %d): %s", resp.StatusCode, string(body))
}
