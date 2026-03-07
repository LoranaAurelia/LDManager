package server

import (
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"web-sealdice/internal/config"
)

// loginBucket 记录单一指纹在窗口期内的失败次数与封禁截止时间。
type loginBucket struct {
	fails       []time.Time
	blockedTill time.Time
	lastSeen    time.Time
}

// loginProtector 基于指纹实现登录限流与临时封禁。
type loginProtector struct {
	enabled         bool
	maxAttempts     int
	window          time.Duration
	block           time.Duration
	trustProxy      bool
	maxBuckets      int
	bucketIdleTTL   time.Duration
	cleanupInterval time.Duration
	lastCleanup     time.Time

	mu       sync.Mutex
	buckets  map[string]*loginBucket
	overflow *loginBucket
}

// newLoginProtector 创建并初始化对应对象。
func newLoginProtector(cfg config.Config) *loginProtector {
	return &loginProtector{
		enabled:         cfg.LoginProtect.Enabled,
		maxAttempts:     cfg.LoginProtect.MaxAttempts,
		window:          time.Duration(cfg.LoginProtect.WindowSeconds) * time.Second,
		block:           time.Duration(cfg.LoginProtect.BlockSeconds) * time.Second,
		trustProxy:      cfg.TrustProxyHeaders,
		maxBuckets:      cfg.LoginProtect.MaxBuckets,
		bucketIdleTTL:   time.Duration(cfg.LoginProtect.BucketIdleTTLSeconds) * time.Second,
		cleanupInterval: time.Duration(cfg.LoginProtect.CleanupIntervalSeconds) * time.Second,
		lastCleanup:     time.Now(),
		buckets:         make(map[string]*loginBucket),
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

	p.maybeCleanupLocked(now)
	b := p.bucketLocked(key, now)
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

	p.maybeCleanupLocked(now)
	b := p.bucketLocked(key, now)
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
func (p *loginProtector) bucketLocked(key string, now time.Time) *loginBucket {
	b := p.buckets[key]
	if b == nil {
		if len(p.buckets) >= p.maxBuckets {
			p.cleanupBucketsLocked(now, true)
		}
		if len(p.buckets) >= p.maxBuckets {
			if !p.evictOldestUnlockedLocked(now) {
				if p.overflow == nil {
					p.overflow = &loginBucket{}
				}
				if now.After(p.overflow.blockedTill) {
					p.overflow.blockedTill = now.Add(p.block)
				}
				p.overflow.lastSeen = now
				return p.overflow
			}
		}
		b = &loginBucket{}
		p.buckets[key] = b
	}
	b.lastSeen = now
	return b
}

func (p *loginProtector) maybeCleanupLocked(now time.Time) {
	if p.cleanupInterval <= 0 {
		return
	}
	if now.Sub(p.lastCleanup) < p.cleanupInterval {
		return
	}
	p.cleanupBucketsLocked(now, false)
	p.lastCleanup = now
}

func (p *loginProtector) cleanupBucketsLocked(now time.Time, aggressive bool) {
	for key, bucket := range p.buckets {
		if bucket == nil {
			delete(p.buckets, key)
			continue
		}
		if now.Before(bucket.blockedTill) {
			continue
		}
		if len(bucket.fails) == 0 {
			if aggressive || now.Sub(bucket.lastSeen) >= p.bucketIdleTTL {
				delete(p.buckets, key)
			}
			continue
		}
		cut := now.Add(-p.window)
		kept := bucket.fails[:0]
		for _, ts := range bucket.fails {
			if ts.After(cut) {
				kept = append(kept, ts)
			}
		}
		bucket.fails = kept
		if len(bucket.fails) == 0 && (aggressive || now.Sub(bucket.lastSeen) >= p.bucketIdleTTL) {
			delete(p.buckets, key)
		}
	}
}

func (p *loginProtector) evictOldestUnlockedLocked(now time.Time) bool {
	type pair struct {
		key      string
		lastSeen time.Time
	}
	items := make([]pair, 0, len(p.buckets))
	for k, b := range p.buckets {
		if b != nil && now.Before(b.blockedTill) {
			continue
		}
		ls := time.Time{}
		if b != nil {
			ls = b.lastSeen
		}
		items = append(items, pair{key: k, lastSeen: ls})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].lastSeen.Before(items[j].lastSeen)
	})
	if len(items) > 0 {
		delete(p.buckets, items[0].key)
		return true
	}
	return false
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
