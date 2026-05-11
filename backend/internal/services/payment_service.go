package services

import (
	"clara-agents/internal/database"
	"clara-agents/internal/models"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dodopayments/dodopayments-go"
	"github.com/dodopayments/dodopayments-go/option"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// WebhookEvent represents a webhook event from DodoPayments
type WebhookEvent struct {
	ID   string                 `json:"id"`
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

// PaymentService handles payment and subscription operations
type PaymentService struct {
	client        *dodopayments.Client
	webhookSecret string
	mongoDB       *database.MongoDB
	userService   *UserService
	tierService   *TierService
	usageLimiter  *UsageLimiterService

	subscriptions *mongo.Collection
	events        *mongo.Collection
}

// NewPaymentService creates a new payment service
func NewPaymentService(
	apiKey, webhookSecret, businessID string,
	mongoDB *database.MongoDB,
	userService *UserService,
	tierService *TierService,
	usageLimiter *UsageLimiterService,
) *PaymentService {
	var client *dodopayments.Client
	if apiKey != "" {
		// Determine environment mode
		env := os.Getenv("DODO_ENVIRONMENT")
		var envOpt option.RequestOption
		if env == "test" {
			envOpt = option.WithEnvironmentTestMode()
		} else {
			envOpt = option.WithEnvironmentLiveMode()
		}

		// Initialize DodoPayments client
		client = dodopayments.NewClient(
			option.WithBearerToken(apiKey),
			envOpt,
		)
		log.Println("✅ DodoPayments client initialized")
	} else {
		log.Println("⚠️  DodoPayments API key not provided, payment features disabled")
	}

	var subscriptions *mongo.Collection
	var events *mongo.Collection
	if mongoDB != nil {
		subscriptions = mongoDB.Database().Collection("subscriptions")
		events = mongoDB.Database().Collection("subscription_events")
	}

	return &PaymentService{
		client:        client,
		webhookSecret: webhookSecret,
		mongoDB:       mongoDB,
		userService:   userService,
		tierService:   tierService,
		usageLimiter:  usageLimiter,
		subscriptions: subscriptions,
		events:        events,
	}
}

// CheckoutResponse represents the response for checkout creation
type CheckoutResponse struct {
	CheckoutURL string `json:"checkout_url"`
	SessionID   string `json:"session_id"`
}

// CreateCheckoutSession creates a checkout session for a subscription
func (s *PaymentService) CreateCheckoutSession(ctx context.Context, userID, userEmail, planID string) (*CheckoutResponse, error) {
	plan := models.GetPlanByID(planID)
	if plan == nil {
		return nil, fmt.Errorf("invalid plan ID: %s", planID)
	}

	if plan.Tier == models.TierFree {
		return nil, fmt.Errorf("cannot create checkout for free plan")
	}

	if plan.ContactSales {
		return nil, fmt.Errorf("enterprise plan requires contact sales")
	}

	if plan.DodoProductID == "" {
		return nil, fmt.Errorf("plan %s does not have a DodoPayments product ID configured", planID)
	}

	// Get or create user in MongoDB (sync from Supabase if new)
	user, err := s.userService.GetUserBySupabaseID(ctx, userID)
	if err != nil {
		// User doesn't exist in MongoDB, sync them from Supabase
		if userEmail == "" {
			return nil, fmt.Errorf("failed to get user and no email provided for sync")
		}
		log.Printf("📝 Syncing new user %s (%s) to MongoDB", userID, userEmail)
		user, err = s.userService.SyncUserFromSupabase(ctx, userID, userEmail)
		if err != nil {
			return nil, fmt.Errorf("failed to sync user: %w", err)
		}
	}

	customerID := user.DodoCustomerID
	if customerID == "" {
		// Create customer in DodoPayments
		if s.client == nil {
			return nil, fmt.Errorf("DodoPayments client not initialized")
		}

		// Generate a customer name from email (DodoPayments requires a name)
		// Use the part before @ as the name, or full email if no @
		customerName := user.Email
		if atIndex := strings.Index(user.Email, "@"); atIndex > 0 {
			customerName = user.Email[:atIndex]
		}

		customer, err := s.client.Customers.New(ctx, dodopayments.CustomerNewParams{
			Email: dodopayments.F(user.Email),
			Name:  dodopayments.F(customerName),
			Metadata: dodopayments.F(map[string]string{
				"supabase_user_id": userID,
			}),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create customer: %w", err)
		}

		customerID = customer.CustomerID
		if err := s.updateUserDodoCustomer(ctx, userID, customerID); err != nil {
			return nil, fmt.Errorf("failed to update customer ID: %w", err)
		}
	}

	if s.client == nil {
		return nil, fmt.Errorf("DodoPayments client not initialized")
	}

	// Create checkout session using the SDK - Link to our existing customer!
	session, err := s.client.CheckoutSessions.New(ctx, dodopayments.CheckoutSessionNewParams{
		CheckoutSessionRequest: dodopayments.CheckoutSessionRequestParam{
			ProductCart: dodopayments.F([]dodopayments.CheckoutSessionRequestProductCartParam{{
				ProductID: dodopayments.F(plan.DodoProductID),
				Quantity:  dodopayments.F(int64(1)),
			}}),
			ReturnURL: dodopayments.F(fmt.Sprintf("%s/settings?tab=billing&checkout=success", getBaseURL())),
			// Attach the checkout to our existing customer
			Customer: dodopayments.F[dodopayments.CustomerRequestUnionParam](dodopayments.AttachExistingCustomerParam{
				CustomerID: dodopayments.F(customerID),
			}),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create checkout session: %w", err)
	}

	return &CheckoutResponse{
		CheckoutURL: session.CheckoutURL,
		SessionID:   session.SessionID,
	}, nil
}

// GetCurrentSubscription gets the user's current subscription
func (s *PaymentService) GetCurrentSubscription(ctx context.Context, userID string) (*models.Subscription, error) {
	// First, check if user has a promo tier in the users collection
	if s.userService != nil {
		user, err := s.userService.GetUserBySupabaseID(ctx, userID)
		if err == nil && user != nil && user.SubscriptionTier != "" {
			// User has a tier set (either from promo or previous subscription)
			sub := &models.Subscription{
				UserID:            userID,
				Tier:              user.SubscriptionTier,
				Status:            user.SubscriptionStatus,
				CancelAtPeriodEnd: false,
			}
			if user.SubscriptionExpiresAt != nil {
				sub.CurrentPeriodEnd = *user.SubscriptionExpiresAt
			}
			return sub, nil
		}
	}

	if s.subscriptions == nil {
		// Return default free tier subscription
		return &models.Subscription{
			UserID: userID,
			Tier:   models.TierFree,
			Status: models.SubStatusActive,
		}, nil
	}

	var sub models.Subscription
	err := s.subscriptions.FindOne(ctx, bson.M{"userId": userID}).Decode(&sub)
	if err == mongo.ErrNoDocuments {
		// No subscription found, return free tier
		return &models.Subscription{
			UserID: userID,
			Tier:   models.TierFree,
			Status: models.SubStatusActive,
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	return &sub, nil
}

// PlanChangeResult represents the result of a plan change
type PlanChangeResult struct {
	Type        string    `json:"type"`      // "upgrade" or "downgrade"
	Immediate   bool      `json:"immediate"` // true for upgrades, false for downgrades
	NewTier     string    `json:"new_tier"`
	EffectiveAt time.Time `json:"effective_at,omitempty"`
}

// ChangePlan handles both upgrades and downgrades
func (s *PaymentService) ChangePlan(ctx context.Context, userID, newPlanID string) (*PlanChangeResult, error) {
	current, err := s.GetCurrentSubscription(ctx, userID)
	if err != nil {
		return nil, err
	}

	newPlan := models.GetPlanByID(newPlanID)
	if newPlan == nil {
		return nil, fmt.Errorf("invalid plan ID: %s", newPlanID)
	}

	currentPlan := models.GetPlanByTier(current.Tier)
	if currentPlan == nil {
		currentPlan = models.GetPlanByTier(models.TierFree)
	}

	// Determine if upgrade or downgrade
	comparison := models.CompareTiers(current.Tier, newPlan.Tier)
	isUpgrade := comparison < 0

	if comparison == 0 {
		return nil, fmt.Errorf("user is already on %s plan", newPlan.Tier)
	}

	if isUpgrade {
		// UPGRADE: Immediate with proration
		if current.DodoSubscriptionID == "" {
			return nil, fmt.Errorf("no active subscription to upgrade")
		}

		if s.client == nil {
			return nil, fmt.Errorf("DodoPayments client not initialized")
		}

		// Change plan using DodoPayments SDK (handles proration automatically)
		err = s.client.Subscriptions.ChangePlan(ctx, current.DodoSubscriptionID, dodopayments.SubscriptionChangePlanParams{
			ProductID:            dodopayments.F(newPlan.DodoProductID),
			ProrationBillingMode: dodopayments.F(dodopayments.SubscriptionChangePlanParamsProrationBillingModeProratedImmediately),
			Quantity:             dodopayments.F(int64(1)),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to change plan: %w", err)
		}

		// Log the change for audit
		log.Printf("✅ Plan changed to %s for subscription %s", newPlan.DodoProductID, current.DodoSubscriptionID)

		// Update subscription in our DB (webhook will confirm, but optimistic update)
		now := time.Now()
		update := bson.M{
			"$set": bson.M{
				"tier":              newPlan.Tier,
				"status":            models.SubStatusActive,
				"scheduledTier":     "",
				"scheduledChangeAt": nil,
				"cancelAtPeriodEnd": false,
				"updatedAt":         now,
			},
		}

		if s.subscriptions != nil {
			_, err = s.subscriptions.UpdateOne(ctx, bson.M{"userId": userID}, update)
			if err != nil {
				log.Printf("⚠️  Failed to update subscription optimistically: %v", err)
			}
		}

		// Invalidate tier cache
		if s.tierService != nil {
			s.tierService.InvalidateCache(userID)
		}

		return &PlanChangeResult{
			Type:      "upgrade",
			Immediate: true,
			NewTier:   newPlan.Tier,
		}, nil
	} else {
		// DOWNGRADE: Schedule for end of period
		if current.DodoSubscriptionID == "" {
			// No active subscription, just update tier
			now := time.Now()
			if s.subscriptions != nil {
				_, err = s.subscriptions.UpdateOne(ctx, bson.M{"userId": userID}, bson.M{
					"$set": bson.M{
						"tier":      newPlan.Tier,
						"status":    models.SubStatusActive,
						"updatedAt": now,
					},
				})
			}
			if s.tierService != nil {
				s.tierService.InvalidateCache(userID)
			}
			return &PlanChangeResult{
				Type:      "downgrade",
				Immediate: true,
				NewTier:   newPlan.Tier,
			}, nil
		}

		// Schedule downgrade for end of period
		periodEnd := current.CurrentPeriodEnd
		if periodEnd.IsZero() {
			periodEnd = time.Now().Add(30 * 24 * time.Hour) // Default to 30 days
		}

		update := bson.M{
			"$set": bson.M{
				"scheduledTier":     newPlan.Tier,
				"scheduledChangeAt": periodEnd,
				"updatedAt":         time.Now(),
			},
		}

		if s.subscriptions != nil {
			_, err = s.subscriptions.UpdateOne(ctx, bson.M{"userId": userID}, update)
			if err != nil {
				return nil, fmt.Errorf("failed to schedule downgrade: %w", err)
			}
		}

		return &PlanChangeResult{
			Type:        "downgrade",
			Immediate:   false,
			NewTier:     newPlan.Tier,
			EffectiveAt: periodEnd,
		}, nil
	}
}

// PreviewPlanChange shows what will happen before confirming
type PlanChangePreview struct {
	ChangeType     string    `json:"change_type"` // "upgrade" or "downgrade"
	Immediate      bool      `json:"immediate"`
	CurrentTier    string    `json:"current_tier"`
	NewTier        string    `json:"new_tier"`
	ProratedAmount int64     `json:"prorated_amount,omitempty"` // cents
	EffectiveAt    time.Time `json:"effective_at,omitempty"`
}

// PreviewPlanChange previews a plan change
func (s *PaymentService) PreviewPlanChange(ctx context.Context, userID, newPlanID string) (*PlanChangePreview, error) {
	current, err := s.GetCurrentSubscription(ctx, userID)
	if err != nil {
		return nil, err
	}

	newPlan := models.GetPlanByID(newPlanID)
	if newPlan == nil {
		return nil, fmt.Errorf("invalid plan ID: %s", newPlanID)
	}

	currentPlan := models.GetPlanByTier(current.Tier)
	if currentPlan == nil {
		currentPlan = models.GetPlanByTier(models.TierFree)
	}

	comparison := models.CompareTiers(current.Tier, newPlan.Tier)
	isUpgrade := comparison < 0

	if comparison == 0 {
		return nil, fmt.Errorf("user is already on %s plan", newPlan.Tier)
	}

	preview := &PlanChangePreview{
		CurrentTier: current.Tier,
		NewTier:     newPlan.Tier,
	}

	if isUpgrade {
		preview.ChangeType = "upgrade"
		preview.Immediate = true
		preview.EffectiveAt = time.Now()

		// Calculate proration if we have period info
		if !current.CurrentPeriodEnd.IsZero() && !current.CurrentPeriodStart.IsZero() {
			daysRemaining := int(time.Until(current.CurrentPeriodEnd).Hours() / 24)
			totalDays := int(current.CurrentPeriodEnd.Sub(current.CurrentPeriodStart).Hours() / 24)
			if daysRemaining > 0 && totalDays > 0 {
				preview.ProratedAmount = s.CalculateProration(
					currentPlan.PriceMonthly,
					newPlan.PriceMonthly,
					daysRemaining,
					totalDays,
				)
			}
		}
	} else {
		preview.ChangeType = "downgrade"
		preview.Immediate = false
		preview.EffectiveAt = current.CurrentPeriodEnd
		if preview.EffectiveAt.IsZero() {
			preview.EffectiveAt = time.Now().Add(30 * 24 * time.Hour)
		}
	}

	return preview, nil
}

// CalculateProration calculates prorated charge for plan change
func (s *PaymentService) CalculateProration(fromPrice, toPrice int64, daysRemaining, totalDays int) int64 {
	if daysRemaining <= 0 || totalDays <= 0 {
		return 0
	}

	// Calculate daily rates
	fromDaily := float64(fromPrice) / float64(totalDays)
	toDaily := float64(toPrice) / float64(totalDays)

	// Calculate difference for remaining days
	difference := (toDaily - fromDaily) * float64(daysRemaining)

	return int64(difference)
}

// CancelSubscription schedules cancellation at period end
func (s *PaymentService) CancelSubscription(ctx context.Context, userID string) error {
	current, err := s.GetCurrentSubscription(ctx, userID)
	if err != nil {
		return err
	}

	if current.Tier == models.TierFree {
		return fmt.Errorf("no active subscription to cancel")
	}

	if current.CancelAtPeriodEnd {
		return fmt.Errorf("subscription is already scheduled for cancellation")
	}

	if current.DodoSubscriptionID != "" && s.client != nil {
		// Cancel in DodoPayments (cancel at next billing date)
		_, err = s.client.Subscriptions.Update(ctx, current.DodoSubscriptionID, dodopayments.SubscriptionUpdateParams{
			CancelAtNextBillingDate: dodopayments.F(true),
		})
		if err != nil {
			return fmt.Errorf("failed to cancel subscription: %w", err)
		}
	}

	// Update in our DB
	update := bson.M{
		"$set": bson.M{
			"cancelAtPeriodEnd": true,
			"status":            models.SubStatusPendingCancel,
			"updatedAt":         time.Now(),
		},
	}

	if s.subscriptions != nil {
		_, err = s.subscriptions.UpdateOne(ctx, bson.M{"userId": userID}, update)
		if err != nil {
			return fmt.Errorf("failed to update subscription: %w", err)
		}
	}

	return nil
}

// ReactivateSubscription undoes cancellation if still in period
func (s *PaymentService) ReactivateSubscription(ctx context.Context, userID string) error {
	current, err := s.GetCurrentSubscription(ctx, userID)
	if err != nil {
		return err
	}

	if !current.CancelAtPeriodEnd {
		return fmt.Errorf("subscription is not scheduled for cancellation")
	}

	if current.DodoSubscriptionID != "" && s.client != nil {
		// Reactivate in DodoPayments (clear cancel_at_next_billing_date)
		_, err = s.client.Subscriptions.Update(ctx, current.DodoSubscriptionID, dodopayments.SubscriptionUpdateParams{
			CancelAtNextBillingDate: dodopayments.F(false),
		})
		if err != nil {
			return fmt.Errorf("failed to reactivate subscription: %w", err)
		}
	}

	// Update in our DB
	update := bson.M{
		"$set": bson.M{
			"cancelAtPeriodEnd": false,
			"status":            models.SubStatusActive,
			"updatedAt":         time.Now(),
		},
	}

	if s.subscriptions != nil {
		_, err = s.subscriptions.UpdateOne(ctx, bson.M{"userId": userID}, update)
		if err != nil {
			return fmt.Errorf("failed to update subscription: %w", err)
		}
	}

	return nil
}

// GetCustomerPortalURL gets the DodoPayments customer portal URL
func (s *PaymentService) GetCustomerPortalURL(ctx context.Context, userID string) (string, error) {
	user, err := s.userService.GetUserBySupabaseID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}

	if user.DodoCustomerID == "" {
		return "", fmt.Errorf("user does not have a DodoPayments customer ID")
	}

	if s.client == nil {
		return "", fmt.Errorf("DodoPayments client not initialized")
	}

	// Create a customer portal session using DodoPayments SDK
	portalSession, err := s.client.Customers.CustomerPortal.New(ctx, user.DodoCustomerID, dodopayments.CustomerCustomerPortalNewParams{})
	if err != nil {
		return "", fmt.Errorf("failed to create customer portal session: %w", err)
	}

	return portalSession.Link, nil
}

// GetAvailablePlans returns all available plans
func (s *PaymentService) GetAvailablePlans() []models.Plan {
	return models.GetAvailablePlans()
}

// DetermineChangeType determines if a plan change is an upgrade or downgrade
func (s *PaymentService) DetermineChangeType(fromTier, toTier string) (isUpgrade, isDowngrade bool) {
	comparison := models.CompareTiers(fromTier, toTier)
	isUpgrade = comparison < 0
	isDowngrade = comparison > 0
	return
}

// VerifyWebhook verifies webhook signature (legacy method for tests)
func (s *PaymentService) VerifyWebhook(payload []byte, signature string) error {
	if s.webhookSecret == "" {
		return fmt.Errorf("webhook secret not configured")
	}

	mac := hmac.New(sha256.New, []byte(s.webhookSecret))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	if signature != expectedSig {
		return fmt.Errorf("invalid webhook signature")
	}

	return nil
}

// VerifyAndParseWebhook verifies and parses webhook using DodoPayments SDK
// DodoPayments uses Standard Webhooks format with headers:
// - webhook-id: unique message ID
// - webhook-signature: v1,<base64_signature>
// - webhook-timestamp: unix timestamp
func (s *PaymentService) VerifyAndParseWebhook(payload []byte, headers http.Header) (*WebhookEvent, error) {
	// If SDK client is available, use it for verification
	if s.client != nil && s.webhookSecret != "" {
		event, err := s.client.Webhooks.Unwrap(payload, headers, option.WithWebhookKey(s.webhookSecret))
		if err != nil {
			return nil, fmt.Errorf("webhook verification failed: %w", err)
		}

		// Successfully verified with SDK - convert and return
		return s.convertSDKEventToWebhookEvent(event)
	}

	// Fallback: Use legacy HMAC verification for tests or when SDK is not available
	signature := headers.Get("Webhook-Signature")
	if signature == "" {
		signature = headers.Get("Dodo-Signature")
	}
	if signature == "" {
		return nil, fmt.Errorf("missing webhook signature header")
	}

	if err := s.VerifyWebhook(payload, signature); err != nil {
		return nil, err
	}

	// Parse the payload
	var event WebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	return &event, nil
}

// convertSDKEventToWebhookEvent converts SDK event to internal WebhookEvent
func (s *PaymentService) convertSDKEventToWebhookEvent(event *dodopayments.UnwrapWebhookEvent) (*WebhookEvent, error) {
	// Convert SDK event to our internal WebhookEvent format
	webhookEvent := &WebhookEvent{
		Type: string(event.Type),
	}

	// Extract event ID and data based on event type
	// The Data field embeds the subscription/payment struct directly
	switch e := event.AsUnion().(type) {
	case dodopayments.SubscriptionActiveWebhookEvent:
		webhookEvent.ID = e.Data.SubscriptionID
		webhookEvent.Data = map[string]interface{}{
			"subscription_id":      e.Data.SubscriptionID,
			"customer_id":          e.Data.Customer.CustomerID,
			"product_id":           e.Data.ProductID,
			"current_period_start": e.Data.PreviousBillingDate.Format(time.RFC3339),
			"current_period_end":   e.Data.NextBillingDate.Format(time.RFC3339),
		}
	case dodopayments.SubscriptionUpdatedWebhookEvent:
		webhookEvent.ID = e.Data.SubscriptionID
		webhookEvent.Data = map[string]interface{}{
			"subscription_id":      e.Data.SubscriptionID,
			"customer_id":          e.Data.Customer.CustomerID,
			"product_id":           e.Data.ProductID,
			"current_period_start": e.Data.PreviousBillingDate.Format(time.RFC3339),
			"current_period_end":   e.Data.NextBillingDate.Format(time.RFC3339),
		}
	case dodopayments.SubscriptionCancelledWebhookEvent:
		webhookEvent.ID = e.Data.SubscriptionID
		webhookEvent.Data = map[string]interface{}{
			"subscription_id": e.Data.SubscriptionID,
			"customer_id":     e.Data.Customer.CustomerID,
		}
	case dodopayments.SubscriptionRenewedWebhookEvent:
		webhookEvent.ID = e.Data.SubscriptionID
		webhookEvent.Data = map[string]interface{}{
			"subscription_id":      e.Data.SubscriptionID,
			"customer_id":          e.Data.Customer.CustomerID,
			"current_period_start": e.Data.PreviousBillingDate.Format(time.RFC3339),
			"current_period_end":   e.Data.NextBillingDate.Format(time.RFC3339),
		}
	case dodopayments.SubscriptionOnHoldWebhookEvent:
		webhookEvent.ID = e.Data.SubscriptionID
		webhookEvent.Data = map[string]interface{}{
			"subscription_id": e.Data.SubscriptionID,
			"customer_id":     e.Data.Customer.CustomerID,
		}
	case dodopayments.PaymentSucceededWebhookEvent:
		webhookEvent.ID = e.Data.PaymentID
		webhookEvent.Data = map[string]interface{}{
			"payment_id":      e.Data.PaymentID,
			"subscription_id": e.Data.SubscriptionID,
		}
	case dodopayments.PaymentFailedWebhookEvent:
		webhookEvent.ID = e.Data.PaymentID
		webhookEvent.Data = map[string]interface{}{
			"payment_id":      e.Data.PaymentID,
			"subscription_id": e.Data.SubscriptionID,
		}
	default:
		// For unknown event types, try to extract basic info
		webhookEvent.ID = fmt.Sprintf("evt_%d", time.Now().UnixNano())
		webhookEvent.Data = make(map[string]interface{})
	}

	return webhookEvent, nil
}

// IsEventProcessed checks if a webhook event has already been processed
func (s *PaymentService) IsEventProcessed(ctx context.Context, eventID string) bool {
	if s.events == nil {
		return false
	}

	count, err := s.events.CountDocuments(ctx, bson.M{"dodoEventId": eventID})
	if err != nil {
		return false
	}

	return count > 0
}

// HandleWebhookEvent processes a verified webhook event
func (s *PaymentService) HandleWebhookEvent(ctx context.Context, event *WebhookEvent) error {
	// Check idempotency
	if s.IsEventProcessed(ctx, event.ID) {
		log.Printf("⚠️  Webhook event %s already processed, skipping", event.ID)
		return fmt.Errorf("webhook event already processed (idempotent)")
	}

	// Log event
	eventDoc := models.SubscriptionEvent{
		ID:          primitive.NewObjectID(),
		DodoEventID: event.ID,
		EventType:   event.Type,
		Metadata:    event.Data,
		CreatedAt:   time.Now(),
	}

	if s.events != nil {
		_, err := s.events.InsertOne(ctx, eventDoc)
		if err != nil {
			log.Printf("⚠️  Failed to log webhook event: %v", err)
		}
	}

	// Handle event based on type
	switch event.Type {
	case "subscription.active":
		return s.handleSubscriptionActive(ctx, event)
	case "subscription.updated":
		return s.handleSubscriptionUpdated(ctx, event)
	case "subscription.on_hold":
		return s.handleSubscriptionOnHold(ctx, event)
	case "subscription.renewed":
		return s.handleSubscriptionRenewed(ctx, event)
	case "subscription.cancelled":
		return s.handleSubscriptionCancelled(ctx, event)
	case "payment.succeeded":
		return s.handlePaymentSucceeded(ctx, event)
	case "payment.failed":
		return s.handlePaymentFailed(ctx, event)
	default:
		log.Printf("⚠️  Unhandled webhook event type: %s", event.Type)
		return nil
	}
}

func (s *PaymentService) handleSubscriptionActive(ctx context.Context, event *WebhookEvent) error {
	subID, _ := event.Data["subscription_id"].(string)
	customerID, _ := event.Data["customer_id"].(string)
	productID, _ := event.Data["product_id"].(string)

	if subID == "" || customerID == "" {
		return fmt.Errorf("missing required fields in subscription.active event")
	}

	// Find plan by product ID
	var plan *models.Plan
	for i := range models.AvailablePlans {
		if models.AvailablePlans[i].DodoProductID == productID {
			plan = &models.AvailablePlans[i]
			break
		}
	}

	if plan == nil {
		return fmt.Errorf("unknown product ID: %s", productID)
	}

	// Find user by customer ID first
	var user models.User
	if s.mongoDB == nil {
		return fmt.Errorf("MongoDB not available")
	}

	err := s.mongoDB.Database().Collection("users").FindOne(ctx, bson.M{"dodoCustomerId": customerID}).Decode(&user)
	if err != nil {
		log.Printf("⚠️  User not found by customer ID %s, trying to fetch from DodoPayments...", customerID)

		// Fallback: Fetch customer from DodoPayments to get email
		if s.client != nil {
			customer, fetchErr := s.client.Customers.Get(ctx, customerID)
			if fetchErr != nil {
				return fmt.Errorf("failed to find user by customer ID and failed to fetch customer: %w", fetchErr)
			}

			// Try to find user by email
			err = s.mongoDB.Database().Collection("users").FindOne(ctx, bson.M{"email": customer.Email}).Decode(&user)
			if err != nil {
				return fmt.Errorf("failed to find user by customer ID or email (%s): %w", customer.Email, err)
			}

			// Update the user's dodoCustomerId with the new customer ID
			log.Printf("✅ Found user by email %s, updating dodoCustomerId to %s", customer.Email, customerID)
			_, updateErr := s.mongoDB.Database().Collection("users").UpdateOne(
				ctx,
				bson.M{"_id": user.ID},
				bson.M{"$set": bson.M{"dodoCustomerId": customerID}},
			)
			if updateErr != nil {
				log.Printf("⚠️  Failed to update dodoCustomerId: %v", updateErr)
			}
		} else {
			return fmt.Errorf("failed to find user by customer ID: %w", err)
		}
	}

	// Parse period dates
	var periodStart, periodEnd time.Time
	if startStr, ok := event.Data["current_period_start"].(string); ok {
		periodStart, _ = time.Parse(time.RFC3339, startStr)
	}
	if endStr, ok := event.Data["current_period_end"].(string); ok {
		periodEnd, _ = time.Parse(time.RFC3339, endStr)
	}

	// Upsert subscription
	now := time.Now()

	if s.subscriptions != nil {
		filter := bson.M{"userId": user.SupabaseUserID}
		update := bson.M{
			"$set": bson.M{
				"userId":             user.SupabaseUserID,
				"dodoSubscriptionId": subID,
				"dodoCustomerId":     customerID,
				"tier":               plan.Tier,
				"status":             models.SubStatusActive,
				"currentPeriodStart": periodStart,
				"currentPeriodEnd":   periodEnd,
				"updatedAt":          now,
			},
			"$setOnInsert": bson.M{
				"createdAt": now,
			},
		}
		opts := options.Update().SetUpsert(true)
		_, err := s.subscriptions.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			return fmt.Errorf("failed to upsert subscription: %w", err)
		}
	}

	// Update user subscription tier
	if s.userService != nil {
		err := s.userService.UpdateSubscriptionWithStatus(ctx, user.SupabaseUserID, plan.Tier, models.SubStatusActive, &periodEnd)
		if err != nil {
			log.Printf("⚠️  Failed to update user subscription: %v", err)
		}
	}

	// Invalidate tier cache
	if s.tierService != nil {
		s.tierService.InvalidateCache(user.SupabaseUserID)
	}

	// Reset usage counters on new subscription activation
	if s.usageLimiter != nil {
		if err := s.usageLimiter.ResetAllCounters(ctx, user.SupabaseUserID); err != nil {
			log.Printf("⚠️  Failed to reset usage counters for user %s: %v", user.SupabaseUserID, err)
		} else {
			log.Printf("✅ [WEBHOOK] Reset usage counters for new subscriber %s", user.SupabaseUserID)
		}
	}

	log.Printf("✅ Subscription activated for user %s: %s", user.SupabaseUserID, plan.Tier)
	return nil
}

func (s *PaymentService) handleSubscriptionUpdated(ctx context.Context, event *WebhookEvent) error {
	subID, _ := event.Data["subscription_id"].(string)
	productID, _ := event.Data["product_id"].(string)
	if subID == "" {
		return fmt.Errorf("missing subscription_id in event")
	}

	// Parse period dates from webhook
	var periodStart, periodEnd time.Time
	if startStr, ok := event.Data["current_period_start"].(string); ok && startStr != "" {
		periodStart, _ = time.Parse(time.RFC3339, startStr)
	}
	if endStr, ok := event.Data["current_period_end"].(string); ok && endStr != "" {
		periodEnd, _ = time.Parse(time.RFC3339, endStr)
	}

	// Get current subscription
	var sub models.Subscription
	if s.subscriptions != nil {
		err := s.subscriptions.FindOne(ctx, bson.M{"dodoSubscriptionId": subID}).Decode(&sub)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				// Subscription doesn't exist yet - this can happen due to webhook race conditions
				// The subscription.active event will create it, so we can safely skip this update
				log.Printf("⚠️  Subscription %s not found yet (race condition), skipping update", subID)
				return nil
			}
			return fmt.Errorf("subscription not found: %w", err)
		}
	} else {
		return fmt.Errorf("MongoDB not available")
	}

	// Find new plan by product ID (for upgrades/downgrades)
	var newPlan *models.Plan
	if productID != "" {
		for i := range models.AvailablePlans {
			if models.AvailablePlans[i].DodoProductID == productID {
				newPlan = &models.AvailablePlans[i]
				break
			}
		}
	}

	// Check if this is a scheduled downgrade being applied
	if sub.HasScheduledChange() && time.Now().After(*sub.ScheduledChangeAt) {
		// Apply scheduled downgrade
		update := bson.M{
			"$set": bson.M{
				"tier":              sub.ScheduledTier,
				"scheduledTier":     "",
				"scheduledChangeAt": nil,
				"updatedAt":         time.Now(),
			},
		}
		if !periodEnd.IsZero() {
			update["$set"].(bson.M)["currentPeriodEnd"] = periodEnd
		}
		if !periodStart.IsZero() {
			update["$set"].(bson.M)["currentPeriodStart"] = periodStart
		}

		if s.subscriptions != nil {
			_, err := s.subscriptions.UpdateOne(ctx, bson.M{"_id": sub.ID}, update)
			if err != nil {
				return fmt.Errorf("failed to apply scheduled downgrade: %w", err)
			}
		}

		// Update user tier
		if s.userService != nil {
			err := s.userService.UpdateSubscriptionWithStatus(ctx, sub.UserID, sub.ScheduledTier, models.SubStatusActive, &periodEnd)
			if err != nil {
				log.Printf("⚠️  Failed to update user tier: %v", err)
			}
		}

		if s.tierService != nil {
			s.tierService.InvalidateCache(sub.UserID)
		}

		log.Printf("✅ Scheduled downgrade applied for subscription %s: %s -> %s", subID, sub.Tier, sub.ScheduledTier)
		return nil
	}

	// Handle tier change from plan upgrade/downgrade
	if newPlan != nil && newPlan.Tier != sub.Tier {
		updateFields := bson.M{
			"tier":      newPlan.Tier,
			"updatedAt": time.Now(),
		}
		if !periodEnd.IsZero() {
			updateFields["currentPeriodEnd"] = periodEnd
		}
		if !periodStart.IsZero() {
			updateFields["currentPeriodStart"] = periodStart
		}

		update := bson.M{"$set": updateFields}

		if s.subscriptions != nil {
			_, err := s.subscriptions.UpdateOne(ctx, bson.M{"_id": sub.ID}, update)
			if err != nil {
				return fmt.Errorf("failed to update subscription tier: %w", err)
			}
		}

		// Update user tier
		if s.userService != nil {
			err := s.userService.UpdateSubscriptionWithStatus(ctx, sub.UserID, newPlan.Tier, models.SubStatusActive, &periodEnd)
			if err != nil {
				log.Printf("⚠️  Failed to update user tier: %v", err)
			}
		}

		// Invalidate tier cache
		if s.tierService != nil {
			s.tierService.InvalidateCache(sub.UserID)
		}

		// If upgrade, reset counters to give immediate access to new limits
		if isUpgrade(sub.Tier, newPlan.Tier) {
			if s.usageLimiter != nil {
				if err := s.usageLimiter.ResetAllCounters(ctx, sub.UserID); err != nil {
					log.Printf("⚠️  Failed to reset usage counters on upgrade: %v", err)
				} else {
					log.Printf("✅ [WEBHOOK] Reset usage counters for upgraded user %s (%s -> %s)", sub.UserID, sub.Tier, newPlan.Tier)
				}
			}
		}

		log.Printf("✅ Subscription updated for %s: %s -> %s", subID, sub.Tier, newPlan.Tier)
		return nil
	}

	// Just update period dates if no tier change
	if !periodEnd.IsZero() || !periodStart.IsZero() {
		updateFields := bson.M{"updatedAt": time.Now()}
		if !periodEnd.IsZero() {
			updateFields["currentPeriodEnd"] = periodEnd
		}
		if !periodStart.IsZero() {
			updateFields["currentPeriodStart"] = periodStart
		}

		if s.subscriptions != nil {
			_, err := s.subscriptions.UpdateOne(ctx, bson.M{"_id": sub.ID}, bson.M{"$set": updateFields})
			if err != nil {
				log.Printf("⚠️  Failed to update subscription period dates: %v", err)
			}
		}
	}

	log.Printf("✅ Subscription updated event processed for %s", subID)
	return nil
}

func (s *PaymentService) handleSubscriptionOnHold(ctx context.Context, event *WebhookEvent) error {
	subID, _ := event.Data["subscription_id"].(string)
	if subID == "" {
		return fmt.Errorf("missing subscription_id in event")
	}

	update := bson.M{
		"$set": bson.M{
			"status":    models.SubStatusOnHold,
			"updatedAt": time.Now(),
		},
	}

	if s.subscriptions != nil {
		_, err := s.subscriptions.UpdateOne(ctx, bson.M{"dodoSubscriptionId": subID}, update)
		if err != nil {
			return fmt.Errorf("failed to update subscription status: %w", err)
		}
	}

	log.Printf("⚠️  Subscription %s put on hold", subID)
	return nil
}

func (s *PaymentService) handleSubscriptionRenewed(ctx context.Context, event *WebhookEvent) error {
	subID, _ := event.Data["subscription_id"].(string)
	if subID == "" {
		return fmt.Errorf("missing subscription_id in event")
	}

	// Parse period dates
	var periodStart, periodEnd time.Time
	if startStr, ok := event.Data["current_period_start"].(string); ok {
		periodStart, _ = time.Parse(time.RFC3339, startStr)
	}
	if endStr, ok := event.Data["current_period_end"].(string); ok {
		periodEnd, _ = time.Parse(time.RFC3339, endStr)
	}

	// Get subscription
	var sub models.Subscription
	if s.subscriptions != nil {
		err := s.subscriptions.FindOne(ctx, bson.M{"dodoSubscriptionId": subID}).Decode(&sub)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				// Subscription doesn't exist yet - this can happen due to webhook race conditions
				// The subscription.active event will create it, so we can safely skip this renewal
				log.Printf("⚠️  Subscription %s not found yet (race condition), skipping renewal", subID)
				return nil
			}
			return fmt.Errorf("subscription not found: %w", err)
		}

		// Check if cancellation was scheduled
		if sub.CancelAtPeriodEnd {
			// Revert to free tier
			update := bson.M{
				"$set": bson.M{
					"tier":              models.TierFree,
					"status":            models.SubStatusCancelled,
					"cancelAtPeriodEnd": false,
					"cancelledAt":       time.Now(),
					"updatedAt":         time.Now(),
				},
			}
			_, err = s.subscriptions.UpdateOne(ctx, bson.M{"_id": sub.ID}, update)
			if err != nil {
				return fmt.Errorf("failed to cancel subscription: %w", err)
			}

			// Update user tier
			if s.userService != nil {
				err = s.userService.UpdateSubscriptionWithStatus(ctx, sub.UserID, models.TierFree, models.SubStatusCancelled, nil)
				if err != nil {
					log.Printf("⚠️  Failed to update user tier: %v", err)
				}
			}

			if s.tierService != nil {
				s.tierService.InvalidateCache(sub.UserID)
			}

			log.Printf("✅ Subscription %s cancelled and reverted to free", subID)
			return nil
		}

		// Check if a downgrade was scheduled (should be applied on renewal)
		if sub.HasScheduledChange() {
			// Apply scheduled downgrade
			update := bson.M{
				"$set": bson.M{
					"tier":                sub.ScheduledTier,
					"scheduledTier":       "",
					"scheduledChangeAt":   nil,
					"currentPeriodStart":  periodStart,
					"currentPeriodEnd":    periodEnd,
					"status":              models.SubStatusActive,
					"updatedAt":           time.Now(),
				},
			}
			_, err = s.subscriptions.UpdateOne(ctx, bson.M{"_id": sub.ID}, update)
			if err != nil {
				return fmt.Errorf("failed to apply scheduled downgrade: %w", err)
			}

			// Update user tier
			if s.userService != nil {
				err = s.userService.UpdateSubscriptionWithStatus(ctx, sub.UserID, sub.ScheduledTier, models.SubStatusActive, &periodEnd)
				if err != nil {
					log.Printf("⚠️  Failed to update user tier: %v", err)
				}
			}

			if s.tierService != nil {
				s.tierService.InvalidateCache(sub.UserID)
			}

			log.Printf("✅ Subscription %s renewed and scheduled downgrade to %s applied", subID, sub.ScheduledTier)
			return nil
		}

		// Normal renewal - just update period dates
		update := bson.M{
			"$set": bson.M{
				"currentPeriodStart": periodStart,
				"currentPeriodEnd":   periodEnd,
				"status":             models.SubStatusActive,
				"updatedAt":          time.Now(),
			},
		}
		_, err = s.subscriptions.UpdateOne(ctx, bson.M{"_id": sub.ID}, update)
		if err != nil {
			return fmt.Errorf("failed to update subscription: %w", err)
		}
	}

	log.Printf("✅ Subscription %s renewed", subID)
	return nil
}

func (s *PaymentService) handleSubscriptionCancelled(ctx context.Context, event *WebhookEvent) error {
	subID, _ := event.Data["subscription_id"].(string)
	if subID == "" {
		return fmt.Errorf("missing subscription_id in event")
	}

	// Get subscription
	var sub models.Subscription
	if s.subscriptions != nil {
		err := s.subscriptions.FindOne(ctx, bson.M{"dodoSubscriptionId": subID}).Decode(&sub)
		if err != nil {
			return fmt.Errorf("subscription not found: %w", err)
		}

		// Revert to free tier
		update := bson.M{
			"$set": bson.M{
				"tier":        models.TierFree,
				"status":      models.SubStatusCancelled,
				"cancelledAt": time.Now(),
				"updatedAt":   time.Now(),
			},
		}
		_, err = s.subscriptions.UpdateOne(ctx, bson.M{"_id": sub.ID}, update)
		if err != nil {
			return fmt.Errorf("failed to cancel subscription: %w", err)
		}

		// Update user tier
		if s.userService != nil {
			err = s.userService.UpdateSubscription(ctx, sub.UserID, models.TierFree, nil)
			if err != nil {
				log.Printf("⚠️  Failed to update user tier: %v", err)
			}
		}

		if s.tierService != nil {
			s.tierService.InvalidateCache(sub.UserID)
		}
	}

	log.Printf("✅ Subscription %s cancelled", subID)
	return nil
}

func (s *PaymentService) handlePaymentSucceeded(ctx context.Context, event *WebhookEvent) error {
	// Payment succeeded - subscription should already be active
	// Just log for audit
	log.Printf("✅ Payment succeeded for subscription")
	return nil
}

func (s *PaymentService) handlePaymentFailed(ctx context.Context, event *WebhookEvent) error {
	subID, _ := event.Data["subscription_id"].(string)
	if subID == "" {
		return fmt.Errorf("missing subscription_id in event")
	}

	// Update subscription status to on_hold
	update := bson.M{
		"$set": bson.M{
			"status":    models.SubStatusOnHold,
			"updatedAt": time.Now(),
		},
	}

	if s.subscriptions != nil {
		_, err := s.subscriptions.UpdateOne(ctx, bson.M{"dodoSubscriptionId": subID}, update)
		if err != nil {
			return fmt.Errorf("failed to update subscription status: %w", err)
		}
	}

	log.Printf("⚠️  Payment failed for subscription %s", subID)
	return nil
}

// SyncSubscriptionFromDodo syncs subscription data from DodoPayments for a user
func (s *PaymentService) SyncSubscriptionFromDodo(ctx context.Context, userID string) (map[string]interface{}, error) {
	if s.client == nil {
		return nil, fmt.Errorf("DodoPayments client not initialized")
	}

	// Get user
	user, err := s.userService.GetUserBySupabaseID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// If user has no dodoCustomerId, try to find by email
	customerID := user.DodoCustomerID
	if customerID == "" {
		// Search for customer by email using the customers list API
		log.Printf("⚠️  User %s has no dodoCustomerId, trying to find customer by email...", userID)

		// For now, we'll need the user to do a new checkout to create the customer link
		return map[string]interface{}{
			"status":  "no_customer",
			"message": "No DodoPayments customer linked. Please initiate a new checkout to link your account.",
		}, nil
	}

	// Get subscriptions from DodoPayments for this customer
	subscriptionsPage, err := s.client.Subscriptions.List(ctx, dodopayments.SubscriptionListParams{
		CustomerID: dodopayments.F(customerID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list subscriptions from DodoPayments: %w", err)
	}

	// Find active subscription (SubscriptionListResponse type)
	var activeSub *dodopayments.SubscriptionListResponse
	for i := range subscriptionsPage.Items {
		sub := &subscriptionsPage.Items[i]
		if sub.Status == dodopayments.SubscriptionStatusActive {
			activeSub = sub
			break
		}
	}

	if activeSub == nil {
		// No active subscription found
		return map[string]interface{}{
			"status":  "no_subscription",
			"message": "No active subscription found in DodoPayments",
			"tier":    models.TierFree,
		}, nil
	}

	// Find plan by product ID
	var plan *models.Plan
	if activeSub.ProductID != "" {
		for i := range models.AvailablePlans {
			if models.AvailablePlans[i].DodoProductID == activeSub.ProductID {
				plan = &models.AvailablePlans[i]
				break
			}
		}
	}

	tier := models.TierFree
	if plan != nil {
		tier = plan.Tier
	}

	// DodoPayments uses NextBillingDate (end of period) and PreviousBillingDate (start of period)
	periodStart := activeSub.PreviousBillingDate
	periodEnd := activeSub.NextBillingDate

	// Update local subscription
	now := time.Now()
	if s.subscriptions != nil {
		filter := bson.M{"userId": userID}
		update := bson.M{
			"$set": bson.M{
				"userId":             userID,
				"dodoSubscriptionId": activeSub.SubscriptionID,
				"dodoCustomerId":     customerID,
				"tier":               tier,
				"status":             models.SubStatusActive,
				"currentPeriodStart": periodStart,
				"currentPeriodEnd":   periodEnd,
				"updatedAt":          now,
			},
			"$setOnInsert": bson.M{
				"createdAt": now,
			},
		}
		opts := options.Update().SetUpsert(true)
		_, err = s.subscriptions.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to update local subscription: %w", err)
		}
	}

	// Update user tier
	if s.userService != nil {
		err = s.userService.UpdateSubscriptionWithStatus(ctx, userID, tier, models.SubStatusActive, &periodEnd)
		if err != nil {
			log.Printf("⚠️  Failed to update user subscription: %v", err)
		}
	}

	// Invalidate cache
	if s.tierService != nil {
		s.tierService.InvalidateCache(userID)
	}

	log.Printf("✅ Synced subscription for user %s: tier=%s, sub_id=%s", userID, tier, activeSub.SubscriptionID)

	return map[string]interface{}{
		"status":          "synced",
		"tier":            tier,
		"subscription_id": activeSub.SubscriptionID,
		"period_end":      periodEnd,
	}, nil
}

// Helper functions

// isUpgrade determines if a tier change is an upgrade
func isUpgrade(oldTier, newTier string) bool {
	oldRank := models.TierOrder[oldTier]
	newRank := models.TierOrder[newTier]
	return newRank > oldRank
}

func (s *PaymentService) updateUserDodoCustomer(ctx context.Context, userID, customerID string) error {
	if s.mongoDB == nil {
		return fmt.Errorf("MongoDB not available")
	}

	_, err := s.mongoDB.Database().Collection("users").UpdateOne(
		ctx,
		bson.M{"supabaseUserId": userID},
		bson.M{"$set": bson.M{"dodoCustomerId": customerID}},
	)
	return err
}

func getBaseURL() string {
	if url := os.Getenv("FRONTEND_URL"); url != "" {
		return strings.TrimSuffix(url, "/")
	}
	return "http://localhost:5173"
}

// UsageStats represents the current usage statistics for a user
type UsageStats struct {
	Schedules          UsageStat         `json:"schedules"`
	APIKeys            UsageStat         `json:"api_keys"`
	ExecutionsToday    UsageStat         `json:"executions_today"`
	RequestsPerMin     UsageStat         `json:"requests_per_min"`
	Messages           UsageStatWithTime `json:"messages"`
	FileUploads        UsageStatWithTime `json:"file_uploads"`
	ImageGenerations   UsageStatWithTime `json:"image_generations"`
	MemoryExtractions  UsageStatWithTime `json:"memory_extractions"` // Daily memory extraction count
}

// UsageStat represents a single usage statistic
type UsageStat struct {
	Current int64 `json:"current"`
	Max     int64 `json:"max"`
}

// UsageStatWithTime represents a usage statistic with reset time
type UsageStatWithTime struct {
	Current int64     `json:"current"`
	Max     int64     `json:"max"`
	ResetAt time.Time `json:"reset_at"`
}

// GetUsageStats returns the current usage statistics for a user
func (s *PaymentService) GetUsageStats(ctx context.Context, userID string) (*UsageStats, error) {
	if s.mongoDB == nil || s.tierService == nil {
		return &UsageStats{}, nil
	}

	// Get user's tier limits
	limits := s.tierService.GetLimits(ctx, userID)

	// Count schedules
	scheduleCount, err := s.mongoDB.Database().Collection("schedules").CountDocuments(ctx, bson.M{"userId": userID})
	if err != nil {
		log.Printf("⚠️  Failed to count schedules for user %s: %v", userID, err)
		scheduleCount = 0
	}

	// Count API keys
	apiKeyCount, err := s.mongoDB.Database().Collection("api_keys").CountDocuments(ctx, bson.M{"userId": userID})
	if err != nil {
		log.Printf("⚠️  Failed to count API keys for user %s: %v", userID, err)
		apiKeyCount = 0
	}

	// Count executions today (from Redis if available, otherwise from MongoDB)
	executionsToday := int64(0)
	today := time.Now().UTC().Format("2006-01-02")
	startOfDay, _ := time.Parse("2006-01-02", today)

	execCount, err := s.mongoDB.Database().Collection("executions").CountDocuments(ctx, bson.M{
		"userId": userID,
		"createdAt": bson.M{
			"$gte": startOfDay,
		},
	})
	if err != nil {
		log.Printf("⚠️  Failed to count executions for user %s: %v", userID, err)
	} else {
		executionsToday = execCount
	}

	// Get usage counts and reset times from UsageLimiterService
	var msgCount, fileCount, imageCount int64
	var msgResetAt, fileResetAt, imageResetAt time.Time

	if s.usageLimiter != nil {
		limiterStats, err := s.usageLimiter.GetUsageStats(ctx, userID)
		if err == nil {
			msgCount = limiterStats.MessagesUsed
			fileCount = limiterStats.FileUploadsUsed
			imageCount = limiterStats.ImageGensUsed
			msgResetAt = limiterStats.MessageResetAt
			fileResetAt = limiterStats.FileUploadResetAt
			imageResetAt = limiterStats.ImageGenResetAt
		} else {
			log.Printf("⚠️  Failed to get usage limiter stats for user %s: %v", userID, err)
			// Set default reset times
			msgResetAt = time.Now().UTC().AddDate(0, 1, 0)
			fileResetAt = time.Now().UTC().AddDate(0, 0, 1)
			imageResetAt = time.Now().UTC().AddDate(0, 0, 1)
		}
	} else {
		// No usageLimiter available, use default reset times
		msgResetAt = time.Now().UTC().AddDate(0, 1, 0)
		fileResetAt = time.Now().UTC().AddDate(0, 0, 1)
		imageResetAt = time.Now().UTC().AddDate(0, 0, 1)
	}

	// Count memory extractions today (completed jobs)
	memoryExtractionCount := int64(0)
	memoryExtractCount, err := s.mongoDB.Database().Collection("memory_extraction_jobs").CountDocuments(ctx, bson.M{
		"userId": userID,
		"status": "completed",
		"processedAt": bson.M{
			"$gte": startOfDay,
		},
	})
	if err != nil {
		log.Printf("⚠️  Failed to count memory extractions for user %s: %v", userID, err)
	} else {
		memoryExtractionCount = memoryExtractCount
	}

	// Calculate next reset time for memory extractions (midnight UTC)
	now := time.Now().UTC()
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)

	return &UsageStats{
		Schedules: UsageStat{
			Current: scheduleCount,
			Max:     int64(limits.MaxSchedules),
		},
		APIKeys: UsageStat{
			Current: apiKeyCount,
			Max:     int64(limits.MaxAPIKeys),
		},
		ExecutionsToday: UsageStat{
			Current: executionsToday,
			Max:     limits.MaxExecutionsPerDay,
		},
		RequestsPerMin: UsageStat{
			Current: 0, // This would need real-time rate limiting data
			Max:     limits.RequestsPerMinute,
		},
		Messages: UsageStatWithTime{
			Current: msgCount,
			Max:     limits.MaxMessagesPerMonth,
			ResetAt: msgResetAt,
		},
		FileUploads: UsageStatWithTime{
			Current: fileCount,
			Max:     limits.MaxFileUploadsPerDay,
			ResetAt: fileResetAt,
		},
		ImageGenerations: UsageStatWithTime{
			Current: imageCount,
			Max:     limits.MaxImageGensPerDay,
			ResetAt: imageResetAt,
		},
		MemoryExtractions: UsageStatWithTime{
			Current: memoryExtractionCount,
			Max:     limits.MaxMemoryExtractionsPerDay,
			ResetAt: nextMidnight,
		},
	}, nil
}
