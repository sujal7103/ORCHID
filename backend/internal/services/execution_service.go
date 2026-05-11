package services

import (
	"clara-agents/internal/database"
	"clara-agents/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ExecutionRecord is the MongoDB representation of an execution
type ExecutionRecord struct {
	ID              primitive.ObjectID      `bson:"_id,omitempty" json:"id"`
	AgentID         string                  `bson:"agentId" json:"agentId"`
	UserID          string                  `bson:"userId" json:"userId"`
	WorkflowVersion int                     `bson:"workflowVersion" json:"workflowVersion"`

	// Trigger info
	TriggerType string             `bson:"triggerType" json:"triggerType"` // manual, scheduled, webhook, api
	ScheduleID  primitive.ObjectID `bson:"scheduleId,omitempty" json:"scheduleId,omitempty"`
	APIKeyID    primitive.ObjectID `bson:"apiKeyId,omitempty" json:"apiKeyId,omitempty"`

	// Execution state
	Status      string                          `bson:"status" json:"status"` // pending, running, completed, failed, partial
	Input       map[string]interface{}          `bson:"input,omitempty" json:"input,omitempty"`
	Output      map[string]interface{}          `bson:"output,omitempty" json:"output,omitempty"`
	BlockStates map[string]*models.BlockState   `bson:"blockStates,omitempty" json:"blockStates,omitempty"`
	Error       string                          `bson:"error,omitempty" json:"error,omitempty"`

	// Standardized API response (clean, well-structured output)
	Result      string                  `bson:"result,omitempty" json:"result,omitempty"`           // Primary text result
	Artifacts   []models.APIArtifact    `bson:"artifacts,omitempty" json:"artifacts,omitempty"`     // Generated charts/images
	Files       []models.APIFile        `bson:"files,omitempty" json:"files,omitempty"`             // Generated files

	// Timing
	StartedAt   time.Time  `bson:"startedAt" json:"startedAt"`
	CompletedAt *time.Time `bson:"completedAt,omitempty" json:"completedAt,omitempty"`
	DurationMs  int64      `bson:"durationMs,omitempty" json:"durationMs,omitempty"`

	// TTL (tier-based retention)
	ExpiresAt time.Time `bson:"expiresAt" json:"expiresAt"`

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
}

// ExecutionService manages execution history in MongoDB
type ExecutionService struct {
	mongoDB     *database.MongoDB
	tierService *TierService
}

// NewExecutionService creates a new execution service
func NewExecutionService(mongoDB *database.MongoDB, tierService *TierService) *ExecutionService {
	return &ExecutionService{
		mongoDB:     mongoDB,
		tierService: tierService,
	}
}

// collection returns the executions collection
func (s *ExecutionService) collection() *mongo.Collection {
	return s.mongoDB.Database().Collection("executions")
}

// Create creates a new execution record
func (s *ExecutionService) Create(ctx context.Context, req *CreateExecutionRequest) (*ExecutionRecord, error) {
	// Calculate retention based on user tier
	retentionDays := 30 // default free tier
	if s.tierService != nil {
		retentionDays = s.tierService.GetExecutionRetentionDays(ctx, req.UserID)
	}

	now := time.Now()
	record := &ExecutionRecord{
		AgentID:         req.AgentID,
		UserID:          req.UserID,
		WorkflowVersion: req.WorkflowVersion,
		TriggerType:     req.TriggerType,
		ScheduleID:      req.ScheduleID,
		APIKeyID:        req.APIKeyID,
		Status:          "pending",
		Input:           req.Input,
		StartedAt:       now,
		ExpiresAt:       now.Add(time.Duration(retentionDays) * 24 * time.Hour),
		CreatedAt:       now,
	}

	result, err := s.collection().InsertOne(ctx, record)
	if err != nil {
		return nil, fmt.Errorf("failed to create execution: %w", err)
	}

	record.ID = result.InsertedID.(primitive.ObjectID)
	log.Printf("📝 [EXECUTION] Created execution %s for agent %s (trigger: %s)",
		record.ID.Hex(), req.AgentID, req.TriggerType)

	return record, nil
}

// CreateExecutionRequest contains the data needed to create an execution
type CreateExecutionRequest struct {
	AgentID         string
	UserID          string
	WorkflowVersion int
	TriggerType     string // manual, scheduled, webhook, api
	ScheduleID      primitive.ObjectID
	APIKeyID        primitive.ObjectID
	Input           map[string]interface{}
}

// UpdateStatus updates the execution status
func (s *ExecutionService) UpdateStatus(ctx context.Context, executionID primitive.ObjectID, status string) error {
	update := bson.M{
		"$set": bson.M{
			"status": status,
		},
	}

	_, err := s.collection().UpdateByID(ctx, executionID, update)
	if err != nil {
		return fmt.Errorf("failed to update execution status: %w", err)
	}

	log.Printf("📊 [EXECUTION] Updated %s status to %s", executionID.Hex(), status)
	return nil
}

// Complete marks an execution as complete with output
func (s *ExecutionService) Complete(ctx context.Context, executionID primitive.ObjectID, result *ExecutionCompleteRequest) error {
	now := time.Now()

	// Get the execution to calculate duration
	exec, err := s.GetByID(ctx, executionID)
	if err != nil {
		return err
	}

	durationMs := now.Sub(exec.StartedAt).Milliseconds()

	// Sanitize output and blockStates to remove large base64 data
	// This prevents MongoDB document size limit (16MB) issues
	sanitizedOutput := sanitizeOutputForStorage(result.Output)
	sanitizedBlockStates := sanitizeBlockStatesForStorageV2(result.BlockStates)

	// Log sanitization to help debug
	log.Printf("🧹 [EXECUTION] Sanitizing execution %s for storage", executionID.Hex())

	update := bson.M{
		"$set": bson.M{
			"status":      result.Status,
			"output":      sanitizedOutput,
			"blockStates": sanitizedBlockStates,
			"error":       result.Error,
			"completedAt": now,
			"durationMs":  durationMs,
			// Store clean API response fields
			"result":    result.Result,
			"artifacts": result.Artifacts,
			"files":     result.Files,
		},
	}

	_, err = s.collection().UpdateByID(ctx, executionID, update)
	if err != nil {
		return fmt.Errorf("failed to complete execution: %w", err)
	}

	log.Printf("✅ [EXECUTION] Completed %s with status %s (duration: %dms)",
		executionID.Hex(), result.Status, durationMs)

	return nil
}

// ExecutionCompleteRequest contains the completion data
type ExecutionCompleteRequest struct {
	Status      string
	Output      map[string]interface{}
	BlockStates map[string]*models.BlockState
	Error       string

	// Clean API response fields
	Result    string                 // Primary text result
	Artifacts []models.APIArtifact   // Generated charts/images
	Files     []models.APIFile       // Generated files
}

// CheckpointBlock persists the status and output of a single block within an execution.
// This is called after each block completes, enabling crash recovery by replaying from
// the last checkpoint rather than re-running the entire workflow.
func (s *ExecutionService) CheckpointBlock(ctx context.Context, executionID primitive.ObjectID, blockID, status string, output map[string]any) error {
	update := bson.M{
		"$set": bson.M{
			fmt.Sprintf("blockCheckpoints.%s", blockID): bson.M{
				"status":    status,
				"output":    output,
				"timestamp": time.Now(),
			},
		},
	}

	_, err := s.collection().UpdateByID(ctx, executionID, update)
	if err != nil {
		// Checkpoint failures are non-fatal — log but don't block execution
		log.Printf("⚠️ [EXECUTION] Failed to checkpoint block %s in %s: %v", blockID, executionID.Hex(), err)
		return err
	}
	return nil
}

// GetByID retrieves an execution by ID
func (s *ExecutionService) GetByID(ctx context.Context, executionID primitive.ObjectID) (*ExecutionRecord, error) {
	var record ExecutionRecord
	err := s.collection().FindOne(ctx, bson.M{"_id": executionID}).Decode(&record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("execution not found")
		}
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}
	return &record, nil
}

