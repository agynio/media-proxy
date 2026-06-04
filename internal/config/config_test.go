package config

import (
	"testing"
	"time"
)

func TestFromEnvDefaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddr != ":8080" {
		t.Fatalf("unexpected listen addr: %s", cfg.ListenAddr)
	}
	if cfg.CORSAllowedOrigin != "https://agyn.dev" {
		t.Fatalf("unexpected cors origin: %s", cfg.CORSAllowedOrigin)
	}
	if cfg.MaxResponseSize != 50*1024*1024 {
		t.Fatalf("unexpected max response size: %d", cfg.MaxResponseSize)
	}
	if cfg.RequestTimeout != 30*time.Second {
		t.Fatalf("unexpected request timeout: %s", cfg.RequestTimeout)
	}
	if cfg.MaxRedirects != 3 {
		t.Fatalf("unexpected max redirects: %d", cfg.MaxRedirects)
	}
	if cfg.MaxImageSize != 4096 {
		t.Fatalf("unexpected max image size: %d", cfg.MaxImageSize)
	}
}

func TestFromEnvMissingRequired(t *testing.T) {
	required := []string{
		"USERS_GRPC_TARGET",
		"FILES_GRPC_TARGET",
	}

	for _, missing := range required {
		t.Run(missing, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv(missing, "")
			if _, err := FromEnv(); err == nil {
				t.Fatalf("expected error for missing %s", missing)
			}
		})
	}
}

func TestFromEnvAllowsEmptyOIDC(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("OIDC_ISSUER_URL", "")
	t.Setenv("OIDC_CLIENT_ID", "")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("expected empty OIDC config to be valid, got %v", err)
	}
	if cfg.OIDCIssuerURL != "" || cfg.OIDCClientID != "" {
		t.Fatalf("expected empty OIDC config, got issuer=%q client=%q", cfg.OIDCIssuerURL, cfg.OIDCClientID)
	}
}

func TestFromEnvRequiresClientIDWhenOIDCEnabled(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("OIDC_CLIENT_ID", "")

	_, err := FromEnv()
	if err == nil || err.Error() != "OIDC_CLIENT_ID must be set when OIDC_ISSUER_URL is set" {
		t.Fatalf("expected client id error, got %v", err)
	}
}

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("OIDC_ISSUER_URL", "https://issuer.example.com")
	t.Setenv("OIDC_CLIENT_ID", "client-id")
	t.Setenv("USERS_GRPC_TARGET", "users:50051")
	t.Setenv("FILES_GRPC_TARGET", "files:50051")
}
