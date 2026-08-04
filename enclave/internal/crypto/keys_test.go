package crypto

import (
	"bytes"
	"testing"
)

func TestNewEnclaveKeys(t *testing.T) {
	keys, err := NewEnclaveKeys()
	if err != nil {
		t.Fatalf("NewEnclaveKeys() error: %v", err)
	}

	if keys.signingKey == nil {
		t.Fatal("signing key should not be nil")
	}

	if keys.encryptionKey == [32]byte{} {
		t.Fatal("encryption key should not be zero")
	}
}

func TestGetKeyInfo(t *testing.T) {
	keys, err := NewEnclaveKeys()
	if err != nil {
		t.Fatalf("NewEnclaveKeys() error: %v", err)
	}

	info := keys.GetKeyInfo("test-attestation-evidence")

	if info.SigningAddress == "" {
		t.Error("signing address should not be empty")
	}
	if info.SigningPubKey == "" {
		t.Error("signing pub key should not be empty")
	}
	if info.EncryptionPubKey == "" {
		t.Error("encryption pub key should not be empty")
	}
	if info.AttestationEvidence != "test-attestation-evidence" {
		t.Error("attestation evidence mismatch")
	}

	// Address should be a valid hex string with 0x prefix
	if len(info.SigningAddress) != 42 {
		t.Errorf("unexpected address length: %d", len(info.SigningAddress))
	}
}

func TestEncryptDecrypt(t *testing.T) {
	keys, err := NewEnclaveKeys()
	if err != nil {
		t.Fatalf("NewEnclaveKeys() error: %v", err)
	}

	tests := []struct {
		name      string
		plaintext []byte
	}{
		{"empty", []byte{}},
		{"short", []byte("hello world")},
		{"json plan", []byte(`{"beneficiaries":[{"address":"rBeneficiary1","split":50},{"address":"rBeneficiary2","split":50}],"quorumThreshold":2}`)},
		{"large", bytes.Repeat([]byte("A"), 10000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := keys.Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt() error: %v", err)
			}

			// Ciphertext should be different from plaintext
			if len(tt.plaintext) > 0 && bytes.Equal(ciphertext, tt.plaintext) {
				t.Error("ciphertext should differ from plaintext")
			}

			decrypted, err := keys.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt() error: %v", err)
			}

			if !bytes.Equal(decrypted, tt.plaintext) {
				t.Errorf("decrypted data mismatch: got %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func TestDecryptTampered(t *testing.T) {
	keys, err := NewEnclaveKeys()
	if err != nil {
		t.Fatalf("NewEnclaveKeys() error: %v", err)
	}

	ciphertext, err := keys.Encrypt([]byte("sensitive plan data"))
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	// Tamper with the ciphertext
	ciphertext[len(ciphertext)-1] ^= 0xFF

	_, err = keys.Decrypt(ciphertext)
	if err == nil {
		t.Error("Decrypt() should fail on tampered ciphertext")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	keys1, err := NewEnclaveKeys()
	if err != nil {
		t.Fatalf("NewEnclaveKeys() keys1 error: %v", err)
	}
	keys2, err := NewEnclaveKeys()
	if err != nil {
		t.Fatalf("NewEnclaveKeys() keys2 error: %v", err)
	}

	ciphertext, err := keys1.Encrypt([]byte("secret data"))
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	_, err = keys2.Decrypt(ciphertext)
	if err == nil {
		t.Error("Decrypt() with wrong key should fail")
	}
}

func TestSignQuorumResult(t *testing.T) {
	keys, err := NewEnclaveKeys()
	if err != nil {
		t.Fatalf("NewEnclaveKeys() error: %v", err)
	}

	sig, err := keys.SignQuorumResult(1, true, 1722600000)
	if err != nil {
		t.Fatalf("SignQuorumResult() error: %v", err)
	}

	// Signature should be 65 bytes (r=32, s=32, v=1)
	if len(sig) != 65 {
		t.Errorf("unexpected signature length: %d, want 65", len(sig))
	}

	// v should be 27 or 28
	if sig[64] != 27 && sig[64] != 28 {
		t.Errorf("unexpected v value: %d", sig[64])
	}

	// Signing the same message twice should produce deterministic-ish results
	// (Ethereum signs use a random k, so signatures differ, but both should verify)
	sig2, err := keys.SignQuorumResult(1, true, 1722600000)
	if err != nil {
		t.Fatalf("SignQuorumResult() second call error: %v", err)
	}
	if len(sig2) != 65 {
		t.Errorf("second signature unexpected length: %d", len(sig2))
	}
}
