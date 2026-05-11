package services

import (
	"clara-agents/internal/database"
	"clara-agents/internal/models"
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
)

// UsageLimiterService tracks and enforces usage limits for messages, file uploads, and image generation
type UsageLimiterService struct {
	tierService *TierService
	redis       *redis.Client
	mongoDB     *database.MongoDB
}

// UsageLimiterStats holds current usage statistics for a user
type UsageLimiterStats struct {
	MessagesUsed      int64     `json:"messages_used"`
	FileUploadsUsed   int64     `json:"file_uploads_used"`
	ImageGensUsed     int64     `json:"image_gens_used"`
	MessageResetAt    time.Time `json:"message_reset_at"`
	FileUploadResetAt time.Time `json:"file_upload_reset_at"`
	ImageGenResetAt   time.Time `json:"image_gen_reset_at"`
}

// LimitExceededError represents a rate limit error
type LimitExceededError struct {
	ErrorCode string    `json:"error_code"`
	Message   string    `json:"message"`
	Limit     int64     `json:"limit"`
	Used      int64     `json:"used"`
	ResetAt   time.Time `json:"reset_at"`
	UpgradeTo string    `json:"upgrade_to"`
}

func (e *LimitExceededError) Error() string {
	return e.Message
}

// NewUsageLimiterService creates a new usage limiter service
func NewUsageLimiterService(tierService *TierService, redis *redis.Client, mongoDB *database.MongoDB) *UsageLimiterService {
	return &UsageLimiterService{
		tierService: tierService,
		redis:       redis,
		mongoDB:     mongoDB,
	}
}

// ========== Message Limits (Monthly - Billing Cycle Reset) ==========

// CheckMessageLimit checks if user can send another message this month
func (s *UsageLimiterService) CheckMessageLimit(ctx context.Context, userID string) error {
	limits := s.tierService.GetLimits(ctx, userID)

	// Unlimited
	if limits.MaxMessagesPerMonth < 0 {
		return nil
	}

	// Get current count
	count, err := s.GetMonthlyMessageCount(ctx, userID)
	if err != nil {
		// On error, allow request (fail open)
		return nil
	}

	// Check limit
	if count >= limits.MaxMessagesPerMonth {
		resetAt, _ := s.getMonthlyResetTime(ctx, userID)
		return &LimitExceededError{
			ErrorCode: "message_limit_exceeded",
			Message:   fmt.Sprintf("Monthly message limit reached (%d/%d). Resets on %s. Upgrade to Pro for 3,000 messages/month.", count, limits.MaxMessagesPerMonth, resetAt.Format("Jan 2")),
			Limit:     limits.MaxMessagesPerMonth,
			Used:      count,
			ResetAt:   resetAt,
			UpgradeTo: s.getSuggestedUpgradeTier(s.tierService.GetUserTier(ctx, userID)),
		}
	}

	return nil
}

// IncrementMessageCount increments the user's monthly message count
func (s *UsageLimiterService) IncrementMessageCount(ctx context.Context, userID string) error {
	key, err := s.getMessageKey(ctx, userID)
	if err != nil {
		return err
	}

	// Increment counter
	_, err = s.redis.Incr(ctx, key).Result()
	if err != nil {
		return err
	}

	// Set expiry (billing period end + 30 days buffer)
	resetAt, err := s.getMonthlyResetTime(ctx, userID)
	if err == nil {
		expiry := time.Until(resetAt.AddDate(0, 1, 0)) // Add 30 days buffer
		s.redis.Expire(ctx, key, expiry)
	}

	return nil
}

// GetMonthlyMessageCount returns the user's current message count for this billing period
func (s *UsageLimiterService) GetMonthlyMessageCount(ctx context.Context, userID string) (int64, error) {
	key, err := s.getMessageKey(ctx, userID)
	if err != nil {
		return 0, err
	}

	count, err := s.redis.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// ========== Anonymous Guest Message Limits (Daily - Per IP, 24h expiry) ==========

const AnonymousMessageLimit int64 = 50

// CheckAnonymousMessageLimit checks if an anonymous user (by IP) can send another message today
func (s *UsageLimiterService) CheckAnonymousMessageLimit(ctx context.Context, clientIP string) error {
	count, err := s.GetAnonymousDailyMessageCount(ctx, clientIP)
	if err != nil {
		// On error, allow request (fail open)
		return nil
	}

	if count >= AnonymousMessageLimit {
		resetAt := s.getNextMidnightUTC()
		return &LimitExceededError{
			ErrorCode: "anonymous_limit_exceeded",
			Message:   fmt.Sprintf("Guest message limit reached (%d/%d). Sign in to continue chatting.", count, AnonymousMessageLimit),
			Limit:     AnonymousMessageLimit,
			Used:      count,
			ResetAt:   resetAt,
			UpgradeTo: "free",
		}
	}

	return nil
}

// IncrementAnonymousMessageCount increments the daily message count for an anonymous IP
func (s *UsageLimiterService) IncrementAnonymousMessageCount(ctx context.Context, clientIP string) error {
	key := s.getAnonymousMessageKey(clientIP)

	_, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		return err
	}

	// Expire at next midnight + 24h buffer
	resetAt := s.getNextMidnightUTC()
	expiry := time.Until(resetAt.Add(24 * time.Hour))
	s.redis.Expire(ctx, key, expiry)

	return nil
}

