package services

import (
	"clara-agents/internal/database"
	"clara-agents/internal/models"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ============================================================================
// MongoDB Records
// ============================================================================

// AgentRecord is the MongoDB representation of an agent
type AgentRecord struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	AgentID     string             `bson:"agentId" json:"agentId"` // String ID for API compatibility
	UserID      string             `bson:"userId" json:"userId"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description,omitempty" json:"description,omitempty"`
	Status      string             `bson:"status" json:"status"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
}

// ToModel converts AgentRecord to models.Agent
func (r *AgentRecord) ToModel() *models.Agent {
	return &models.Agent{
		ID:          r.AgentID,
		UserID:      r.UserID,
		Name:        r.Name,
		Description: r.Description,
		Status:      r.Status,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

// WorkflowRecord is the MongoDB representation of a workflow
type WorkflowRecord struct {
	ID              primitive.ObjectID  `bson:"_id,omitempty" json:"_id,omitempty"`
	WorkflowID      string              `bson:"workflowId" json:"workflowId"` // String ID for API compatibility
	AgentID         string              `bson:"agentId" json:"agentId"`
	Blocks          []models.Block      `bson:"blocks" json:"blocks"`
	Connections     []models.Connection `bson:"connections" json:"connections"`
	Variables       []models.Variable   `bson:"variables" json:"variables"`
	Version         int                 `bson:"version" json:"version"`
	WorkflowModelID string              `bson:"workflowModelId,omitempty" json:"workflowModelId,omitempty"`
	CreatedAt       time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt       time.Time           `bson:"updatedAt" json:"updatedAt"`
}

// ToModel converts WorkflowRecord to models.Workflow
func (r *WorkflowRecord) ToModel() *models.Workflow {
	return &models.Workflow{
		ID:              r.WorkflowID,
		AgentID:         r.AgentID,
		Blocks:          r.Blocks,
		Connections:     r.Connections,
		Variables:       r.Variables,
		Version:         r.Version,
		WorkflowModelID: r.WorkflowModelID,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}

// WorkflowVersionRecord stores historical workflow versions
type WorkflowVersionRecord struct {
	ID          primitive.ObjectID  `bson:"_id,omitempty" json:"_id,omitempty"`
	AgentID     string              `bson:"agentId" json:"agentId"`
	Version     int                 `bson:"version" json:"version"`
	Blocks      []models.Block      `bson:"blocks" json:"blocks"`
	Connections []models.Connection `bson:"connections" json:"connections"`
	Variables   []models.Variable   `bson:"variables" json:"variables"`
	Description string              `bson:"description,omitempty" json:"description,omitempty"`
	CreatedAt   time.Time           `bson:"createdAt" json:"createdAt"`
}

// WorkflowVersionResponse is the API response for workflow versions
type WorkflowVersionResponse struct {
	Version     int       `json:"version"`
	Description string    `json:"description,omitempty"`
	BlockCount  int       `json:"blockCount"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ============================================================================
// AgentService
// ============================================================================

// AgentService handles agent and workflow operations using MongoDB
type AgentService struct {
	mongoDB *database.MongoDB
}

// NewAgentService creates a new agent service
func NewAgentService(mongoDB *database.MongoDB) *AgentService {
	return &AgentService{mongoDB: mongoDB}
}

// Collection helpers
func (s *AgentService) agentsCollection() *mongo.Collection {
	return s.mongoDB.Database().Collection("agents")
}

func (s *AgentService) workflowsCollection() *mongo.Collection {
	return s.mongoDB.Database().Collection("workflows")
}

func (s *AgentService) workflowVersionsCollection() *mongo.Collection {
	return s.mongoDB.Database().Collection("workflow_versions")
}

// ============================================================================
// Agent CRUD Operations
// ============================================================================

// CreateAgent creates a new agent for a user with auto-generated ID
func (s *AgentService) CreateAgent(userID, name, description string) (*models.Agent, error) {
	id := uuid.New().String()
	return s.CreateAgentWithID(id, userID, name, description)
}

// CreateAgentWithID creates a new agent with a specific ID (for frontend-generated IDs)
func (s *AgentService) CreateAgentWithID(id, userID, name, description string) (*models.Agent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()

	record := &AgentRecord{
		AgentID:     id,
		UserID:      userID,
		Name:        name,
		Description: description,
		Status:      "draft",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	_, err := s.agentsCollection().InsertOne(ctx, record)
	if err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	log.Printf("📝 [AGENT] Created agent %s for user %s", id, userID)
	return record.ToModel(), nil
}

// GetAgent retrieves an agent by ID for a specific user
func (s *AgentService) GetAgent(agentID, userID string) (*models.Agent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var record AgentRecord
	err := s.agentsCollection().FindOne(ctx, bson.M{
		"agentId": agentID,
		"userId":  userID,
	}).Decode(&record)

	if err == mongo.ErrNoDocuments {
		return nil, fmt.Errorf("agent not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	agent := record.ToModel()

	// Also load the workflow if it exists
	workflow, err := s.GetWorkflow(agentID)
	if err == nil {
		agent.Workflow = workflow
	}

	return agent, nil
}

// GetAgentByID retrieves an agent by ID only (for internal/scheduled use)
// WARNING: This bypasses user ownership check - use only for scheduled jobs
func (s *AgentService) GetAgentByID(agentID string) (*models.Agent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var record AgentRecord
	err := s.agentsCollection().FindOne(ctx, bson.M{
		"agentId": agentID,
	}).Decode(&record)

	if err == mongo.ErrNoDocuments {
		return nil, fmt.Errorf("agent not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	agent := record.ToModel()

	// Also load the workflow if it exists
	workflow, err := s.GetWorkflow(agentID)
	if err == nil {
		agent.Workflow = workflow
	}

	return agent, nil
}

// ListAgents returns all agents for a user
func (s *AgentService) ListAgents(userID string) ([]*models.Agent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := s.agentsCollection().Find(ctx, bson.M{"userId": userID},
		options.Find().SetSort(bson.D{{Key: "updatedAt", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	defer cursor.Close(ctx)

	var records []AgentRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, fmt.Errorf("failed to decode agents: %w", err)
	}

	agents := make([]*models.Agent, len(records))
	for i, record := range records {
		agents[i] = record.ToModel()
	}

	return agents, nil
}

// UpdateAgent updates an agent's metadata
func (s *AgentService) UpdateAgent(agentID, userID string, req *models.UpdateAgentRequest) (*models.Agent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// First check if agent exists and belongs to user
	agent, err := s.GetAgent(agentID, userID)
	if err != nil {
		return nil, err
	}

	// Build update document
	updateFields := bson.M{"updatedAt": time.Now()}
	if req.Name != "" {
		updateFields["name"] = req.Name
		agent.Name = req.Name
	}
	if req.Description != "" {
		updateFields["description"] = req.Description
		agent.Description = req.Description
	}
	if req.Status != "" {
		updateFields["status"] = req.Status
		agent.Status = req.Status
	}

	_, err = s.agentsCollection().UpdateOne(ctx,
		bson.M{"agentId": agentID, "userId": userID},
		bson.M{"$set": updateFields})
	if err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	agent.UpdatedAt = time.Now()
	return agent, nil
}

// DeleteAgent deletes an agent and its workflow/versions
func (s *AgentService) DeleteAgent(agentID, userID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Delete agent
	result, err := s.agentsCollection().DeleteOne(ctx, bson.M{
		"agentId": agentID,
		"userId":  userID,
	})
	if err != nil {
		return fmt.Errorf("failed to delete agent: %w", err)
	}
	if result.DeletedCount == 0 {
		return fmt.Errorf("agent not found")
	}

	// Cascade delete workflow
	s.workflowsCollection().DeleteOne(ctx, bson.M{"agentId": agentID})

	// Cascade delete workflow versions
	s.workflowVersionsCollection().DeleteMany(ctx, bson.M{"agentId": agentID})

	log.Printf("🗑️ [AGENT] Deleted agent %s and associated workflows", agentID)
	return nil
}

// DeleteAllByUser deletes all agents, workflows, and workflow versions for a user (GDPR compliance)
func (s *AgentService) DeleteAllByUser(ctx context.Context, userID string) (int64, error) {
	if userID == "" {
		return 0, fmt.Errorf("user ID is required")
	}

	// Get all agent IDs for this user first (for cascade deletion)
	cursor, err := s.agentsCollection().Find(ctx, bson.M{"userId": userID})
	if err != nil {
		return 0, fmt.Errorf("failed to find user agents: %w", err)
	}
	defer cursor.Close(ctx)

	var agentIDs []string
	for cursor.Next(ctx) {
		var agent struct {
			AgentID string `bson:"agentId"`
		}
		if err := cursor.Decode(&agent); err == nil {
			agentIDs = append(agentIDs, agent.AgentID)
		}
	}

	// Delete all workflow versions for these agents
	if len(agentIDs) > 0 {
		_, err := s.workflowVersionsCollection().DeleteMany(ctx, bson.M{
			"agentId": bson.M{"$in": agentIDs},
		})
		if err != nil {
			log.Printf("⚠️ [GDPR] Failed to delete workflow versions: %v", err)
		}

		// Delete all workflows for these agents
		_, err = s.workflowsCollection().DeleteMany(ctx, bson.M{
			"agentId": bson.M{"$in": agentIDs},
		})
		if err != nil {
			log.Printf("⚠️ [GDPR] Failed to delete workflows: %v", err)
		}
	}

	// Delete all agents for this user
	result, err := s.agentsCollection().DeleteMany(ctx, bson.M{"userId": userID})
	if err != nil {
		return 0, fmt.Errorf("failed to delete agents: %w", err)
	}

	log.Printf("🗑️ [GDPR] Deleted %d agents and associated data for user %s", result.DeletedCount, userID)
	return result.DeletedCount, nil
}

// ============================================================================
// Workflow Operations
// ============================================================================

// SaveWorkflow creates or updates a workflow for an agent
func (s *AgentService) SaveWorkflow(agentID, userID string, req *models.SaveWorkflowRequest) (*models.Workflow, error) {
	return s.SaveWorkflowWithDescription(agentID, userID, req, "")
}

// SaveWorkflowWithDescription creates or updates a workflow with a version description
func (s *AgentService) SaveWorkflowWithDescription(agentID, userID string, req *models.SaveWorkflowRequest, description string) (*models.Workflow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Verify agent exists and belongs to user
	_, err := s.GetAgent(agentID, userID)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	// Check if workflow exists
	var existingWorkflow WorkflowRecord
	err = s.workflowsCollection().FindOne(ctx, bson.M{"agentId": agentID}).Decode(&existingWorkflow)

	var workflow *models.Workflow
	var newVersion int

	if err == mongo.ErrNoDocuments {
		// Create new workflow
		workflowID := uuid.New().String()
		newVersion = 1

		record := &WorkflowRecord{
			WorkflowID:      workflowID,
			AgentID:         agentID,
			Blocks:          req.Blocks,
			Connections:     req.Connections,
			Variables:       req.Variables,
			Version:         newVersion,
			WorkflowModelID: req.WorkflowModelID,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		_, err = s.workflowsCollection().InsertOne(ctx, record)
		if err != nil {
			return nil, fmt.Errorf("failed to create workflow: %w", err)
		}

		workflow = record.ToModel()
	} else if err != nil {
		return nil, fmt.Errorf("failed to check existing workflow: %w", err)
	} else {
		// Update existing workflow
		newVersion = existingWorkflow.Version + 1

		_, err = s.workflowsCollection().UpdateOne(ctx,
			bson.M{"agentId": agentID},
			bson.M{"$set": bson.M{
				"blocks":          req.Blocks,
				"connections":     req.Connections,
				"variables":       req.Variables,
				"version":         newVersion,
				"workflowModelId": req.WorkflowModelID,
				"updatedAt":       now,
			}})
		if err != nil {
			return nil, fmt.Errorf("failed to update workflow: %w", err)
		}

		workflow = &models.Workflow{
			ID:              existingWorkflow.WorkflowID,
			AgentID:         agentID,
			Blocks:          req.Blocks,
			Connections:     req.Connections,
			Variables:       req.Variables,
			Version:         newVersion,
			WorkflowModelID: req.WorkflowModelID,
			CreatedAt:       existingWorkflow.CreatedAt,
			UpdatedAt:       now,
		}
	}

	// Only save workflow version snapshot when explicitly requested (e.g., when AI generates/modifies workflow)
	if req.CreateVersion {
		versionDescription := description
		if req.VersionDescription != "" {
			versionDescription = req.VersionDescription
		}

		versionRecord := &WorkflowVersionRecord{
			AgentID:     agentID,
			Version:     newVersion,
			Blocks:      req.Blocks,
			Connections: req.Connections,
			Variables:   req.Variables,
			Description: versionDescription,
			CreatedAt:   now,
		}

		_, err = s.workflowVersionsCollection().InsertOne(ctx, versionRecord)
		if err != nil {
			log.Printf("⚠️ [WORKFLOW] Failed to save version snapshot: %v", err)
			// Don't fail the whole operation for version snapshot failure
		} else {
			log.Printf("📸 [WORKFLOW] Saved version %d snapshot for agent %s", newVersion, agentID)
		}
	}

	// Update agent's updated_at
	s.agentsCollection().UpdateOne(ctx,
		bson.M{"agentId": agentID},
		bson.M{"$set": bson.M{"updatedAt": now}})

	return workflow, nil
}

// GetWorkflow retrieves a workflow for an agent
func (s *AgentService) GetWorkflow(agentID string) (*models.Workflow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var record WorkflowRecord
	err := s.workflowsCollection().FindOne(ctx, bson.M{"agentId": agentID}).Decode(&record)

	if err == mongo.ErrNoDocuments {
		return nil, fmt.Errorf("workflow not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow: %w", err)
	}

	return record.ToModel(), nil
}

// ============================================================================
// Workflow Version History
// ============================================================================

// ListWorkflowVersions returns all versions for an agent's workflow
func (s *AgentService) ListWorkflowVersions(agentID, userID string) ([]WorkflowVersionResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Verify agent belongs to user
	_, err := s.GetAgent(agentID, userID)
	if err != nil {
		return nil, err
	}

	cursor, err := s.workflowVersionsCollection().Find(ctx,
		bson.M{"agentId": agentID},
		options.Find().SetSort(bson.D{{Key: "version", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("failed to list workflow versions: %w", err)
	}
	defer cursor.Close(ctx)

	var records []WorkflowVersionRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, fmt.Errorf("failed to decode versions: %w", err)
	}

	versions := make([]WorkflowVersionResponse, len(records))
	for i, record := range records {
		versions[i] = WorkflowVersionResponse{
			Version:     record.Version,
			Description: record.Description,
			BlockCount:  len(record.Blocks),
			CreatedAt:   record.CreatedAt,
		}
	}

	return versions, nil
}

// GetWorkflowVersion retrieves a specific workflow version
func (s *AgentService) GetWorkflowVersion(agentID, userID string, version int) (*models.Workflow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Verify agent belongs to user
	_, err := s.GetAgent(agentID, userID)
	if err != nil {
		return nil, err
	}

	var record WorkflowVersionRecord
	err = s.workflowVersionsCollection().FindOne(ctx, bson.M{
		"agentId": agentID,
		"version": version,
	}).Decode(&record)

	if err == mongo.ErrNoDocuments {
		return nil, fmt.Errorf("workflow version not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow version: %w", err)
	}

	return &models.Workflow{
		AgentID:     record.AgentID,
		Blocks:      record.Blocks,
		Connections: record.Connections,
		Variables:   record.Variables,
		Version:     record.Version,
		CreatedAt:   record.CreatedAt,
	}, nil
}

// RestoreWorkflowVersion restores a workflow to a previous version
func (s *AgentService) RestoreWorkflowVersion(agentID, userID string, version int) (*models.Workflow, error) {
	// Get the version to restore
	versionWorkflow, err := s.GetWorkflowVersion(agentID, userID, version)
	if err != nil {
		return nil, err
	}

	// Save as new version with description - restoring always creates a version snapshot
	req := &models.SaveWorkflowRequest{
		Blocks:            versionWorkflow.Blocks,
		Connections:       versionWorkflow.Connections,
		Variables:         versionWorkflow.Variables,
		CreateVersion:     true, // Always create version when restoring
		VersionDescription: fmt.Sprintf("Restored from version %d", version),
	}

	return s.SaveWorkflowWithDescription(agentID, userID, req, "")
}

// ============================================================================
// Pagination Methods
// ============================================================================

// ListAgentsPaginated returns agents with pagination support
func (s *AgentService) ListAgentsPaginated(userID string, limit, offset int) (*models.PaginatedAgentsResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	// Get total count
	total, err := s.agentsCollection().CountDocuments(ctx, bson.M{"userId": userID})
	if err != nil {
		return nil, fmt.Errorf("failed to count agents: %w", err)
	}

	// Get agents with pagination
	cursor, err := s.agentsCollection().Find(ctx,
		bson.M{"userId": userID},
		options.Find().
			SetSort(bson.D{{Key: "updatedAt", Value: -1}}).
			SetSkip(int64(offset)).
			SetLimit(int64(limit)))
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}
	defer cursor.Close(ctx)

	var records []AgentRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, fmt.Errorf("failed to decode agents: %w", err)
	}

	// Build list items with workflow info
	agents := make([]models.AgentListItem, len(records))
	for i, record := range records {
		item := models.AgentListItem{
			ID:          record.AgentID,
			Name:        record.Name,
			Description: record.Description,
			Status:      record.Status,
			CreatedAt:   record.CreatedAt,
			UpdatedAt:   record.UpdatedAt,
		}

		// Get workflow info
		workflow, err := s.GetWorkflow(record.AgentID)
		if err == nil {
			item.HasWorkflow = true
			item.BlockCount = len(workflow.Blocks)
		}

		agents[i] = item
	}

	return &models.PaginatedAgentsResponse{
		Agents:  agents,
		Total:   int(total),
		Limit:   limit,
		Offset:  offset,
		HasMore: offset+len(agents) < int(total),
	}, nil
}

// GetRecentAgents returns the 10 most recently updated agents for the landing page
func (s *AgentService) GetRecentAgents(userID string) (*models.RecentAgentsResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := s.agentsCollection().Find(ctx,
		bson.M{"userId": userID},
		options.Find().
			SetSort(bson.D{{Key: "updatedAt", Value: -1}}).
			SetLimit(10))
	if err != nil {
		return nil, fmt.Errorf("failed to get recent agents: %w", err)
	}
	defer cursor.Close(ctx)

	var records []AgentRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, fmt.Errorf("failed to decode agents: %w", err)
	}

	agents := make([]models.AgentListItem, len(records))
	for i, record := range records {
		item := models.AgentListItem{
			ID:          record.AgentID,
			Name:        record.Name,
			Description: record.Description,
			Status:      record.Status,
			CreatedAt:   record.CreatedAt,
			UpdatedAt:   record.UpdatedAt,
		}

		// Get workflow info
		workflow, err := s.GetWorkflow(record.AgentID)
		if err == nil {
			item.HasWorkflow = true
			item.BlockCount = len(workflow.Blocks)
		}

		agents[i] = item
	}

	return &models.RecentAgentsResponse{
		Agents: agents,
	}, nil
}

// ============================================================================
// Sync Method (for first-message persistence)
// ============================================================================

// SyncAgent creates or updates an agent with its workflow in a single operation
// This is called when a user sends their first message to persist the local agent
func (s *AgentService) SyncAgent(agentID, userID string, req *models.SyncAgentRequest) (*models.Agent, *models.Workflow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	now := time.Now()

	// Check if agent already exists
	existingAgent, err := s.GetAgent(agentID, userID)
	if err != nil && err.Error() != "agent not found" {
		return nil, nil, fmt.Errorf("failed to check existing agent: %w", err)
	}

	var agent *models.Agent

	if existingAgent != nil {
		// Update existing agent
		agent, err = s.UpdateAgent(agentID, userID, &models.UpdateAgentRequest{
			Name:        req.Name,
			Description: req.Description,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("failed to update agent: %w", err)
		}
	} else {
		// Create new agent with the provided ID
		record := &AgentRecord{
			AgentID:     agentID,
			UserID:      userID,
			Name:        req.Name,
			Description: req.Description,
			Status:      "draft",
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		_, err = s.agentsCollection().InsertOne(ctx, record)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create agent: %w", err)
		}

		agent = record.ToModel()
	}

	// Save the workflow
	workflow, err := s.SaveWorkflow(agentID, userID, &req.Workflow)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to save workflow: %w", err)
	}

	return agent, workflow, nil
}

// ============================================================================
// Index Initialization
// ============================================================================

// EnsureIndexes creates indexes for agents and workflows collections
func (s *AgentService) EnsureIndexes(ctx context.Context) error {
	// Agents collection indexes
	agentIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "agentId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "userId", Value: 1},
				{Key: "updatedAt", Value: -1},
			},
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
	}

	_, err := s.agentsCollection().Indexes().CreateMany(ctx, agentIndexes)
	if err != nil {
		return fmt.Errorf("failed to create agent indexes: %w", err)
	}

	// Workflows collection indexes
	workflowIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "agentId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err = s.workflowsCollection().Indexes().CreateMany(ctx, workflowIndexes)
	if err != nil {
		return fmt.Errorf("failed to create workflow indexes: %w", err)
	}

	// Workflow versions collection indexes
	versionIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "agentId", Value: 1},
				{Key: "version", Value: -1},
			},
		},
	}

	_, err = s.workflowVersionsCollection().Indexes().CreateMany(ctx, versionIndexes)
	if err != nil {
		return fmt.Errorf("failed to create workflow version indexes: %w", err)
	}

	log.Println("✅ [AGENT] Ensured indexes for agents, workflows, and workflow_versions collections")
	return nil
}
