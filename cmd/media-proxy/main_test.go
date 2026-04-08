package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agynio/media-proxy/internal/config"
)

func TestCORSPreflightAllowsCredentials(t *testing.T) {
	cfg := config.Config{CORSAllowedOrigin: "https://example.test"}
	handler := newCORSHandler(cfg, http.NewServeMux())

	req := httptest.NewRequest(http.MethodOptions, "/proxy", nil)
	req.Header.Set("Origin", cfg.CORSAllowedOrigin)
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	resp := recorder.Result()
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("failed to close response body: %v", err)
	}

	if value := resp.Header.Get("Access-Control-Allow-Credentials"); value != "true" {
		t.Fatalf("expected Access-Control-Allow-Credentials true, got %q", value)
	}
}
