package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"clara-agents/internal/models"
)

// Config holds all application configuration
type Config struct {
	Port        string
	DatabaseURL string // MySQL DSN
	MongoDBURI  string // MongoDB connection URI
	RedisURL    string
	SearXNGURL  string
	SupabaseURL string
	SupabaseKey string

	// JWT auth (standalone)
	JWTSecret               string
	JWTAccessTokenExpiry    time.Duration
	JWTRefreshTokenExpiry   time.Duration

	// CORS
	AllowedOrigins string
	FrontendURL    string
	BackendURL     string

	// Google OAuth (optional — requires Redis)
	GoogleClientID     string
	GoogleClientSecret string

	// Upload
	UploadDir string

	// DodoPayments configuration
	DodoAPIKey        string
	DodoWebhookSecret string
	DodoBusinessID    string
	DodoEnvironment   string // "live" or "test"

	// Promotional campaign configuration
	PromoEnabled   bool
	PromoStartDate time.Time
	PromoEndDate   time.Time
	PromoDuration  int // days

	// Superadmin configuration
	SuperadminUserIDs []string // List of user IDs with superadmin access
}

// Load loads configuration from environment variables with defaults
func Load() *Config {
	// Parse superadmin user IDs (comma-separated)
	superadminEnv := getEnv("SUPERADMIN_USER_IDS", "")
	var superadminUserIDs []string
	if superadminEnv != "" {
		superadminUserIDs = strings.Split(superadminEnv, ",")
		// Trim whitespace from each ID
		for i := range superadminUserIDs {
			superadminUserIDs[i] = strings.TrimSpace(superadminUserIDs[i])
		}
	}

	jwtAccessExpiry := getDurationEnv("JWT_ACCESS_TOKEN_EXPIRY", 15*time.Minute)
	jwtRefreshExpiry := getDurationEnv("JWT_REFRESH_TOKEN_EXPIRY", 168*time.Hour)

	return &Config{
		Port:        getEnv("PORT", "3001"),
		DatabaseURL: getEnv("DATABASE_URL", ""),
		MongoDBURI:  getEnv("MONGODB_URI", ""),
		RedisURL:    getEnv("REDIS_URL", ""),
		SearXNGURL:  getEnv("SEARXNG_URL", ""),
		SupabaseURL: getEnv("SUPABASE_URL", ""),
		SupabaseKey: getEnv("SUPABASE_KEY", ""),

		// JWT auth
		JWTSecret:             getEnv("JWT_SECRET", ""),
		JWTAccessTokenExpiry:  jwtAccessExpiry,
		JWTRefreshTokenExpiry: jwtRefreshExpiry,

		// CORS / URLs
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", ""),
		FrontendURL:    getEnv("FRONTEND_URL", "http://localhost:3000"),
		BackendURL:     getEnv("BACKEND_URL", "http://localhost:3001"),

		// Google OAuth
		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),

		// Upload
		UploadDir: getEnv("UPLOAD_DIR", "/app/uploads"),

		// DodoPayments configuration
		DodoAPIKey:        getEnv("DODO_API_KEY", ""),
		DodoWebhookSecret: getEnv("DODO_WEBHOOK_SECRET", ""),
		DodoBusinessID:    getEnv("DODO_BUSINESS_ID", ""),
		DodoEnvironment:   getEnv("DODO_ENVIRONMENT", "test"),

		// Promotional campaign configuration
		PromoEnabled:   getBoolEnv("PROMO_ENABLED", false),
		PromoStartDate: getTimeEnv("PROMO_START_DATE", "2026-01-01T00:00:00Z"),
		PromoEndDate:   getTimeEnv("PROMO_END_DATE", "2026-02-01T00:00:00Z"),
		PromoDuration:  getIntEnv("PROMO_DURATION_DAYS", 30),

		// Superadmin configuration
		SuperadminUserIDs: superadminUserIDs,
	}
}

// LoadProviders loads providers configuration from JSON file
func LoadProviders(filePath string) (*models.ProvidersConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read providers file: %w", err)
	}

	var config models.ProvidersConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse providers JSON: %w", err)
	}

	return &config, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getBoolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getIntEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return defaultValue
}

func getDurationEnv(key string, defaultValue time.Duration) time.Duration {
	value := getEnv(key, "")
	if value == "" {
		return defaultValue
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return defaultValue
	}
	return parsed
}

func getTimeEnv(key string, defaultValue string) time.Time {
	value := getEnv(key, defaultValue)
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		// If parsing fails, return zero time
		return time.Time{}
	}
	return parsed
}
