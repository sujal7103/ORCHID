package services

import (
	"clara-agents/internal/database"
	"clara-agents/internal/models"
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// SchedulerService manages scheduled agent executions
type SchedulerService struct {
	scheduler        gocron.Scheduler
	mongoDB          *database.MongoDB
	redisService     *RedisService
	agentService     *AgentService
	executionService *ExecutionService
	workflowExecutor models.WorkflowExecuteFunc
	instanceID       string
	mu               sync.RWMutex
	jobs             map[string]gocron.Job // scheduleID -> job
}

// NewSchedulerService creates a new scheduler service
func NewSchedulerService(
	mongoDB *database.MongoDB,
	redisService *RedisService,
	agentService *AgentService,
	executionService *ExecutionService,
) (*SchedulerService, error) {
	// Create scheduler with second-level precision
	scheduler, err := gocron.NewScheduler(
		gocron.WithLocation(time.UTC),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create scheduler: %w", err)
	}

	return &SchedulerService{
		scheduler:        scheduler,
		mongoDB:          mongoDB,
		redisService:     redisService,
		agentService:     agentService,
		executionService: executionService,
		instanceID:       uuid.New().String(),
		jobs:             make(map[string]gocron.Job),
	}, nil
}

// Start starts the scheduler and loads all enabled schedules
func (s *SchedulerService) Start(ctx context.Context) error {
	log.Println("⏰ Starting scheduler service...")

	// Load and register all enabled schedules
	if err := s.loadSchedules(ctx); err != nil {
		log.Printf("⚠️ Failed to load schedules: %v", err)
	}

	// Start the scheduler
	s.scheduler.Start()
	log.Println("✅ Scheduler service started")

	return nil
}

// Stop stops the scheduler
func (s *SchedulerService) Stop() error {
	log.Println("⏹️ Stopping scheduler service...")
	return s.scheduler.Shutdown()
}

// SetWorkflowExecutor sets the workflow executor function (used for deferred initialization)
func (s *SchedulerService) SetWorkflowExecutor(executor models.WorkflowExecuteFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.workflowExecutor = executor
}

// loadSchedules loads all enabled schedules from MongoDB and registers them
func (s *SchedulerService) loadSchedules(ctx context.Context) error {
	if s.mongoDB == nil {
		log.Println("⚠️ MongoDB not available, skipping schedule loading")
		return nil
	}

	collection := s.mongoDB.Database().Collection("schedules")

	cursor, err := collection.Find(ctx, bson.M{"enabled": true})
	if err != nil {
		return fmt.Errorf("failed to query schedules: %w", err)
	}
	defer cursor.Close(ctx)

	var count int
	for cursor.Next(ctx) {
		var schedule models.Schedule
		if err := cursor.Decode(&schedule); err != nil {
			log.Printf("⚠️ Failed to decode schedule: %v", err)
			continue
		}

		if err := s.registerJob(&schedule); err != nil {
			log.Printf("⚠️ Failed to register schedule %s: %v", schedule.ID.Hex(), err)
			continue
		}
		count++
	}

	log.Printf("✅ Loaded %d schedules", count)
	return nil
}

// registerJob registers a schedule with gocron
func (s *SchedulerService) registerJob(schedule *models.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate timezone
	_, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		return fmt.Errorf("invalid timezone %s: %w", schedule.Timezone, err)
	}

	// Build cron expression with timezone prefix (CRON_TZ=America/New_York 0 9 * * *)
	cronWithTZ := fmt.Sprintf("CRON_TZ=%s %s", schedule.Timezone, schedule.CronExpression)

	// Create the job
	job, err := s.scheduler.NewJob(
		gocron.CronJob(cronWithTZ, false),
		gocron.NewTask(func() {
			s.executeScheduledJob(schedule)
		}),
		gocron.WithName(schedule.ID.Hex()),
		gocron.WithTags(schedule.AgentID, schedule.UserID),
	)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	s.jobs[schedule.ID.Hex()] = job
	log.Printf("📅 Registered schedule %s for agent %s (cron: %s, tz: %s)",
		schedule.ID.Hex(), schedule.AgentID, schedule.CronExpression, schedule.Timezone)

	return nil
}

