package config

import (
	"path/filepath"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RCLONE_MANAGER_DATA_DIR", dir)
	t.Setenv("PORT", "9091")
	t.Setenv("RCLONE_MANAGER_HOST", "127.0.0.1")
	t.Setenv("RCLONE_MANAGER_CORS_ORIGINS", "https://a.example.com, https://b.example.com ")
	t.Setenv("RCLONE_MANAGER_LOG_LEVEL", "DEBUG")
	t.Setenv("RCLONE_MANAGER_LOG_FORMAT", "JSON")
	t.Setenv("RCLONE_DEFAULT_TRANSFERS", "12")
	t.Setenv("RCLONE_MAX_CONCURRENT_JOBS", "0") // clamped to >= 1
	t.Setenv("RCLONE_TIMEZONE", "Asia/Tokyo")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Addr() != "127.0.0.1:9091" {
		t.Fatalf("unexpected Addr: %q", cfg.Addr())
	}
	if cfg.LogLevel != "debug" || cfg.LogFormat != "json" {
		t.Fatalf("unexpected log config: level=%q format=%q", cfg.LogLevel, cfg.LogFormat)
	}
	if len(cfg.CORSOrigins) != 2 || cfg.CORSOrigins[0] != "https://a.example.com" || cfg.CORSOrigins[1] != "https://b.example.com" {
		t.Fatalf("unexpected CORSOrigins: %v", cfg.CORSOrigins)
	}
	if cfg.Defaults.Transfers != 12 || cfg.Defaults.MaxConcurrentJobs != 1 || cfg.Defaults.Timezone != "Asia/Tokyo" {
		t.Fatalf("unexpected defaults: %+v", cfg.Defaults)
	}
	if cfg.DBPath != filepath.Join(dir, "rclone-manager.db") {
		t.Fatalf("unexpected DBPath: %q", cfg.DBPath)
	}
}
