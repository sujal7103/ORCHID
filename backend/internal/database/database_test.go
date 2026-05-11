package database

import (
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	// Create a temporary database file
	tmpFile := "test_database.db"
	defer os.Remove(tmpFile)

	db, err := New(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	if db == nil {
		t.Fatal("Expected non-nil database")
	}

	// Test connection
	if err := db.Ping(); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}
}

func TestNew_InvalidPath(t *testing.T) {
	// Test with invalid path
	_, err := New("/invalid/path/that/does/not/exist/test.db")
	if err == nil {
		t.Fatal("Expected error for invalid path, got nil")
	}
}

func TestInitialize(t *testing.T) {
	// Create a temporary database
	tmpFile := "test_init.db"
	defer os.Remove(tmpFile)

	db, err := New(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Initialize schema
	if err := db.Initialize(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Verify tables were created
	tables := []string{
		"providers",
		"models",
		"provider_model_filters",
		"model_capabilities",
		"model_refresh_log",
	}

	for _, table := range tables {
		var name string
		query := "SELECT name FROM sqlite_master WHERE type='table' AND name=?"
		err := db.QueryRow(query, table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s was not created: %v", table, err)
		}
	}
}

func TestInitialize_ForeignKeys(t *testing.T) {
	tmpFile := "test_fk.db"
	defer os.Remove(tmpFile)

	db, err := New(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	if err := db.Initialize(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Verify foreign keys are enabled
	var fkEnabled int
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled)
	if err != nil {
		t.Fatalf("Failed to check foreign keys: %v", err)
	}

	if fkEnabled != 1 {
		t.Error("Foreign keys are not enabled")
	}
}

func TestInitialize_Indexes(t *testing.T) {
	tmpFile := "test_indexes.db"
	defer os.Remove(tmpFile)

	db, err := New(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	if err := db.Initialize(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Verify indexes were created
	indexes := []string{
		"idx_models_provider",
		"idx_models_visible",
		"idx_filters_provider",
		"idx_capabilities_provider",
	}

	for _, index := range indexes {
		var name string
		query := "SELECT name FROM sqlite_master WHERE type='index' AND name=?"
		err := db.QueryRow(query, index).Scan(&name)
		if err != nil {
			t.Errorf("Index %s was not created: %v", index, err)
		}
	}
}

func TestInitialize_Idempotent(t *testing.T) {
	tmpFile := "test_idempotent.db"
	defer os.Remove(tmpFile)

	db, err := New(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	// Initialize multiple times - should not error
	if err := db.Initialize(); err != nil {
		t.Fatalf("First initialization failed: %v", err)
	}

	if err := db.Initialize(); err != nil {
		t.Fatalf("Second initialization failed: %v", err)
	}

	if err := db.Initialize(); err != nil {
		t.Fatalf("Third initialization failed: %v", err)
	}
}

func TestInitialize_TableOrder(t *testing.T) {
	// This test verifies that tables are created in the correct order
	// to satisfy foreign key constraints
	tmpFile := "test_order.db"
	defer os.Remove(tmpFile)

	db, err := New(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	if err := db.Initialize(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Try to insert test data with foreign key relationships
	// Insert provider (parent)
	_, err = db.Exec(`INSERT INTO providers (name, base_url, api_key) VALUES (?, ?, ?)`,
		"Test Provider", "https://test.example.com", "test-key")
	if err != nil {
		t.Fatalf("Failed to insert provider: %v", err)
	}

	// Insert model (child)
	_, err = db.Exec(`INSERT INTO models (id, provider_id, name) VALUES (?, ?, ?)`,
		"test-model", 1, "Test Model")
	if err != nil {
		t.Fatalf("Failed to insert model: %v", err)
	}

	// Verify data
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM models WHERE provider_id = 1").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query models: %v", err)
	}

	if count != 1 {
		t.Errorf("Expected 1 model, got %d", count)
	}
}

func TestDatabase_ForeignKeyConstraints(t *testing.T) {
	tmpFile := "test_fk_constraints.db"
	defer os.Remove(tmpFile)

	db, err := New(tmpFile)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	defer db.Close()

	if err := db.Initialize(); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	// Try to insert model without provider (should fail)
	_, err = db.Exec(`INSERT INTO models (id, provider_id, name) VALUES (?, ?, ?)`,
		"test-model", 999, "Test Model")

	if err == nil {
		t.Error("Expected foreign key constraint error, got nil")
	}
}
