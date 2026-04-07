package ssrf

import (
	"net"
	"strings"
	"testing"
)

func TestDefaultCheckerDeniedIPs(t *testing.T) {
	checker, err := DefaultChecker()
	if err != nil {
		t.Fatalf("failed to build checker: %v", err)
	}

	testCases := []string{
		"127.0.0.1",
		"10.10.10.10",
		"172.16.0.5",
		"192.168.1.1",
		"169.254.1.1",
		"::1",
		"fe80::1",
		"fc00::1",
		"::ffff:127.0.0.1",
	}

	for _, value := range testCases {
		t.Run(value, func(t *testing.T) {
			err := checker.Control("tcp", addressForIP(value), nil)
			if err == nil {
				t.Fatalf("expected %s to be denied", value)
			}
			if !IsDeniedError(err) {
				t.Fatalf("expected denied error for %s, got %v", value, err)
			}
		})
	}
}

func TestDefaultCheckerAllowedIPs(t *testing.T) {
	checker, err := DefaultChecker()
	if err != nil {
		t.Fatalf("failed to build checker: %v", err)
	}

	testCases := []string{
		"8.8.8.8",
		"1.1.1.1",
		"2001:4860:4860::8888",
		"::ffff:8.8.8.8",
	}

	for _, value := range testCases {
		t.Run(value, func(t *testing.T) {
			err := checker.Control("tcp", addressForIP(value), nil)
			if err != nil {
				t.Fatalf("expected %s to be allowed, got %v", value, err)
			}
		})
	}
}

func TestCheckerEdgeCases(t *testing.T) {
	checker, err := DefaultChecker()
	if err != nil {
		t.Fatalf("failed to build checker: %v", err)
	}

	if !checker.Denied(nil) {
		t.Fatalf("expected nil IP to be denied")
	}

	ip := net.ParseIP("")
	if ip != nil {
		t.Fatalf("expected empty parse to be nil")
	}
}

func addressForIP(value string) string {
	if strings.Contains(value, ":") {
		return "[" + value + "]:80"
	}
	return value + ":80"
}
