package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultListenAddr        = ":8080"
	defaultCORSAllowedOrigin = "https://agyn.dev"
	defaultMaxResponseSize   = int64(50 * 1024 * 1024)
	defaultRequestTimeout    = 30 * time.Second
	defaultMaxRedirects      = 3
	defaultMaxImageSize      = 4096
)

type Config struct {
	ListenAddr        string
	OIDCIssuerURL     string
	OIDCClientID      string
	UsersGRPCTarget   string
	FilesGRPCTarget   string
	CORSAllowedOrigin string
	MaxResponseSize   int64
	RequestTimeout    time.Duration
	MaxRedirects      int
	MaxImageSize      int
}

func FromEnv() (Config, error) {
	cfg := Config{}

	cfg.ListenAddr = strings.TrimSpace(os.Getenv("LISTEN_ADDR"))
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	}

	var err error
	cfg.OIDCIssuerURL, err = requiredEnv("OIDC_ISSUER_URL")
	if err != nil {
		return Config{}, err
	}
	cfg.OIDCClientID, err = requiredEnv("OIDC_CLIENT_ID")
	if err != nil {
		return Config{}, err
	}
	cfg.UsersGRPCTarget, err = requiredEnv("USERS_GRPC_TARGET")
	if err != nil {
		return Config{}, err
	}
	cfg.FilesGRPCTarget, err = requiredEnv("FILES_GRPC_TARGET")
	if err != nil {
		return Config{}, err
	}

	cfg.CORSAllowedOrigin = strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGIN"))
	if cfg.CORSAllowedOrigin == "" {
		cfg.CORSAllowedOrigin = defaultCORSAllowedOrigin
	}

	maxResponseSize := strings.TrimSpace(os.Getenv("MAX_RESPONSE_SIZE"))
	if maxResponseSize == "" {
		cfg.MaxResponseSize = defaultMaxResponseSize
	} else {
		parsed, err := strconv.ParseInt(maxResponseSize, 10, 64)
		if err != nil {
			return Config{}, fmt.Errorf("MAX_RESPONSE_SIZE must be an integer")
		}
		if parsed <= 0 {
			return Config{}, fmt.Errorf("MAX_RESPONSE_SIZE must be positive")
		}
		cfg.MaxResponseSize = parsed
	}

	requestTimeout := strings.TrimSpace(os.Getenv("REQUEST_TIMEOUT"))
	if requestTimeout == "" {
		cfg.RequestTimeout = defaultRequestTimeout
	} else {
		parsed, err := time.ParseDuration(requestTimeout)
		if err != nil {
			return Config{}, fmt.Errorf("REQUEST_TIMEOUT must be a valid duration")
		}
		if parsed <= 0 {
			return Config{}, fmt.Errorf("REQUEST_TIMEOUT must be positive")
		}
		cfg.RequestTimeout = parsed
	}

	maxRedirects := strings.TrimSpace(os.Getenv("MAX_REDIRECTS"))
	if maxRedirects == "" {
		cfg.MaxRedirects = defaultMaxRedirects
	} else {
		parsed, err := strconv.Atoi(maxRedirects)
		if err != nil {
			return Config{}, fmt.Errorf("MAX_REDIRECTS must be an integer")
		}
		if parsed <= 0 {
			return Config{}, fmt.Errorf("MAX_REDIRECTS must be positive")
		}
		cfg.MaxRedirects = parsed
	}

	maxImageSize := strings.TrimSpace(os.Getenv("MAX_IMAGE_SIZE"))
	if maxImageSize == "" {
		cfg.MaxImageSize = defaultMaxImageSize
	} else {
		parsed, err := strconv.Atoi(maxImageSize)
		if err != nil {
			return Config{}, fmt.Errorf("MAX_IMAGE_SIZE must be an integer")
		}
		if parsed <= 0 {
			return Config{}, fmt.Errorf("MAX_IMAGE_SIZE must be positive")
		}
		cfg.MaxImageSize = parsed
	}

	return cfg, nil
}

func requiredEnv(name string) (string, error) {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return "", fmt.Errorf("%s must be set", name)
	}
	return value, nil
}
