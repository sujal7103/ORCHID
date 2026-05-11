package security

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// OAuthState represents a stored OAuth state token
type OAuthState struct {
	UserID    string
	Service   string // "gmail" or "googlesheets"
	ExpiresAt time.Time
}

// OAuthStateStore manages OAuth state tokens with expiration
type OAuthStateStore struct {
	states map[string]*OAuthState
	mutex  sync.RWMutex
}

// NewOAuthStateStore creates a new OAuth state store
func NewOAuthStateStore() *OAuthStateStore {
	store := &OAuthStateStore{
		states: make(map[string]*OAuthState),
	}

	// Start cleanup goroutine to remove expired states every minute
	go store.cleanupExpired()

	return store
}

// GenerateState generates a cryptographically secure random state token
func (s *OAuthStateStore) GenerateState(userID, service string) (string, error) {
	// Generate 32 random bytes (256 bits)
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}

	stateToken := hex.EncodeToString(randomBytes)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Store state with 10-minute expiration
	s.states[stateToken] = &OAuthState{
		UserID:    userID,
		Service:   service,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}

	return stateToken, nil
}

// ValidateState validates a state token and returns the associated user ID
// The state token is consumed (one-time use) after validation
func (s *OAuthStateStore) ValidateState(stateToken string) (string, string, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	state, exists := s.states[stateToken]
	if !exists {
		return "", "", errors.New("invalid or expired state token")
	}

	// Check expiration
	if time.Now().After(state.ExpiresAt) {
		delete(s.states, stateToken)
		return "", "", errors.New("state token expired")
	}

	// Delete state token (one-time use for CSRF protection)
	userID := state.UserID
	service := state.Service
	delete(s.states, stateToken)

	return userID, service, nil
}

// cleanupExpired removes expired state tokens every minute
func (s *OAuthStateStore) cleanupExpired() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.mutex.Lock()
		now := time.Now()
		for token, state := range s.states {
			if now.After(state.ExpiresAt) {
				delete(s.states, token)
			}
		}
		s.mutex.Unlock()
	}
}

// Count returns the number of active state tokens (for monitoring)
func (s *OAuthStateStore) Count() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.states)
}
