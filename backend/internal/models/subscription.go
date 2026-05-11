package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Subscription status constants
const (
	SubStatusActive        = "active"
	SubStatusOnHold        = "on_hold"        // Payment failed, grace period
	SubStatusPendingCancel = "pending_cancel" // Will cancel at period end
	SubStatusCancelled     = "cancelled"
	SubStatusPaused        = "paused"
)

// Subscription tiers
const (
	TierFree            = "free"
	TierPro             = "pro"
	TierMax             = "max"
	TierEnterprise      = "enterprise"
	TierLegacyUnlimited = "legacy_unlimited" // For grandfathered users
)

// Plan represents a subscription plan with pricing
type Plan struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Tier          string     `json:"tier"`
	PriceMonthly  int64      `json:"price_monthly"` // cents
	DodoProductID string     `json:"dodo_product_id"`
	Features      []string   `json:"features"`
	Limits        TierLimits `json:"limits"`
	ContactSales  bool       `json:"contact_sales"` // true for enterprise
}

// Subscription tracks a user's subscription state
type Subscription struct {
	ID                 primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID             string             `bson:"userId" json:"user_id"`
	DodoSubscriptionID string             `bson:"dodoSubscriptionId,omitempty" json:"dodo_subscription_id,omitempty"`
	DodoCustomerID     string             `bson:"dodoCustomerId,omitempty" json:"dodo_customer_id,omitempty"`

	// Current state
	Tier   string `bson:"tier" json:"tier"`
	Status string `bson:"status" json:"status"`

	// Billing info
	CurrentPeriodStart time.Time `bson:"currentPeriodStart,omitempty" json:"current_period_start,omitempty"`
	CurrentPeriodEnd   time.Time `bson:"currentPeriodEnd,omitempty" json:"current_period_end,omitempty"`

	// Scheduled changes (for downgrades/cancellations)
	ScheduledTier     string     `bson:"scheduledTier,omitempty" json:"scheduled_tier,omitempty"`
	ScheduledChangeAt *time.Time `bson:"scheduledChangeAt,omitempty" json:"scheduled_change_at,omitempty"`
	CancelAtPeriodEnd bool       `bson:"cancelAtPeriodEnd" json:"cancel_at_period_end"`

	// Timestamps
	CreatedAt   time.Time  `bson:"createdAt" json:"created_at"`
	UpdatedAt   time.Time  `bson:"updatedAt" json:"updated_at"`
	CancelledAt *time.Time `bson:"cancelledAt,omitempty" json:"cancelled_at,omitempty"`
}

// IsActive returns true if subscription is currently active (user has access)
func (s *Subscription) IsActive() bool {
	switch s.Status {
	case SubStatusActive, SubStatusOnHold, SubStatusPendingCancel:
		return true
	default:
		return false
	}
}

// IsExpired returns true if subscription period has ended
func (s *Subscription) IsExpired() bool {
	return !s.CurrentPeriodEnd.IsZero() && s.CurrentPeriodEnd.Before(time.Now())
}

// HasScheduledChange returns true if there's a scheduled tier change
func (s *Subscription) HasScheduledChange() bool {
	return s.ScheduledTier != "" && s.ScheduledChangeAt != nil
}

// SubscriptionEvent for audit logging
type SubscriptionEvent struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID         string             `bson:"userId" json:"user_id"`
	SubscriptionID string             `bson:"subscriptionId" json:"subscription_id"`
	EventType      string             `bson:"eventType" json:"event_type"`
	FromTier       string             `bson:"fromTier,omitempty" json:"from_tier,omitempty"`
	ToTier         string             `bson:"toTier,omitempty" json:"to_tier,omitempty"`
	DodoEventID    string             `bson:"dodoEventId,omitempty" json:"dodo_event_id,omitempty"`
	Metadata       map[string]any     `bson:"metadata,omitempty" json:"metadata,omitempty"`
	CreatedAt      time.Time          `bson:"createdAt" json:"created_at"`
}

// TierOrder defines the order of tiers for comparison
var TierOrder = map[string]int{
	TierFree:            0,
	TierPro:             1,
	TierMax:             2,
	TierEnterprise:      3,
	TierLegacyUnlimited: 4, // Highest tier
}

// CompareTiers compares two tiers and returns:
// -1 if fromTier < toTier (upgrade)
// 0 if fromTier == toTier (same)
// 1 if fromTier > toTier (downgrade)
func CompareTiers(fromTier, toTier string) int {
	fromOrder, fromOk := TierOrder[fromTier]
	toOrder, toOk := TierOrder[toTier]

	if !fromOk || !toOk {
		// Unknown tier, treat as same
		return 0
	}

	if fromOrder < toOrder {
		return -1
	} else if fromOrder > toOrder {
		return 1
	}
	return 0
}

// AvailablePlans returns all available subscription plans
var AvailablePlans = []Plan{
	{
		ID:            "free",
		Name:          "Free",
		Tier:          TierFree,
		PriceMonthly:  0,
		DodoProductID: "",
		Features:      []string{"Basic features", "Limited usage"},
		Limits:        GetTierLimits(TierFree),
		ContactSales:  false,
	},
	{
		ID:            "pro",
		Name:          "Pro",
		Tier:          TierPro,
		PriceMonthly:  1499,                        // $14.99 in cents - configure DodoProductID when ready
		DodoProductID: "pdt_0NVGlqj3fgVkEeAmygtuj", // Set when creating product in DodoPayments
		Features:      []string{"Advanced features", "Higher limits", "Priority support"},
		Limits:        GetTierLimits(TierPro),
		ContactSales:  false,
	},
	{
		ID:            "max",
		Name:          "Max",
		Tier:          TierMax,
		PriceMonthly:  3999,                        // $39.99 in cents - configure DodoProductID when ready
		DodoProductID: "pdt_0NVGm0KQk5F4a8NVoaQst", // Set when creating product in DodoPayments
		Features:      []string{"All Pro features", "Maximum limits", "Premium support"},
		Limits:        GetTierLimits(TierMax),
		ContactSales:  false,
	},
	{
		ID:            "enterprise",
		Name:          "Enterprise",
		Tier:          TierEnterprise,
		PriceMonthly:  0,
		DodoProductID: "",
		Features:      []string{"Custom features", "Unlimited usage", "Dedicated support"},
		Limits:        GetTierLimits(TierEnterprise),
		ContactSales:  true,
	},
}

// GetPlanByID returns a plan by its ID
func GetPlanByID(planID string) *Plan {
	for i := range AvailablePlans {
		if AvailablePlans[i].ID == planID {
			return &AvailablePlans[i]
		}
	}
	return nil
}

// GetPlanByTier returns a plan by its tier
func GetPlanByTier(tier string) *Plan {
	for i := range AvailablePlans {
		if AvailablePlans[i].Tier == tier {
			return &AvailablePlans[i]
		}
	}
	return nil
}

// GetAvailablePlans returns all available plans
func GetAvailablePlans() []Plan {
	return AvailablePlans
}
