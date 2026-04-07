package proxy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	authorizationv1 "github.com/agynio/media-proxy/.gen/go/agynio/api/authorization/v1"
	filesv1 "github.com/agynio/media-proxy/.gen/go/agynio/api/files/v1"
	"github.com/agynio/media-proxy/internal/identity"
	"github.com/agynio/media-proxy/internal/resize"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (h *Handler) handleFile(ctx context.Context, w http.ResponseWriter, fileID string, options requestOptions) error {
	identityCtx := identity.WithIdentity(ctx, options.identity)

	if err := h.checkAuthorization(identityCtx, options.identity, fileID); err != nil {
		return err
	}

	metadata, err := h.files.GetFileMetadata(identityCtx, &filesv1.GetFileMetadataRequest{FileId: fileID})
	if err != nil {
		return mapFilesError(err)
	}
	fileInfo := metadata.GetFile()
	if fileInfo == nil {
		return responseError{Status: http.StatusBadGateway, Err: fmt.Errorf("missing file metadata")}
	}
	contentType := strings.TrimSpace(fileInfo.GetContentType())
	if contentType == "" {
		return responseError{Status: http.StatusBadGateway, Err: fmt.Errorf("content type missing")}
	}
	mediaType, err := normalizeContentType(contentType)
	if err != nil {
		return responseError{Status: http.StatusBadGateway, Err: err}
	}
	if h.cfg.MaxResponseSize > 0 && fileInfo.GetSizeBytes() > h.cfg.MaxResponseSize {
		return responseError{Status: http.StatusRequestEntityTooLarge}
	}

	content, err := h.fetchFileContent(identityCtx, fileID)
	if err != nil {
		if errors.Is(err, errResponseTooLarge) {
			return responseError{Status: http.StatusRequestEntityTooLarge}
		}
		return mapFilesError(err)
	}

	output := resize.Result{Bytes: content, ContentType: contentType}
	if options.size != nil && strings.HasPrefix(mediaType, "image/") {
		resized, err := resize.Image(content, mediaType, *options.size)
		if err != nil {
			return responseError{Status: http.StatusBadGateway, Err: err}
		}
		output = resized
	}

	rangeResult, err := applyRange(output.Bytes, options.rangeHeader)
	if err != nil {
		var rangeErr rangeError
		if errors.As(err, &rangeErr) {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", rangeErr.size))
			return responseError{Status: http.StatusRequestedRangeNotSatisfiable, Err: err}
		}
		return err
	}

	setProxyHeaders(w, output.ContentType)
	if rangeResult.status == http.StatusPartialContent {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Range", rangeResult.contentRange)
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(rangeResult.data)))
	w.WriteHeader(rangeResult.status)
	_, _ = w.Write(rangeResult.data)
	return nil
}

func (h *Handler) checkAuthorization(ctx context.Context, resolved identity.ResolvedIdentity, fileID string) error {
	user := fmt.Sprintf("identity:%s", resolved.IdentityID)
	object := fmt.Sprintf("file:%s", fileID)

	resp, err := h.authz.Check(ctx, &authorizationv1.CheckRequest{
		TupleKey: &authorizationv1.TupleKey{
			User:     user,
			Relation: "can_read",
			Object:   object,
		},
	})
	if err != nil {
		return responseError{Status: http.StatusBadGateway, Err: err}
	}
	if resp == nil {
		return responseError{Status: http.StatusBadGateway, Err: fmt.Errorf("authorization response missing")}
	}
	if !resp.GetAllowed() {
		return responseError{Status: http.StatusForbidden}
	}
	return nil
}

func (h *Handler) fetchFileContent(ctx context.Context, fileID string) ([]byte, error) {
	stream, err := h.files.GetFileContent(ctx, &filesv1.GetFileContentRequest{FileId: fileID})
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	for {
		resp, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		chunk := resp.GetChunkData()
		if len(chunk) == 0 {
			continue
		}
		if h.cfg.MaxResponseSize > 0 && int64(buf.Len()+len(chunk)) > h.cfg.MaxResponseSize {
			return nil, errResponseTooLarge
		}
		if _, err := buf.Write(chunk); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

func mapFilesError(err error) error {
	if status.Code(err) == codes.NotFound {
		return responseError{Status: http.StatusNotFound}
	}
	if status.Code(err) == codes.InvalidArgument {
		return responseError{Status: http.StatusBadRequest}
	}
	return responseError{Status: http.StatusBadGateway, Err: err}
}
