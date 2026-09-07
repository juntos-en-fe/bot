package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenAppliesInitialMigration(t *testing.T) {
	db, err := Open(context.Background(), filepath.Join(t.TempDir(), "nested", "bot.db"))
	if err != nil {
		t.Fatalf("Open returned an error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	var migrationCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM schema_migrations WHERE name = ?", "0001_initial.sql").Scan(&migrationCount); err != nil {
		t.Fatalf("query applied migration: %v", err)
	}
	if migrationCount != 1 {
		t.Fatalf("applied migration count = %d, want 1", migrationCount)
	}

	if _, err := db.Exec("INSERT INTO bot_metadata (key, value) VALUES (?, ?)", "test", "value"); err != nil {
		t.Fatalf("bot_metadata was not created: %v", err)
	}
}
