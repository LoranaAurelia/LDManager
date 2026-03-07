package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SessionManager 管理登录态 Cookie 的签发与校验。
// SessionManager 负责会话 Cookie 的签发、校验与销毁。
type SessionManager struct {
	cookieName string
	cookiePath string
	trustProxy bool
	ttl        time.Duration
	secret     []byte

	maxEntries      int
	cleanupInterval time.Duration
	lastCleanup     time.Time
	seq             uint64
	mu              sync.Mutex
	sessions        map[string]sessionRecord
}

type sessionRecord struct {
	exp int64
	seq uint64
}

type sessionTokenPayload struct {
	SID string `json:"sid"`
	Exp int64  `json:"exp"`
}

// NewSessionManager 创建基于 HMAC-SHA256 的会话管理器。
// NewSessionManager 创建并初始化对应对象。
func NewSessionManager(cookieName string, cookiePath string, trustProxy bool, ttl time.Duration, secret string, maxEntries int, cleanupInterval time.Duration) *SessionManager {
	path := strings.TrimSpace(cookiePath)
	if path == "" || !strings.HasPrefix(path, "/") {
		path = "/"
	}
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}
	return &SessionManager{
		cookieName:      cookieName,
		cookiePath:      path,
		trustProxy:      trustProxy,
		ttl:             ttl,
		secret:          []byte(secret),
		maxEntries:      maxEntries,
		cleanupInterval: cleanupInterval,
		lastCleanup:     time.Now(),
		sessions:        make(map[string]sessionRecord),
	}
}

// Create 为当前请求签发登录 Cookie。
// Create 实现该函数对应的业务逻辑。
func (m *SessionManager) Create(w http.ResponseWriter, r *http.Request) error {
	if len(m.secret) == 0 {
		return errors.New("session secret is empty")
	}
	now := time.Now()
	expireAt := now.Add(m.ttl).Unix()
	sid, err := newSessionID()
	if err != nil {
		return err
	}

	payloadObj := sessionTokenPayload{SID: sid, Exp: expireAt}
	payloadJSON, err := json.Marshal(payloadObj)
	if err != nil {
		return err
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadJSON)
	sig := base64.RawURLEncoding.EncodeToString(m.sign(payload))
	token := payload + "." + sig

	m.mu.Lock()
	m.maybeCleanupLocked(now.Unix())
	m.seq++
	m.sessions[sid] = sessionRecord{exp: expireAt, seq: m.seq}
	if len(m.sessions) > m.maxEntries {
		m.evictOldestLocked(sid)
	}
	m.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    token,
		Path:     m.cookiePath,
		HttpOnly: true,
		Secure:   isSecureRequest(r, m.trustProxy),
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(expireAt, 0),
		MaxAge:   int(m.ttl.Seconds()),
	})
	return nil
}

// Destroy 实现该函数对应的业务逻辑。
func (m *SessionManager) Destroy(w http.ResponseWriter, r *http.Request) {
	if r != nil {
		if c, err := r.Cookie(m.cookieName); err == nil {
			if payload, ok := m.parseToken(c.Value); ok {
				m.mu.Lock()
				delete(m.sessions, payload.SID)
				m.mu.Unlock()
			}
		}
	}
	http.SetCookie(w, &http.Cookie{
		Name:     m.cookieName,
		Value:    "",
		Path:     m.cookiePath,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// IsAuthenticated 校验请求中携带的会话 Cookie 是否有效。
// IsAuthenticated 判断并返回条件结果。
func (m *SessionManager) IsAuthenticated(r *http.Request) bool {
	c, err := r.Cookie(m.cookieName)
	if err != nil || strings.TrimSpace(c.Value) == "" {
		return false
	}
	payload, ok := m.parseToken(c.Value)
	if !ok {
		return false
	}
	nowUnix := time.Now().Unix()
	if nowUnix > payload.Exp {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maybeCleanupLocked(nowUnix)
	stored, exists := m.sessions[payload.SID]
	if !exists {
		return false
	}
	if stored.exp != payload.Exp || nowUnix > stored.exp {
		delete(m.sessions, payload.SID)
		return false
	}
	return true
}

func (m *SessionManager) parseToken(token string) (sessionTokenPayload, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return sessionTokenPayload{}, false
	}
	expectedSig := base64.RawURLEncoding.EncodeToString(m.sign(parts[0]))
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return sessionTokenPayload{}, false
	}
	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return sessionTokenPayload{}, false
	}
	var payload sessionTokenPayload
	if err := json.Unmarshal(payloadRaw, &payload); err != nil {
		return sessionTokenPayload{}, false
	}
	if strings.TrimSpace(payload.SID) == "" || payload.Exp <= 0 {
		return sessionTokenPayload{}, false
	}
	return payload, true
}

func (m *SessionManager) maybeCleanupLocked(nowUnix int64) {
	now := time.Unix(nowUnix, 0)
	if now.Sub(m.lastCleanup) < m.cleanupInterval {
		return
	}
	for sid, rec := range m.sessions {
		if nowUnix > rec.exp {
			delete(m.sessions, sid)
		}
	}
	m.lastCleanup = now
}

func (m *SessionManager) evictOldestLocked(exceptSID string) {
	var oldestSID string
	var oldestExp int64
	var oldestSeq uint64
	for sid, rec := range m.sessions {
		if sid == exceptSID {
			continue
		}
		if oldestSID == "" || rec.exp < oldestExp || (rec.exp == oldestExp && rec.seq < oldestSeq) {
			oldestSID = sid
			oldestExp = rec.exp
			oldestSeq = rec.seq
		}
	}
	if oldestSID != "" {
		delete(m.sessions, oldestSID)
	}
}

func newSessionID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// sign 实现该函数对应的业务逻辑。
func (m *SessionManager) sign(payload string) []byte {
	h := hmac.New(sha256.New, m.secret)
	_, _ = h.Write([]byte(payload))
	return h.Sum(nil)
}

// isSecureRequest 判断当前请求是否应被视为安全请求（HTTPS）。
// 当 trustProxy=true 时，会额外信任反代头：
// isSecureRequest 判断并返回条件结果。
func isSecureRequest(r *http.Request, trustProxy bool) bool {
	if r != nil && r.TLS != nil {
		return true
	}
	if r == nil {
		return false
	}
	if !trustProxy {
		return false
	}
	proto := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")))
	if proto == "https" {
		return true
	}
	forwardedSSL := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Ssl")))
	return forwardedSSL == "on"
}
