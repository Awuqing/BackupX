package backint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	dir := t.TempDir()
	storagePath := filepath.Join(dir, "storage.json")
	if err := os.WriteFile(storagePath, []byte(`{"basePath":"/tmp/backup"}`), 0644); err != nil {
		t.Fatal(err)
	}
	input := `
; 注释
#STORAGE_TYPE = local_disk
#STORAGE_CONFIG_JSON = ` + storagePath + `
#PARALLEL_FACTOR = 4
#COMPRESS = true
#KEY_PREFIX = /hana/backups/
#CATALOG_DB = ` + filepath.Join(dir, "catalog.db") + `
`
	cfg, err := ParseConfig(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.StorageType != "local_disk" {
		t.Errorf("StorageType: %q", cfg.StorageType)
	}
	if cfg.ParallelFactor != 4 {
		t.Errorf("ParallelFactor: %d", cfg.ParallelFactor)
	}
	if !cfg.Compress {
		t.Errorf("Compress should be true")
	}
	if cfg.KeyPrefix != "hana/backups" {
		t.Errorf("KeyPrefix should be trimmed: %q", cfg.KeyPrefix)
	}
	if cfg.StorageConfig["basePath"] != "/tmp/backup" {
		t.Errorf("StorageConfig mismatch: %+v", cfg.StorageConfig)
	}
}

func TestParseConfig_MissingStorageType(t *testing.T) {
	input := `PARALLEL_FACTOR = 1`
	if _, err := ParseConfig(strings.NewReader(input)); err == nil {
		t.Fatal("expected error for missing STORAGE_TYPE")
	}
}

func TestParseConfig_InlineStorageConfig(t *testing.T) {
	input := `STORAGE_TYPE = local_disk
STORAGE_CONFIG = {"basePath":"/x"}
`
	cfg, err := ParseConfig(strings.NewReader(input))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.StorageConfig["basePath"] != "/x" {
		t.Errorf("inline config not parsed: %+v", cfg.StorageConfig)
	}
}

func TestParseConfig_InvalidParallel(t *testing.T) {
	input := `STORAGE_TYPE = local_disk
STORAGE_CONFIG = {}
PARALLEL_FACTOR = oops
`
	if _, err := ParseConfig(strings.NewReader(input)); err == nil {
		t.Fatal("expected error for invalid PARALLEL_FACTOR")
	}
}
