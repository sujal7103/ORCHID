package services

import (
	"clara-agents/internal/models"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestNewPaymentService(t *testing.T) {
	service := NewPaymentService("test_key", "webhook_secret", "business_id", nil, nil, nil, nil)
	if service == nil {
		t.Fatal("Expected non-nil payment service")
	}
}

func TestPaymentService_DetermineChangeType(t *testing.T) {
	service := NewPaymentService("", "", "", nil, nil, nil, nil)

	tests := []struct {
		name        string
		fromTier    string
		toTier      string
		isUpgrade   bool
		isDowngrade bool
	}{
		{"free to pro", models.TierFree, models.TierPro, true, false},
		{"pro to max", models.TierPro, models.TierMax, true, false},
		{"max to pro", models.TierMax, models.TierPro, false, true},
		{"pro to free", models.TierPro, models.TierFree, false, true},
		{"same tier", models.TierPro, models.TierPro, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isUpgrade, isDowngrade := service.DetermineChangeType(tt.fromTier, tt.toTier)
			if isUpgrade != tt.isUpgrade {
				t.Errorf("isUpgrade = %v, want %v", isUpgrade, tt.isUpgrade)
			}
			if isDowngrade != tt.isDowngrade {
				t.Errorf("isDowngrade = %v, want %v", isDowngrade, tt.isDowngrade)
			}
		})
	}
}

func TestPaymentService_VerifyWebhookSignature(t *testing.T) {
	secret := "test_webhook_secret"
	service := NewPaymentService("", secret, "", nil, nil, nil, nil)

	payload := []byte(`{"type":"subscription.active","data":{}}`)

	// Generate valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	validSig := hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name      string
		signature string
		expectErr bool
	}{
		{"valid signature", validSig, false},
		{"invalid signature", "invalid_sig", true},
		{"empty signature", "", true},
		{"wrong signature", "abcd1234", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.VerifyWebhook(payload, tt.signature)
			if tt.expectErr && err == nil {
				t.Error("Expected error but got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

func TestPaymentService_VerifyWebhook_NoSecret(t *testing.T) {
	service := NewPaymentService("", "", "", nil, nil, nil, nil)
	payload := []byte(`{"type":"subscription.active"}`)

	err := service.VerifyWebhook(payload, "signature")
	if err == nil {
		t.Error("Expected error when webhook secret is not configured")
	}
}

func TestPaymentService_CalculateProration(t *testing.T) {
	service := NewPaymentService("", "", "", nil, nil, nil, nil)

	tests := []struct {
		name           string
		fromPrice      int64 // cents
		toPrice        int64 // cents
		daysRemaining  int
		totalDays      int
		expectedCharge int64
	}{
		{
			name:           "upgrade mid-month",
			fromPrice:      1000, // $10/month
			toPrice:        2000, // $20/month
			daysRemaining:  15,
			totalDays:      30,
			expectedCharge: 500, // $5 (half month difference)
		},
		{
			name:           "upgrade near end",
			fromPrice:      1000,
			toPrice:        2000,
			daysRemaining:  3,
			totalDays:      30,
			expectedCharge: 100, // ~$1
		},
		{
			name:           "downgrade mid-month",
			fromPrice:      2000,
			toPrice:        1000,
			daysRemaining:  15,
			totalDays:      30,
			expectedCharge: -500, // Credit
		},
		{
			name:           "zero days remaining",
			fromPrice:      1000,
			toPrice:        2000,
			daysRemaining:  0,
			totalDays:      30,
			expectedCharge: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			charge := service.CalculateProration(
				tt.fromPrice, tt.toPrice,
				tt.daysRemaining, tt.totalDays,
			)
			// Allow 10% variance for rounding
			variance := float64(tt.expectedCharge) * 0.1
			if tt.expectedCharge < 0 {
				variance = -variance
			}
			if float64(charge) < float64(tt.expectedCharge)-variance ||
				float64(charge) > float64(tt.expectedCharge)+variance {
				t.Errorf("Proration = %d, want ~%d", charge, tt.expectedCharge)
			}
		})
	}
}

func TestPaymentService_GetAvailablePlans(t *testing.T) {
	service := NewPaymentService("", "", "", nil, nil, nil, nil)

	plans := service.GetAvailablePlans()

	// Should have free, pro, max, enterprise
	if len(plans) < 4 {
		t.Errorf("Expected at least 4 plans, got %d", len(plans))
	}

	// Verify enterprise has contact_sales flag
	var enterprisePlan *models.Plan
	for i := range plans {
		if plans[i].Tier == models.TierEnterprise {
			enterprisePlan = &plans[i]
			break
		}
	}

	if enterprisePlan == nil {
		t.Fatal("Enterprise plan not found")
	}

	if !enterprisePlan.ContactSales {
		t.Error("Enterprise plan should have ContactSales=true")
	}
	if enterprisePlan.PriceMonthly != 0 {
		t.Error("Enterprise plan should have 0 price (contact sales)")
	}

	// Verify pricing order: free < pro < max
	var freePlan, proPlan, maxPlan *models.Plan
	for i := range plans {
		switch plans[i].Tier {
		case models.TierFree:
			freePlan = &plans[i]
		case models.TierPro:
			proPlan = &plans[i]
		case models.TierMax:
			maxPlan = &plans[i]
		}
	}

	if freePlan == nil || proPlan == nil || maxPlan == nil {
		t.Fatal("Missing required plans")
	}

	if freePlan.PriceMonthly != 0 {
		t.Error("Free plan should be $0")
	}
	if proPlan.PriceMonthly >= maxPlan.PriceMonthly {
		t.Error("Pro+ should be more expensive than Pro")
	}
}

func TestPaymentService_GetCurrentSubscription_NoMongoDB(t *testing.T) {
	service := NewPaymentService("", "", "", nil, nil, nil, nil)
	ctx := context.Background()

	sub, err := service.GetCurrentSubscription(ctx, "user-123")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if sub == nil {
		t.Fatal("Expected subscription, got nil")
	}

	if sub.Tier != models.TierFree {
		t.Errorf("Expected free tier, got %s", sub.Tier)
	}

	if sub.Status != models.SubStatusActive {
		t.Errorf("Expected active status, got %s", sub.Status)
	}
}

func TestPaymentService_PreviewPlanChange(t *testing.T) {
	service := NewPaymentService("", "", "", nil, nil, nil, nil)
	ctx := context.Background()

	tests := []struct {
		name        string
		currentTier string
		newPlanID   string
		expectError bool
	}{
		{"free to pro", models.TierFree, "pro", false},
		{"pro to max", models.TierPro, "max", false},
		{"max to pro", models.TierMax, "pro", false},
		{"invalid plan", models.TierFree, "invalid", true},
		{"same tier", models.TierFree, "free", true}, // Default tier is free without MongoDB
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test would need MongoDB to set up current subscription
			// For now, just test the error cases
			if tt.expectError {
				_, err := service.PreviewPlanChange(ctx, "user-123", tt.newPlanID)
				if err == nil {
					t.Error("Expected error but got nil")
				}
			}
		})
	}
}
