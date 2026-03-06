package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
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
}

// NewSessionManager 创建基于 HMAC-SHA256 的会话管理器。
// NewSessionManager 创建并初始化对应对象。
func NewSessionManager(cookieName string, cookiePath string, trustProxy bool, ttl time.Duration, secret string) *SessionManager {
	path := strings.TrimSpace(cookiePath)
	if path == "" || !strings.HasPrefix(path, "/") {
		path = "/"
	}
	return &SessionManager{
		cookieName: cookieName,
		cookiePath: path,
		trustProxy: trustProxy,
		ttl:        ttl,
		secret:     []byte(secret),
	}
}

// Create 为当前请求签发登录 Cookie。
// Create 实现该函数对应的业务逻辑。
func (m *SessionManager) Create(w http.ResponseWriter, r *http.Request) error {
	if len(m.secret) == 0 {
		return errors.New("session secret is empty")
	}

	expireAt := time.Now().Add(m.ttl).Unix()
	payload := strconv.FormatInt(expireAt, 10)
	sig := m.sign(payload)

	token := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." +
		base64.RawURLEncoding.EncodeToString(sig)

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
func (m *SessionManager) Destroy(w http.ResponseWriter, _ *http.Request) {
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

	parts := strings.Split(c.Value, ".")
	if len(parts) != 2 {
		return false
	}

	payloadRaw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	sigRaw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}

	payload := string(payloadRaw)
	expected := m.sign(payload)
	if !hmac.Equal(sigRaw, expected) {
		return false
	}

	expUnix, err := strconv.ParseInt(payload, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().After(time.Unix(expUnix, 0)) {
		return false
	}
	return true
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
