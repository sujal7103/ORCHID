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

// NexusSessionStore handles MongoDB CRUD for Nexus sessions
type NexusSessionStore struct {
	collection *mongo.Collection
}

// NewNexusSessionStore creates a new session store
func NewNexusSessionStore(mongodb *database.MongoDB) *NexusSessionStore {
	return &NexusSessionStore{
		collection: mongodb.Collection(database.CollectionNexusSessions),
	}
}

// GetOrCreate retrieves the user's session or creates one if none exists
func (s *NexusSessionStore) GetOrCreate(ctx context.Context, userID string) (*models.NexusSession, error) {
	var session models.NexusSession
	err := s.collection.FindOne(ctx, bson.M{"userId": userID}).Decode(&session)
	if err == nil {
		return &session, nil
	}
	if err != mongo.ErrNoDocuments {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Create new session
	now := time.Now()
	session = models.NexusSession{
		ID:             primitive.NewObjectID(),
		UserID:         userID,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastActivityAt: now,
	}

	_, err = s.collection.InsertOne(ctx, session)
	if err != nil {
		// Race condition: another request created the session concurrently
		var existing models.NexusSession
		if findErr := s.collection.FindOne(ctx, bson.M{"userId": userID}).Decode(&existing); findErr == nil {
			return &existing, nil
		}
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &session, nil
}

// UpdateActivity touches the session's last activity timestamp
func (s *NexusSessionStore) UpdateActivity(ctx context.Context, userID string) error {
	_, err := s.collection.UpdateOne(ctx, bson.M{"userId": userID}, bson.M{
		"$set": bson.M{
			"lastActivityAt": time.Now(),
			"updatedAt":      time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update activity: %w", err)
	}
	return nil
}

// AddRecentTask adds a task ID to the rolling recent list (max 50)
func (s *NexusSessionStore) AddRecentTask(ctx context.Context, userID string, taskID primitive.ObjectID) error {
	_, err := s.collection.UpdateOne(ctx, bson.M{"userId": userID}, bson.M{
		"$push": bson.M{
			"recentTaskIds": bson.M{
				"$each":     []primitive.ObjectID{taskID},
				"$position": 0,
				"$slice":    50,
			},
		},
		"$set": bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		return fmt.Errorf("failed to add recent task: %w", err)
	}
	return nil
}

// RemoveRecentTask removes a task ID from the recent list
func (s *NexusSessionStore) RemoveRecentTask(ctx context.Context, userID string, taskID primitive.ObjectID) error {
	_, err := s.collection.UpdateOne(ctx, bson.M{"userId": userID}, bson.M{
		"$pull": bson.M{"recentTaskIds": taskID},
		"$set":  bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		return fmt.Errorf("failed to remove recent task: %w", err)
	}
	return nil
}

// SetContextSummary updates the rolling context summary
func (s *NexusSessionStore) SetContextSummary(ctx context.Context, userID string, summary string) error {
	_, err := s.collection.UpdateOne(ctx, bson.M{"userId": userID}, bson.M{
		"$set": bson.M{
			"contextSummary": summary,
			"updatedAt":      time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to set context summary: %w", err)
	}
	return nil
}

// AddActiveDaemon adds a daemon to the active list
func (s *NexusSessionStore) AddActiveDaemon(ctx context.Context, userID string, daemonID primitive.ObjectID) error {
	_, err := s.collection.UpdateOne(ctx, bson.M{"userId": userID}, bson.M{
		"$addToSet": bson.M{"activeDaemonIds": daemonID},
		"$set":      bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		return fmt.Errorf("failed to add active daemon: %w", err)
	}
	return nil
}

// RemoveActiveDaemon removes a daemon from the active list
func (s *NexusSessionStore) RemoveActiveDaemon(ctx context.Context, userID string, daemonID primitive.ObjectID) error {
	_, err := s.collection.UpdateOne(ctx, bson.M{"userId": userID}, bson.M{
		"$pull": bson.M{"activeDaemonIds": daemonID},
		"$set":  bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		return fmt.Errorf("failed to remove active daemon: %w", err)
	}
	return nil
}

// AddActiveTask adds a task to the active list
func (s *NexusSessionStore) AddActiveTask(ctx context.Context, userID string, taskID primitive.ObjectID) error {
	_, err := s.collection.UpdateOne(ctx, bson.M{"userId": userID}, bson.M{
		"$addToSet": bson.M{"activeTaskIds": taskID},
		"$set":      bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		return fmt.Errorf("failed to add active task: %w", err)
	}
	return nil
}

// RemoveActiveTask removes a task from the active list
func (s *NexusSessionStore) RemoveActiveTask(ctx context.Context, userID string, taskID primitive.ObjectID) error {
	_, err := s.collection.UpdateOne(ctx, bson.M{"userId": userID}, bson.M{
		"$pull": bson.M{"activeTaskIds": taskID},
		"$set":  bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		return fmt.Errorf("failed to remove active task: %w", err)
	}
	return nil
}

// ClearAllActive clears active daemon and task IDs from all sessions (used on startup).
// Orphaned active tasks are moved to the recent list so they remain visible.
func (s *NexusSessionStore) ClearAllActive(ctx context.Context) (int64, error) {
	// First, find sessions with active tasks and move those task IDs to recent
	cursor, err := s.collection.Find(ctx, bson.M{
		"activeTaskIds": bson.M{"$exists": true, "$ne": bson.A{}},
	})
	if err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var session models.NexusSession
			if err := cursor.Decode(&session); err != nil {
				continue
			}
			if len(session.ActiveTaskIDs) > 0 {
				// Push orphaned active tasks to front of recent list
				_, _ = s.collection.UpdateOne(ctx, bson.M{"_id": session.ID}, bson.M{
					"$push": bson.M{
						"recentTaskIds": bson.M{
							"$each":     session.ActiveTaskIDs,
							"$position": 0,
							"$slice":    50,
						},
					},
				})
			}
		}
	}

	// Then clear all active state
	result, err := s.collection.UpdateMany(ctx, bson.M{
		"$or": []bson.M{
			{"activeDaemonIds": bson.M{"$exists": true, "$ne": bson.A{}}},
			{"activeTaskIds": bson.M{"$exists": true, "$ne": bson.A{}}},
		},
	}, bson.M{
		"$set": bson.M{
			"activeDaemonIds": bson.A{},
			"activeTaskIds":   bson.A{},
			"updatedAt":       time.Now(),
		},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to clear active state: %w", err)
	}
	return result.ModifiedCount, nil
}

// IncrementStats increments the task completion counters
func (s *NexusSessionStore) IncrementStats(ctx context.Context, userID string, completed bool) error {
	inc := bson.M{"totalTasks": 1}
	if completed {
		inc["completedTasks"] = 1
	} else {
		inc["failedTasks"] = 1
	}

	_, err := s.collection.UpdateOne(ctx, bson.M{"userId": userID}, bson.M{
		"$inc": inc,
		"$set": bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		return fmt.Errorf("failed to increment stats: %w", err)
	}
	return nil
}

// PinSkill adds a skill to the pinned list
func (s *NexusSessionStore) PinSkill(ctx context.Context, userID string, skillID primitive.ObjectID) error {
	_, err := s.collection.UpdateOne(ctx, bson.M{"userId": userID}, bson.M{
		"$addToSet": bson.M{"pinnedSkillIds": skillID},
		"$set":      bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		return fmt.Errorf("failed to pin skill: %w", err)
	}
	return nil
}

// UnpinSkill removes a skill from the pinned list
func (s *NexusSessionStore) UnpinSkill(ctx context.Context, userID string, skillID primitive.ObjectID) error {
	_, err := s.collection.UpdateOne(ctx, bson.M{"userId": userID}, bson.M{
		"$pull": bson.M{"pinnedSkillIds": skillID},
		"$set":  bson.M{"updatedAt": time.Now()},
	})
	if err != nil {
		return fmt.Errorf("failed to unpin skill: %w", err)
	}
	return nil
}

// SetModelID updates the preferred model
func (s *NexusSessionStore) SetModelID(ctx context.Context, userID string, modelID string) error {
	_, err := s.collection.UpdateOne(ctx, bson.M{"userId": userID}, bson.M{
		"$set": bson.M{
			"modelId":   modelID,
			"updatedAt": time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to set model ID: %w", err)
	}
	return nil
}

// GetByUser returns the session for a specific user
func (s *NexusSessionStore) GetByUser(ctx context.Context, userID string) (*models.NexusSession, error) {
	var session models.NexusSession
	err := s.collection.FindOne(ctx, bson.M{"userId": userID}, options.FindOne()).Decode(&session)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return &session, nil
}
