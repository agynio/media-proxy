package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/agynio/media-proxy/internal/resize"
	"github.com/agynio/media-proxy/internal/ssrf"
)

func (h *Handler) handleExternal(ctx context.Context, w http.ResponseWriter, target *url.URL, options requestOptions) error {
	requestCtx, cancel := context.WithTimeout(ctx, h.cfg.RequestTimeout)
	defer cancel()

	resp, contentType, err := h.fetchAndValidateExternal(requestCtx, target, options.rangeHeader)
	if err != nil {
		return err
	}

	if options.size != nil && strings.HasPrefix(contentType, "image/") && resp.StatusCode == http.StatusPartialContent && options.rangeHeader != "" {
		resp.Body.Close()
		resp, contentType, err = h.fetchAndValidateExternal(requestCtx, target, "")
		if err != nil {
			return err
		}
	}

	defer resp.Body.Close()

	if options.size != nil && strings.HasPrefix(contentType, "image/") {
		return h.writeResizedImage(w, resp, contentType, *options.size, options.rangeHeader)
	}

	return h.streamExternalResponse(w, resp, contentType)
}

func (h *Handler) fetchAndValidateExternal(ctx context.Context, target *url.URL, rangeHeader string) (*http.Response, string, error) {
	resp, err := h.fetchExternal(ctx, target, rangeHeader)
	if err != nil {
		return nil, "", mapExternalError(err)
	}
	if err := checkExternalStatus(resp.StatusCode); err != nil {
		resp.Body.Close()
		return nil, "", err
	}
	contentType, err := normalizeContentType(resp.Header.Get("Content-Type"))
	if err != nil {
		resp.Body.Close()
		return nil, "", responseError{Status: http.StatusUnsupportedMediaType, Err: err}
	}
	if !isAllowedExternalContentType(contentType) {
		resp.Body.Close()
		return nil, "", responseError{Status: http.StatusUnsupportedMediaType}
	}
	return resp, contentType, nil
}

func (h *Handler) fetchExternal(ctx context.Context, target *url.URL, rangeHeader string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	if rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}
	return h.httpClient.Do(req)
}

func checkExternalStatus(statusCode int) error {
	if statusCode == http.StatusNotFound {
		return responseError{Status: http.StatusNotFound}
	}
	if statusCode >= http.StatusInternalServerError {
		return responseError{Status: http.StatusBadGateway}
	}
	if statusCode >= http.StatusBadRequest {
		return responseError{Status: statusCode}
	}
	return nil
}

func (h *Handler) streamExternalResponse(w http.ResponseWriter, resp *http.Response, contentType string) error {
	if h.cfg.MaxResponseSize > 0 && resp.ContentLength > h.cfg.MaxResponseSize {
		return responseError{Status: http.StatusRequestEntityTooLarge}
	}

	setProxyHeaders(w, contentType)
	if resp.StatusCode == http.StatusPartialContent {
		w.Header().Set("Accept-Ranges", "bytes")
		if contentRange := resp.Header.Get("Content-Range"); contentRange != "" {
			w.Header().Set("Content-Range", contentRange)
		}
	}

	if resp.ContentLength >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}

	if resp.ContentLength < 0 && h.cfg.MaxResponseSize > 0 {
		data, err := readWithLimit(resp.Body, h.cfg.MaxResponseSize)
		if err != nil {
			if errors.Is(err, errResponseTooLarge) {
				return responseError{Status: http.StatusRequestEntityTooLarge}
			}
			return err
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(data)
		return nil
	}

	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
	return nil
}

func (h *Handler) writeResizedImage(w http.ResponseWriter, resp *http.Response, contentType string, size int, rangeHeader string) error {
	if h.cfg.MaxResponseSize > 0 && resp.ContentLength > h.cfg.MaxResponseSize {
		return responseError{Status: http.StatusRequestEntityTooLarge}
	}

	body, err := readWithLimit(resp.Body, h.cfg.MaxResponseSize)
	if err != nil {
		if errors.Is(err, errResponseTooLarge) {
			return responseError{Status: http.StatusRequestEntityTooLarge}
		}
		return err
	}

	resized, err := resize.Image(body, contentType, size)
	if err != nil {
		return responseError{Status: http.StatusBadGateway, Err: err}
	}

	rangeResult, err := applyRange(resized.Bytes, rangeHeader)
	if err != nil {
		var rangeErr rangeError
		if errors.As(err, &rangeErr) {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", rangeErr.size))
			return responseError{Status: http.StatusRequestedRangeNotSatisfiable, Err: err}
		}
		return err
	}

	setProxyHeaders(w, resized.ContentType)
	if rangeResult.status == http.StatusPartialContent {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Range", rangeResult.contentRange)
	}
	contentLength := len(rangeResult.data)
	w.Header().Set("Content-Length", strconv.Itoa(contentLength))
	w.WriteHeader(rangeResult.status)
	_, _ = w.Write(rangeResult.data)
	return nil
}

func isAllowedExternalContentType(contentType string) bool {
	return strings.HasPrefix(contentType, "image/") || strings.HasPrefix(contentType, "video/") || strings.HasPrefix(contentType, "audio/")
}

func mapExternalError(err error) error {
	if ssrf.IsDeniedError(err) {
		return responseError{Status: http.StatusUnprocessableEntity, Err: err}
	}
	if errors.Is(err, errTooManyRedirects) || errors.Is(err, errInvalidRedirectURL) {
		return responseError{Status: http.StatusBadGateway, Err: err}
	}
	if isTimeoutError(err) {
		return responseError{Status: http.StatusGatewayTimeout, Err: err}
	}
	return responseError{Status: http.StatusBadGateway, Err: err}
}

func isTimeoutError(err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	return false
}
