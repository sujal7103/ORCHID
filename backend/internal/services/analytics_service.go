package services

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"

	"clara-agents/internal/database"
	"clara-agents/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// AnalyticsService handles minimal usage tracking (non-invasive)
type AnalyticsService struct {
	mongoDB *database.MongoDB
}

// NewAnalyticsService creates a new analytics service
func NewAnalyticsService(mongoDB *database.MongoDB) *AnalyticsService {
	return &AnalyticsService{
		mongoDB: mongoDB,
	}
}

// ChatSessionAnalytics stores minimal chat session data
type ChatSessionAnalytics struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID         string             `bson:"userId" json:"userId"`
	ConversationID string             `bson:"conversationId" json:"conversationId"`
	SessionID      string             `bson:"sessionId" json:"sessionId"` // WebSocket connection ID
	
	// Minimal metrics
	MessageCount   int       `bson:"messageCount" json:"messageCount"`
	StartedAt      time.Time `bson:"startedAt" json:"startedAt"`
	EndedAt        *time.Time `bson:"endedAt,omitempty" json:"endedAt,omitempty"`
	DurationMs     int64     `bson:"durationMs,omitempty" json:"durationMs,omitempty"`
	
	// Optional context (if available)
	ModelID        string    `bson:"modelId,omitempty" json:"modelId,omitempty"`
	DisabledTools  bool      `bson:"disabledTools,omitempty" json:"disabledTools,omitempty"`
}

// AgentUsageAnalytics stores minimal agent execution context
type AgentUsageAnalytics struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID      string             `bson:"userId" json:"userId"`
	AgentID     string             `bson:"agentId" json:"agentId"`
	ExecutionID primitive.ObjectID `bson:"executionId" json:"executionId"`
	
	TriggerType string    `bson:"triggerType" json:"triggerType"` // chat, api, scheduled
	ExecutedAt  time.Time `bson:"executedAt" json:"executedAt"`
}

// TrackChatSessionStart records the start of a chat session
func (s *AnalyticsService) TrackChatSessionStart(ctx context.Context, sessionID, userID, conversationID string) error {
	if s.mongoDB == nil {
		return nil // Analytics disabled
	}
	
	session := &ChatSessionAnalytics{
		UserID:         userID,
		ConversationID: conversationID,
		SessionID:      sessionID,
		MessageCount:   0,
		StartedAt:      time.Now(),
	}
	
	_, err := s.collection("chat_sessions").InsertOne(ctx, session)
	if err != nil {
		log.Printf("⚠️  [ANALYTICS] Failed to track session start: %v", err)
		return err
	}
	
	return nil
}

// TrackChatSessionEnd records the end of a chat session
func (s *AnalyticsService) TrackChatSessionEnd(ctx context.Context, sessionID string, messageCount int) error {
	if s.mongoDB == nil {
		return nil
	}
	
	now := time.Now()
	
	// Find the session
	var session ChatSessionAnalytics
	err := s.collection("chat_sessions").FindOne(ctx, bson.M{"sessionId": sessionID}).Decode(&session)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Session wasn't tracked (maybe analytics added after session started)
			return nil
		}
		return err
	}
	
	durationMs := now.Sub(session.StartedAt).Milliseconds()
	
	// Update the session
	_, err = s.collection("chat_sessions").UpdateOne(
		ctx,
		bson.M{"sessionId": sessionID},
		bson.M{
			"$set": bson.M{
				"endedAt":      now,
				"messageCount": messageCount,
				"durationMs":   durationMs,
			},
		},
	)
	
	if err != nil {
		log.Printf("⚠️  [ANALYTICS] Failed to track session end: %v", err)
	}
	
	return err
}

// UpdateChatSessionModel updates the model used in a session
func (s *AnalyticsService) UpdateChatSessionModel(ctx context.Context, sessionID, modelID string, disabledTools bool) error {
	if s.mongoDB == nil {
		return nil
	}
	
	_, err := s.collection("chat_sessions").UpdateOne(
		ctx,
		bson.M{"sessionId": sessionID},
		bson.M{
			"$set": bson.M{
				"modelId":       modelID,
				"disabledTools": disabledTools,
			},
		},
	)
	
	return err
}

// TrackAgentUsage records when an agent is used
func (s *AnalyticsService) TrackAgentUsage(ctx context.Context, userID, agentID string, executionID primitive.ObjectID, triggerType string) error {
	if s.mongoDB == nil {
		return nil
	}
	
	usage := &AgentUsageAnalytics{
		UserID:      userID,
		AgentID:     agentID,
		ExecutionID: executionID,
		TriggerType: triggerType,
		ExecutedAt:  time.Now(),
	}
	
	_, err := s.collection("agent_usage").InsertOne(ctx, usage)
	if err != nil {
		log.Printf("⚠️  [ANALYTICS] Failed to track agent usage: %v", err)
	}
	
	return err
}

