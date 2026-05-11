package services

import (
	"math"
	"testing"
	"time"
)

// TestCalculateRecencyScore tests the recency score calculation
func TestCalculateRecencyScore(t *testing.T) {
	service := &MemoryDecayService{}
	config := DefaultDecayConfig()
	now := time.Now()

	tests := []struct {
		name           string
		lastAccessedAt *time.Time
		createdAt      time.Time
		expectedScore  float64
		tolerance      float64
	}{
		{
			name:           "Just accessed (0 days)",
			lastAccessedAt: &now,
			createdAt:      now.AddDate(0, 0, -30),
			expectedScore:  1.0,
			tolerance:      0.01,
		},
		{
			name:           "1 week ago (~0.70)",
			lastAccessedAt: timePtr(now.AddDate(0, 0, -7)),
			createdAt:      now.AddDate(0, 0, -30),
			expectedScore:  0.70,
			tolerance:      0.05,
		},
		{
			name:           "1 month ago (~0.22)",
			lastAccessedAt: timePtr(now.AddDate(0, 0, -30)),
			createdAt:      now.AddDate(0, 0, -60),
			expectedScore:  0.22,
			tolerance:      0.05,
		},
		{
			name:           "3 months ago (~0.01)",
			lastAccessedAt: timePtr(now.AddDate(0, 0, -90)),
			createdAt:      now.AddDate(0, 0, -120),
			expectedScore:  0.01,
			tolerance:      0.02,
		},
		{
			name:           "Never accessed (use createdAt)",
			lastAccessedAt: nil,
			createdAt:      now.AddDate(0, 0, -7),
			expectedScore:  0.70,
			tolerance:      0.05,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := service.calculateRecencyScore(tt.lastAccessedAt, tt.createdAt, now, config.RecencyDecayRate)
			if math.Abs(score-tt.expectedScore) > tt.tolerance {
				t.Errorf("Expected score ~%.2f, got %.2f (tolerance: %.2f)", tt.expectedScore, score, tt.tolerance)
			}
		})
	}
}

// TestCalculateFrequencyScore tests the frequency score calculation
func TestCalculateFrequencyScore(t *testing.T) {
	service := &MemoryDecayService{}
	config := DefaultDecayConfig()

	tests := []struct {
		name          string
		accessCount   int64
		expectedScore float64
	}{
		{
			name:          "0 accesses",
			accessCount:   0,
			expectedScore: 0.0,
		},
		{
			name:          "10 accesses (50%)",
			accessCount:   10,
			expectedScore: 0.5,
		},
		{
			name:          "20 accesses (100%)",
			accessCount:   20,
			expectedScore: 1.0,
		},
		{
			name:          "40 accesses (capped at 100%)",
			accessCount:   40,
			expectedScore: 1.0,
		},
		{
			name:          "5 accesses (25%)",
			accessCount:   5,
			expectedScore: 0.25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := service.calculateFrequencyScore(tt.accessCount, config.FrequencyMax)
			if math.Abs(score-tt.expectedScore) > 0.01 {
				t.Errorf("Expected score %.2f, got %.2f", tt.expectedScore, score)
			}
		})
	}
}