// unregisterJob removes a job from the scheduler
func (s *SchedulerService) unregisterJob(scheduleID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, exists := s.jobs[scheduleID]
	if !exists {
		return nil
	}

	if err := s.scheduler.RemoveJob(job.ID()); err != nil {
		return fmt.Errorf("failed to remove job: %w", err)
	}

	delete(s.jobs, scheduleID)
	log.Printf("🗑️ Unregistered schedule %s", scheduleID)

	return nil
}

// executeScheduledJob executes a scheduled agent workflow
func (s *SchedulerService) executeScheduledJob(schedule *models.Schedule) {
	ctx := context.Background()

	// Create a unique lock key for this schedule execution window
	// Using minute-level granularity to prevent duplicate runs within the same minute
	lockKey := fmt.Sprintf("schedule-lock:%s:%d", schedule.ID.Hex(), time.Now().Unix()/60)

	// Try to acquire distributed lock
	acquired, err := s.redisService.AcquireLock(ctx, lockKey, s.instanceID, 5*time.Minute)
	if err != nil {
		log.Printf("❌ Failed to acquire lock for schedule %s: %v", schedule.ID.Hex(), err)
		return
	}

	if !acquired {
		// Another instance is handling this execution
		log.Printf("⏭️ Schedule %s already being executed by another instance", schedule.ID.Hex())
		return
	}

	// Release lock when done
	defer func() {
		if _, err := s.redisService.ReleaseLock(ctx, lockKey, s.instanceID); err != nil {
			log.Printf("⚠️ Failed to release lock for schedule %s: %v", schedule.ID.Hex(), err)
		}
	}()

	log.Printf("▶️ Executing scheduled job for agent %s (schedule: %s)", schedule.AgentID, schedule.ID.Hex())

	// Get the agent and workflow
	// Note: We use a system context here since scheduled jobs run without a user session
	agent, err := s.agentService.GetAgentByID(schedule.AgentID)
	if err != nil {
		log.Printf("❌ Failed to get agent %s: %v", schedule.AgentID, err)
		s.updateScheduleStats(ctx, schedule.ID, false, schedule)
		return
	}

	if agent.Workflow == nil {
		log.Printf("❌ Agent %s has no workflow", schedule.AgentID)
		s.updateScheduleStats(ctx, schedule.ID, false, schedule)
		return
	}

	// Build input from template
	input := make(map[string]interface{})
	if schedule.InputTemplate != nil {
		for k, v := range schedule.InputTemplate {
			input[k] = v
		}
	}

	// CRITICAL: Inject user context for credential resolution and tool execution
	// Without this, tools like send_discord_message cannot access user credentials
	input["__user_id__"] = schedule.UserID
	log.Printf("🔐 [SCHEDULER] Injecting user context: __user_id__=%s for schedule %s", schedule.UserID, schedule.ID.Hex())

	// Create execution record in MongoDB
	var execRecord *ExecutionRecord
	if s.executionService != nil {
		var err error
		execRecord, err = s.executionService.Create(ctx, &CreateExecutionRequest{
			AgentID:         schedule.AgentID,
			UserID:          schedule.UserID,
			WorkflowVersion: agent.Workflow.Version,
			TriggerType:     "scheduled",
			ScheduleID:      schedule.ID,
			Input:           input,
		})
		if err != nil {
			log.Printf("⚠️ Failed to create execution record: %v", err)
		} else {
			// Mark as running
			s.executionService.UpdateStatus(ctx, execRecord.ID, "running")
		}
	}

	// Check if workflow executor is available
	s.mu.RLock()
	executor := s.workflowExecutor
	s.mu.RUnlock()

	if executor == nil {
		log.Printf("❌ Workflow executor not set for schedule %s", schedule.ID.Hex())
		s.updateScheduleStats(ctx, schedule.ID, false, schedule)
		if execRecord != nil {
			s.executionService.Complete(ctx, execRecord.ID, &ExecutionCompleteRequest{
				Status: "failed",
				Error:  "Workflow executor not available",
			})
		}
		return
	}

	// Execute the workflow using the callback function
	result, execErr := executor(agent.Workflow, input)

	// Determine success status
	status := "failed"
	if result != nil {
		status = result.Status
	}
	success := status == "completed" && execErr == nil

	// Complete the execution record
	if execRecord != nil {
		completeReq := &ExecutionCompleteRequest{
			Status: status,
		}
		if execErr != nil {
			completeReq.Error = execErr.Error()
		} else if result != nil {
			completeReq.Output = result.Output
			completeReq.BlockStates = result.BlockStates
			if result.Error != "" {
				completeReq.Error = result.Error
			}
		}
		s.executionService.Complete(ctx, execRecord.ID, completeReq)
		log.Printf("📊 Scheduled execution %s completed with status: %s", execRecord.ID.Hex(), status)
	}

	if success {
		log.Printf("✅ Scheduled execution completed successfully for agent %s", schedule.AgentID)
	} else {
		errMsg := ""
		if execErr != nil {
			errMsg = execErr.Error()
		} else if result != nil && result.Error != "" {
			errMsg = result.Error
		}
		log.Printf("❌ Scheduled execution failed for agent %s: %s (status: %s)", schedule.AgentID, errMsg, status)
	}

	// Update schedule statistics and next run time
	s.updateScheduleStats(ctx, schedule.ID, success, schedule)
}