// collection returns a MongoDB collection
func (s *AnalyticsService) collection(name string) *mongo.Collection {
	return s.mongoDB.Database().Collection(name)
}

// loadProvidersConfig reads and parses the providers.json file
func (s *AnalyticsService) loadProvidersConfig() (*models.ProvidersConfig, error) {
	// Get the path to providers.json (relative to backend root)
	providersPath := filepath.Join("providers.json")

	// Read the file
	data, err := os.ReadFile(providersPath)
	if err != nil {
		log.Printf("⚠️  [ANALYTICS] Failed to read providers.json: %v", err)
		return nil, err
	}

	// Parse JSON
	var config models.ProvidersConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Printf("⚠️  [ANALYTICS] Failed to parse providers.json: %v", err)
		return nil, err
	}

	return &config, nil
}

// GetOverviewStats returns system overview statistics
func (s *AnalyticsService) GetOverviewStats(ctx context.Context) (map[string]interface{}, error) {
	if s.mongoDB == nil {
		return map[string]interface{}{
			"total_users":       0,
			"active_chats":      0,
			"total_messages":    0,
			"api_calls_today":   0,
			"active_providers":  0,
			"total_models":      0,
			"total_agents":      0,
			"agent_executions":  0,
			"agents_run_today":  0,
		}, nil
	}

	// Count total chat sessions (approximation of active chats)
	activeChats, _ := s.collection("chat_sessions").CountDocuments(ctx, bson.M{"endedAt": bson.M{"$exists": false}})

	// Count total messages from all sessions
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{"_id": nil, "totalMessages": bson.M{"$sum": "$messageCount"}}}},
	}
	cursor, err := s.collection("chat_sessions").Aggregate(ctx, pipeline)
	var totalMessages int64
	if err == nil {
		var results []bson.M
		if err := cursor.All(ctx, &results); err == nil && len(results) > 0 {
			if count, ok := results[0]["totalMessages"].(int32); ok {
				totalMessages = int64(count)
			} else if count, ok := results[0]["totalMessages"].(int64); ok {
				totalMessages = count
			}
		}
	}

	// Count unique users
	uniqueUsers, _ := s.collection("chat_sessions").Distinct(ctx, "userId", bson.M{})

	// Count API calls today
	today := time.Now().Truncate(24 * time.Hour)
	apiCallsToday, _ := s.collection("chat_sessions").CountDocuments(ctx, bson.M{"startedAt": bson.M{"$gte": today}})

	// Count total models from providers.json
	totalModels := 0
	providersConfig, err := s.loadProvidersConfig()
	if err == nil {
		for _, provider := range providersConfig.Providers {
			// Only count enabled providers
			if provider.Enabled {
				// Count models from ModelAliases map
				totalModels += len(provider.ModelAliases)
			}
		}
	}

	// Count active providers (providers that have been used in the last 30 days)
	activeProviders := 0
	thirtyDaysAgo := time.Now().Add(-30 * 24 * time.Hour)

	// Get distinct model IDs used in the last 30 days
	usedModels, err := s.collection("chat_sessions").Distinct(ctx, "modelId", bson.M{
		"modelId": bson.M{"$exists": true, "$ne": ""},
		"startedAt": bson.M{"$gte": thirtyDaysAgo},
	})

	if err == nil && providersConfig != nil {
		// Create a set of used model IDs for faster lookup
		usedModelSet := make(map[string]bool)
		for _, modelID := range usedModels {
			if modelStr, ok := modelID.(string); ok {
				usedModelSet[modelStr] = true
			}
		}

		// Check each provider to see if any of their models were used
		for _, provider := range providersConfig.Providers {
			if !provider.Enabled {
				continue
			}

			// Check if any model from this provider was used
			for modelAlias := range provider.ModelAliases {
				if usedModelSet[modelAlias] {
					activeProviders++
					break // Count provider only once
				}
			}
		}
	}

	// Count agent metrics
	totalAgentExecutions, _ := s.collection("agent_usage").CountDocuments(ctx, bson.M{})
	agentsRunToday, _ := s.collection("agent_usage").CountDocuments(ctx, bson.M{"executedAt": bson.M{"$gte": today}})

	// Count unique agents
	uniqueAgents, _ := s.collection("agent_usage").Distinct(ctx, "agentId", bson.M{})
	totalAgents := len(uniqueAgents)

	return map[string]interface{}{
		"total_users":       len(uniqueUsers),
		"active_chats":      activeChats,
		"total_messages":    totalMessages,
		"api_calls_today":   apiCallsToday,
		"active_providers":  activeProviders,
		"total_models":      totalModels,
		"total_agents":      totalAgents,
		"agent_executions":  totalAgentExecutions,
		"agents_run_today":  agentsRunToday,
	}, nil
}

