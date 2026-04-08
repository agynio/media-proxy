//go:build e2e

package e2e

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	externalImageURL = "https://www.w3.org/Graphics/PNG/nurbcup2si.png"
	externalHTMLURL  = "https://www.w3.org/"
)

func TestExternalProxy_PublicImage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyURL(externalImageURL, 0), nil)
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
	if len(body) == 0 {
		t.Fatal("expected non-empty response body")
	}
}

func TestExternalProxy_PublicImageWithResize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyURL(externalImageURL, 200), nil)
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
	if bounds.Dx() > 200 || bounds.Dy() > 200 {
		t.Fatalf("expected resized image within 200px, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestExternalProxy_NonImageURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyURL(externalHTMLURL, 0), nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	response, err := newAuthenticatedClient(accessToken).Do(request)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusUnsupportedMediaType {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("expected status 415, got %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
}

func TestExternalProxy_Unauthenticated(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyURL(externalImageURL, 0), nil)
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

func TestExternalProxy_InvalidToken(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, proxyURL(externalImageURL, 0), nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	response, err := newAuthenticatedClient("invalid").Do(request)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("expected status 401, got %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
}

func TestExternalProxy_MissingURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaProxyURL+"/proxy", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}

	response, err := newAuthenticatedClient(accessToken).Do(request)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusBadRequest {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("expected status 400, got %d: %s", response.StatusCode, strings.TrimSpace(string(body)))
	}
}