// updateScheduleStats updates the schedule's run statistics and next run time
func (s *SchedulerService) updateScheduleStats(ctx context.Context, scheduleID primitive.ObjectID, success bool, schedule *models.Schedule) {
	if s.mongoDB == nil {
		return
	}

	collection := s.mongoDB.Database().Collection("schedules")

	now := time.Now()

	// Calculate next run time
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	cronSchedule, err := parser.Parse(schedule.CronExpression)
	var nextRun time.Time
	if err == nil {
		loc, locErr := time.LoadLocation(schedule.Timezone)
		if locErr == nil {
			nextRun = cronSchedule.Next(now.In(loc))
		} else {
			nextRun = cronSchedule.Next(now)
		}
	}

	update := bson.M{
		"$set": bson.M{
			"lastRunAt": now,
			"updatedAt": now,
			"nextRunAt": nextRun,
		},
		"$inc": bson.M{
			"totalRuns": 1,
		},
	}

	if success {
		update["$inc"].(bson.M)["successfulRuns"] = 1
	} else {
		update["$inc"].(bson.M)["failedRuns"] = 1
	}

	if _, err := collection.UpdateByID(ctx, scheduleID, update); err != nil {
		log.Printf("⚠️ Failed to update schedule stats: %v", err)
	} else {
		log.Printf("📅 Updated next run time to %v for schedule %s", nextRun, scheduleID.Hex())
	}
}

// CreateSchedule creates a new schedule for an agent
func (s *SchedulerService) CreateSchedule(ctx context.Context, agentID, userID string, req *models.CreateScheduleRequest) (*models.Schedule, error) {
	if s.mongoDB == nil {
		return nil, fmt.Errorf("MongoDB not available")
	}

	// Validate cron expression
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(req.CronExpression); err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}

	// Validate timezone
	loc, err := time.LoadLocation(req.Timezone)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone: %w", err)
	}

	// Check user's schedule limit
	limits := models.GetTierLimits(s.getUserTier(ctx, userID))
	if limits.MaxSchedules > 0 {
		count, err := s.countUserSchedules(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to check schedule limit: %w", err)
		}
		if count >= int64(limits.MaxSchedules) {
			return nil, fmt.Errorf("active schedule limit reached (%d/%d). Pause an existing schedule to create a new one", count, limits.MaxSchedules)
		}
	}

	// Check if agent already has a schedule
	existing, _ := s.GetScheduleByAgentID(ctx, agentID, userID)
	if existing != nil {
		return nil, fmt.Errorf("agent already has a schedule")
	}

	// Calculate next run time
	schedule, _ := parser.Parse(req.CronExpression)
	nextRun := schedule.Next(time.Now().In(loc))

	// Default enabled to true
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	now := time.Now()
	doc := &models.Schedule{
		ID:             primitive.NewObjectID(),
		AgentID:        agentID,
		UserID:         userID,
		CronExpression: req.CronExpression,
		Timezone:       req.Timezone,
		Enabled:        enabled,
		InputTemplate:  req.InputTemplate,
		NextRunAt:      &nextRun,
		TotalRuns:      0,
		SuccessfulRuns: 0,
		FailedRuns:     0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	collection := s.mongoDB.Database().Collection("schedules")
	if _, err := collection.InsertOne(ctx, doc); err != nil {
		return nil, fmt.Errorf("failed to create schedule: %w", err)
	}

	// Register with scheduler if enabled
	if enabled {
		if err := s.registerJob(doc); err != nil {
			log.Printf("⚠️ Failed to register new schedule: %v", err)
		}
	}

	log.Printf("✅ Created schedule %s for agent %s", doc.ID.Hex(), agentID)
	return doc, nil
}