// GetProviderAnalytics returns usage analytics per provider
func (s *AnalyticsService) GetProviderAnalytics(ctx context.Context) ([]map[string]interface{}, error) {
	if s.mongoDB == nil {
		return []map[string]interface{}{}, nil
	}

	// Group by model ID and aggregate usage
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"modelId": bson.M{"$exists": true, "$ne": ""}}}},
		{{Key: "$group", Value: bson.M{
			"_id":            "$modelId",
			"total_requests": bson.M{"$sum": 1},
			"last_used_at":   bson.M{"$max": "$startedAt"},
		}}},
		{{Key: "$sort", Value: bson.M{"total_requests": -1}}},
	}

	cursor, err := s.collection("chat_sessions").Aggregate(ctx, pipeline)
	if err != nil {
		return []map[string]interface{}{}, err
	}

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return []map[string]interface{}{}, err
	}

	analytics := make([]map[string]interface{}, 0, len(results))
	for _, result := range results {
		analytics = append(analytics, map[string]interface{}{
			"provider_id":    result["_id"],
			"provider_name":  result["_id"], // TODO: Resolve from providers.json
			"total_requests": result["total_requests"],
			"total_tokens":   0, // TODO: Track tokens
			"estimated_cost": nil,
			"active_models":  []string{},
			"last_used_at":   result["last_used_at"],
		})
	}

	return analytics, nil
}

// GetChatAnalytics returns chat usage statistics
func (s *AnalyticsService) GetChatAnalytics(ctx context.Context) (map[string]interface{}, error) {
	if s.mongoDB == nil {
		return map[string]interface{}{
			"total_chats":           0,
			"active_chats":          0,
			"total_messages":        0,
			"avg_messages_per_chat": 0.0,
			"chats_created_today":   0,
			"messages_sent_today":   0,
			"time_series":           []map[string]interface{}{},
		}, nil
	}

	totalChats, _ := s.collection("chat_sessions").CountDocuments(ctx, bson.M{})
	activeChats, _ := s.collection("chat_sessions").CountDocuments(ctx, bson.M{"endedAt": bson.M{"$exists": false}})

	// Total messages
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{"_id": nil, "totalMessages": bson.M{"$sum": "$messageCount"}}}},
	}
	cursor, _ := s.collection("chat_sessions").Aggregate(ctx, pipeline)
	var totalMessages int64
	var results []bson.M
	if cursor.All(ctx, &results) == nil && len(results) > 0 {
		if count, ok := results[0]["totalMessages"].(int32); ok {
			totalMessages = int64(count)
		} else if count, ok := results[0]["totalMessages"].(int64); ok {
			totalMessages = count
		}
	}

	avgMessages := 0.0
	if totalChats > 0 {
		avgMessages = float64(totalMessages) / float64(totalChats)
	}

	// Today's stats
	today := time.Now().Truncate(24 * time.Hour)
	chatsToday, _ := s.collection("chat_sessions").CountDocuments(ctx, bson.M{"startedAt": bson.M{"$gte": today}})

	// Get time series data for the last 30 days
	timeSeries, _ := s.getTimeSeriesData(ctx, 30)

	return map[string]interface{}{
		"total_chats":           totalChats,
		"active_chats":          activeChats,
		"total_messages":        totalMessages,
		"avg_messages_per_chat": avgMessages,
		"chats_created_today":   chatsToday,
		"messages_sent_today":   0, // TODO: Track per-day messages
		"time_series":           timeSeries,
	}, nil
}