// GetByIDAndUser retrieves an execution by ID ensuring user ownership
func (s *ExecutionService) GetByIDAndUser(ctx context.Context, executionID primitive.ObjectID, userID string) (*ExecutionRecord, error) {
	var record ExecutionRecord
	err := s.collection().FindOne(ctx, bson.M{
		"_id":    executionID,
		"userId": userID,
	}).Decode(&record)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("execution not found")
		}
		return nil, fmt.Errorf("failed to get execution: %w", err)
	}
	return &record, nil
}

// ListByAgent returns paginated executions for an agent
func (s *ExecutionService) ListByAgent(ctx context.Context, agentID, userID string, opts *ListExecutionsOptions) (*PaginatedExecutions, error) {
	filter := bson.M{
		"agentId": agentID,
		"userId":  userID,
	}

	if opts != nil && opts.Status != "" {
		filter["status"] = opts.Status
	}

	if opts != nil && opts.TriggerType != "" {
		filter["triggerType"] = opts.TriggerType
	}

	return s.listWithFilter(ctx, filter, opts)
}

// ListByUser returns paginated executions for a user
func (s *ExecutionService) ListByUser(ctx context.Context, userID string, opts *ListExecutionsOptions) (*PaginatedExecutions, error) {
	filter := bson.M{
		"userId": userID,
	}

	if opts != nil && opts.Status != "" {
		filter["status"] = opts.Status
	}

	if opts != nil && opts.TriggerType != "" {
		filter["triggerType"] = opts.TriggerType
	}

	if opts != nil && opts.AgentID != "" {
		filter["agentId"] = opts.AgentID
	}

	return s.listWithFilter(ctx, filter, opts)
}

