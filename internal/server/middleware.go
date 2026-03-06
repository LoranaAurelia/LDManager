package server

import (
	"net/http"
	"strings"
)

// withSecurityHeaders 实现该函数对应的业务逻辑。
func (s *Server) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set(
			"Content-Security-Policy",
			strings.Join([]string{
				"default-src 'self'",
				"script-src 'self' 'unsafe-eval'",
				"style-src 'self' 'unsafe-inline'",
				"img-src 'self' data: blob:",
				"font-src 'self' data:",
				"connect-src 'self' ws: wss:",
				"object-src 'none'",
				"base-uri 'self'",
				"frame-ancestors 'none'",
				"form-action 'self'",
			}, "; "),
		)
		if isRequestSecure(r, s.cfg.TrustProxyHeaders) {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// isRequestSecure 判断并返回条件结果。
func isRequestSecure(r *http.Request, trustProxy bool) bool {
	if r != nil && r.TLS != nil {
		return true
	}
	if r == nil || !trustProxy {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Ssl")), "on")
}