// getTimeSeriesData returns daily statistics for the specified number of days
func (s *AnalyticsService) getTimeSeriesData(ctx context.Context, days int) ([]map[string]interface{}, error) {
	if s.mongoDB == nil {
		return []map[string]interface{}{}, nil
	}

	// Calculate start date
	startDate := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Truncate(24 * time.Hour)

	// Aggregate chats and messages by day
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"startedAt": bson.M{"$gte": startDate}}}},
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{
				"$dateToString": bson.M{
					"format": "%Y-%m-%d",
					"date":   "$startedAt",
				},
			},
			"chat_count":    bson.M{"$sum": 1},
			"message_count": bson.M{"$sum": "$messageCount"},
			"unique_users":  bson.M{"$addToSet": "$userId"},
		}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}

	cursor, err := s.collection("chat_sessions").Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("⚠️  [ANALYTICS] Failed to aggregate time series: %v", err)
		return []map[string]interface{}{}, err
	}

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return []map[string]interface{}{}, err
	}

	// Aggregate agent executions by day
	agentPipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"executedAt": bson.M{"$gte": startDate}}}},
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{
				"$dateToString": bson.M{
					"format": "%Y-%m-%d",
					"date":   "$executedAt",
				},
			},
			"agent_count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}

	agentCursor, err := s.collection("agent_usage").Aggregate(ctx, agentPipeline)
	var agentResults []bson.M
	if err == nil {
		agentCursor.All(ctx, &agentResults)
	}

	// Create a map of agent counts by date for easy lookup
	agentCountByDate := make(map[string]int64)
	for _, result := range agentResults {
		date, _ := result["_id"].(string)
		count := int64(0)
		if c, ok := result["agent_count"].(int32); ok {
			count = int64(c)
		} else if c, ok := result["agent_count"].(int64); ok {
			count = c
		}
		agentCountByDate[date] = count
	}

	// Convert to response format with agent data
	timeSeries := make([]map[string]interface{}, 0, len(results))
	for _, result := range results {
		date, _ := result["_id"].(string)
		chatCount := int64(0)
		messageCount := int64(0)
		uniqueUsers := 0

		if count, ok := result["chat_count"].(int32); ok {
			chatCount = int64(count)
		} else if count, ok := result["chat_count"].(int64); ok {
			chatCount = count
		}

		if count, ok := result["message_count"].(int32); ok {
			messageCount = int64(count)
		} else if count, ok := result["message_count"].(int64); ok {
			messageCount = count
		}

		if users, ok := result["unique_users"].(primitive.A); ok {
			uniqueUsers = len(users)
		}

		// Get agent count for this date
		agentCount := agentCountByDate[date]

		timeSeries = append(timeSeries, map[string]interface{}{
			"date":          date,
			"chat_count":    chatCount,
			"message_count": messageCount,
			"user_count":    uniqueUsers,
			"agent_count":   agentCount,
		})
	}

	// Fill in missing dates with zeros
	filledSeries := s.fillMissingDates(timeSeries, startDate, days)

	return filledSeries, nil
}

// fillMissingDates ensures all dates in the range have entries (with zeros if no data)
func (s *AnalyticsService) fillMissingDates(data []map[string]interface{}, startDate time.Time, days int) []map[string]interface{} {
	// Create a map of existing dates
	dataMap := make(map[string]map[string]interface{})
	for _, entry := range data {
		if date, ok := entry["date"].(string); ok {
			dataMap[date] = entry
		}
	}

	// Fill all dates
	result := make([]map[string]interface{}, 0, days)
	for i := 0; i < days; i++ {
		date := startDate.Add(time.Duration(i) * 24 * time.Hour)
		dateStr := date.Format("2006-01-02")

		if entry, exists := dataMap[dateStr]; exists {
			result = append(result, entry)
		} else {
			result = append(result, map[string]interface{}{
				"date":          dateStr,
				"chat_count":    0,
				"message_count": 0,
				"user_count":    0,
				"agent_count":   0,
			})
		}
	}

	return result
}

// EnsureIndexes creates indexes for analytics collections
func (s *AnalyticsService) EnsureIndexes(ctx context.Context) error {
	if s.mongoDB == nil {
		return nil
	}
	
	// Chat sessions indexes
	_, err := s.collection("chat_sessions").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "startedAt", Value: -1}}},
		{Keys: bson.D{{Key: "sessionId", Value: 1}}},
		{Keys: bson.D{{Key: "startedAt", Value: -1}}},
	})
	if err != nil {
		return err
	}
	
	// Agent usage indexes
	_, err = s.collection("agent_usage").Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "userId", Value: 1}, {Key: "executedAt", Value: -1}}},
		{Keys: bson.D{{Key: "agentId", Value: 1}, {Key: "executedAt", Value: -1}}},
		{Keys: bson.D{{Key: "executedAt", Value: -1}}},
	})
	
	log.Println("✅ [ANALYTICS] Indexes created")
	return err
}

// MigrateChatSessionTimestamps fixes existing chat sessions that don't have proper startedAt timestamps
// Uses the MongoDB ObjectID creation time as the startedAt value
func (s *AnalyticsService) MigrateChatSessionTimestamps(ctx context.Context) (int, error) {
	if s.mongoDB == nil {
		return 0, nil
	}

	log.Println("🔄 [ANALYTICS MIGRATION] Starting chat session timestamp migration...")

	// Find all sessions without startedAt or with zero time
	zeroTime := time.Time{}
	cursor, err := s.collection("chat_sessions").Find(ctx, bson.M{
		"$or": []bson.M{
			{"startedAt": bson.M{"$exists": false}},
			{"startedAt": zeroTime},
			{"startedAt": nil},
		},
	})
	if err != nil {
		log.Printf("❌ [ANALYTICS MIGRATION] Failed to query sessions: %v", err)
		return 0, err
	}
	defer cursor.Close(ctx)

	updatedCount := 0
	for cursor.Next(ctx) {
		var session ChatSessionAnalytics
		if err := cursor.Decode(&session); err != nil {
			log.Printf("⚠️  [ANALYTICS MIGRATION] Failed to decode session: %v", err)
			continue
		}

		// Extract timestamp from MongoDB ObjectID
		// ObjectID first 4 bytes are Unix timestamp
		createdAt := session.ID.Timestamp()

		// Update the session with the extracted timestamp
		_, err := s.collection("chat_sessions").UpdateOne(
			ctx,
			bson.M{"_id": session.ID},
			bson.M{"$set": bson.M{"startedAt": createdAt}},
		)
		if err != nil {
			log.Printf("⚠️  [ANALYTICS MIGRATION] Failed to update session %s: %v", session.ID.Hex(), err)
			continue
		}

		updatedCount++
	}

	if err := cursor.Err(); err != nil {
		log.Printf("❌ [ANALYTICS MIGRATION] Cursor error: %v", err)
		return updatedCount, err
	}

	log.Printf("✅ [ANALYTICS MIGRATION] Successfully migrated %d chat sessions with proper timestamps", updatedCount)
	return updatedCount, nil
}


