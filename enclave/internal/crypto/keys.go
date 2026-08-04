// Package crypto provides key generation, encryption, decryption, and signing
// for the Vault Enclave. On first boot inside the TEE, an ECDSA signing key
// and an X25519 encryption key are generated; the private keys never leave
// the enclave process.
//
// Design Principle #5: Confidentiality is a means, not the pitch.
// The plan — who, how much, when — is exactly what must stay sealed.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"sync"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// EnclaveKeys holds the enclave's cryptographic identity.
// Generated once on boot; the private keys never leave this process.
type EnclaveKeys struct {
	mu sync.RWMutex

	// signingKey is the ECDSA (secp256k1) key used to sign quorum results.
	// The VaultRegistry on-chain verifies against the corresponding public key.
	signingKey *ecdsa.PrivateKey

	// encryptionKey is a 256-bit symmetric key used to encrypt/decrypt sealed plans.
	// In production with full FCC, this would be an asymmetric X25519 key
	// for client-side encryption. For the MVP reference implementation,
	// we use AES-256-GCM with a key that only exists inside this TEE process.
	encryptionKey [32]byte
}

// KeyInfo contains the public parts of the enclave's keys for external consumption.
type KeyInfo struct {
	// SigningAddress is the Ethereum address derived from the signing public key.
	// This is registered on-chain as the enclave oracle address.
	SigningAddress string `json:"signingAddress"`

	// SigningPubKey is the hex-encoded compressed public key (secp256k1).
	SigningPubKey string `json:"signingPubKey"`

	// EncryptionPubKey is the hex-encoded public key for plan encryption.
	// In MVP, this is the SHA-256 hash of the internal symmetric key (acts as
	// a fingerprint — clients encrypt plans locally with a shared-secret
	// derived from this, or more practically for the MVP, plans are encrypted
	// by this service upon receipt).
	EncryptionPubKey string `json:"encryptionPubKey"`

	// AttestationEvidence is the remote attestation proof from the TEE platform.
	// In Google Cloud Confidential Space, this would be a signed attestation
	// token. For local development/testing, this contains a placeholder.
	AttestationEvidence string `json:"attestationEvidence"`
}

// NewEnclaveKeys generates fresh cryptographic keys on first boot.
// This MUST be called inside the TEE — the private keys never leave.
func NewEnclaveKeys() (*EnclaveKeys, error) {
	// Generate ECDSA signing key (secp256k1 — same curve as Ethereum)
	signingKey, err := ecdsa.GenerateKey(ethcrypto.S256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate signing key: %w", err)
	}

	// Generate 256-bit AES encryption key
	var encKey [32]byte
	if _, err := io.ReadFull(rand.Reader, encKey[:]); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}

	return &EnclaveKeys{
		signingKey:    signingKey,
		encryptionKey: encKey,
	}, nil
}

// GetKeyInfo returns the public parts of the enclave's keys.
func (k *EnclaveKeys) GetKeyInfo(attestationEvidence string) KeyInfo {
	k.mu.RLock()
	defer k.mu.RUnlock()

	address := ethcrypto.PubkeyToAddress(k.signingKey.PublicKey)
	pubKeyBytes := elliptic.MarshalCompressed(
		k.signingKey.PublicKey.Curve,
		k.signingKey.PublicKey.X,
		k.signingKey.PublicKey.Y,
	)

	// Create a fingerprint of the encryption key for external reference
	encKeyHash := sha256.Sum256(k.encryptionKey[:])

	return KeyInfo{
		SigningAddress:      address.Hex(),
		SigningPubKey:       hex.EncodeToString(pubKeyBytes),
		EncryptionPubKey:    hex.EncodeToString(encKeyHash[:]),
		AttestationEvidence: attestationEvidence,
	}
}

// SigningAddress returns the Ethereum address of the signing key.
func (k *EnclaveKeys) SigningAddress() string {
	k.mu.RLock()
	defer k.mu.RUnlock()
	return ethcrypto.PubkeyToAddress(k.signingKey.PublicKey).Hex()
}

// Encrypt encrypts data using AES-256-GCM with the enclave's symmetric key.
// The nonce is prepended to the ciphertext.
func (k *EnclaveKeys) Encrypt(plaintext []byte) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	block, err := aes.NewCipher(k.encryptionKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// nonce is prepended to the ciphertext
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts data encrypted with Encrypt.
func (k *EnclaveKeys) Decrypt(ciphertext []byte) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	block, err := aes.NewCipher(k.encryptionKey[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

// SignQuorumResult signs a quorum result message using EIP-191 personal_sign.
// The message format: keccak256(abi.encodePacked(vaultId, quorumMet, timestamp))
// The on-chain contract verifies this signature against the registered enclave address.
func (k *EnclaveKeys) SignQuorumResult(vaultID uint64, quorumMet bool, timestamp uint64) ([]byte, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()

	// Construct the message to sign (matching Solidity's abi.encodePacked)
	quorumByte := byte(0)
	if quorumMet {
		quorumByte = 1
	}

	// Pack: vaultId (uint256) + quorumMet (bool as uint8) + timestamp (uint256)
	msg := make([]byte, 0, 65)
	vaultIDBytes := new(big.Int).SetUint64(vaultID).Bytes()
	// Pad vaultId to 32 bytes
	padded := make([]byte, 32)
	copy(padded[32-len(vaultIDBytes):], vaultIDBytes)
	msg = append(msg, padded...)
	msg = append(msg, quorumByte)
	// Pad timestamp to 32 bytes
	tsBytes := new(big.Int).SetUint64(timestamp).Bytes()
	tsPadded := make([]byte, 32)
	copy(tsPadded[32-len(tsBytes):], tsBytes)
	msg = append(msg, tsPadded...)

	// Hash the message
	hash := ethcrypto.Keccak256(msg)

	// EIP-191 prefix
	prefixed := ethcrypto.Keccak256(
		[]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(hash))),
		hash,
	)

	// Sign
	sig, err := ethcrypto.Sign(prefixed, k.signingKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign: %w", err)
	}

	// Adjust v value for Ethereum (27 or 28)
	if sig[64] < 27 {
		sig[64] += 27
	}

	return sig, nil
}
