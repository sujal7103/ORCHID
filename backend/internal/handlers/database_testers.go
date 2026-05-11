package handlers

import (
	"clara-agents/internal/models"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// testMongoDBWithDriver tests MongoDB connection using the official driver
func testMongoDBWithDriver(ctx context.Context, connectionString, database string) *models.TestCredentialResponse {
	// Set client options with timeout
	clientOptions := options.Client().
		ApplyURI(connectionString).
		SetConnectTimeout(10 * time.Second).
		SetServerSelectionTimeout(10 * time.Second)

	// Connect to MongoDB
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return &models.TestCredentialResponse{
			Success: false,
			Message: "Failed to create MongoDB client",
			Details: err.Error(),
		}
	}
	defer func() {
		disconnectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		client.Disconnect(disconnectCtx)
	}()

	// Ping the database to verify connection
	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return &models.TestCredentialResponse{
			Success: false,
			Message: "Failed to connect to MongoDB",
			Details: err.Error(),
		}
	}

	// Try to access the specified database and list collections
	db := client.Database(database)
	collections, err := db.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return &models.TestCredentialResponse{
			Success: false,
			Message: "Connected but failed to access database",
			Details: fmt.Sprintf("Database '%s': %s", database, err.Error()),
		}
	}

	// Get server info
	var serverStatus bson.M
	err = db.RunCommand(ctx, bson.D{{Key: "serverStatus", Value: 1}}).Decode(&serverStatus)

	details := fmt.Sprintf("Database: %s\nCollections: %d", database, len(collections))

	if err == nil {
		if version, ok := serverStatus["version"].(string); ok {
			details = fmt.Sprintf("Server version: %s\n%s", version, details)
		}
	}

	if len(collections) > 0 && len(collections) <= 5 {
		details += fmt.Sprintf("\nCollections: %v", collections)
	} else if len(collections) > 5 {
		details += fmt.Sprintf("\nCollections (first 5): %v...", collections[:5])
	}

	return &models.TestCredentialResponse{
		Success: true,
		Message: "MongoDB connection successful!",
		Details: details,
	}
}

// testRedisWithDriver tests Redis connection using the official driver
func testRedisWithDriver(ctx context.Context, host, port, password, dbNum string) *models.TestCredentialResponse {
	// Parse database number
	db, err := strconv.Atoi(dbNum)
	if err != nil {
		db = 0
	}

	// Create Redis client
	addr := fmt.Sprintf("%s:%s", host, port)
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  10 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	})
	defer client.Close()

	// Test connection with PING
	pong, err := client.Ping(ctx).Result()
	if err != nil {
		// Provide more helpful error messages
		errMsg := err.Error()
		if password == "" && (contains(errMsg, "NOAUTH") || contains(errMsg, "AUTH")) {
			return &models.TestCredentialResponse{
				Success: false,
				Message: "Redis requires authentication",
				Details: "This Redis server requires a password. Please provide the password in credentials.",
			}
		}
		if contains(errMsg, "connection refused") {
			return &models.TestCredentialResponse{
				Success: false,
				Message: "Connection refused",
				Details: fmt.Sprintf("Could not connect to Redis at %s. Please verify the host and port are correct.", addr),
			}
		}
		if contains(errMsg, "timeout") || contains(errMsg, "deadline") {
			return &models.TestCredentialResponse{
				Success: false,
				Message: "Connection timed out",
				Details: fmt.Sprintf("Redis server at %s did not respond in time.", addr),
			}
		}
		return &models.TestCredentialResponse{
			Success: false,
			Message: "Failed to connect to Redis",
			Details: err.Error(),
		}
	}

	// Get server info
	info, err := client.Info(ctx, "server").Result()
	details := fmt.Sprintf("Address: %s\nDatabase: %d\nPing response: %s", addr, db, pong)

	if err == nil {
		// Parse server info for version
		version := parseRedisInfoField(info, "redis_version")
		if version != "" {
			details = fmt.Sprintf("Redis version: %s\n%s", version, details)
		}

		// Get memory info
		memInfo, memErr := client.Info(ctx, "memory").Result()
		if memErr == nil {
			usedMemory := parseRedisInfoField(memInfo, "used_memory_human")
			if usedMemory != "" {
				details += fmt.Sprintf("\nMemory used: %s", usedMemory)
			}
		}
	}

	// Get key count for current database
	dbSize, err := client.DBSize(ctx).Result()
	if err == nil {
		details += fmt.Sprintf("\nKeys in DB %d: %d", db, dbSize)
	}

	return &models.TestCredentialResponse{
		Success: true,
		Message: "Redis connection successful!",
		Details: details,
	}
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsIgnoreCase(s, substr))
}

func containsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldSlice(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFoldSlice(s1, s2 string) bool {
	if len(s1) != len(s2) {
		return false
	}
	for i := 0; i < len(s1); i++ {
		c1, c2 := s1[i], s2[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

// parseRedisInfoField extracts a field value from Redis INFO output
func parseRedisInfoField(info, field string) string {
	lines := splitLines(info)
	prefix := field + ":"
	for _, line := range lines {
		if len(line) > len(prefix) && line[:len(prefix)] == prefix {
			return trimSpace(line[len(prefix):])
		}
	}
	return ""
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r' || s[start] == '\n') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r' || s[end-1] == '\n') {
		end--
	}
	return s[start:end]
}