// GetSchedule retrieves a schedule by ID
func (s *SchedulerService) GetSchedule(ctx context.Context, scheduleID, userID string) (*models.Schedule, error) {
	if s.mongoDB == nil {
		return nil, fmt.Errorf("MongoDB not available")
	}

	objID, err := primitive.ObjectIDFromHex(scheduleID)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule ID")
	}

	collection := s.mongoDB.Database().Collection("schedules")

	var schedule models.Schedule
	err = collection.FindOne(ctx, bson.M{
		"_id":    objID,
		"userId": userID,
	}).Decode(&schedule)

	if err == mongo.ErrNoDocuments {
		return nil, fmt.Errorf("schedule not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}

	return &schedule, nil
}

// GetScheduleByAgentID retrieves a schedule by agent ID
func (s *SchedulerService) GetScheduleByAgentID(ctx context.Context, agentID, userID string) (*models.Schedule, error) {
	if s.mongoDB == nil {
		return nil, fmt.Errorf("MongoDB not available")
	}

	collection := s.mongoDB.Database().Collection("schedules")

	var schedule models.Schedule
	err := collection.FindOne(ctx, bson.M{
		"agentId": agentID,
		"userId":  userID,
	}).Decode(&schedule)

	if err == mongo.ErrNoDocuments {
		return nil, fmt.Errorf("schedule not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get schedule: %w", err)
	}

	return &schedule, nil
}

// UpdateSchedule updates a schedule
func (s *SchedulerService) UpdateSchedule(ctx context.Context, scheduleID, userID string, req *models.UpdateScheduleRequest) (*models.Schedule, error) {
	if s.mongoDB == nil {
		return nil, fmt.Errorf("MongoDB not available")
	}

	// Get existing schedule
	schedule, err := s.GetSchedule(ctx, scheduleID, userID)
	if err != nil {
		return nil, err
	}

	// Build update
	update := bson.M{
		"updatedAt": time.Now(),
	}

	if req.CronExpression != nil {
		// Validate cron expression
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(*req.CronExpression); err != nil {
			return nil, fmt.Errorf("invalid cron expression: %w", err)
		}
		update["cronExpression"] = *req.CronExpression
		schedule.CronExpression = *req.CronExpression
	}

	if req.Timezone != nil {
		if _, err := time.LoadLocation(*req.Timezone); err != nil {
			return nil, fmt.Errorf("invalid timezone: %w", err)
		}
		update["timezone"] = *req.Timezone
		schedule.Timezone = *req.Timezone
	}

	if req.InputTemplate != nil {
		update["inputTemplate"] = req.InputTemplate
		schedule.InputTemplate = req.InputTemplate
	}

	if req.Enabled != nil {
		update["enabled"] = *req.Enabled
		schedule.Enabled = *req.Enabled
	}

	// Recalculate next run time
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	cronSchedule, _ := parser.Parse(schedule.CronExpression)
	loc, _ := time.LoadLocation(schedule.Timezone)
	nextRun := cronSchedule.Next(time.Now().In(loc))
	update["nextRunAt"] = nextRun

	collection := s.mongoDB.Database().Collection("schedules")
	_, err = collection.UpdateByID(ctx, schedule.ID, bson.M{"$set": update})
	if err != nil {
		return nil, fmt.Errorf("failed to update schedule: %w", err)
	}

	// Re-register job if cron or timezone changed, or enable/disable
	s.unregisterJob(scheduleID)
	if schedule.Enabled {
		schedule.NextRunAt = &nextRun
		if err := s.registerJob(schedule); err != nil {
			log.Printf("⚠️ Failed to re-register schedule: %v", err)
		}
	}

	return schedule, nil
}

// DeleteSchedule deletes a schedule
func (s *SchedulerService) DeleteSchedule(ctx context.Context, scheduleID, userID string) error {
	if s.mongoDB == nil {
		return fmt.Errorf("MongoDB not available")
	}

	objID, err := primitive.ObjectIDFromHex(scheduleID)
	if err != nil {
		return fmt.Errorf("invalid schedule ID")
	}

	// Unregister from scheduler
	s.unregisterJob(scheduleID)

	collection := s.mongoDB.Database().Collection("schedules")
	result, err := collection.DeleteOne(ctx, bson.M{
		"_id":    objID,
		"userId": userID,
	})

	if err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("schedule not found")
	}

	log.Printf("🗑️ Deleted schedule %s", scheduleID)
	return nil
}