// GetAnonymousDailyMessageCount returns how many messages an anonymous IP has sent today
func (s *UsageLimiterService) GetAnonymousDailyMessageCount(ctx context.Context, clientIP string) (int64, error) {
	key := s.getAnonymousMessageKey(clientIP)

	count, err := s.redis.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// ========== File Upload Limits (Daily - Midnight UTC Reset) ==========

// CheckFileUploadLimit checks if user can upload another file today
func (s *UsageLimiterService) CheckFileUploadLimit(ctx context.Context, userID string) error {
	limits := s.tierService.GetLimits(ctx, userID)

	// Unlimited
	if limits.MaxFileUploadsPerDay < 0 {
		return nil
	}

	// Get current count
	count, err := s.GetDailyFileUploadCount(ctx, userID)
	if err != nil {
		// On error, allow request (fail open)
		return nil
	}

	// Check limit
	if count >= limits.MaxFileUploadsPerDay {
		resetAt := s.getNextMidnightUTC()
		return &LimitExceededError{
			ErrorCode: "file_upload_limit_exceeded",
			Message:   fmt.Sprintf("Daily file upload limit reached (%d/%d). Resets at midnight UTC. Upgrade to Pro for 10 uploads/day.", count, limits.MaxFileUploadsPerDay),
			Limit:     limits.MaxFileUploadsPerDay,
			Used:      count,
			ResetAt:   resetAt,
			UpgradeTo: s.getSuggestedUpgradeTier(s.tierService.GetUserTier(ctx, userID)),
		}
	}

	return nil
}

// IncrementFileUploadCount increments the user's daily file upload count
func (s *UsageLimiterService) IncrementFileUploadCount(ctx context.Context, userID string) error {
	key := s.getFileUploadKey(userID)

	// Increment counter
	_, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		return err
	}

	// Set expiry to next midnight + 24 hours buffer
	resetAt := s.getNextMidnightUTC()
	expiry := time.Until(resetAt.Add(24 * time.Hour))
	s.redis.Expire(ctx, key, expiry)

	return nil
}

// GetDailyFileUploadCount returns the user's current file upload count for today
func (s *UsageLimiterService) GetDailyFileUploadCount(ctx context.Context, userID string) (int64, error) {
	key := s.getFileUploadKey(userID)

	count, err := s.redis.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// ========== Image Generation Limits (Daily - Midnight UTC Reset) ==========

// CheckImageGenLimit checks if user can generate another image today
func (s *UsageLimiterService) CheckImageGenLimit(ctx context.Context, userID string) error {
	limits := s.tierService.GetLimits(ctx, userID)

	// Unlimited
	if limits.MaxImageGensPerDay < 0 {
		return nil
	}

	// Get current count
	count, err := s.GetDailyImageGenCount(ctx, userID)
	if err != nil {
		// On error, allow request (fail open)
		return nil
	}

	// Check limit
	if count >= limits.MaxImageGensPerDay {
		resetAt := s.getNextMidnightUTC()
		return &LimitExceededError{
			ErrorCode: "image_gen_limit_exceeded",
			Message:   fmt.Sprintf("Daily image generation limit reached (%d/%d). Resets at midnight UTC. Upgrade to Pro for 25 images/day.", count, limits.MaxImageGensPerDay),
			Limit:     limits.MaxImageGensPerDay,
			Used:      count,
			ResetAt:   resetAt,
			UpgradeTo: s.getSuggestedUpgradeTier(s.tierService.GetUserTier(ctx, userID)),
		}
	}

	return nil
}

// IncrementImageGenCount increments the user's daily image generation count
func (s *UsageLimiterService) IncrementImageGenCount(ctx context.Context, userID string) error {
	key := s.getImageGenKey(userID)

	// Increment counter
	_, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		return err
	}

	// Set expiry to next midnight + 24 hours buffer
	resetAt := s.getNextMidnightUTC()
	expiry := time.Until(resetAt.Add(24 * time.Hour))
	s.redis.Expire(ctx, key, expiry)

	return nil
}

