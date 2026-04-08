//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

var testPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89,
	0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, 0x54,
	0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00, 0x05,
	0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4,
	0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44,
	0xae, 0x42, 0x60, 0x82,
}

func TestFileProxy_UploadAndProxy(t *testing.T) {
	uploadCtx, uploadCancel := context.WithTimeout(context.Background(), 30*time.Second)
	fileID := uploadTestFile(t, uploadCtx, "e2e-upload.png", "image/png", testPNG)
	uploadCancel()

	proxyCtx, proxyCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer proxyCancel()

	request, err := http.NewRequestWithContext(proxyCtx, http.MethodGet, proxyURL("agyn://file/"+fileID, 0), nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	response, err := newAuthenticatedClient(accessToken).Do(request)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("expected status 200, got %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	contentType := strings.TrimSpace(response.Header.Get("Content-Type"))
	if contentType != "image/png" {
		t.Fatalf("expected content-type image/png, got %q", contentType)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if !bytes.Equal(body, testPNG) {
		t.Fatal("proxied file did not match uploaded content")
	}
}

func TestFileProxy_UploadAndProxyWithResize(t *testing.T) {
	uploadCtx, uploadCancel := context.WithTimeout(context.Background(), 30*time.Second)
	fileID := uploadTestFile(t, uploadCtx, "e2e-resize.png", "image/png", testPNG)
	uploadCancel()

	proxyCtx, proxyCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer proxyCancel()

	request, err := http.NewRequestWithContext(proxyCtx, http.MethodGet, proxyURL("agyn://file/"+fileID, 100), nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	response, err := newAuthenticatedClient(accessToken).Do(request)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("expected status 200, got %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("expected non-empty response body")
	}

	image := decodeImage(t, body)
	bounds := image.Bounds()
	if bounds.Dx() > 100 || bounds.Dy() > 100 {
		t.Fatalf("expected resized image within 100px, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestFileProxy_NotFoundFile(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyURL("agyn://file/"+uuid.NewString(), 0), nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	response, err := newAuthenticatedClient(accessToken).Do(request)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("expected status 404, got %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
}

func TestFileProxy_RangeRequest(t *testing.T) {
	uploadCtx, uploadCancel := context.WithTimeout(context.Background(), 30*time.Second)
	fileID := uploadTestFile(t, uploadCtx, "e2e-range.png", "image/png", testPNG)
	uploadCancel()

	proxyCtx, proxyCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer proxyCancel()

	request, err := http.NewRequestWithContext(proxyCtx, http.MethodGet, proxyURL("agyn://file/"+fileID, 0), nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Set("Range", "bytes=0-10")

	response, err := newAuthenticatedClient(accessToken).Do(request)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusPartialContent {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("expected status 206, got %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}

	contentRange := strings.TrimSpace(response.Header.Get("Content-Range"))
	if !strings.HasPrefix(contentRange, "bytes 0-10/") {
		t.Fatalf("expected content-range header for bytes 0-10, got %q", contentRange)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	if len(body) != 11 {
		t.Fatalf("expected 11 bytes, got %d", len(body))
	}
}

func TestFileProxy_Unauthenticated(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyURL("agyn://file/"+uuid.NewString(), 0), nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	response, err := newClient().Do(request)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("expected status 401, got %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
}
