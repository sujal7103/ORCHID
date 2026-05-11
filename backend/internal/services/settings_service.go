package services

import (
	"clara-agents/internal/database"
	"clara-agents/internal/models"
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// SettingsService handles system-wide settings
type SettingsService struct {
	db   *database.DB
	once sync.Once
}

var (
	settingsService *SettingsService
	settingsOnce    sync.Once
)

// GetSettingsService returns the singleton settings service
func GetSettingsService() *SettingsService {
	settingsOnce.Do(func() {
		settingsService = &SettingsService{}
	})
	return settingsService
}

// SetDB sets the database connection
func (s *SettingsService) SetDB(db *database.DB) {
	s.once.Do(func() {
		s.db = db
		s.initTable()
	})
}

// initTable creates the settings table if it doesn't exist
func (s *SettingsService) initTable() {
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS settings (
			` + "`key`" + ` VARCHAR(255) PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`

	if _, err := s.db.Exec(createTableSQL); err != nil {
		fmt.Printf("⚠️  [SETTINGS] Failed to create settings table: %v\n", err)
	}
}

// Get retrieves a setting by key
func (s *SettingsService) Get(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE `key` = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil // Not found is not an error
	}
	return value, err
}

// Set updates or creates a setting
func (s *SettingsService) Set(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO settings (`key`, value, updated_at) VALUES (?, ?, ?) ON DUPLICATE KEY UPDATE value = ?, updated_at = ?",
		key, value, time.Now(), value, time.Now(),
	)
	return err
}

// GetSystemModelAssignments retrieves all system model assignments
func (s *SettingsService) GetSystemModelAssignments(ctx context.Context) (*models.SystemModelAssignments, error) {
	assignments := &models.SystemModelAssignments{}

	// Get all assignments
	toolSelector, _ := s.Get(ctx, models.SettingKeyToolSelector)
	memoryExtractor, _ := s.Get(ctx, models.SettingKeyMemoryExtractor)
	titleGenerator, _ := s.Get(ctx, models.SettingKeyTitleGenerator)
	workflowValidator, _ := s.Get(ctx, models.SettingKeyWorkflowValidator)
	agentDefault, _ := s.Get(ctx, models.SettingKeyAgentDefault)

	assignments.ToolSelector = toolSelector
	assignments.MemoryExtractor = memoryExtractor
	assignments.TitleGenerator = titleGenerator
	assignments.WorkflowValidator = workflowValidator
	assignments.AgentDefault = agentDefault

	return assignments, nil
}

// SetSystemModelAssignments updates all system model assignments
func (s *SettingsService) SetSystemModelAssignments(ctx context.Context, assignments *models.SystemModelAssignments) error {
	// Set each assignment (empty strings will clear the setting)
	if assignments.ToolSelector != "" {
		if err := s.Set(ctx, models.SettingKeyToolSelector, assignments.ToolSelector); err != nil {
			return err
		}
	}
	if assignments.MemoryExtractor != "" {
		if err := s.Set(ctx, models.SettingKeyMemoryExtractor, assignments.MemoryExtractor); err != nil {
			return err
		}
	}
	if assignments.TitleGenerator != "" {
		if err := s.Set(ctx, models.SettingKeyTitleGenerator, assignments.TitleGenerator); err != nil {
			return err
		}
	}
	if assignments.WorkflowValidator != "" {
		if err := s.Set(ctx, models.SettingKeyWorkflowValidator, assignments.WorkflowValidator); err != nil {
			return err
		}
	}
	if assignments.AgentDefault != "" {
		if err := s.Set(ctx, models.SettingKeyAgentDefault, assignments.AgentDefault); err != nil {
			return err
		}
	}

	return nil
}