// GetAgentAnalytics returns comprehensive agent activity analytics
func (s *AnalyticsService) GetAgentAnalytics(ctx context.Context) (map[string]interface{}, error) {
	if s.mongoDB == nil {
		return map[string]interface{}{
			"total_agents":      0,
			"deployed_agents":   0,
			"total_executions":  0,
			"active_schedules":  0,
			"executions_today":  0,
			"time_series":       []map[string]interface{}{},
		}, nil
	}

	// Count total agents
	totalAgents, _ := s.collection("agents").CountDocuments(ctx, bson.M{})

	// Count deployed agents (status = "deployed")
	deployedAgents, _ := s.collection("agents").CountDocuments(ctx, bson.M{"status": "deployed"})

	// Count total agent executions
	totalExecutions, _ := s.collection("agent_usage").CountDocuments(ctx, bson.M{})

	// Count active schedules
	activeSchedules, _ := s.collection("schedules").CountDocuments(ctx, bson.M{"enabled": true})

	// Count executions today
	today := time.Now().Truncate(24 * time.Hour)
	executionsToday, _ := s.collection("agent_usage").CountDocuments(ctx, bson.M{"executedAt": bson.M{"$gte": today}})

	// Get time series data for the last 30 days
	timeSeries, _ := s.getAgentTimeSeriesData(ctx, 30)

	return map[string]interface{}{
		"total_agents":      totalAgents,
		"deployed_agents":   deployedAgents,
		"total_executions":  totalExecutions,
		"active_schedules":  activeSchedules,
		"executions_today":  executionsToday,
		"time_series":       timeSeries,
	}, nil
}

// getAgentTimeSeriesData returns daily agent activity statistics
func (s *AnalyticsService) getAgentTimeSeriesData(ctx context.Context, days int) ([]map[string]interface{}, error) {
	if s.mongoDB == nil {
		return []map[string]interface{}{}, nil
	}

	startDate := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Truncate(24 * time.Hour)

	// Aggregate agents created by day
	agentsCreatedPipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"createdAt": bson.M{"$gte": startDate}}}},
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{
				"$dateToString": bson.M{
					"format": "%Y-%m-%d",
					"date":   "$createdAt",
				},
			},
			"agents_created": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}

	agentsCreatedCursor, err := s.collection("agents").Aggregate(ctx, agentsCreatedPipeline)
	var agentsCreatedResults []bson.M
	if err == nil {
		agentsCreatedCursor.All(ctx, &agentsCreatedResults)
	}

	// Aggregate agents deployed by day (when updatedAt changed to deployed status)
	agentsDeployedPipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{
			"status":    "deployed",
			"updatedAt": bson.M{"$gte": startDate},
		}}},
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{
				"$dateToString": bson.M{
					"format": "%Y-%m-%d",
					"date":   "$updatedAt",
				},
			},
			"agents_deployed": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}

	agentsDeployedCursor, err := s.collection("agents").Aggregate(ctx, agentsDeployedPipeline)
	var agentsDeployedResults []bson.M
	if err == nil {
		agentsDeployedCursor.All(ctx, &agentsDeployedResults)
	}

	// Aggregate agent executions by day
	agentRunsPipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"executedAt": bson.M{"$gte": startDate}}}},
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{
				"$dateToString": bson.M{
					"format": "%Y-%m-%d",
					"date":   "$executedAt",
				},
			},
			"agent_runs": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}

	agentRunsCursor, err := s.collection("agent_usage").Aggregate(ctx, agentRunsPipeline)
	var agentRunsResults []bson.M
	if err == nil {
		agentRunsCursor.All(ctx, &agentRunsResults)
	}

	// Aggregate schedules created by day
	schedulesCreatedPipeline := mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"createdAt": bson.M{"$gte": startDate}}}},
		{{Key: "$group", Value: bson.M{
			"_id": bson.M{
				"$dateToString": bson.M{
					"format": "%Y-%m-%d",
					"date":   "$createdAt",
				},
			},
			"schedules_created": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"_id": 1}}},
	}

	schedulesCreatedCursor, err := s.collection("schedules").Aggregate(ctx, schedulesCreatedPipeline)
	var schedulesCreatedResults []bson.M
	if err == nil {
		schedulesCreatedCursor.All(ctx, &schedulesCreatedResults)
	}

	// Create maps for easy lookup
	agentsCreatedByDate := make(map[string]int64)
	agentsDeployedByDate := make(map[string]int64)
	agentRunsByDate := make(map[string]int64)
	schedulesCreatedByDate := make(map[string]int64)

	for _, result := range agentsCreatedResults {
		date, _ := result["_id"].(string)
		count := extractInt64(result, "agents_created")
		agentsCreatedByDate[date] = count
	}

	for _, result := range agentsDeployedResults {
		date, _ := result["_id"].(string)
		count := extractInt64(result, "agents_deployed")
		agentsDeployedByDate[date] = count
	}

	for _, result := range agentRunsResults {
		date, _ := result["_id"].(string)
		count := extractInt64(result, "agent_runs")
		agentRunsByDate[date] = count
	}

	for _, result := range schedulesCreatedResults {
		date, _ := result["_id"].(string)
		count := extractInt64(result, "schedules_created")
		schedulesCreatedByDate[date] = count
	}

	// Fill all dates with data
	timeSeries := make([]map[string]interface{}, 0, days)
	for i := 0; i < days; i++ {
		date := startDate.Add(time.Duration(i) * 24 * time.Hour)
		dateStr := date.Format("2006-01-02")

		timeSeries = append(timeSeries, map[string]interface{}{
			"date":              dateStr,
			"agents_created":    agentsCreatedByDate[dateStr],
			"agents_deployed":   agentsDeployedByDate[dateStr],
			"agent_runs":        agentRunsByDate[dateStr],
			"schedules_created": schedulesCreatedByDate[dateStr],
		})
	}

	return timeSeries, nil
}