// listWithFilter performs the actual paginated list query
func (s *ExecutionService) listWithFilter(ctx context.Context, filter bson.M, opts *ListExecutionsOptions) (*PaginatedExecutions, error) {
	// Default pagination
	limit := int64(20)
	page := int64(1)

	if opts != nil {
		if opts.Limit > 0 && opts.Limit <= 100 {
			limit = int64(opts.Limit)
		}
		if opts.Page > 0 {
			page = int64(opts.Page)
		}
	}

	skip := (page - 1) * limit

	// Count total
	total, err := s.collection().CountDocuments(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to count executions: %w", err)
	}

	// Find with pagination (newest first)
	findOpts := options.Find().
		SetSort(bson.D{{Key: "startedAt", Value: -1}}).
		SetSkip(skip).
		SetLimit(limit)

	cursor, err := s.collection().Find(ctx, filter, findOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to find executions: %w", err)
	}
	defer cursor.Close(ctx)

	var executions []ExecutionRecord
	if err := cursor.All(ctx, &executions); err != nil {
		return nil, fmt.Errorf("failed to decode executions: %w", err)
	}

	return &PaginatedExecutions{
		Executions: executions,
		Total:      total,
		Page:       page,
		Limit:      limit,
		HasMore:    skip+int64(len(executions)) < total,
	}, nil
}

// ListExecutionsOptions contains query options for listing executions
type ListExecutionsOptions struct {
	Page        int
	Limit       int
	Status      string // filter by status
	TriggerType string // filter by trigger type
	AgentID     string // filter by agent (for user-wide queries)
}

// PaginatedExecutions is the response for paginated execution lists
type PaginatedExecutions struct {
	Executions []ExecutionRecord `json:"executions"`
	Total      int64             `json:"total"`
	Page       int64             `json:"page"`
	Limit      int64             `json:"limit"`
	HasMore    bool              `json:"hasMore"`
}

