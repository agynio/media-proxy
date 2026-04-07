package proxy

import (
	"errors"
	"net/http"
	"testing"
)

func TestApplyRangeFullContent(t *testing.T) {
	data := []byte("hello world")

	resp, err := applyRange(data, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.status)
	}
	if string(resp.data) != string(data) {
		t.Fatalf("expected full content")
	}
}

func TestApplyRangePartialContent(t *testing.T) {
	data := []byte("hello world")

	resp, err := applyRange(data, "bytes=0-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.status != http.StatusPartialContent {
		t.Fatalf("expected status 206, got %d", resp.status)
	}
	if string(resp.data) != "he" {
		t.Fatalf("expected range data 'he', got %q", string(resp.data))
	}
	if resp.contentRange != "bytes 0-1/11" {
		t.Fatalf("unexpected content range: %s", resp.contentRange)
	}
}

func TestApplyRangeOpenEnded(t *testing.T) {
	data := []byte("hello world")

	resp, err := applyRange(data, "bytes=0-")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.status != http.StatusPartialContent {
		t.Fatalf("expected status 206, got %d", resp.status)
	}
	if string(resp.data) != string(data) {
		t.Fatalf("expected full content")
	}
	if resp.contentRange != "bytes 0-10/11" {
		t.Fatalf("unexpected content range: %s", resp.contentRange)
	}
}

func TestApplyRangeSuffix(t *testing.T) {
	data := []byte("hello world")

	resp, err := applyRange(data, "bytes=-5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.status != http.StatusPartialContent {
		t.Fatalf("expected status 206, got %d", resp.status)
	}
	if string(resp.data) != "world" {
		t.Fatalf("expected range data 'world', got %q", string(resp.data))
	}
	if resp.contentRange != "bytes 6-10/11" {
		t.Fatalf("unexpected content range: %s", resp.contentRange)
	}
}

func TestApplyRangeInvalid(t *testing.T) {
	data := []byte("hello world")

	invalidRanges := []string{"bytes=20-30", "bytes=0-1,2-3"}
	for _, header := range invalidRanges {
		t.Run(header, func(t *testing.T) {
			_, err := applyRange(data, header)
			if err == nil {
				t.Fatalf("expected error for %s", header)
			}
			var rerr rangeError
			if !errors.As(err, &rerr) {
				t.Fatalf("expected rangeError for %s", header)
			}
			if rerr.size != int64(len(data)) {
				t.Fatalf("expected size %d, got %d", len(data), rerr.size)
			}
		})
	}
}