// GetModelAnalytics returns usage analytics per model, joined with MySQL model metadata
func (s *AnalyticsService) GetModelAnalytics(ctx context.Context, modelService *ModelService) ([]map[string]interface{}, error) {
	// Fetch all models from MySQL
	allModels, err := modelService.GetAll(false)
	if err != nil {
		return []map[string]interface{}{}, err
	}

	// Build a lookup map: modelID -> model
	modelMap := make(map[string]models.Model, len(allModels))
	for _, m := range allModels {
		modelMap[m.ID] = m
	}

	// Aggregate usage counts from MongoDB chat_sessions grouped by modelId
	usageCounts := make(map[string]int64)
	if s.mongoDB != nil {
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{"modelId": bson.M{"$exists": true, "$ne": ""}}}},
			{{Key: "$group", Value: bson.M{
				"_id":   "$modelId",
				"count": bson.M{"$sum": 1},
			}}},
		}
		cursor, err := s.collection("chat_sessions").Aggregate(ctx, pipeline)
		if err == nil {
			var results []bson.M
			if cursor.All(ctx, &results) == nil {
				for _, r := range results {
					if id, ok := r["_id"].(string); ok {
						usageCounts[id] = extractInt64(r, "count")
					}
				}
			}
		}
	}

	// Build result: one entry per model
	analytics := make([]map[string]interface{}, 0, len(allModels))
	for _, m := range allModels {
		var recommendationTier interface{} = nil // Model struct has no such field
		analytics = append(analytics, map[string]interface{}{
			"model_id":           m.ID,
			"model_name":         m.Name,
			"provider_name":      m.ProviderName,
			"usage_count":        usageCounts[m.ID],
			"agents_enabled":     m.AgentsEnabled,
			"recommendation_tier": recommendationTier,
		})
	}

	return analytics, nil
}