// GetStats returns execution statistics for an agent
func (s *ExecutionService) GetStats(ctx context.Context, agentID, userID string) (*ExecutionStats, error) {
	filter := bson.M{
		"agentId": agentID,
		"userId":  userID,
	}

	// Get counts by status
	pipeline := mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.M{
			"_id":         "$status",
			"count":       bson.M{"$sum": 1},
			"avgDuration": bson.M{"$avg": "$durationMs"},
		}}},
	}

	cursor, err := s.collection().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate stats: %w", err)
	}
	defer cursor.Close(ctx)

	stats := &ExecutionStats{
		ByStatus: make(map[string]StatusStats),
	}

	var results []struct {
		ID          string  `bson:"_id"`
		Count       int64   `bson:"count"`
		AvgDuration float64 `bson:"avgDuration"`
	}

	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode stats: %w", err)
	}

	for _, r := range results {
		stats.Total += r.Count
		stats.ByStatus[r.ID] = StatusStats{
			Count:       r.Count,
			AvgDuration: int64(r.AvgDuration),
		}
		if r.ID == "completed" {
			stats.SuccessCount = r.Count
		} else if r.ID == "failed" {
			stats.FailedCount = r.Count
		}
	}

	if stats.Total > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.Total) * 100
	}

	return stats, nil
}

// ExecutionStats contains aggregated execution statistics
type ExecutionStats struct {
	Total        int64                  `json:"total"`
	SuccessCount int64                  `json:"successCount"`
	FailedCount  int64                  `json:"failedCount"`
	SuccessRate  float64                `json:"successRate"`
	ByStatus     map[string]StatusStats `json:"byStatus"`
}

// StatusStats contains stats for a single status
type StatusStats struct {
	Count       int64 `json:"count"`
	AvgDuration int64 `json:"avgDurationMs"`
}

// DeleteExpired removes executions past their TTL (called by cleanup job)
func (s *ExecutionService) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := s.collection().DeleteMany(ctx, bson.M{
		"expiresAt": bson.M{"$lt": time.Now()},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired executions: %w", err)
	}

	if result.DeletedCount > 0 {
		log.Printf("🗑️ [EXECUTION] Deleted %d expired executions", result.DeletedCount)
	}

	return result.DeletedCount, nil
}

// DeleteAllByUser deletes all executions for a user (GDPR compliance)
func (s *ExecutionService) DeleteAllByUser(ctx context.Context, userID string) (int64, error) {
	if userID == "" {
		return 0, fmt.Errorf("user ID is required")
	}

	result, err := s.collection().DeleteMany(ctx, bson.M{"userId": userID})
	if err != nil {
		return 0, fmt.Errorf("failed to delete user executions: %w", err)
	}

	log.Printf("🗑️ [GDPR] Deleted %d executions for user %s", result.DeletedCount, userID)
	return result.DeletedCount, nil
}

// EnsureIndexes creates the necessary indexes for the executions collection
func (s *ExecutionService) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		// User + startedAt for listing user's executions
		{
			Keys: bson.D{
				{Key: "userId", Value: 1},
				{Key: "startedAt", Value: -1},
			},
		},
		// Agent + startedAt for listing agent's executions
		{
			Keys: bson.D{
				{Key: "agentId", Value: 1},
				{Key: "startedAt", Value: -1},
			},
		},
		// TTL index for automatic deletion
		{
			Keys:    bson.D{{Key: "expiresAt", Value: 1}},
			Options: options.Index().SetExpireAfterSeconds(0),
		},
		// Status index for filtering
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		// Schedule ID for scheduled execution lookups
		{
			Keys: bson.D{{Key: "scheduleId", Value: 1}},
			Options: options.Index().SetSparse(true),
		},
	}

	_, err := s.collection().Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create execution indexes: %w", err)
	}

	log.Println("✅ [EXECUTION] Ensured indexes for executions collection")
	return nil
}

