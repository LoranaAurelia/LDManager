package server

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"web-sealdice/internal/config"
)

// loginBucket 记录单一指纹在窗口期内的失败次数与封禁截止时间。
type loginBucket struct {
	fails       []time.Time
	blockedTill time.Time
}

// loginProtector 基于指纹实现登录限流与临时封禁。
type loginProtector struct {
	enabled     bool
	maxAttempts int
	window      time.Duration
	block       time.Duration
	trustProxy  bool

	mu      sync.Mutex
	buckets map[string]*loginBucket
}

// newLoginProtector 创建并初始化对应对象。
func newLoginProtector(cfg config.Config) *loginProtector {
	return &loginProtector{
		enabled:     cfg.LoginProtect.Enabled,
		maxAttempts: cfg.LoginProtect.MaxAttempts,
		window:      time.Duration(cfg.LoginProtect.WindowSeconds) * time.Second,
		block:       time.Duration(cfg.LoginProtect.BlockSeconds) * time.Second,
		trustProxy:  cfg.TrustProxyHeaders,
		buckets:     make(map[string]*loginBucket),
	}
}

// Allow 实现该函数对应的业务逻辑。
func (p *loginProtector) Allow(r *http.Request) (bool, int) {
	if p == nil || !p.enabled {
		return true, 0
	}
	key := clientFingerprint(r, p.trustProxy)
	now := time.Now()

	p.mu.Lock()
	defer p.mu.Unlock()

	b := p.bucketLocked(key)
	if now.Before(b.blockedTill) {
		retry := int(time.Until(b.blockedTill).Seconds())
		if retry < 1 {
			retry = 1
		}
		return false, retry
	}
	return true, 0
}

// RecordFailure 实现该函数对应的业务逻辑。
func (p *loginProtector) RecordFailure(r *http.Request) int {
	if p == nil || !p.enabled {
		return 0
	}
	key := clientFingerprint(r, p.trustProxy)
	now := time.Now()

	p.mu.Lock()
	defer p.mu.Unlock()

	b := p.bucketLocked(key)
	b.fails = append(b.fails, now)
	cut := now.Add(-p.window)
	kept := b.fails[:0]
	for _, ts := range b.fails {
		if ts.After(cut) {
			kept = append(kept, ts)
		}
	}
	b.fails = kept
	if len(b.fails) >= p.maxAttempts {
		b.blockedTill = now.Add(p.block)
		b.fails = b.fails[:0]
		return int(p.block.Seconds())
	}
	return 0
}

// RecordSuccess 实现该函数对应的业务逻辑。
func (p *loginProtector) RecordSuccess(r *http.Request) {
	if p == nil || !p.enabled {
		return
	}
	key := clientFingerprint(r, p.trustProxy)
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.buckets, key)
}

// bucketLocked 实现该函数对应的业务逻辑。
func (p *loginProtector) bucketLocked(key string) *loginBucket {
	b := p.buckets[key]
	if b == nil {
		b = &loginBucket{}
		p.buckets[key] = b
	}
	return b
}

// clientFingerprint 实现该函数对应的业务逻辑。
func clientFingerprint(r *http.Request, trustProxy bool) string {
	ip := clientAddressForLog(r, trustProxy)
	ua := strings.TrimSpace(r.Header.Get("User-Agent"))
	al := strings.TrimSpace(r.Header.Get("Accept-Language"))
	ch := strings.TrimSpace(r.Header.Get("Sec-CH-UA"))
	return strings.Join([]string{ip, ua, al, ch}, "|")
}

// clientAddressForLog 实现该函数对应的业务逻辑。
func clientAddressForLog(r *http.Request, trustProxy bool) string {
	if r == nil {
		return ""
	}
	if trustProxy {
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				v := strings.TrimSpace(parts[0])
				if v != "" {
					return v
				}
			}
		}
		if xr := strings.TrimSpace(r.Header.Get("X-Real-IP")); xr != "" {
			return xr
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
