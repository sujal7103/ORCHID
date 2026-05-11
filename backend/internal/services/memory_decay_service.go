package services

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"clara-agents/internal/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// MemoryDecayService handles memory scoring and archival using PageRank-like algorithm
type MemoryDecayService struct {
	mongodb    *database.MongoDB
	collection *mongo.Collection
}

// DecayConfig holds the decay algorithm configuration
type DecayConfig struct {
	RecencyWeight    float64 // Default: 0.4
	FrequencyWeight  float64 // Default: 0.3
	EngagementWeight float64 // Default: 0.3
	RecencyDecayRate float64 // Default: 0.05
	FrequencyMax     int64   // Default: 20
	ArchiveThreshold float64 // Default: 0.15
}

// DefaultDecayConfig returns the default decay configuration
func DefaultDecayConfig() DecayConfig {
	return DecayConfig{
		RecencyWeight:    0.4,
		FrequencyWeight:  0.3,
		EngagementWeight: 0.3,
		RecencyDecayRate: 0.05,
		FrequencyMax:     20,
		ArchiveThreshold: 0.15,
	}
}

// NewMemoryDecayService creates a new memory decay service
func NewMemoryDecayService(mongodb *database.MongoDB) *MemoryDecayService {
	return &MemoryDecayService{
		mongodb:    mongodb,
		collection: mongodb.Collection(database.CollectionMemories),
	}
}

// RunDecayJob runs the full decay job for all users
func (s *MemoryDecayService) RunDecayJob(ctx context.Context) error {
	log.Printf("🔄 [MEMORY-DECAY] Starting decay job")

	config := DefaultDecayConfig()

	// Get all unique user IDs with active memories
	userIDs, err := s.getActiveUserIDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get active user IDs: %w", err)
	}

	log.Printf("📊 [MEMORY-DECAY] Processing %d users with active memories", len(userIDs))

	// Process each user
	totalRecalculated := 0
	totalArchived := 0

	for _, userID := range userIDs {
		recalculated, archived, err := s.RunDecayJobForUser(ctx, userID, config)
		if err != nil {
			log.Printf("⚠️ [MEMORY-DECAY] Failed to process user %s: %v", userID, err)
			continue
		}

		totalRecalculated += recalculated
		totalArchived += archived
	}

	log.Printf("✅ [MEMORY-DECAY] Decay job completed: %d memories recalculated, %d archived", totalRecalculated, totalArchived)
	return nil
}

// RunDecayJobForUser runs decay job for a specific user
func (s *MemoryDecayService) RunDecayJobForUser(ctx context.Context, userID string, config DecayConfig) (int, int, error) {
	// Get all active memories for user
	filter := bson.M{
		"userId":     userID,
		"isArchived": false,
	}

	cursor, err := s.collection.Find(ctx, filter)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to find memories: %w", err)
	}
	defer cursor.Close(ctx)

	var memories []struct {
		ID               primitive.ObjectID `bson:"_id"`
		AccessCount      int64              `bson:"accessCount"`
		LastAccessedAt   *time.Time         `bson:"lastAccessedAt"`
		SourceEngagement float64            `bson:"sourceEngagement"`
		CreatedAt        time.Time          `bson:"createdAt"`
	}

	if err := cursor.All(ctx, &memories); err != nil {
		return 0, 0, fmt.Errorf("failed to decode memories: %w", err)
	}

	if len(memories) == 0 {
		return 0, 0, nil
	}

	// Calculate scores for all memories
	now := time.Now()
	memoriesToArchive := []primitive.ObjectID{}
	memoriesToUpdate := []mongo.WriteModel{}

	for _, mem := range memories {
		newScore := s.calculateMemoryScore(mem.AccessCount, mem.LastAccessedAt, mem.SourceEngagement, mem.CreatedAt, now, config)

		// Check if should be archived
		if newScore < config.ArchiveThreshold {
			memoriesToArchive = append(memoriesToArchive, mem.ID)
		} else {
			// Update score
			update := mongo.NewUpdateOneModel().
				SetFilter(bson.M{"_id": mem.ID}).
				SetUpdate(bson.M{
					"$set": bson.M{
						"score":     newScore,
						"updatedAt": now,
					},
				})
			memoriesToUpdate = append(memoriesToUpdate, update)
		}
	}

	// Bulk update scores
	recalculated := 0
	if len(memoriesToUpdate) > 0 {
		result, err := s.collection.BulkWrite(ctx, memoriesToUpdate)
		if err != nil {
			log.Printf("⚠️ [MEMORY-DECAY] Failed to update scores for user %s: %v", userID, err)
		} else {
			recalculated = int(result.ModifiedCount)
		}
	}

	// Archive low-score memories
	archived := 0
	if len(memoriesToArchive) > 0 {
		archived, err = s.archiveMemoriesBulk(ctx, memoriesToArchive, now)
		if err != nil {
			log.Printf("⚠️ [MEMORY-DECAY] Failed to archive memories for user %s: %v", userID, err)
		}
	}

	log.Printf("📊 [MEMORY-DECAY] User %s: %d memories recalculated, %d archived", userID, recalculated, archived)
	return recalculated, archived, nil
}

