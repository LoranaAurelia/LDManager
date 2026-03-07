package server

import (
	"testing"
	"time"

	"web-sealdice/internal/auth"
	"web-sealdice/internal/config"
)

func TestApplyRuntimeConfigPreservesSessionsWhenSessionSettingsUnchanged(t *testing.T) {
	cfg := config.Default()
	s := &Server{
		cfg:           cfg,
		configPath:    "config.yaml",
		passwordStore: auth.NewPasswordStore(cfg.PasswordHashPath()),
		sessions: auth.NewSessionManager(
			cfg.SessionCookieName,
			cfg.BasePath,
			cfg.TrustProxyHeaders,
			time.Duration(cfg.SessionTTL())*time.Hour,
			cfg.SessionSecret,
			cfg.SessionMaxEntries,
			time.Duration(cfg.SessionCleanupInterval)*time.Second,
		),
		loginProtector: newLoginProtector(cfg),
	}

	before := s.sessions
	next := cfg
	next.LogRetentionCount = cfg.LogRetentionCount + 1
	s.applyRuntimeConfig(next)
	if s.sessions != before {
		t.Fatalf("sessions manager should be preserved when session settings unchanged")
	}
}

func TestApplyRuntimeConfigRebuildsSessionsWhenSessionSettingsChanged(t *testing.T) {
	cfg := config.Default()
	s := &Server{
		cfg:           cfg,
		configPath:    "config.yaml",
		passwordStore: auth.NewPasswordStore(cfg.PasswordHashPath()),
		sessions: auth.NewSessionManager(
			cfg.SessionCookieName,
			cfg.BasePath,
			cfg.TrustProxyHeaders,
			time.Duration(cfg.SessionTTL())*time.Hour,
			cfg.SessionSecret,
			cfg.SessionMaxEntries,
			time.Duration(cfg.SessionCleanupInterval)*time.Second,
		),
		loginProtector: newLoginProtector(cfg),
	}

	before := s.sessions
	next := cfg
	next.SessionTTLHours = cfg.SessionTTLHours + 1
	s.applyRuntimeConfig(next)
	if s.sessions == before {
		t.Fatalf("sessions manager should be rebuilt when session settings changed")
	}
}
