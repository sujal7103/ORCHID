package security

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// Hash represents a SHA-256 hash (32 bytes)
type Hash [32]byte

// CalculateFileHash computes the SHA-256 hash of a file
func CalculateFileHash(path string) (*Hash, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var result Hash
	copy(result[:], hash.Sum(nil))
	return &result, nil
}

// CalculateDataHash computes the SHA-256 hash of byte data
func CalculateDataHash(data []byte) *Hash {
	hashArray := sha256.Sum256(data)
	hash := Hash(hashArray)
	return &hash
}

// String returns the hash as a hex string
func (h *Hash) String() string {
	return hex.EncodeToString(h[:])
}

// Bytes returns the hash as a byte slice
func (h *Hash) Bytes() []byte {
	return h[:]
}

// Equal compares two hashes using constant-time comparison
// This prevents timing attacks
func (h *Hash) Equal(other *Hash) bool {
	if other == nil {
		return false
	}
	return subtle.ConstantTimeCompare(h[:], other[:]) == 1
}

// FromHexString creates a Hash from a hex string
func FromHexString(s string) (*Hash, error) {
	bytes, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("invalid hex string: %w", err)
	}

	if len(bytes) != 32 {
		return nil, fmt.Errorf("invalid hash length: expected 32 bytes, got %d", len(bytes))
	}

	var hash Hash
	copy(hash[:], bytes)
	return &hash, nil
}

// Verify checks if the given data matches the hash
func (h *Hash) Verify(data []byte) bool {
	computed := CalculateDataHash(data)
	return h.Equal(computed)
}

// VerifyFile checks if the given file matches the hash
func (h *Hash) VerifyFile(path string) (bool, error) {
	computed, err := CalculateFileHash(path)
	if err != nil {
		return false, err
	}
	return h.Equal(computed), nil
}