// GetDailyImageGenCount returns the user's current image generation count for today
func (s *UsageLimiterService) GetDailyImageGenCount(ctx context.Context, userID string) (int64, error) {
	key := s.getImageGenKey(userID)

	count, err := s.redis.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// ========== Utility Methods ==========

// GetUsageStats returns comprehensive usage statistics for a user
func (s *UsageLimiterService) GetUsageStats(ctx context.Context, userID string) (*UsageLimiterStats, error) {
	msgCount, _ := s.GetMonthlyMessageCount(ctx, userID)
	fileCount, _ := s.GetDailyFileUploadCount(ctx, userID)
	imageCount, _ := s.GetDailyImageGenCount(ctx, userID)

	msgResetAt, _ := s.getMonthlyResetTime(ctx, userID)
	dailyResetAt := s.getNextMidnightUTC()

	return &UsageLimiterStats{
		MessagesUsed:      msgCount,
		FileUploadsUsed:   fileCount,
		ImageGensUsed:     imageCount,
		MessageResetAt:    msgResetAt,
		FileUploadResetAt: dailyResetAt,
		ImageGenResetAt:   dailyResetAt,
	}, nil
}

// ResetMonthlyCounters resets the monthly message counter for a user
func (s *UsageLimiterService) ResetMonthlyCounters(ctx context.Context, userID string) error {
	key, err := s.getMessageKey(ctx, userID)
	if err != nil {
		return err
	}
	return s.redis.Del(ctx, key).Err()
}

// ResetAllCounters resets all usage counters for a user (used on tier upgrade)
func (s *UsageLimiterService) ResetAllCounters(ctx context.Context, userID string) error {
	// Reset monthly message counter
	msgKey, _ := s.getMessageKey(ctx, userID)
	s.redis.Del(ctx, msgKey)

	// Reset daily file upload counter
	fileKey := s.getFileUploadKey(userID)
	s.redis.Del(ctx, fileKey)

	// Reset daily image gen counter
	imageKey := s.getImageGenKey(userID)
	s.redis.Del(ctx, imageKey)

	return nil
}

// ========== Private Helper Methods ==========

// getMessageKey generates the Redis key for monthly message count
func (s *UsageLimiterService) getMessageKey(ctx context.Context, userID string) (string, error) {
	billingPeriodKey, err := s.getBillingPeriodKey(ctx, userID)
	if err != nil {
		// Fallback to calendar month for free users
		billingPeriodKey = time.Now().UTC().Format("2006-01")
	}
	return fmt.Sprintf("messages:%s:%s", userID, billingPeriodKey), nil
}

// getAnonymousMessageKey generates the Redis key for daily anonymous message count by IP
func (s *UsageLimiterService) getAnonymousMessageKey(clientIP string) string {
	date := time.Now().UTC().Format("2006-01-02")
	return fmt.Sprintf("anon_messages:%s:%s", clientIP, date)
}

// getFileUploadKey generates the Redis key for daily file upload count
func (s *UsageLimiterService) getFileUploadKey(userID string) string {
	date := time.Now().UTC().Format("2006-01-02")
	return fmt.Sprintf("file_uploads:%s:%s", userID, date)
}

// getImageGenKey generates the Redis key for daily image generation count
func (s *UsageLimiterService) getImageGenKey(userID string) string {
	date := time.Now().UTC().Format("2006-01-02")
	return fmt.Sprintf("image_gens:%s:%s", userID, date)
}

// getBillingPeriodKey returns a unique key for the current billing period
func (s *UsageLimiterService) getBillingPeriodKey(ctx context.Context, userID string) (string, error) {
	// Get subscription from MongoDB
	subscription, err := s.getSubscription(ctx, userID)
	if err != nil || subscription == nil {
		// Free tier - use calendar month
		return time.Now().UTC().Format("2006-01"), nil
	}

	// Paid tier - use billing cycle start date
	return subscription.CurrentPeriodStart.Format("2006-01-02"), nil
}

// getSubscription retrieves the user's subscription from MongoDB
func (s *UsageLimiterService) getSubscription(ctx context.Context, userID string) (*models.Subscription, error) {
	if s.mongoDB == nil {
		return nil, fmt.Errorf("MongoDB not available")
	}

	collection := s.mongoDB.Database().Collection("subscriptions")
	var subscription models.Subscription

	err := collection.FindOne(ctx, bson.M{"userId": userID}).Decode(&subscription)
	if err != nil {
		return nil, err
	}

	return &subscription, nil
}

// getMonthlyResetTime returns when the monthly message count will reset
func (s *UsageLimiterService) getMonthlyResetTime(ctx context.Context, userID string) (time.Time, error) {
	subscription, err := s.getSubscription(ctx, userID)
	if err != nil || subscription == nil {
		// Free tier - reset at end of current month (first day of next month at midnight)
		now := time.Now().UTC()
		// Get first day of next month properly (handles year rollover)
		firstDayNextMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)
		return firstDayNextMonth, nil
	}

	// Paid tier - reset at billing cycle end
	return subscription.CurrentPeriodEnd, nil
}

// getNextMidnightUTC returns the next midnight UTC time
func (s *UsageLimiterService) getNextMidnightUTC() time.Time {
	now := time.Now().UTC()
	tomorrow := now.AddDate(0, 0, 1)
	return time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, time.UTC)
}

// getSuggestedUpgradeTier suggests which tier to upgrade to based on current tier
func (s *UsageLimiterService) getSuggestedUpgradeTier(currentTier string) string {
	switch currentTier {
	case models.TierFree:
		return "pro"
	case models.TierPro:
		return "max"
	default:
		return "max"
	}
}