// sanitizeOutputForStorage sanitizes execution output by converting to JSON, stripping base64 data, and converting back
// This approach handles any nested structure including typed structs
func sanitizeOutputForStorage(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		log.Printf("⚠️ [EXECUTION] Failed to marshal output for sanitization: %v", err)
		return data
	}

	originalSize := len(jsonBytes)

	// Apply regex patterns to strip base64 data
	sanitized := stripBase64FromJSON(string(jsonBytes))

	// Unmarshal back
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(sanitized), &result); err != nil {
		log.Printf("⚠️ [EXECUTION] Failed to unmarshal sanitized output: %v", err)
		return data
	}

	// Apply internal field filtering to remove noise fields (model, tokens, etc.)
	result = filterInternalFields(result)

	newSize := len(sanitized)
	if originalSize != newSize {
		log.Printf("🧹 [EXECUTION] Sanitized output: %d -> %d bytes (%.1f%% reduction)",
			originalSize, newSize, float64(originalSize-newSize)/float64(originalSize)*100)
	}

	return result
}

// filterInternalFields recursively removes internal fields from the output map
func filterInternalFields(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	result := make(map[string]interface{})
	for key, value := range data {
		// Skip internal fields
		if internalFieldsToFilter[key] {
			continue
		}

		// Recursively filter nested maps
		if nested, ok := value.(map[string]interface{}); ok {
			result[key] = filterInternalFields(nested)
		} else if slice, ok := value.([]interface{}); ok {
			// Handle arrays
			filteredSlice := make([]interface{}, len(slice))
			for i, item := range slice {
				if itemMap, ok := item.(map[string]interface{}); ok {
					filteredSlice[i] = filterInternalFields(itemMap)
				} else {
					filteredSlice[i] = item
				}
			}
			result[key] = filteredSlice
		} else {
			result[key] = value
		}
	}

	return result
}

// sanitizeBlockStatesForStorageV2 sanitizes block states using JSON approach
func sanitizeBlockStatesForStorageV2(states map[string]*models.BlockState) map[string]*models.BlockState {
	if states == nil {
		return nil
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(states)
	if err != nil {
		log.Printf("⚠️ [EXECUTION] Failed to marshal block states for sanitization: %v", err)
		return states
	}

	originalSize := len(jsonBytes)

	// Apply regex patterns to strip base64 data
	sanitized := stripBase64FromJSON(string(jsonBytes))

	// Unmarshal back
	var result map[string]*models.BlockState
	if err := json.Unmarshal([]byte(sanitized), &result); err != nil {
		log.Printf("⚠️ [EXECUTION] Failed to unmarshal sanitized block states: %v", err)
		return states
	}

	// Apply internal field filtering to block state outputs
	for blockID, state := range result {
		if state != nil && state.Outputs != nil {
			result[blockID].Outputs = filterInternalFields(state.Outputs)
		}
	}

	newSize := len(sanitized)
	if originalSize != newSize {
		log.Printf("🧹 [EXECUTION] Sanitized block states: %d -> %d bytes (%.1f%% reduction)",
			originalSize, newSize, float64(originalSize-newSize)/float64(originalSize)*100)
	}

	return result
}

// stripBase64FromJSON removes base64 image data from JSON string using regex
func stripBase64FromJSON(jsonStr string) string {
	// Pattern 1: data URI images (data:image/xxx;base64,...)
	dataURIPattern := regexp.MustCompile(`"data:image/[^;]+;base64,[A-Za-z0-9+/=]+"`)
	jsonStr = dataURIPattern.ReplaceAllString(jsonStr, `"[BASE64_IMAGE_STRIPPED]"`)

	// Pattern 2: Long base64-like strings in "data" fields
	dataFieldPattern := regexp.MustCompile(`"data"\s*:\s*"[A-Za-z0-9+/=]{500,}"`)
	jsonStr = dataFieldPattern.ReplaceAllString(jsonStr, `"data":"[BASE64_DATA_STRIPPED]"`)

	// Pattern 3: Long base64-like strings in "image", "plot", "chart" fields
	imageFieldPattern := regexp.MustCompile(`"(image|plot|chart|figure|png|jpeg|base64)"\s*:\s*"[A-Za-z0-9+/=]{500,}"`)
	jsonStr = imageFieldPattern.ReplaceAllString(jsonStr, `"$1":"[BASE64_IMAGE_STRIPPED]"`)

	// Pattern 4: Any remaining very long strings that look like base64
	// Note: Go RE2 has max repeat count of 1000, so we use {1000,} to catch long strings
	longStringPattern := regexp.MustCompile(`"[A-Za-z0-9+/=]{1000,}"`)
	jsonStr = longStringPattern.ReplaceAllString(jsonStr, `"[LARGE_DATA_STRIPPED]"`)

	// Pattern 5: Handle nested JSON strings containing base64 (e.g., in "Result" field of tool calls)
	// This handles cases where the Result is a JSON string that contains base64
	resultFieldPattern := regexp.MustCompile(`"Result"\s*:\s*"\{[^"]*"data"\s*:\s*\\"[A-Za-z0-9+/=]{100,}\\"[^"]*\}"`)
	if resultFieldPattern.MatchString(jsonStr) {
		// For Result fields containing JSON with base64, we need to escape the replacement
		jsonStr = resultFieldPattern.ReplaceAllStringFunc(jsonStr, func(match string) string {
			// Strip the base64 within the nested JSON
			innerPattern := regexp.MustCompile(`\\"data\\"\s*:\s*\\"[A-Za-z0-9+/=]+\\"`)
			return innerPattern.ReplaceAllString(match, `\"data\":\"[BASE64_STRIPPED]\"`)
		})
	}

	return jsonStr
}

// sanitizeForStorage removes large base64 data from maps to prevent MongoDB document size limit issues
// It replaces base64 image data with a placeholder while preserving metadata
func sanitizeForStorage(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	result := make(map[string]interface{})
	for key, value := range data {
		result[key] = sanitizeValue(value)
	}
	return result
}

// sanitizeBlockStatesForStorage sanitizes all block states
func sanitizeBlockStatesForStorage(states map[string]*models.BlockState) map[string]*models.BlockState {
	if states == nil {
		return nil
	}

	result := make(map[string]*models.BlockState)
	for blockID, state := range states {
		if state == nil {
			continue
		}
		sanitizedState := &models.BlockState{
			Status:      state.Status,
			Inputs:      sanitizeForStorage(state.Inputs),
			Outputs:     sanitizeForStorage(state.Outputs),
			Error:       state.Error,
			StartedAt:   state.StartedAt,
			CompletedAt: state.CompletedAt,
		}
		result[blockID] = sanitizedState
	}
	return result
}

// sanitizeValue recursively sanitizes a value, replacing large base64 strings
func sanitizeValue(value interface{}) interface{} {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case string:
		return sanitizeString(v)
	case map[string]interface{}:
		return sanitizeMap(v)
	case []interface{}:
		return sanitizeSlice(v)
	default:
		return value
	}
}

