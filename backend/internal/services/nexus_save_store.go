package services

import (
	"context"
	"fmt"
	"time"

	"clara-agents/internal/database"
	"clara-agents/internal/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// NexusSaveStore handles CRUD for saved outputs in MongoDB
type NexusSaveStore struct {
	collection *mongo.Collection
}

// NewNexusSaveStore creates a new save store
func NewNexusSaveStore(mongodb *database.MongoDB) *NexusSaveStore {
	return &NexusSaveStore{
		collection: mongodb.Collection(database.CollectionNexusSaves),
	}
}

// SaveFilters controls listing behaviour
type SaveFilters struct {
	Tag    string
	Limit  int64
	Offset int64
}

// Create inserts a new save
func (s *NexusSaveStore) Create(ctx context.Context, save *models.NexusSave) error {
	now := time.Now()
	save.CreatedAt = now
	save.UpdatedAt = now

	result, err := s.collection.InsertOne(ctx, save)
	if err != nil {
		return fmt.Errorf("failed to create save: %w", err)
	}
	save.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

// GetByID returns a save by ID, scoped to user
func (s *NexusSaveStore) GetByID(ctx context.Context, userID string, saveID primitive.ObjectID) (*models.NexusSave, error) {
	var save models.NexusSave
	err := s.collection.FindOne(ctx, bson.M{
		"_id":    saveID,
		"userId": userID,
	}).Decode(&save)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("save not found")
		}
		return nil, fmt.Errorf("failed to get save: %w", err)
	}
	return &save, nil
}

// List returns saves for a user with optional tag filter, sorted by createdAt desc
func (s *NexusSaveStore) List(ctx context.Context, userID string, filters SaveFilters) ([]models.NexusSave, error) {
	filter := bson.M{"userId": userID}
	if filters.Tag != "" {
		filter["tags"] = filters.Tag
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetLimit(limit)
	if filters.Offset > 0 {
		opts.SetSkip(filters.Offset)
	}

	cursor, err := s.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list saves: %w", err)
	}
	defer cursor.Close(ctx)

	var saves []models.NexusSave
	if err := cursor.All(ctx, &saves); err != nil {
		return nil, fmt.Errorf("failed to decode saves: %w", err)
	}
	return saves, nil
}

// GetBySourceTaskID checks if a save already exists for a given source task
func (s *NexusSaveStore) GetBySourceTaskID(ctx context.Context, userID string, taskID primitive.ObjectID) (*models.NexusSave, error) {
	var save models.NexusSave
	err := s.collection.FindOne(ctx, bson.M{
		"userId":       userID,
		"sourceTaskId": taskID,
	}).Decode(&save)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Not found is not an error
		}
		return nil, fmt.Errorf("failed to check save by source task: %w", err)
	}
	return &save, nil
}

// GetByIDs returns saves matching a list of IDs for a user
func (s *NexusSaveStore) GetByIDs(ctx context.Context, userID string, ids []primitive.ObjectID) ([]models.NexusSave, error) {
	cursor, err := s.collection.Find(ctx, bson.M{
		"_id":    bson.M{"$in": ids},
		"userId": userID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get saves by IDs: %w", err)
	}
	defer cursor.Close(ctx)

	var saves []models.NexusSave
	if err := cursor.All(ctx, &saves); err != nil {
		return nil, fmt.Errorf("failed to decode saves: %w", err)
	}
	return saves, nil
}

// Update partially updates a save
func (s *NexusSaveStore) Update(ctx context.Context, userID string, saveID primitive.ObjectID, updates map[string]interface{}) error {
	updates["updatedAt"] = time.Now()
	result, err := s.collection.UpdateOne(ctx, bson.M{
		"_id":    saveID,
		"userId": userID,
	}, bson.M{"$set": updates})
	if err != nil {
		return fmt.Errorf("failed to update save: %w", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("save not found")
	}
	return nil
}

// Delete removes a save by ID
func (s *NexusSaveStore) Delete(ctx context.Context, userID string, saveID primitive.ObjectID) error {
	result, err := s.collection.DeleteOne(ctx, bson.M{
		"_id":    saveID,
		"userId": userID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete save: %w", err)
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("save not found")
	}
	return nil
}
