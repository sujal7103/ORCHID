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

// NexusProjectStore handles CRUD for Nexus projects in MongoDB
type NexusProjectStore struct {
	collection *mongo.Collection
}

// NewNexusProjectStore creates a new project store
func NewNexusProjectStore(mongodb *database.MongoDB) *NexusProjectStore {
	return &NexusProjectStore{
		collection: mongodb.Collection(database.CollectionNexusProjects),
	}
}

// Create inserts a new project
func (s *NexusProjectStore) Create(ctx context.Context, project *models.NexusProject) error {
	now := time.Now()
	project.CreatedAt = now
	project.UpdatedAt = now
	if project.Icon == "" {
		project.Icon = "folder"
	}
	if project.Color == "" {
		project.Color = "#2196F3"
	}

	result, err := s.collection.InsertOne(ctx, project)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}
	project.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

// GetByID returns a project by ID, scoped to user
func (s *NexusProjectStore) GetByID(ctx context.Context, userID string, projectID primitive.ObjectID) (*models.NexusProject, error) {
	var project models.NexusProject
	err := s.collection.FindOne(ctx, bson.M{
		"_id":    projectID,
		"userId": userID,
	}).Decode(&project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("project not found")
		}
		return nil, fmt.Errorf("failed to get project: %w", err)
	}
	return &project, nil
}

// List returns non-archived projects for a user, sorted by sortOrder then createdAt
func (s *NexusProjectStore) List(ctx context.Context, userID string) ([]models.NexusProject, error) {
	cursor, err := s.collection.Find(ctx, bson.M{
		"userId":     userID,
		"isArchived": bson.M{"$ne": true},
	}, options.Find().SetSort(bson.D{
		{Key: "sortOrder", Value: 1},
		{Key: "createdAt", Value: -1},
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}
	defer cursor.Close(ctx)

	var projects []models.NexusProject
	if err := cursor.All(ctx, &projects); err != nil {
		return nil, fmt.Errorf("failed to decode projects: %w", err)
	}
	return projects, nil
}

// Update partially updates a project (only user-owned fields)
func (s *NexusProjectStore) Update(ctx context.Context, userID string, projectID primitive.ObjectID, updates map[string]interface{}) error {
	updates["updatedAt"] = time.Now()
	result, err := s.collection.UpdateOne(ctx, bson.M{
		"_id":    projectID,
		"userId": userID,
	}, bson.M{"$set": updates})
	if err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("project not found")
	}
	return nil
}

// Delete removes a project by ID
func (s *NexusProjectStore) Delete(ctx context.Context, userID string, projectID primitive.ObjectID) error {
	result, err := s.collection.DeleteOne(ctx, bson.M{
		"_id":    projectID,
		"userId": userID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("project not found")
	}
	return nil
}

// Archive soft-deletes a project by setting isArchived=true
func (s *NexusProjectStore) Archive(ctx context.Context, userID string, projectID primitive.ObjectID) error {
	return s.Update(ctx, userID, projectID, map[string]interface{}{
		"isArchived": true,
	})
}
