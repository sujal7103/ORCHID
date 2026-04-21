package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/argon2"
)

// User represents an authenticated user
type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

// ExtractToken extracts the JWT token from an Authorization header value.
// Supports "Bearer <token>" format.
func ExtractToken(authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("empty authorization header")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid authorization header format")
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errors.New("empty token")
	}

	return token, nil
}

// LocalJWTAuth handles local JWT-based authentication
type LocalJWTAuth struct {
	SecretKey          []byte
	AccessTokenExpiry  time.Duration // Default: 15 minutes
	RefreshTokenExpiry time.Duration // Default: 7 days
}

// NewLocalJWTAuth creates a new local JWT auth instance
func NewLocalJWTAuth(secretKey string, accessExpiry, refreshExpiry time.Duration) (*LocalJWTAuth, error) {
	if secretKey == "" {
		return nil, errors.New("JWT secret key cannot be empty")
	}

	if accessExpiry == 0 {
		accessExpiry = 15 * time.Minute
	}

	if refreshExpiry == 0 {
		refreshExpiry = 7 * 24 * time.Hour
	}

	return &LocalJWTAuth{
		SecretKey:          []byte(secretKey),
		AccessTokenExpiry:  accessExpiry,
		RefreshTokenExpiry: refreshExpiry,
	}, nil
}

// JWTClaims represents the JWT token claims
type JWTClaims struct {
	UserID  string `json:"sub"`
	Email   string `json:"email"`
	Role    string `json:"role"`
	TokenID string `json:"jti"` // For refresh token tracking
	jwt.RegisteredClaims
}

// GenerateTokens generates both access and refresh tokens
func (a *LocalJWTAuth) GenerateTokens(userID, email, role string) (accessToken, refreshToken string, err error) {
	// Generate unique token ID for refresh token
	tokenID, err := generateTokenID()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate token ID: %w", err)
	}

	// Access token (short-lived)
	accessClaims := JWTClaims{
		UserID: userID,
		Email:  email,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.AccessTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "orchid-local",
		},
	}

	accessTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessToken, err = accessTokenObj.SignedString(a.SecretKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign access token: %w", err)
	}

	// Refresh token (long-lived)
	refreshClaims := JWTClaims{
		UserID:  userID,
		Email:   email,
		Role:    role,
		TokenID: tokenID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(a.RefreshTokenExpiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "orchid-local",
		},
	}

	refreshTokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	refreshToken, err = refreshTokenObj.SignedString(a.SecretKey)
	if err != nil {
		return "", "", fmt.Errorf("failed to sign refresh token: %w", err)
	}

	return accessToken, refreshToken, nil
}

// VerifyAccessToken verifies an access token and returns the user
func (a *LocalJWTAuth) VerifyAccessToken(tokenString string) (*User, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.SecretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return &User{
			ID:    claims.UserID,
			Email: claims.Email,
			Role:  claims.Role,
		}, nil
	}

	return nil, errors.New("invalid token")
}

// VerifyRefreshToken verifies a refresh token and returns claims
func (a *LocalJWTAuth) VerifyRefreshToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return a.SecretKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse refresh token: %w", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid refresh token")
}

// Argon2 password hashing parameters (OWASP recommended)
const (
	argon2Time      = 3           // Number of iterations
	argon2Memory    = 64 * 1024   // 64MB
	argon2Threads   = 4           // Parallelism
	argon2KeyLength = 32          // 32 bytes (256 bits)
	saltLength      = 16          // 16 bytes salt
)

// HashPassword hashes a password using Argon2id
func (a *LocalJWTAuth) HashPassword(password string) (string, error) {
	// Generate random salt
	salt := make([]byte, saltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	// Hash password with Argon2id
	hash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLength)

	// Encode salt and hash to base64
	saltEncoded := base64.RawStdEncoding.EncodeToString(salt)
	hashEncoded := base64.RawStdEncoding.EncodeToString(hash)

	// Format: argon2id$salt$hash
	return fmt.Sprintf("argon2id$%s$%s", saltEncoded, hashEncoded), nil
}

// VerifyPassword verifies a password against an Argon2id hash
func (a *LocalJWTAuth) VerifyPassword(hashedPassword, password string) (bool, error) {
	// Parse hash format: argon2id$salt$hash
	parts := []byte(hashedPassword)
	if len(parts) < 10 || string(parts[:9]) != "argon2id$" {
		return false, fmt.Errorf("invalid hash format: missing argon2id prefix")
	}

	// Split by $ delimiter
	hashParts := []string{}
	start := 9 // Skip "argon2id$"
	for i := start; i < len(parts); i++ {
		if parts[i] == '$' {
			hashParts = append(hashParts, string(parts[start:i]))
			start = i + 1
		}
	}
	// Add the last part
	if start < len(parts) {
		hashParts = append(hashParts, string(parts[start:]))
	}

	if len(hashParts) != 2 {
		return false, fmt.Errorf("invalid hash format: expected 2 parts, got %d", len(hashParts))
	}

	saltEncoded := hashParts[0]
	hashEncoded := hashParts[1]

	// Decode salt and hash from base64
	salt, err := base64.RawStdEncoding.DecodeString(saltEncoded)
	if err != nil {
		return false, fmt.Errorf("failed to decode salt: %w", err)
	}

	expectedHash, err := base64.RawStdEncoding.DecodeString(hashEncoded)
	if err != nil {
		return false, fmt.Errorf("failed to decode hash: %w", err)
	}

	// Hash provided password with same salt
	actualHash := argon2.IDKey([]byte(password), salt, argon2Time, argon2Memory, argon2Threads, argon2KeyLength)

	// Constant-time comparison
	if len(actualHash) != len(expectedHash) {
		return false, nil
	}

	var equal byte = 0
	for i := 0; i < len(actualHash); i++ {
		equal |= actualHash[i] ^ expectedHash[i]
	}

	return equal == 0, nil
}

// generateTokenID generates a random token ID for refresh tokens
func generateTokenID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// ValidatePassword checks if password meets requirements
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must be at least 8 characters long")
	}

	var (
		hasUpper   bool
		hasLower   bool
		hasNumber  bool
		hasSpecial bool
	)

	for _, char := range password {
		switch {
		case 'A' <= char && char <= 'Z':
			hasUpper = true
		case 'a' <= char && char <= 'z':
			hasLower = true
		case '0' <= char && char <= '9':
			hasNumber = true
		case char == '!' || char == '@' || char == '#' || char == '$' || char == '%' || char == '^' || char == '&' || char == '*':
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasNumber {
		return errors.New("password must contain at least one number")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character (!@#$%^&*)")
	}

	return nil
}