// GetCollectionStats returns raw document counts and daily series from major MongoDB collections
func (s *AnalyticsService) GetCollectionStats(ctx context.Context) (map[string]interface{}, error) {
	empty := map[string]interface{}{
		"totalUsers":          int64(0),
		"totalChats":          int64(0),
		"totalAgents":         int64(0),
		"totalExecutions":     int64(0),
		"totalWorkflows":      int64(0),
		"totalFeedback":       int64(0),
		"totalTelemetryEvents": int64(0),
		"totalSubscriptions":  int64(0),
		"userSignupsByDay":    []map[string]interface{}{},
		"chatsByDay":          []map[string]interface{}{},
		"executionsByDay":     []map[string]interface{}{},
		"topAgents":           []map[string]interface{}{},
	}

	if s.mongoDB == nil {
		return empty, nil
	}

	// Simple document counts
	totalUsers, _ := s.collection("users").CountDocuments(ctx, bson.M{})
	totalChats, _ := s.collection("conversations").CountDocuments(ctx, bson.M{})
	totalAgents, _ := s.collection("agents").CountDocuments(ctx, bson.M{})
	totalExecutions, _ := s.collection("executions").CountDocuments(ctx, bson.M{})
	totalWorkflows, _ := s.collection("workflows").CountDocuments(ctx, bson.M{})
	totalFeedback, _ := s.collection("feedback").CountDocuments(ctx, bson.M{})
	totalTelemetryEvents, _ := s.collection("telemetry_events").CountDocuments(ctx, bson.M{})
	totalSubscriptions, _ := s.collection("subscriptions").CountDocuments(ctx, bson.M{})

	// Daily series helper: group by dateField over last 30 days
	startDate := time.Now().Add(-30 * 24 * time.Hour).Truncate(24 * time.Hour)

	buildDailySeries := func(collName, dateField string) []map[string]interface{} {
		pipeline := mongo.Pipeline{
			{{Key: "$match", Value: bson.M{dateField: bson.M{"$gte": startDate}}}},
			{{Key: "$group", Value: bson.M{
				"_id": bson.M{
					"$dateToString": bson.M{"format": "%Y-%m-%d", "date": "$" + dateField},
				},
				"value": bson.M{"$sum": 1},
			}}},
			{{Key: "$sort", Value: bson.M{"_id": 1}}},
		}
		cursor, err := s.collection(collName).Aggregate(ctx, pipeline)
		if err != nil {
			return []map[string]interface{}{}
		}
		var results []bson.M
		if cursor.All(ctx, &results) != nil {
			return []map[string]interface{}{}
		}

		// Build a map keyed by date string then fill all 30 days
		byDate := make(map[string]int64, len(results))
		for _, r := range results {
			if d, ok := r["_id"].(string); ok {
				byDate[d] = extractInt64(r, "value")
			}
		}

		series := make([]map[string]interface{}, 0, 30)
		for i := 0; i < 30; i++ {
			d := startDate.Add(time.Duration(i) * 24 * time.Hour).Format("2006-01-02")
			series = append(series, map[string]interface{}{
				"date":  d,
				"value": byDate[d],
			})
		}
		return series
	}

	userSignupsByDay := buildDailySeries("users", "createdAt")
	chatsByDay := buildDailySeries("conversations", "createdAt")
	executionsByDay := buildDailySeries("executions", "startedAt")

	// Top 5 agents by execution count
	topAgentsPipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":   "$agentId",
			"count": bson.M{"$sum": 1},
		}}},
		{{Key: "$sort", Value: bson.M{"count": -1}}},
		{{Key: "$limit", Value: 5}},
		{{Key: "$lookup", Value: bson.M{
			"from":         "agents",
			"localField":   "_id",
			"foreignField": "_id",
			"as":           "agentInfo",
		}}},
	}
	topAgentsCursor, err := s.collection("executions").Aggregate(ctx, topAgentsPipeline)
	topAgents := []map[string]interface{}{}
	if err == nil {
		var topAgentsRaw []bson.M
		if topAgentsCursor.All(ctx, &topAgentsRaw) == nil {
			for _, r := range topAgentsRaw {
				name := ""
				if info, ok := r["agentInfo"].(primitive.A); ok && len(info) > 0 {
					if agentDoc, ok := info[0].(bson.M); ok {
						if n, ok := agentDoc["name"].(string); ok {
							name = n
						}
					}
				}
				if name == "" {
					// Fall back to agentId string
					if id, ok := r["_id"].(string); ok {
						name = id
					}
				}
				topAgents = append(topAgents, map[string]interface{}{
					"name":  name,
					"count": extractInt64(r, "count"),
				})
			}
		}
	}

	return map[string]interface{}{
		"totalUsers":           totalUsers,
		"totalChats":           totalChats,
		"totalAgents":          totalAgents,
		"totalExecutions":      totalExecutions,
		"totalWorkflows":       totalWorkflows,
		"totalFeedback":        totalFeedback,
		"totalTelemetryEvents": totalTelemetryEvents,
		"totalSubscriptions":   totalSubscriptions,
		"userSignupsByDay":     userSignupsByDay,
		"chatsByDay":           chatsByDay,
		"executionsByDay":      executionsByDay,
		"topAgents":            topAgents,
	}, nil
}

// extractInt64 safely extracts an int64 value from a bson.M result
func extractInt64(result bson.M, key string) int64 {
	if count, ok := result[key].(int32); ok {
		return int64(count)
	} else if count, ok := result[key].(int64); ok {
		return count
	}
	return 0
}

