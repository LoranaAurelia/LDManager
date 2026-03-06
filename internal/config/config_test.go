package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeAndValidate(t *testing.T) {
	cfg := Config{
		ListenHost:        "",
		ListenPort:        0,
		BasePath:          "services/",
		DataDir:           "",
		AuthDir:           "",
		MetricsRefreshSec: 0,
		SessionCookieName: "",
		SessionTTLHours:   0,
		SessionSecret:     "test-secret",
	}

	cfg.normalize()

	if cfg.ListenHost != "0.0.0.0" {
		t.Fatalf("unexpected listen host: %s", cfg.ListenHost)
	}
	if cfg.ListenPort != 3210 {
		t.Fatalf("unexpected listen port: %d", cfg.ListenPort)
	}
	if cfg.BasePath != "/services" {
		t.Fatalf("unexpected base path: %s", cfg.BasePath)
	}
	if cfg.DataDir != "./data" || cfg.AuthDir != "./auth" {
		t.Fatalf("unexpected dir defaults: data=%s auth=%s", cfg.DataDir, cfg.AuthDir)
	}
	if cfg.LogRetentionCount != 10 {
		t.Fatalf("unexpected log retention default: %d", cfg.LogRetentionCount)
	}
	if cfg.LogRetentionDays != 30 {
		t.Fatalf("unexpected log retention days default: %d", cfg.LogRetentionDays)
	}
	if cfg.LogMaxMB != 2048 {
		t.Fatalf("unexpected log max mb default: %d", cfg.LogMaxMB)
	}
	if cfg.MetricsRefreshSec != 2 {
		t.Fatalf("unexpected metrics refresh default: %d", cfg.MetricsRefreshSec)
	}
	if !cfg.FileManager.Enabled {
		t.Fatalf("file manager should be enabled by default")
	}
	if cfg.FileManager.UploadMaxMB != 2048 {
		t.Fatalf("unexpected file upload max default: %d", cfg.FileManager.UploadMaxMB)
	}
	if cfg.SessionTTLHours != 48 {
		t.Fatalf("unexpected session ttl default: %d", cfg.SessionTTLHours)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestLoadOrCreateCreatesConfigFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "sealpanel.yaml")

	cfg, created, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate failed: %v", err)
	}
	if !created {
		t.Fatalf("expected created=true on first load")
	}
	if strings.TrimSpace(cfg.SessionSecret) == "" {
		t.Fatalf("session secret should not be empty")
	}

	cfg2, created2, err := LoadOrCreate(path)
	if err != nil {
		t.Fatalf("LoadOrCreate second call failed: %v", err)
	}
	if created2 {
		t.Fatalf("expected created=false on second load")
	}
	if cfg2.SessionSecret != cfg.SessionSecret {
		t.Fatalf("session secret should be preserved between loads")
	}
}
