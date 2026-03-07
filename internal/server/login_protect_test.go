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

func TestLoginProtectorDoesNotEvictActiveBlockedBuckets(t *testing.T) {
	cfg := config.Default()
	cfg.LoginProtect.Enabled = true
	cfg.LoginProtect.MaxAttempts = 1
	cfg.LoginProtect.WindowSeconds = 60
	cfg.LoginProtect.BlockSeconds = 60
	cfg.LoginProtect.MaxBuckets = 2
	cfg.LoginProtect.CleanupIntervalSeconds = 3600
	cfg.LoginProtect.BucketIdleTTLSeconds = 3600

	p := newLoginProtector(cfg)
	r1 := httptest.NewRequest("POST", "/api/auth/login", nil)
	r1.RemoteAddr = "127.0.0.1:10001"
	r2 := httptest.NewRequest("POST", "/api/auth/login", nil)
	r2.RemoteAddr = "127.0.0.2:10002"
	r3 := httptest.NewRequest("POST", "/api/auth/login", nil)
	r3.RemoteAddr = "127.0.0.3:10003"

	p.RecordFailure(r1)
	p.RecordFailure(r2)

	if ok, _ := p.Allow(r1); ok {
		t.Fatalf("first identity should be blocked")
	}
	if ok, _ := p.Allow(r2); ok {
		t.Fatalf("second identity should be blocked")
	}

	if ok, _ := p.Allow(r3); ok {
		t.Fatalf("new identity should be denied while all buckets are actively blocked")
	}

	k1 := clientFingerprint(r1, p.trustProxy)
	k2 := clientFingerprint(r2, p.trustProxy)

	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.buckets[k1]; !ok {
		t.Fatalf("blocked bucket for r1 should not be evicted")
	}
	if _, ok := p.buckets[k2]; !ok {
		t.Fatalf("blocked bucket for r2 should not be evicted")
	}
	if len(p.buckets) != cfg.LoginProtect.MaxBuckets {
		t.Fatalf("bucket count should stay at cap when saturated with blocked buckets")
	}
}
