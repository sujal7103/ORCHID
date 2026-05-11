package middleware

import (
	"clara-agents/internal/services"
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"
)

// DefaultMaxConcurrentExecutions is the default cap on simultaneous workflow
// executions per user. Prevents a single user from saturating all resources.
const DefaultMaxConcurrentExecutions = 3

// executionSlot tracks a single concurrent execution with its acquire time.
type executionSlot struct {
	count     atomic.Int32
	lastAcquire atomic.Int64 // unix timestamp of last AcquireExecution
}

// ExecutionLimiter middleware checks daily execution limits based on user tier
type ExecutionLimiter struct {
	tierService          *services.TierService
	redis                *redis.Client
	concurrentExecutions sync.Map // userID → *executionSlot
	maxConcurrentPerUser int
	maxSlotAge           time.Duration // auto-release slots older than this
}

// NewExecutionLimiter creates a new execution limiter middleware
func NewExecutionLimiter(tierService *services.TierService, redisClient *redis.Client) *ExecutionLimiter {
	return &ExecutionLimiter{
		tierService:          tierService,
		redis:                redisClient,
		maxConcurrentPerUser: DefaultMaxConcurrentExecutions,
		maxSlotAge:           15 * time.Minute, // auto-release stuck slots after 15 min
	}
}

// CheckLimit verifies if user can execute another workflow today
func (el *ExecutionLimiter) CheckLimit(c *fiber.Ctx) error {
	userID := c.Locals("user_id")
	if userID == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	userIDStr, ok := userID.(string)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid user ID",
		})
	}

	ctx := context.Background()

	// Get user's tier limits
	limits := el.tierService.GetLimits(ctx, userIDStr)

	// If unlimited executions, skip check
	if limits.MaxExecutionsPerDay == -1 {
		return c.Next()
	}

	// Get today's execution count from Redis
	today := time.Now().UTC().Format("2006-01-02")
	key := fmt.Sprintf("executions:%s:%s", userIDStr, today)

	// Get current count
	count, err := el.redis.Get(ctx, key).Int64()
	if err != nil && err != redis.Nil {
		log.Printf("⚠️  Failed to get execution count from Redis: %v", err)
		// On Redis error, allow execution but log warning
		return c.Next()
	}

	// Check if limit exceeded
	if count >= limits.MaxExecutionsPerDay {
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error":     "Daily execution limit exceeded",
			"limit":     limits.MaxExecutionsPerDay,
			"used":      count,
			"reset_at":  getNextMidnightUTC(),
		})
	}

	// Store current count in context for post-execution increment
	c.Locals("execution_count_key", key)

	return c.Next()
}

// IncrementCount increments the execution counter after successful execution start
func (el *ExecutionLimiter) IncrementCount(userID string) error {
	if el.redis == nil {
		return nil // Redis not available, skip increment
	}

	ctx := context.Background()
	today := time.Now().UTC().Format("2006-01-02")
	key := fmt.Sprintf("executions:%s:%s", userID, today)

	// Increment counter
	pipe := el.redis.Pipeline()
	pipe.Incr(ctx, key)

	// Set expiry to end of day + 1 day (to allow historical querying)
	midnight := getNextMidnightUTC()
	expiryDuration := time.Until(midnight) + 24*time.Hour
	pipe.Expire(ctx, key, expiryDuration)

	_, err := pipe.Exec(ctx)
	if err != nil {
		log.Printf("⚠️  Failed to increment execution count: %v", err)
		return err
	}

	log.Printf("✅ Incremented execution count for user %s (key: %s)", userID, key)
	return nil
}

// GetRemainingExecutions returns how many executions user has left today
func (el *ExecutionLimiter) GetRemainingExecutions(userID string) (int64, error) {
	if el.redis == nil {
		return -1, nil // Redis not available, return unlimited
	}

	ctx := context.Background()

	// Get user's tier limits
	limits := el.tierService.GetLimits(ctx, userID)
	if limits.MaxExecutionsPerDay == -1 {
		return -1, nil // Unlimited
	}

	// Get today's count
	today := time.Now().UTC().Format("2006-01-02")
	key := fmt.Sprintf("executions:%s:%s", userID, today)

	count, err := el.redis.Get(ctx, key).Int64()
	if err == redis.Nil {
		return limits.MaxExecutionsPerDay, nil // No executions today
	}
	if err != nil {
		return -1, err
	}

	remaining := limits.MaxExecutionsPerDay - count
	if remaining < 0 {
		return 0, nil
	}

	return remaining, nil
}

