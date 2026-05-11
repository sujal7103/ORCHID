package models

import "time"

// DeviceAuthRequest represents a pending device authorization request
// Stored in MongoDB with TTL index for automatic expiration
type DeviceAuthRequest struct {
	ID             string           `bson:"_id,omitempty" json:"id"`
	DeviceCode     string           `bson:"deviceCode" json:"device_code"`   // 32-char hex, shown to CLI
	UserCode       string           `bson:"userCode" json:"user_code"`       // 8-char alphanumeric (XXXX-XXXX)
	ClientInfo     DeviceClientInfo `bson:"clientInfo" json:"client_info"`
	Status         string           `bson:"status" json:"status"`            // pending, authorized, expired, denied
	UserID         string           `bson:"userId,omitempty" json:"user_id"` // Set when authorized
	UserEmail      string           `bson:"userEmail,omitempty" json:"user_email"`
	IPAddress      string           `bson:"ipAddress" json:"ip_address"`
	UserAgent      string           `bson:"userAgent" json:"user_agent"`
	CreatedAt      time.Time        `bson:"createdAt" json:"created_at"`
	ExpiresAt      time.Time        `bson:"expiresAt" json:"expires_at"` // TTL index
	AuthorizedAt   *time.Time       `bson:"authorizedAt,omitempty" json:"authorized_at,omitempty"`
	LastPollAt     *time.Time       `bson:"lastPollAt,omitempty" json:"last_poll_at,omitempty"`
	PollCount      int              `bson:"pollCount" json:"poll_count"`
	FailedAttempts int              `bson:"failedAttempts" json:"failed_attempts"` // For brute force protection
}

// DeviceClientInfo contains information about the CLI client
type DeviceClientInfo struct {
	ClientID string `bson:"clientId" json:"client_id"`
	Version  string `bson:"version" json:"version"`
	Platform string `bson:"platform" json:"platform"` // darwin, linux, windows
}

// UserDevice represents a connected device for a user
// Stored in MongoDB for persistent device management
type UserDevice struct {
	ID               string     `bson:"_id,omitempty" json:"id"`
	UserID           string     `bson:"userId" json:"user_id"`
	DeviceID         string     `bson:"deviceId" json:"device_id"`           // Unique device identifier (UUID)
	Name             string     `bson:"name" json:"name"`                    // User-editable name
	Platform         string     `bson:"platform" json:"platform"`            // darwin, linux, windows
	ClientVersion    string     `bson:"clientVersion" json:"client_version"`
	RefreshTokenHash string     `bson:"refreshTokenHash" json:"-"`           // SHA-256 hash for validation (never expose)
	IsActive         bool       `bson:"isActive" json:"is_active"`
	LastActiveAt     time.Time  `bson:"lastActiveAt" json:"last_active_at"`
	LastIPAddress    string     `bson:"lastIpAddress" json:"last_ip_address"`
	LastLocation     string     `bson:"lastLocation,omitempty" json:"last_location,omitempty"` // Geo-approximation
	CreatedAt        time.Time  `bson:"createdAt" json:"created_at"`
	UpdatedAt        time.Time  `bson:"updatedAt" json:"updated_at"`
	RevokedAt        *time.Time `bson:"revokedAt,omitempty" json:"revoked_at,omitempty"`
}

// DeviceAuthAuditLog for tracking device auth events
type DeviceAuthAuditLog struct {
	ID        string                 `bson:"_id,omitempty" json:"id"`
	EventType string                 `bson:"eventType" json:"event_type"` // device_code_generated, user_code_entered, device_authorized, device_revoked, token_refreshed
	UserID    string                 `bson:"userId,omitempty" json:"user_id,omitempty"`
	DeviceID  string                 `bson:"deviceId,omitempty" json:"device_id,omitempty"`
	Metadata  map[string]interface{} `bson:"metadata" json:"metadata"`
	CreatedAt time.Time              `bson:"createdAt" json:"created_at"`
}

// Device auth status constants
const (
	DeviceAuthStatusPending    = "pending"
	DeviceAuthStatusAuthorized = "authorized"
	DeviceAuthStatusExpired    = "expired"
	DeviceAuthStatusDenied     = "denied"
)

// Device auth event types
const (
	DeviceAuthEventCodeGenerated      = "device_code_generated"
	DeviceAuthEventCodeEntered        = "user_code_entered"
	DeviceAuthEventAuthorized         = "device_authorized"
	DeviceAuthEventRevoked            = "device_revoked"
	DeviceAuthEventTokenRefreshed     = "token_refreshed"
	DeviceAuthEventLoginFailed        = "login_failed"
	DeviceAuthEventSuspiciousActivity = "suspicious_activity"
)

// DeviceCodeResponse is the response when generating a device code
type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"` // seconds
	Interval                int    `json:"interval"`   // polling interval in seconds
}

// DeviceTokenResponse is the response when polling for token succeeds
type DeviceTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"` // seconds
	RefreshToken string `json:"refresh_token"`
	DeviceID     string `json:"device_id"`
	User         struct {
		ID    string `json:"id"`
		Email string `json:"email"`
	} `json:"user"`
}

// DeviceTokenErrorResponse is returned when token polling fails
type DeviceTokenErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	Interval         int    `json:"interval,omitempty"` // For slow_down errors
}

// Device token error codes (RFC 8628)
const (
	DeviceTokenErrorAuthorizationPending = "authorization_pending"
	DeviceTokenErrorSlowDown             = "slow_down"
	DeviceTokenErrorAccessDenied         = "access_denied"
	DeviceTokenErrorExpiredToken          = "expired_token"
	DeviceTokenErrorInvalidGrant         = "invalid_grant"
)

// DeviceAuthorizeRequest is the request to authorize a device
type DeviceAuthorizeRequest struct {
	UserCode string `json:"user_code"`
}

// DeviceAuthorizeResponse is the response after authorizing
type DeviceAuthorizeResponse struct {
	Success    bool              `json:"success"`
	DeviceInfo *DeviceClientInfo `json:"device_info,omitempty"`
}

// DeviceListResponse is the list of user's devices
type DeviceListResponse struct {
	Devices []UserDeviceInfo `json:"devices"`
}

// UserDeviceInfo is the public info about a device
type UserDeviceInfo struct {
	ID            string    `json:"id"`
	DeviceID      string    `json:"device_id"`
	Name          string    `json:"name"`
	Platform      string    `json:"platform"`
	ClientVersion string    `json:"client_version"`
	IsActive      bool      `json:"is_active"`
	IsCurrent     bool      `json:"is_current"` // True if this is the device making the request
	LastActiveAt  time.Time `json:"last_active_at"`
	LastIPAddress string    `json:"last_ip_address"`
	LastLocation  string    `json:"last_location,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// DeviceUpdateRequest is the request to update device info
type DeviceUpdateRequest struct {
	Name string `json:"name"`
}

// DeviceRefreshRequest is the request to refresh access token
type DeviceRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
	DeviceID     string `json:"device_id"`
}
