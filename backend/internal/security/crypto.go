package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
	"os"
)

// EncryptionKey represents a 32-byte AES-256 key
type EncryptionKey [32]byte

// GenerateKey creates a cryptographically secure random encryption key
func GenerateKey() (*EncryptionKey, error) {
	var key EncryptionKey
	if _, err := io.ReadFull(rand.Reader, key[:]); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}
	return &key, nil
}

// EncryptFile encrypts a file using AES-256-GCM and saves it to destPath
// Returns the encryption key used (needed for decryption)
func EncryptFile(srcPath, destPath string) (*EncryptionKey, error) {
	// Generate encryption key
	key, err := GenerateKey()
	if err != nil {
		return nil, err
	}

	// Read source file
	plaintext, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read source file: %w", err)
	}

	// Encrypt data
	ciphertext, err := EncryptData(plaintext, key)
	if err != nil {
		return nil, err
	}

	// Write encrypted file
	if err := os.WriteFile(destPath, ciphertext, 0600); err != nil {
		return nil, fmt.Errorf("failed to write encrypted file: %w", err)
	}

	return key, nil
}

// DecryptFile decrypts a file and returns the plaintext data
// File is NOT written to disk - returned in memory only
func DecryptFile(srcPath string, key *EncryptionKey) ([]byte, error) {
	// Read encrypted file
	ciphertext, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read encrypted file: %w", err)
	}

	// Decrypt data
	plaintext, err := DecryptData(ciphertext, key)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// EncryptData encrypts data using AES-256-GCM
func EncryptData(plaintext []byte, key *EncryptionKey) ([]byte, error) {
	// Create cipher block
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate
	// Format: [nonce][ciphertext+tag]
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	return ciphertext, nil
}

// DecryptData decrypts data using AES-256-GCM
func DecryptData(ciphertext []byte, key *EncryptionKey) ([]byte, error) {
	// Create cipher block
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Extract nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt and verify authentication tag
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed (tampered or wrong key): %w", err)
	}

	return plaintext, nil
}

// SecureDeleteFile deletes a file and attempts to overwrite its contents first
// Note: This is best-effort on modern filesystems with journaling/SSDs
func SecureDeleteFile(path string) error {
	// Get file info
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// Open file for writing
	file, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	// Overwrite with zeros
	zeros := make([]byte, info.Size())
	if _, err := file.Write(zeros); err != nil {
		file.Close()
		return err
	}

	// Sync to disk
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}

	// Overwrite with random data
	random := make([]byte, info.Size())
	if _, err := io.ReadFull(rand.Reader, random); err != nil {
		file.Close()
		return err
	}

	if _, err := file.Seek(0, 0); err != nil {
		file.Close()
		return err
	}

	if _, err := file.Write(random); err != nil {
		file.Close()
		return err
	}

	// Final sync
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}

	file.Close()

	// Delete file
	return os.Remove(path)
}