// CheckConcurrentLimit rejects the request if the user already has too many
// workflows executing simultaneously. Call ReleaseExecution when done.
func (el *ExecutionLimiter) CheckConcurrentLimit(c *fiber.Ctx) error {
	userID := c.Locals("user_id")
	if userID == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Unauthorized"})
	}
	userIDStr, ok := userID.(string)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Invalid user ID"})
	}

	slot := el.getOrCreateSlot(userIDStr)
	el.autoReleaseIfStale(userIDStr, slot)
	current := slot.count.Add(1)
	if int(current) > el.maxConcurrentPerUser {
		slot.count.Add(-1)
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
			"error":       "Too many concurrent executions",
			"max_allowed": el.maxConcurrentPerUser,
			"active":      int(current) - 1,
		})
	}
	slot.lastAcquire.Store(time.Now().Unix())

	// Store in locals so the handler can call ReleaseExecution later
	c.Locals("concurrent_user_id", userIDStr)
	return c.Next()
}

// AcquireExecution increments the concurrent counter for a user.
// Returns false if the limit is exceeded (caller should not proceed).
func (el *ExecutionLimiter) AcquireExecution(userID string) bool {
	slot := el.getOrCreateSlot(userID)
	// Auto-release stale slots before checking (prevents permanent lockout)
	el.autoReleaseIfStale(userID, slot)
	current := slot.count.Add(1)
	if int(current) > el.maxConcurrentPerUser {
		slot.count.Add(-1)
		log.Printf("⚠️ [LIMITER] User %s rejected: %d/%d concurrent executions",
			userID, int(current)-1, el.maxConcurrentPerUser)
		return false
	}
	slot.lastAcquire.Store(time.Now().Unix())
	log.Printf("📊 [LIMITER] User %s: %d/%d concurrent executions",
		userID, int(current), el.maxConcurrentPerUser)
	return true
}

// ReleaseExecution decrements the concurrent counter for a user.
// Must be called when a workflow execution finishes (success or failure).
func (el *ExecutionLimiter) ReleaseExecution(userID string) {
	slot := el.getOrCreateSlot(userID)
	val := slot.count.Add(-1)
	if val < 0 {
		slot.count.Store(0) // safety: never go negative
	}
	log.Printf("📊 [LIMITER] User %s released: %d/%d concurrent executions",
		userID, max(int64(val), 0), el.maxConcurrentPerUser)
}

// autoReleaseIfStale resets the counter to 0 if the slot has been held longer
// than maxSlotAge. This prevents permanent lockout from leaked slots.
func (el *ExecutionLimiter) autoReleaseIfStale(userID string, slot *executionSlot) {
	current := slot.count.Load()
	if current <= 0 {
		return
	}
	acquired := slot.lastAcquire.Load()
	if acquired == 0 {
		return
	}
	age := time.Since(time.Unix(acquired, 0))
	if age > el.maxSlotAge {
		slot.count.Store(0)
		log.Printf("🔓 [LIMITER] Auto-released stale slots for user %s (held for %s, count was %d)",
			userID, age.Round(time.Second), current)
	}
}

// ResetUser forces the concurrent execution counter for a user to zero.
// Use this to unblock users who got stuck due to leaked slots.
func (el *ExecutionLimiter) ResetUser(userID string) {
	el.concurrentExecutions.Delete(userID)
	log.Printf("🔓 [LIMITER] Force-reset concurrent slots for user %s", userID)
}

// ResetAll forces all concurrent execution counters to zero.
func (el *ExecutionLimiter) ResetAll() {
	el.concurrentExecutions.Range(func(key, _ any) bool {
		el.concurrentExecutions.Delete(key)
		return true
	})
	log.Printf("🔓 [LIMITER] Force-reset ALL concurrent slots")
}

func (el *ExecutionLimiter) getOrCreateSlot(userID string) *executionSlot {
	if v, ok := el.concurrentExecutions.Load(userID); ok {
		return v.(*executionSlot)
	}
	slot := &executionSlot{}
	actual, _ := el.concurrentExecutions.LoadOrStore(userID, slot)
	return actual.(*executionSlot)
}

// getNextMidnightUTC returns the next midnight UTC
func getNextMidnightUTC() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
}
