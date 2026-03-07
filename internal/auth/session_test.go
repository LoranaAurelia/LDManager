package auth

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionCreateAndAuthenticate(t *testing.T) {
	mgr := NewSessionManager("test_session", "/", true, 2*time.Hour, "test-secret", 1000, time.Minute)
	resp := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "https://example.com", nil)

	if err := mgr.Create(resp, req); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	cookies := resp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected cookie to be set")
	}
	if !cookies[0].Secure {
		t.Fatalf("expected secure cookie on https request")
	}

	verifyReq := httptest.NewRequest("GET", "/", nil)
	verifyReq.AddCookie(cookies[0])
	if !mgr.IsAuthenticated(verifyReq) {
		t.Fatalf("expected authenticated request")
	}
}

func TestSessionDestroy(t *testing.T) {
	mgr := NewSessionManager("test_session", "/panel", false, time.Hour, "test-secret", 1000, time.Minute)
	resp := httptest.NewRecorder()
	mgr.Destroy(resp, nil)

	cookies := resp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected destroy cookie")
	}
	if cookies[0].MaxAge != -1 {
		t.Fatalf("expected MaxAge=-1, got %d", cookies[0].MaxAge)
	}
	if cookies[0].Path != "/panel" {
		t.Fatalf("expected cookie path /panel, got %s", cookies[0].Path)
	}
}

func TestIsSecureRequestByHeadersAndTLS(t *testing.T) {
	httpReq := httptest.NewRequest("GET", "http://example.com", nil)
	if isSecureRequest(httpReq, true) {
		t.Fatalf("plain http should not be secure")
	}

	proxyReq := httptest.NewRequest("GET", "http://example.com", nil)
	proxyReq.Header.Set("X-Forwarded-Proto", "https")
	if !isSecureRequest(proxyReq, true) {
		t.Fatalf("x-forwarded-proto=https should be treated as secure")
	}
	if isSecureRequest(proxyReq, false) {
		t.Fatalf("proxy headers should be ignored when trustProxy=false")
	}

	tlsReq := httptest.NewRequest("GET", "https://example.com", nil)
	tlsReq.TLS = &tls.ConnectionState{}
	if !isSecureRequest(tlsReq, false) {
		t.Fatalf("tls request should be secure")
	}
}

func TestSessionDestroyRevokesActiveToken(t *testing.T) {
	mgr := NewSessionManager("test_session", "/", false, time.Hour, "test-secret", 1000, time.Minute)
	resp := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "http://example.com", nil)
	if err := mgr.Create(resp, req); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	cookies := resp.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatalf("expected cookie")
	}
	verifyReq := httptest.NewRequest("GET", "/", nil)
	verifyReq.AddCookie(cookies[0])
	if !mgr.IsAuthenticated(verifyReq) {
		t.Fatalf("expected authenticated before destroy")
	}
	destroyResp := httptest.NewRecorder()
	mgr.Destroy(destroyResp, verifyReq)
	if mgr.IsAuthenticated(verifyReq) {
		t.Fatalf("expected token revoked after destroy")
	}
}
