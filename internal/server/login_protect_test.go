package server

import (
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"web-sealdice/internal/config"
)

func TestLoginProtectorBlocksAfterThreshold(t *testing.T) {
	cfg := config.Default()
	cfg.LoginProtect.Enabled = true
	cfg.LoginProtect.MaxAttempts = 2
	cfg.LoginProtect.WindowSeconds = 60
	cfg.LoginProtect.BlockSeconds = 60

	p := newLoginProtector(cfg)
	r := httptest.NewRequest("POST", "/api/auth/login", nil)
	r.RemoteAddr = "127.0.0.1:12345"

	if ok, _ := p.Allow(r); !ok {
		t.Fatalf("should allow first request")
	}
	p.RecordFailure(r)
	if ok, _ := p.Allow(r); !ok {
		t.Fatalf("should still allow before threshold")
	}
	p.RecordFailure(r)
	if ok, _ := p.Allow(r); ok {
		t.Fatalf("should be blocked after threshold")
	}
}

func TestLoginProtectorSuccessClearsFailures(t *testing.T) {
	cfg := config.Default()
	cfg.LoginProtect.Enabled = true
	cfg.LoginProtect.MaxAttempts = 2
	cfg.LoginProtect.WindowSeconds = 60
	cfg.LoginProtect.BlockSeconds = 60

	p := newLoginProtector(cfg)
	r := httptest.NewRequest("POST", "/api/auth/login", nil)
	r.RemoteAddr = "127.0.0.1:12345"

	p.RecordFailure(r)
	p.RecordSuccess(r)
	p.RecordFailure(r)
	if ok, _ := p.Allow(r); !ok {
		t.Fatalf("should not be blocked after success reset")
	}
}

func TestLoginProtectorWindowExpires(t *testing.T) {
	cfg := config.Default()
	cfg.LoginProtect.Enabled = true
	cfg.LoginProtect.MaxAttempts = 2
	cfg.LoginProtect.WindowSeconds = 1
	cfg.LoginProtect.BlockSeconds = 1

	p := newLoginProtector(cfg)
	r := httptest.NewRequest("POST", "/api/auth/login", nil)
	r.RemoteAddr = "127.0.0.1:12345"

	p.RecordFailure(r)
	time.Sleep(1200 * time.Millisecond)
	p.RecordFailure(r)
	if ok, _ := p.Allow(r); !ok {
		t.Fatalf("old failures should expire with window")
	}
}

func TestLoginProtectorBucketCap(t *testing.T) {
	cfg := config.Default()
	cfg.LoginProtect.Enabled = true
	cfg.LoginProtect.MaxBuckets = 3
	cfg.LoginProtect.CleanupIntervalSeconds = 3600
	cfg.LoginProtect.BucketIdleTTLSeconds = 3600

	p := newLoginProtector(cfg)
	for i := 0; i < 10; i++ {
		r := httptest.NewRequest("POST", "/api/auth/login", nil)
		r.RemoteAddr = fmt.Sprintf("127.0.0.1:%d", 10000+i)
		p.RecordFailure(r)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.buckets) > cfg.LoginProtect.MaxBuckets {
		t.Fatalf("bucket cap exceeded: got %d > %d", len(p.buckets), cfg.LoginProtect.MaxBuckets)
	}
}
