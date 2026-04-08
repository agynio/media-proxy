//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	filesv1 "github.com/agynio/media-proxy/.gen/go/agynio/api/files/v1"
	"github.com/agynio/media-proxy/internal/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	mockAuthTokenURL     = "https://mockauth.dev/r/301ebb13-15a8-48f4-baac-e3fa25be29fc/oidc/token"
	mockAuthClientID     = "client_MU95KU3gHQf5Ir7p"
	mockAuthClientSecret = "XPKka2i9uzISrKZ95zxli8sY51BK4eTJ"
)

var (
	mediaProxyURL = envOrDefault("MEDIA_PROXY_URL", "http://media-proxy:8080")
	filesAddress  = envOrDefault("FILES_ADDRESS", "files:50051")
	gatewayURL    = envOrDefault("GATEWAY_URL", "http://gateway-gateway:8080")

	accessToken      string
	resolvedIdentity identity.ResolvedIdentity
)

type mePayload struct {
	IdentityID   string `json:"identity_id"`
	IdentityType string `json:"identity_type"`
}

func TestMain(m *testing.M) {
	cleanup := &cleanupStack{}
	ctx := context.Background()

	if err := setupCredentials(ctx, cleanup); err != nil {
		exitWithSetupError(cleanup, fmt.Errorf("setup credentials: %w", err))
	}

	exitCode := m.Run()
	cleanup.Run()
	os.Exit(exitCode)
}

type cleanupStack struct {
	fns []func()
}

func (c *cleanupStack) Add(fn func()) {
	c.fns = append(c.fns, fn)
}

func (c *cleanupStack) Run() {
	for i := len(c.fns) - 1; i >= 0; i-- {
		c.fns[i]()
	}
}

func exitWithSetupError(cleanup *cleanupStack, err error) {
	cleanup.Run()
	fmt.Fprintf(os.Stderr, "e2e setup failed: %v\n", err)
	os.Exit(1)
}

func setupCredentials(ctx context.Context, _ *cleanupStack) error {
	requestCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	token, err := requestOIDCAccessToken(requestCtx)
	if err != nil {
		return err
	}

	meCtx, meCancel := context.WithTimeout(ctx, 15*time.Second)
	defer meCancel()

	identityInfo, err := fetchIdentity(meCtx, token)
	if err != nil {
		return err
	}

	accessToken = token
	resolvedIdentity = identityInfo
	return nil
}

func requestOIDCAccessToken(ctx context.Context) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("username", "e2e-test-user")
	form.Set("scope", "openid profile email")
	form.Set("client_id", mockAuthClientID)
	form.Set("client_secret", mockAuthClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, mockAuthTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := newClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("mockauth token request failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}

	token := strings.TrimSpace(payload.AccessToken)
	if token == "" {
		return "", fmt.Errorf("mockauth access_token missing")
	}

	return token, nil
}

func fetchIdentity(ctx context.Context, token string) (identity.ResolvedIdentity, error) {
	client := newAuthenticatedClient(token)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, gatewayURL+"/me", nil)
	if err != nil {
		return identity.ResolvedIdentity{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return identity.ResolvedIdentity{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return identity.ResolvedIdentity{}, fmt.Errorf("me endpoint failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload mePayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return identity.ResolvedIdentity{}, err
	}

	identityID := strings.TrimSpace(payload.IdentityID)
	if identityID == "" {
		return identity.ResolvedIdentity{}, fmt.Errorf("identity_id missing")
	}
	identityType, err := identity.ParseIdentityType(payload.IdentityType)
	if err != nil {
		return identity.ResolvedIdentity{}, err
	}

	return identity.ResolvedIdentity{IdentityID: identityID, IdentityType: identityType}, nil
}

func proxyURL(rawURL string, size int) string {
	params := url.Values{}
	params.Set("url", strings.TrimSpace(rawURL))
	if size > 0 {
		params.Set("size", strconv.Itoa(size))
	}

	return mediaProxyURL + "/proxy?" + params.Encode()
}

func uploadTestFile(t *testing.T, ctx context.Context, filename, contentType string, data []byte) string {
	t.Helper()

	conn, err := grpc.NewClient(filesAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("dial files: %v", err)
	}
	t.Cleanup(func() {
		_ = conn.Close()
	})

	resolved := resolvedIdentity
	if resolved.IdentityID == "" {
		t.Fatal("identity id missing")
	}
	if resolved.IdentityType == "" {
		t.Fatal("identity type missing")
	}
	if len(data) == 0 {
		t.Fatal("file data is empty")
	}

	uploadCtx := identity.AppendToOutgoingContext(identity.WithIdentity(ctx, resolved))
	client := filesv1.NewFilesServiceClient(conn)
	stream, err := client.UploadFile(uploadCtx)
	if err != nil {
		t.Fatalf("start upload: %v", err)
	}

	metadata := &filesv1.UploadFileRequest{
		Payload: &filesv1.UploadFileRequest_Metadata{
			Metadata: &filesv1.UploadFileMetadata{
				Filename:    filename,
				ContentType: contentType,
				SizeBytes:   int64(len(data)),
			},
		},
	}
	if err := stream.Send(metadata); err != nil {
		t.Fatalf("send metadata: %v", err)
	}

	chunkSplit := len(data) / 2
	if chunkSplit == 0 {
		chunkSplit = len(data)
	}
	if err := stream.Send(&filesv1.UploadFileRequest{
		Payload: &filesv1.UploadFileRequest_Chunk{
			Chunk: &filesv1.UploadFileChunk{Data: data[:chunkSplit]},
		},
	}); err != nil {
		t.Fatalf("send chunk: %v", err)
	}
	if chunkSplit < len(data) {
		if err := stream.Send(&filesv1.UploadFileRequest{
			Payload: &filesv1.UploadFileRequest_Chunk{
				Chunk: &filesv1.UploadFileChunk{Data: data[chunkSplit:]},
			},
		}); err != nil {
			t.Fatalf("send chunk: %v", err)
		}
	}

	resp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("finish upload: %v", err)
	}
	if resp == nil || resp.GetFile() == nil {
		t.Fatal("missing file in upload response")
	}

	fileID := strings.TrimSpace(resp.GetFile().GetId())
	if fileID == "" {
		t.Fatal("missing file id in upload response")
	}

	return fileID
}

func decodeImage(t *testing.T, data []byte) image.Image {
	t.Helper()

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decode image: %v", err)
	}

	return img
}

func newClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second}
}

func newAuthenticatedClient(token string) *http.Client {
	client := newClient()
	client.Transport = bearerTransport{token: token, base: client.Transport}
	return client
}

type bearerTransport struct {
	token string
	base  http.RoundTripper
}

func (t bearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}

	clone := req.Clone(req.Context())
	clone.Header.Set("Authorization", "Bearer "+t.token)
	return base.RoundTrip(clone)
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
