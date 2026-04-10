package proxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/agynio/media-proxy/internal/config"
	"github.com/agynio/media-proxy/internal/ssrf"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

func TestHandleExternalErrorMapping(t *testing.T) {
	target, err := url.Parse("http://example.test/resource")
	if err != nil {
		t.Fatalf("failed to parse url: %v", err)
	}

	testCases := []struct {
		name      string
		roundTrip roundTripFunc
		expected  int
	}{
		{
			name: "not-found",
			roundTrip: func(*http.Request) (*http.Response, error) {
				return newTestResponse(http.StatusNotFound), nil
			},
			expected: http.StatusNotFound,
		},
		{
			name: "server-error",
			roundTrip: func(*http.Request) (*http.Response, error) {
				return newTestResponse(http.StatusInternalServerError), nil
			},
			expected: http.StatusBadGateway,
		},
		{
			name: "timeout",
			roundTrip: func(*http.Request) (*http.Response, error) {
				return nil, timeoutError{}
			},
			expected: http.StatusGatewayTimeout,
		},
		{
			name: "ssrf",
			roundTrip: func(*http.Request) (*http.Response, error) {
				return nil, ssrf.DeniedError{IP: net.ParseIP("127.0.0.1")}
			},
			expected: http.StatusUnprocessableEntity,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			handler := &Handler{
				cfg: config.Config{RequestTimeout: time.Second},
				httpClient: &http.Client{
					Transport: testCase.roundTrip,
				},
			}

			recorder := httptest.NewRecorder()
			err := handler.handleExternal(context.Background(), recorder, target, requestOptions{})
			assertResponseStatus(t, err, testCase.expected)
		})
	}
}

func TestCheckExternalStatus(t *testing.T) {
	testCases := []struct {
		status     int
		expected   int
		expectsErr bool
	}{
		{status: http.StatusOK, expectsErr: false},
		{status: http.StatusNotFound, expected: http.StatusNotFound, expectsErr: true},
		{status: http.StatusBadRequest, expected: http.StatusBadRequest, expectsErr: true},
		{status: http.StatusInternalServerError, expected: http.StatusBadGateway, expectsErr: true},
	}

	for _, testCase := range testCases {
		t.Run(http.StatusText(testCase.status), func(t *testing.T) {
			err := checkExternalStatus(testCase.status)
			if testCase.expectsErr {
				assertResponseStatus(t, err, testCase.expected)
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}
}

func TestReadWithLimit(t *testing.T) {
	data, err := readWithLimit(strings.NewReader("hello"), 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected data: %s", string(data))
	}

	_, err = readWithLimit(strings.NewReader("hello"), 4)
	if err == nil {
		t.Fatalf("expected error for oversized payload")
	}
	if !errors.Is(err, errResponseTooLarge) {
		t.Fatalf("expected response too large error, got %v", err)
	}
}

func TestMapExternalError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "ssrf",
			err:      ssrf.DeniedError{IP: net.ParseIP("127.0.0.1")},
			expected: http.StatusUnprocessableEntity,
		},
		{
			name:     "timeout",
			err:      context.DeadlineExceeded,
			expected: http.StatusGatewayTimeout,
		},
		{
			name:     "redirect",
			err:      errTooManyRedirects,
			expected: http.StatusBadGateway,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assertResponseStatus(t, mapExternalError(testCase.err), testCase.expected)
		})
	}
}

func TestMapFilesError(t *testing.T) {
	testCases := []struct {
		name     string
		err      error
		expected int
	}{
		{
			name:     "not-found",
			err:      status.Error(codes.NotFound, "missing"),
			expected: http.StatusNotFound,
		},
		{
			name:     "invalid",
			err:      status.Error(codes.InvalidArgument, "bad"),
			expected: http.StatusBadRequest,
		},
		{
			name:     "permission-denied",
			err:      status.Error(codes.PermissionDenied, "denied"),
			expected: http.StatusForbidden,
		},
		{
			name:     "unauthenticated",
			err:      status.Error(codes.Unauthenticated, "missing"),
			expected: http.StatusUnauthorized,
		},
		{
			name:     "default",
			err:      status.Error(codes.Internal, "boom"),
			expected: http.StatusBadGateway,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assertResponseStatus(t, mapFilesError(testCase.err), testCase.expected)
		})
	}
}

func assertResponseStatus(t *testing.T, err error, expected int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error status %d", expected)
	}
	var respErr responseError
	if !errors.As(err, &respErr) {
		t.Fatalf("expected responseError, got %v", err)
	}
	if respErr.Status != expected {
		t.Fatalf("expected status %d, got %d", expected, respErr.Status)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type timeoutError struct{}

func (timeoutError) Error() string {
	return "timeout"
}

func (timeoutError) Timeout() bool {
	return true
}

func (timeoutError) Temporary() bool {
	return true
}

func newTestResponse(statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader("")),
		Header: http.Header{
			"Content-Type": []string{"image/png"},
		},
	}
}