// sanitizeString checks if a string is base64 image data and replaces it
func sanitizeString(s string) string {
	// If string is too short, keep it
	if len(s) < 500 {
		return s
	}

	// Check for data URI prefix (base64 image)
	if regexp.MustCompile(`^data:image/[^;]+;base64,`).MatchString(s) {
		return "[BASE64_IMAGE_STRIPPED_FOR_STORAGE]"
	}

	// Check for long base64-like strings (no spaces, mostly alphanumeric + /+=)
	if regexp.MustCompile(`^[A-Za-z0-9+/=]{500,}$`).MatchString(s) {
		return "[BASE64_DATA_STRIPPED_FOR_STORAGE]"
	}

	// If string is extremely long (>100KB), truncate it
	if len(s) > 100000 {
		return s[:1000] + "... [TRUNCATED_FOR_STORAGE]"
	}

	return s
}

// internalFieldsToFilter contains fields that should not be exposed to API consumers
// These are internal execution details that add noise to the output
var internalFieldsToFilter = map[string]bool{
	"model":              true, // Internal model ID (use _workflowModelId instead)
	"__user_id__":        true, // Internal user context
	"_workflowModelId":   true, // Internal workflow model reference
	"tokens":             true, // Token usage (available in metadata)
	"iterations":         true, // Internal execution iterations
	"start":              true, // Internal start state
	"value":              true, // Redundant with response
	"input":              true, // Already stored separately
	"rawResponse":        true, // Huge and redundant
}

