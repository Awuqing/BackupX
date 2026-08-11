package database

import (
	"path/filepath"
	"testing"

	"backupx/server/internal/config"
	"backupx/server/internal/logger"
)

func TestOpenConfiguresSQLiteForSingleMasterConcurrency(t *testing.T) {
	log, err := logger.New(config.LogConfig{Level: "error"})
	if err != nil {
		t.Fatal(err)
	}
	db, err := Open(config.DatabaseConfig{Path: filepath.Join(t.TempDir(), "backupx.db")}, log)
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	var journalMode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		t.Fatal(err)
	}
	if journalMode != "delete" {
		t.Fatalf("journal_mode = %q, want delete", journalMode)
	}
	var busyTimeout int
	if err := db.Raw("PRAGMA busy_timeout").Scan(&busyTimeout).Error; err != nil {
		t.Fatal(err)
	}
	if busyTimeout != 5000 {
		t.Fatalf("busy_timeout = %d, want 5000", busyTimeout)
	}
}