// TestCalculateMemoryScore tests the complete PageRank algorithm
func TestCalculateMemoryScore(t *testing.T) {
	service := &MemoryDecayService{}
	config := DefaultDecayConfig()
	now := time.Now()

	tests := []struct {
		name             string
		accessCount      int64
		lastAccessedAt   *time.Time
		sourceEngagement float64
		createdAt        time.Time
		minScore         float64
		maxScore         float64
		description      string
	}{
		{
			name:             "High quality memory (recent, frequent, engaging)",
			accessCount:      25,
			lastAccessedAt:   &now,
			sourceEngagement: 0.9,
			createdAt:        now.AddDate(0, 0, -30),
			minScore:         0.85,
			maxScore:         1.0,
			description:      "Should have very high score",
		},
		{
			name:             "Medium quality memory",
			accessCount:      10,
			lastAccessedAt:   timePtr(now.AddDate(0, 0, -7)),
			sourceEngagement: 0.6,
			createdAt:        now.AddDate(0, 0, -30),
			minScore:         0.50,
			maxScore:         0.70,
			description:      "Should have medium score",
		},
		{
			name:             "Low quality memory (old, never accessed, low engagement)",
			accessCount:      0,
			lastAccessedAt:   nil,
			sourceEngagement: 0.2,
			createdAt:        now.AddDate(0, 0, -90),
			minScore:         0.0,
			maxScore:         0.15,
			description:      "Should be below archive threshold",
		},
		{
			name:             "Decaying memory (moderately old, few accesses)",
			accessCount:      3,
			lastAccessedAt:   timePtr(now.AddDate(0, 0, -30)),
			sourceEngagement: 0.5,
			createdAt:        now.AddDate(0, 0, -60),
			minScore:         0.15,
			maxScore:         0.35,
			description:      "Should be approaching archive threshold",
		},
		{
			name:             "High engagement saves old memory",
			accessCount:      5,
			lastAccessedAt:   timePtr(now.AddDate(0, 0, -60)),
			sourceEngagement: 0.95,
			createdAt:        now.AddDate(0, 0, -90),
			minScore:         0.30,
			maxScore:         0.50,
			description:      "High engagement keeps it above archive threshold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := service.calculateMemoryScore(
				tt.accessCount,
				tt.lastAccessedAt,
				tt.sourceEngagement,
				tt.createdAt,
				now,
				config,
			)

			if score < tt.minScore || score > tt.maxScore {
				t.Errorf("%s: Expected score between %.2f and %.2f, got %.2f",
					tt.description, tt.minScore, tt.maxScore, score)
			}

			t.Logf("Score: %.3f (recency: %.2f, frequency: %.2f, engagement: %.2f)",
				score,
				service.calculateRecencyScore(tt.lastAccessedAt, tt.createdAt, now, config.RecencyDecayRate),
				service.calculateFrequencyScore(tt.accessCount, config.FrequencyMax),
				tt.sourceEngagement,
			)
		})
	}
}

// TestArchiveThreshold ensures memories below threshold get archived
func TestArchiveThreshold(t *testing.T) {
	service := &MemoryDecayService{}
	config := DefaultDecayConfig()
	now := time.Now()

	// Create a memory that should be archived
	accessCount := int64(0)
	lastAccessedAt := (*time.Time)(nil)
	sourceEngagement := 0.2
	createdAt := now.AddDate(0, 0, -90)

	score := service.calculateMemoryScore(
		accessCount,
		lastAccessedAt,
		sourceEngagement,
		createdAt,
		now,
		config,
	)

	if score >= config.ArchiveThreshold {
		t.Errorf("Expected score below archive threshold (%.2f), got %.2f", config.ArchiveThreshold, score)
	}
}

// TestDecayConfigWeights ensures weights add up to 1.0
func TestDecayConfigWeights(t *testing.T) {
	config := DefaultDecayConfig()

	totalWeight := config.RecencyWeight + config.FrequencyWeight + config.EngagementWeight

	if math.Abs(totalWeight-1.0) > 0.001 {
		t.Errorf("Weights should add up to 1.0, got %.3f", totalWeight)
	}
}