// DeleteAllByUser deletes all schedules for a user (GDPR compliance)
func (s *SchedulerService) DeleteAllByUser(ctx context.Context, userID string) (int64, error) {
	if s.mongoDB == nil {
		return 0, nil // No MongoDB, no schedules to delete
	}

	if userID == "" {
		return 0, fmt.Errorf("user ID is required")
	}

	// First, unregister all jobs for this user from the scheduler
	collection := s.mongoDB.Database().Collection("schedules")
	cursor, err := collection.Find(ctx, bson.M{"userId": userID})
	if err != nil {
		return 0, fmt.Errorf("failed to find user schedules: %w", err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var schedule struct {
			ID primitive.ObjectID `bson:"_id"`
		}
		if err := cursor.Decode(&schedule); err == nil {
			s.unregisterJob(schedule.ID.Hex())
		}
	}

	// Delete all schedules
	result, err := collection.DeleteMany(ctx, bson.M{"userId": userID})
	if err != nil {
		return 0, fmt.Errorf("failed to delete user schedules: %w", err)
	}

	log.Printf("🗑️ [GDPR] Deleted %d schedules for user %s", result.DeletedCount, userID)
	return result.DeletedCount, nil
}

// TriggerNow triggers an immediate execution of a schedule
func (s *SchedulerService) TriggerNow(ctx context.Context, scheduleID, userID string) error {
	schedule, err := s.GetSchedule(ctx, scheduleID, userID)
	if err != nil {
		return err
	}

	// Execute in background
	go s.executeScheduledJob(schedule)

	return nil
}

// countUserSchedules counts the number of ENABLED schedules for a user
// Only enabled schedules count toward the limit - paused schedules don't consume quota
// This allows users to pause schedules to free up slots for new ones
func (s *SchedulerService) countUserSchedules(ctx context.Context, userID string) (int64, error) {
	collection := s.mongoDB.Database().Collection("schedules")
	return collection.CountDocuments(ctx, bson.M{"userId": userID, "enabled": true})
}

// ScheduleUsage represents the user's schedule usage stats
type ScheduleUsage struct {
	Active    int64 `json:"active"`
	Paused    int64 `json:"paused"`
	Total     int64 `json:"total"`
	Limit     int   `json:"limit"`
	CanCreate bool  `json:"canCreate"`
}

// GetScheduleUsage returns the user's schedule usage statistics
func (s *SchedulerService) GetScheduleUsage(ctx context.Context, userID string) (*ScheduleUsage, error) {
	collection := s.mongoDB.Database().Collection("schedules")

	// Count active (enabled) schedules
	active, err := collection.CountDocuments(ctx, bson.M{"userId": userID, "enabled": true})
	if err != nil {
		return nil, fmt.Errorf("failed to count active schedules: %w", err)
	}

	// Count paused (disabled) schedules
	paused, err := collection.CountDocuments(ctx, bson.M{"userId": userID, "enabled": false})
	if err != nil {
		return nil, fmt.Errorf("failed to count paused schedules: %w", err)
	}

	// Get user's limit
	limits := models.GetTierLimits(s.getUserTier(ctx, userID))
	limit := limits.MaxSchedules

	// Can create if active < limit (or limit is -1 for unlimited)
	canCreate := limit < 0 || active < int64(limit)

	return &ScheduleUsage{
		Active:    active,
		Paused:    paused,
		Total:     active + paused,
		Limit:     limit,
		CanCreate: canCreate,
	}, nil
}

// getUserTier gets the user's subscription tier (placeholder - will be implemented with UserService)
func (s *SchedulerService) getUserTier(ctx context.Context, userID string) string {
	// TODO: Look up user's tier from MongoDB
	return "free"
}

// InitializeIndexes creates the necessary indexes for the schedules collection
func (s *SchedulerService) InitializeIndexes(ctx context.Context) error {
	if s.mongoDB == nil {
		return nil
	}

	collection := s.mongoDB.Database().Collection("schedules")

	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "agentId", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "userId", Value: 1}, {Key: "enabled", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "nextRunAt", Value: 1}, {Key: "enabled", Value: 1}},
		},
	}

	_, err := collection.Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("failed to create indexes: %w", err)
	}

	log.Println("✅ Schedule indexes created")
	return nil
}
