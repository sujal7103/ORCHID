package security

import (
	"crypto/rand"
	"runtime"
	"sync"
)

// SecureString holds sensitive string data that can be securely wiped
type SecureString struct {
	data []byte
	mu   sync.Mutex
}

// NewSecureString creates a new SecureString from a regular string
func NewSecureString(s string) *SecureString {
	ss := &SecureString{
		data: []byte(s),
	}
	// Set finalizer to wipe memory when garbage collected
	runtime.SetFinalizer(ss, func(s *SecureString) {
		s.Wipe()
	})
	return ss
}

// String returns the string value (use sparingly)
func (s *SecureString) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		return ""
	}
	return string(s.data)
}

// Bytes returns the byte slice (use sparingly)
func (s *SecureString) Bytes() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		return nil
	}
	// Return a copy to prevent external modification
	result := make([]byte, len(s.data))
	copy(result, s.data)
	return result
}

// Len returns the length of the string
func (s *SecureString) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.data == nil {
		return 0
	}
	return len(s.data)
}

// IsEmpty returns true if the string is empty or wiped
func (s *SecureString) IsEmpty() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data == nil || len(s.data) == 0
}

// Wipe securely erases the string data from memory
func (s *SecureString) Wipe() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data == nil {
		return
	}

	// Three-pass wipe: zeros, random, zeros
	// Pass 1: Overwrite with zeros
	for i := range s.data {
		s.data[i] = 0
	}

	// Pass 2: Overwrite with random data
	if len(s.data) > 0 {
		random := make([]byte, len(s.data))
		rand.Read(random)
		copy(s.data, random)
		// Wipe the random buffer too
		for i := range random {
			random[i] = 0
		}
	}

	// Pass 3: Overwrite with zeros again
	for i := range s.data {
		s.data[i] = 0
	}

	// Clear the slice
	s.data = nil
}

// WipeBytes securely wipes a byte slice
func WipeBytes(data []byte) {
	if data == nil {
		return
	}

	// Pass 1: Zeros
	for i := range data {
		data[i] = 0
	}

	// Pass 2: Random
	if len(data) > 0 {
		random := make([]byte, len(data))
		rand.Read(random)
		copy(data, random)
		// Wipe random buffer
		for i := range random {
			random[i] = 0
		}
	}

	// Pass 3: Zeros
	for i := range data {
		data[i] = 0
	}
}

// WipeString securely wipes a string by converting to byte slice
// Note: Strings in Go are immutable, so this creates a copy
// Use SecureString for better security
func WipeString(s *string) {
	if s == nil || *s == "" {
		return
	}
	data := []byte(*s)
	WipeBytes(data)
	*s = ""
}

// SecureBuffer is a buffer that automatically wipes its contents
type SecureBuffer struct {
	data []byte
	mu   sync.Mutex
}

// NewSecureBuffer creates a new secure buffer with given size
func NewSecureBuffer(size int) *SecureBuffer {
	sb := &SecureBuffer{
		data: make([]byte, size),
	}
	runtime.SetFinalizer(sb, func(b *SecureBuffer) {
		b.Wipe()
	})
	return sb
}

// Write writes data to the buffer
func (b *SecureBuffer) Write(data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(data) <= len(b.data) {
		copy(b.data, data)
	}
}

// Read reads data from the buffer
func (b *SecureBuffer) Read() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]byte, len(b.data))
	copy(result, b.data)
	return result
}

// Wipe securely erases the buffer
func (b *SecureBuffer) Wipe() {
	b.mu.Lock()
	defer b.mu.Unlock()
	WipeBytes(b.data)
	b.data = nil
}