// TestRecencyDecayFormula verifies the exponential decay formula
func TestRecencyDecayFormula(t *testing.T) {
	service := &MemoryDecayService{}
	now := time.Now()

	// Test known values
	tests := []struct {
		daysAgo       int
		decayRate     float64
		expectedScore float64
		tolerance     float64
	}{
		{daysAgo: 0, decayRate: 0.05, expectedScore: 1.0, tolerance: 0.01},
		{daysAgo: 7, decayRate: 0.05, expectedScore: 0.704, tolerance: 0.01},
		{daysAgo: 14, decayRate: 0.05, expectedScore: 0.496, tolerance: 0.01},
		{daysAgo: 30, decayRate: 0.05, expectedScore: 0.223, tolerance: 0.01},
		{daysAgo: 60, decayRate: 0.05, expectedScore: 0.050, tolerance: 0.01},
		{daysAgo: 90, decayRate: 0.05, expectedScore: 0.011, tolerance: 0.01},
	}

	for _, tt := range tests {
		lastAccessed := now.AddDate(0, 0, -tt.daysAgo)
		createdAt := now.AddDate(0, 0, -tt.daysAgo-30)
		score := service.calculateRecencyScore(&lastAccessed, createdAt, now, tt.decayRate)

		if math.Abs(score-tt.expectedScore) > tt.tolerance {
			t.Errorf("Day %d: Expected %.3f, got %.3f (diff: %.3f)",
				tt.daysAgo, tt.expectedScore, score, math.Abs(score-tt.expectedScore))
		}
	}
}

// TestFrequencyScoreLinear ensures frequency score is linear up to max
func TestFrequencyScoreLinear(t *testing.T) {
	service := &MemoryDecayService{}
	frequencyMax := int64(20)

	for i := int64(0); i <= frequencyMax*2; i += 2 {
		score := service.calculateFrequencyScore(i, frequencyMax)

		expected := math.Min(1.0, float64(i)/float64(frequencyMax))
		if math.Abs(score-expected) > 0.001 {
			t.Errorf("Access count %d: Expected %.3f, got %.3f", i, expected, score)
		}
	}
}

// TestMemoryLifecycle tests a realistic memory lifecycle
func TestMemoryLifecycle(t *testing.T) {
	service := &MemoryDecayService{}
	config := DefaultDecayConfig()
	now := time.Now()

	// Day 0: Memory created with high engagement
	createdAt := now.AddDate(0, 0, -90)
	accessCount := int64(0)
	lastAccessed := (*time.Time)(nil)
	sourceEngagement := 0.85

	// Initial score (only engagement matters)
	score0 := service.calculateMemoryScore(accessCount, lastAccessed, sourceEngagement, createdAt, createdAt, config)
	t.Logf("Day 0: Score %.3f", score0)

	// Day 7: Accessed once
	day7 := createdAt.AddDate(0, 0, 7)
	accessCount = 1
	lastAccessed = &day7
	score7 := service.calculateMemoryScore(accessCount, lastAccessed, sourceEngagement, createdAt, day7, config)
	t.Logf("Day 7: Score %.3f (accessed once)", score7)

	// Day 30: Accessed 5 more times
	day30 := createdAt.AddDate(0, 0, 30)
	accessCount = 6
	lastAccessed = &day30
	score30 := service.calculateMemoryScore(accessCount, lastAccessed, sourceEngagement, createdAt, day30, config)
	t.Logf("Day 30: Score %.3f (accessed 6 times total)", score30)

	// Day 60: No new accesses (recency drops)
	day60 := createdAt.AddDate(0, 0, 60)
	score60 := service.calculateMemoryScore(accessCount, lastAccessed, sourceEngagement, createdAt, day60, config)
	t.Logf("Day 60: Score %.3f (no new accesses, recency drops)", score60)

	// Day 90: Still no accesses (further decay)
	day90 := createdAt.AddDate(0, 0, 90)
	score90 := service.calculateMemoryScore(accessCount, lastAccessed, sourceEngagement, createdAt, day90, config)
	t.Logf("Day 90: Score %.3f (continued decay)", score90)

	// Score should decrease over time without accesses
	if score60 >= score30 {
		t.Error("Score should decrease from day 30 to day 60 without accesses")
	}
	if score90 >= score60 {
		t.Error("Score should continue decreasing from day 60 to day 90")
	}

	// Should still be above archive threshold due to high engagement
	if score90 < config.ArchiveThreshold {
		t.Errorf("High engagement memory should stay above archive threshold (%.2f), got %.3f",
			config.ArchiveThreshold, score90)
	}
}

// Helper function to create time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}
