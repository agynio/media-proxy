package proxy

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	authorizationv1 "github.com/agynio/media-proxy/.gen/go/agynio/api/authorization/v1"
	filesv1 "github.com/agynio/media-proxy/.gen/go/agynio/api/files/v1"
	"github.com/agynio/media-proxy/internal/auth"
	"github.com/agynio/media-proxy/internal/config"
	"github.com/agynio/media-proxy/internal/httpauth"
	"github.com/agynio/media-proxy/internal/identity"
	"github.com/agynio/media-proxy/internal/ssrf"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	cacheControlValue       = "private, max-age=3600, immutable"
	contentDispositionValue = "inline"
)

var (
	errResponseTooLarge   = errors.New("response exceeds max size")
	errTooManyRedirects   = errors.New("too many redirects")
	errInvalidRedirectURL = errors.New("invalid redirect url")
)

type Handler struct {
	cfg        config.Config
	resolver   *auth.Resolver
	files      filesv1.FilesServiceClient
	authz      authorizationv1.AuthorizationServiceClient
	httpClient *http.Client
}

type requestOptions struct {
	size        *int
	rangeHeader string
	identity    identity.ResolvedIdentity
}

type responseError struct {
	Status int
	Err    error
}

func (e responseError) Error() string {
	if e.Err == nil {
		return http.StatusText(e.Status)
	}
	return e.Err.Error()
}

func NewHandler(cfg config.Config, resolver *auth.Resolver, files filesv1.FilesServiceClient, authz authorizationv1.AuthorizationServiceClient) (*Handler, error) {
	if resolver == nil {
		return nil, fmt.Errorf("resolver is required")
	}
	if files == nil {
		return nil, fmt.Errorf("files client is required")
	}
	if authz == nil {
		return nil, fmt.Errorf("authorization client is required")
	}

	checker, err := ssrf.DefaultChecker()
	if err != nil {
		return nil, err
	}
	dialer := checker.NewDialer()
	transport := &http.Transport{
		Proxy:             http.ProxyFromEnvironment,
		DialContext:       dialer.DialContext,
		ForceAttemptHTTP2: true,
	}

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) > cfg.MaxRedirects {
				return errTooManyRedirects
			}
			if req.URL == nil {
				return errInvalidRedirectURL
			}
			scheme := strings.ToLower(req.URL.Scheme)
			if scheme != "http" && scheme != "https" {
				return errInvalidRedirectURL
			}
			return nil
		},
	}

	return &Handler{
		cfg:        cfg,
		resolver:   resolver,
		files:      files,
		authz:      authz,
		httpClient: client,
	}, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed)
		return
	}

	accessToken, ok := httpauth.ExtractBearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, http.StatusUnauthorized)
		return
	}

	resolved, err := h.resolver.ResolveFromToken(r.Context(), accessToken)
	if err != nil {
		statusCode := http.StatusBadGateway
		if status.Code(err) == codes.Unauthenticated {
			statusCode = http.StatusUnauthorized
		}
		writeError(w, statusCode)
		return
	}

	sizeParam, err := parseSizeParam(r.URL.Query().Get("size"), h.cfg.MaxImageSize)
	if err != nil {
		writeError(w, http.StatusBadRequest)
		return
	}

	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if rawURL == "" {
		writeError(w, http.StatusBadRequest)
		return
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		writeError(w, http.StatusBadRequest)
		return
	}
	if parsedURL.Scheme == "" {
		writeError(w, http.StatusBadRequest)
		return
	}

	options := requestOptions{
		size:        sizeParam,
		rangeHeader: strings.TrimSpace(r.Header.Get("Range")),
		identity:    resolved,
	}

	switch strings.ToLower(parsedURL.Scheme) {
	case "http", "https":
		if parsedURL.Host == "" {
			writeError(w, http.StatusBadRequest)
			return
		}
		err = h.handleExternal(r.Context(), w, parsedURL, options)
	case "agyn":
		fileID, parseErr := parseFileID(parsedURL)
		if parseErr != nil {
			writeError(w, http.StatusBadRequest)
			return
		}
		err = h.handleFile(r.Context(), w, fileID, options)
	default:
		writeError(w, http.StatusBadRequest)
		return
	}

	if err != nil {
		var respErr responseError
		if errors.As(err, &respErr) {
			writeError(w, respErr.Status)
			return
		}
		writeError(w, http.StatusBadGateway)
	}
}

func parseSizeParam(value string, max int) (*int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return nil, err
	}
	if parsed <= 0 {
		return nil, fmt.Errorf("size must be positive")
	}
	if max > 0 && parsed > max {
		parsed = max
	}
	return &parsed, nil
}

func parseFileID(target *url.URL) (string, error) {
	if target == nil {
		return "", fmt.Errorf("url is required")
	}
	if !strings.EqualFold(target.Host, "file") {
		return "", fmt.Errorf("unsupported agyn host")
	}
	path := strings.TrimPrefix(target.Path, "/")
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("file id is required")
	}
	if strings.Contains(path, "/") {
		return "", fmt.Errorf("file id must not contain slashes")
	}
	return path, nil
}

func normalizeContentType(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("content type is required")
	}
	mediaType, _, err := mime.ParseMediaType(trimmed)
	if err != nil {
		return "", err
	}
	return strings.ToLower(mediaType), nil
}

func setProxyHeaders(w http.ResponseWriter, contentType string) {
	w.Header().Set("Cache-Control", cacheControlValue)
	w.Header().Set("Content-Disposition", contentDispositionValue)
	w.Header().Set("Content-Type", contentType)
}

func readWithLimit(reader io.Reader, max int64) ([]byte, error) {
	if max <= 0 {
		return io.ReadAll(reader)
	}
	limited := &io.LimitedReader{R: reader, N: max + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, errResponseTooLarge
	}
	return data, nil
}

func writeError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}