// calculateMemoryScore calculates the PageRank-like score for a memory
func (s *MemoryDecayService) calculateMemoryScore(
	accessCount int64,
	lastAccessedAt *time.Time,
	sourceEngagement float64,
	createdAt time.Time,
	now time.Time,
	config DecayConfig,
) float64 {
	// Calculate recency score
	recencyScore := s.calculateRecencyScore(lastAccessedAt, createdAt, now, config.RecencyDecayRate)

	// Calculate frequency score
	frequencyScore := s.calculateFrequencyScore(accessCount, config.FrequencyMax)

	// Engagement score is directly from source conversation
	engagementScore := sourceEngagement

	// Weighted combination (PageRank-like)
	finalScore := (config.RecencyWeight * recencyScore) +
		(config.FrequencyWeight * frequencyScore) +
		(config.EngagementWeight * engagementScore)

	return finalScore
}

// calculateRecencyScore calculates recency score using exponential decay
// RecencyScore = exp(-0.05 × days_since_last_access)
// - Recent: 1.0
// - 1 week: ~0.70
// - 1 month: ~0.22
// - 3 months: ~0.01
func (s *MemoryDecayService) calculateRecencyScore(lastAccessedAt *time.Time, createdAt time.Time, now time.Time, decayRate float64) float64 {
	var referenceTime time.Time

	// Use last accessed time if available, otherwise use creation time
	if lastAccessedAt != nil {
		referenceTime = *lastAccessedAt
	} else {
		referenceTime = createdAt
	}

	// Calculate days since last access/creation
	daysSince := now.Sub(referenceTime).Hours() / 24.0

	// Exponential decay: exp(-decayRate × days)
	recencyScore := math.Exp(-decayRate * daysSince)

	return recencyScore
}

// calculateFrequencyScore calculates frequency score based on access count
// FrequencyScore = min(1.0, access_count / max)
// - 0 accesses: 0.0
// - 10 accesses: 0.5 (if max=20)
// - 20+ accesses: 1.0
func (s *MemoryDecayService) calculateFrequencyScore(accessCount int64, frequencyMax int64) float64 {
	if accessCount <= 0 {
		return 0.0
	}

	frequencyScore := float64(accessCount) / float64(frequencyMax)

	// Cap at 1.0
	if frequencyScore > 1.0 {
		frequencyScore = 1.0
	}

	return frequencyScore
}

// archiveMemoriesBulk archives multiple memories at once
func (s *MemoryDecayService) archiveMemoriesBulk(ctx context.Context, memoryIDs []primitive.ObjectID, now time.Time) (int, error) {
	filter := bson.M{
		"_id": bson.M{"$in": memoryIDs},
	}

	update := bson.M{
		"$set": bson.M{
			"isArchived": true,
			"archivedAt": now,
			"updatedAt":  now,
		},
	}

	result, err := s.collection.UpdateMany(ctx, filter, update)
	if err != nil {
		return 0, fmt.Errorf("failed to archive memories: %w", err)
	}

	log.Printf("📦 [MEMORY-DECAY] Archived %d memories", result.ModifiedCount)
	return int(result.ModifiedCount), nil
}

// getActiveUserIDs gets all unique user IDs with active memories
func (s *MemoryDecayService) getActiveUserIDs(ctx context.Context) ([]string, error) {
	filter := bson.M{"isArchived": false}

	distinctUserIDs, err := s.collection.Distinct(ctx, "userId", filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get distinct user IDs: %w", err)
	}

	userIDs := make([]string, 0, len(distinctUserIDs))
	for _, id := range distinctUserIDs {
		if userID, ok := id.(string); ok {
			userIDs = append(userIDs, userID)
		}
	}

	return userIDs, nil
}

// GetMemoryScore calculates and returns the current score for a specific memory (for testing/debugging)
func (s *MemoryDecayService) GetMemoryScore(ctx context.Context, memoryID primitive.ObjectID) (float64, error) {
	var memory struct {
		AccessCount      int64      `bson:"accessCount"`
		LastAccessedAt   *time.Time `bson:"lastAccessedAt"`
		SourceEngagement float64    `bson:"sourceEngagement"`
		CreatedAt        time.Time  `bson:"createdAt"`
	}

	err := s.collection.FindOne(ctx, bson.M{"_id": memoryID}).Decode(&memory)
	if err != nil {
		return 0, fmt.Errorf("failed to find memory: %w", err)
	}

	config := DefaultDecayConfig()
	now := time.Now()

	score := s.calculateMemoryScore(
		memory.AccessCount,
		memory.LastAccessedAt,
		memory.SourceEngagement,
		memory.CreatedAt,
		now,
		config,
	)

	return score, nil
}
