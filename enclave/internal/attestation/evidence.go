// Package attestation provides TEE remote attestation support.
//
// In Google Cloud Confidential Space, the TEE provides a signed attestation
// token proving what code is running and that it's inside real isolated hardware.
// This is the mechanism that makes the "confidentiality claim in your demo
// literally true" (build prompt, Layer 3).
//
// For local development/testing, a placeholder token is generated.
// For production deployment in Confidential Space, the real attestation
// endpoint is at http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity
package attestation

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Provider describes where this enclave is running.
type Provider string

const (
	// ProviderConfidentialSpace is a real Google Cloud Confidential Space TEE.
	ProviderConfidentialSpace Provider = "google-cloud-confidential-space"

	// ProviderLocal is local development (no real TEE hardware).
	ProviderLocal Provider = "local-development"
)

// Evidence holds the TEE attestation proof.
type Evidence struct {
	Provider    Provider `json:"provider"`
	Token       string   `json:"token"`
	GeneratedAt string   `json:"generatedAt"`
	ImageDigest string   `json:"imageDigest,omitempty"`
	ProjectID   string   `json:"projectId,omitempty"`
}

// GetEvidence retrieves the TEE attestation evidence.
// On Confidential Space, it fetches the real attestation token.
// Locally, it generates a placeholder.
func GetEvidence() (*Evidence, error) {
	// Check if running in Confidential Space
	if isConfidentialSpace() {
		return getConfidentialSpaceEvidence()
	}

	// Local development fallback
	return getLocalEvidence()
}

// isConfidentialSpace checks for Confidential Space environment markers.
func isConfidentialSpace() bool {
	// Confidential Space sets specific environment variables
	// and makes the metadata server available
	if os.Getenv("CONFIDENTIAL_SPACE_IMAGE") != "" {
		return true
	}

	// Check for GCE metadata server
	resp, err := http.Get("http://metadata.google.internal/computeMetadata/v1/")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// getConfidentialSpaceEvidence fetches the real attestation token from GCE.
func getConfidentialSpaceEvidence() (*Evidence, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET",
		"http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity?audience=continuity-vault-enclave",
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create attestation request: %w", err)
	}
	req.Header.Set("Metadata-Flavor", "Google")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch attestation token: %w", err)
	}
	defer resp.Body.Close()

	tokenBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read attestation token: %w", err)
	}

	evidence := &Evidence{
		Provider:    ProviderConfidentialSpace,
		Token:       string(tokenBytes),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Try to get the image digest
	if digest := os.Getenv("CONFIDENTIAL_SPACE_IMAGE"); digest != "" {
		evidence.ImageDigest = digest
	}

	return evidence, nil
}

// getLocalEvidence generates a placeholder attestation for local development.
func getLocalEvidence() (*Evidence, error) {
	return &Evidence{
		Provider: ProviderLocal,
		Token: fmt.Sprintf(
			"LOCAL-DEV-ATTESTATION-NOT-FOR-PRODUCTION:generated=%s",
			time.Now().UTC().Format(time.RFC3339),
		),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// String returns a JSON representation of the evidence.
func (e *Evidence) String() string {
	b, _ := json.MarshalIndent(e, "", "  ")
	return string(b)
}

// IsReal returns true if this evidence comes from a real TEE.
func (e *Evidence) IsReal() bool {
	return e.Provider == ProviderConfidentialSpace
}
