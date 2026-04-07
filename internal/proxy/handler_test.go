package proxy

import (
	"net/url"
	"testing"
)

func TestParseSizeParam(t *testing.T) {
	value, err := parseSizeParam("120", 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value == nil || *value != 120 {
		t.Fatalf("expected size 120, got %v", value)
	}

	value, err = parseSizeParam("", 200)
	if err != nil {
		t.Fatalf("unexpected error for empty: %v", err)
	}
	if value != nil {
		t.Fatalf("expected nil size for empty input")
	}

	value, err = parseSizeParam("-5", 200)
	if err == nil {
		t.Fatalf("expected error for negative size")
	}
	if value != nil {
		t.Fatalf("expected nil size for negative input")
	}

	value, err = parseSizeParam("500", 200)
	if err != nil {
		t.Fatalf("unexpected error for max clamp: %v", err)
	}
	if value == nil || *value != 200 {
		t.Fatalf("expected size clamped to 200, got %v", value)
	}
}

func TestParseFileID(t *testing.T) {
	validURL, err := url.Parse("agyn://file/abc123")
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}
	fileID, err := parseFileID(validURL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fileID != "abc123" {
		t.Fatalf("expected file id abc123, got %s", fileID)
	}

	missingID, err := url.Parse("agyn://file/")
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}
	if _, err := parseFileID(missingID); err == nil {
		t.Fatalf("expected error for missing file id")
	}

	wrongHost, err := url.Parse("agyn://files/abc123")
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}
	if _, err := parseFileID(wrongHost); err == nil {
		t.Fatalf("expected error for wrong host")
	}

	withSlash, err := url.Parse("agyn://file/foo/bar")
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}
	if _, err := parseFileID(withSlash); err == nil {
		t.Fatalf("expected error for file id with slash")
	}
}

func TestIsAllowedExternalContentType(t *testing.T) {
	if !isAllowedExternalContentType("image/png") {
		t.Fatalf("expected image/png to be allowed")
	}
	if !isAllowedExternalContentType("video/mp4") {
		t.Fatalf("expected video/mp4 to be allowed")
	}
	if isAllowedExternalContentType("text/html") {
		t.Fatalf("expected text/html to be rejected")
	}
}