// sanitizeMap recursively sanitizes a map
func sanitizeMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for key, value := range m {
		// Skip internal fields that shouldn't be exposed to API consumers
		if internalFieldsToFilter[key] {
			continue
		}

		// Special handling for known artifact/image fields
		if key == "artifacts" {
			result[key] = sanitizeArtifacts(value)
			continue
		}
		if key == "plots" || key == "images" || key == "base64_images" {
			result[key] = sanitizePlots(value)
			continue
		}
		// Handle toolCalls array which contains Result fields with JSON+base64
		if key == "toolCalls" {
			result[key] = sanitizeToolCallsForStorage(value)
			continue
		}
		result[key] = sanitizeValue(value)
	}

	return result
}

// sanitizeSlice recursively sanitizes a slice
func sanitizeSlice(s []interface{}) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		result[i] = sanitizeValue(v)
	}
	return result
}

// sanitizeArtifacts handles the artifacts array, keeping metadata but removing data
func sanitizeArtifacts(value interface{}) interface{} {
	artifacts, ok := value.([]interface{})
	if !ok {
		return value
	}

	result := make([]interface{}, 0, len(artifacts))
	for _, a := range artifacts {
		artifact, ok := a.(map[string]interface{})
		if !ok {
			continue
		}

		// Keep metadata, remove actual data
		sanitized := map[string]interface{}{
			"type":   artifact["type"],
			"format": artifact["format"],
			"title":  artifact["title"],
			"data":   "[BASE64_IMAGE_STRIPPED_FOR_STORAGE]",
		}
		result = append(result, sanitized)
	}

	log.Printf("🧹 [EXECUTION] Sanitized %d artifacts for storage", len(result))
	return result
}

// sanitizePlots handles plots/images arrays from E2B responses
func sanitizePlots(value interface{}) interface{} {
	plots, ok := value.([]interface{})
	if !ok {
		// Could be a single plot as map
		if plotMap, ok := value.(map[string]interface{}); ok {
			return sanitizeSinglePlot(plotMap)
		}
		return value
	}

	result := make([]interface{}, 0, len(plots))
	for _, p := range plots {
		if plot, ok := p.(map[string]interface{}); ok {
			result = append(result, sanitizeSinglePlot(plot))
		}
	}

	log.Printf("🧹 [EXECUTION] Sanitized %d plots for storage", len(result))
	return result
}

// sanitizeSinglePlot removes base64 data from a single plot while keeping metadata
func sanitizeSinglePlot(plot map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range plot {
		if k == "data" || k == "image" || k == "base64" {
			result[k] = "[BASE64_IMAGE_STRIPPED_FOR_STORAGE]"
		} else {
			result[k] = v
		}
	}
	return result
}

// sanitizeToolCallsForStorage sanitizes tool call results (JSON strings with base64)
func sanitizeToolCallsForStorage(toolCalls interface{}) interface{} {
	calls, ok := toolCalls.([]interface{})
	if !ok {
		return toolCalls
	}

	result := make([]interface{}, 0, len(calls))
	for _, tc := range calls {
		call, ok := tc.(map[string]interface{})
		if !ok {
			result = append(result, tc)
			continue
		}

		sanitized := make(map[string]interface{})
		for k, v := range call {
			if k == "result" {
				// Result is a JSON string - parse, sanitize, re-stringify
				if resultStr, ok := v.(string); ok && len(resultStr) > 1000 {
					var resultData map[string]interface{}
					if err := json.Unmarshal([]byte(resultStr), &resultData); err == nil {
						sanitizedResult := sanitizeMap(resultData)
						if sanitizedJSON, err := json.Marshal(sanitizedResult); err == nil {
							sanitized[k] = string(sanitizedJSON)
							continue
						}
					}
					// If parsing fails, just truncate
					if len(resultStr) > 5000 {
						sanitized[k] = resultStr[:5000] + "... [TRUNCATED]"
						continue
					}
				}
			}
			sanitized[k] = v
		}
		result = append(result, sanitized)
	}

	return result
}