// UserListItemGDPR represents a GDPR-compliant user list item
type UserListItemGDPR struct {
	UserID         string    `json:"user_id"`
	EmailDomain    string    `json:"email_domain,omitempty"`
	Tier           string    `json:"tier"`
	CreatedAt      time.Time `json:"created_at"`
	LastActive     *time.Time `json:"last_active,omitempty"`
	TotalChats     int64     `json:"total_chats"`
	TotalMessages  int64     `json:"total_messages"`
	TotalAgentRuns int64     `json:"total_agent_runs"`
	HasOverrides   bool      `json:"has_overrides"`
}

// GetUserListGDPR returns a GDPR-compliant paginated user list
// Only includes aggregated analytics, no PII except anonymized user IDs and email domains
func (s *AnalyticsService) GetUserListGDPR(ctx context.Context, page, pageSize int, tierFilter, searchFilter string) ([]UserListItemGDPR, int64, error) {
	if s.mongoDB == nil {
		return []UserListItemGDPR{}, 0, nil
	}

	// Get unique users from chat sessions with their activity
	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":          "$userId",
			"total_chats":  bson.M{"$sum": 1},
			"total_messages": bson.M{"$sum": "$messageCount"},
			"last_active":  bson.M{"$max": "$startedAt"},
			"first_seen":   bson.M{"$min": "$startedAt"},
		}}},
	}

	cursor, err := s.collection("chat_sessions").Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}

	var sessionStats []bson.M
	if err := cursor.All(ctx, &sessionStats); err != nil {
		return nil, 0, err
	}

	// Get agent usage counts per user
	agentPipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id":              "$userId",
			"total_agent_runs": bson.M{"$sum": 1},
		}}},
	}

	agentCursor, err := s.collection("agent_usage").Aggregate(ctx, agentPipeline)
	var agentStats []bson.M
	agentCountByUser := make(map[string]int64)
	if err == nil {
		agentCursor.All(ctx, &agentStats)
		for _, stat := range agentStats {
			userID, _ := stat["_id"].(string)
			count := extractInt64(stat, "total_agent_runs")
			agentCountByUser[userID] = count
		}
	}

	// Build user list
	users := make([]UserListItemGDPR, 0, len(sessionStats))
	for _, stat := range sessionStats {
		userID, ok := stat["_id"].(string)
		if !ok || userID == "" {
			continue
		}

		// Extract email domain from user ID if it's an email format
		emailDomain := extractEmailDomain(userID)

		totalChats := extractInt64(stat, "total_chats")
		totalMessages := extractInt64(stat, "total_messages")
		totalAgentRuns := agentCountByUser[userID]

		var lastActive *time.Time
		if lastActiveVal, ok := stat["last_active"].(primitive.DateTime); ok {
			t := lastActiveVal.Time()
			lastActive = &t
		}

		var createdAt time.Time
		if firstSeenVal, ok := stat["first_seen"].(primitive.DateTime); ok {
			createdAt = firstSeenVal.Time()
		}

		user := UserListItemGDPR{
			UserID:         anonymizeUserID(userID), // Anonymize user ID
			EmailDomain:    emailDomain,
			Tier:           "free", // Default, would come from user service in production
			CreatedAt:      createdAt,
			LastActive:     lastActive,
			TotalChats:     totalChats,
			TotalMessages:  totalMessages,
			TotalAgentRuns: totalAgentRuns,
			HasOverrides:   false, // Would check user service for overrides
		}

		users = append(users, user)
	}

	// Sort by last active (most recent first)
	// Note: For production, this should be done in the database query
	// Simple in-memory sort for now
	totalCount := int64(len(users))

	// Pagination
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= len(users) {
		return []UserListItemGDPR{}, totalCount, nil
	}
	if end > len(users) {
		end = len(users)
	}

	return users[start:end], totalCount, nil
}

// extractEmailDomain extracts the domain from an email address
func extractEmailDomain(email string) string {
	parts := splitString(email, "@")
	if len(parts) == 2 {
		return "@" + parts[1]
	}
	return ""
}

// splitString is a simple string split helper
func splitString(s, sep string) []string {
	result := []string{}
	current := ""
	sepLen := len(sep)

	for i := 0; i < len(s); i++ {
		if i+sepLen <= len(s) && s[i:i+sepLen] == sep {
			result = append(result, current)
			current = ""
			i += sepLen - 1
		} else {
			current += string(s[i])
		}
	}
	result = append(result, current)
	return result
}

// anonymizeUserID creates a privacy-safe representation of a user ID
func anonymizeUserID(userID string) string {
	// For emails, show first 3 chars + *** + domain
	parts := splitString(userID, "@")
	if len(parts) == 2 {
		prefix := "***"
		if len(parts[0]) > 3 {
			prefix = parts[0][:3] + "***"
		}
		return prefix + "@" + parts[1]
	}
	// For non-email IDs, just show first 8 chars
	if len(userID) > 8 {
		return userID[:8] + "..."
	}
	return userID
}
