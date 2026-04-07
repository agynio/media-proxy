package proxy

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type byteRange struct {
	start int64
	end   int64
}

type rangeResponse struct {
	data         []byte
	status       int
	contentRange string
}

type rangeError struct {
	size int64
}

func (e rangeError) Error() string {
	return "invalid range"
}

func applyRange(data []byte, header string) (rangeResponse, error) {
	trimmed := strings.TrimSpace(header)
	if trimmed == "" {
		return rangeResponse{data: data, status: http.StatusOK}, nil
	}
	size := int64(len(data))
	if size == 0 {
		return rangeResponse{}, rangeError{size: size}
	}
	rangeValue, err := parseRange(trimmed, size)
	if err != nil {
		return rangeResponse{}, rangeError{size: size}
	}

	clampedEnd := rangeValue.end
	if clampedEnd >= size {
		clampedEnd = size - 1
	}
	if rangeValue.start < 0 || rangeValue.start >= size || clampedEnd < rangeValue.start {
		return rangeResponse{}, rangeError{size: size}
	}

	return rangeResponse{
		data:         data[rangeValue.start : clampedEnd+1],
		status:       http.StatusPartialContent,
		contentRange: fmt.Sprintf("bytes %d-%d/%d", rangeValue.start, clampedEnd, size),
	}, nil
}

func parseRange(header string, size int64) (byteRange, error) {
	if !strings.HasPrefix(header, "bytes=") {
		return byteRange{}, fmt.Errorf("invalid range unit")
	}
	value := strings.TrimPrefix(header, "bytes=")
	if strings.Contains(value, ",") {
		return byteRange{}, fmt.Errorf("multiple ranges are not supported")
	}
	parts := strings.SplitN(value, "-", 2)
	if len(parts) != 2 {
		return byteRange{}, fmt.Errorf("invalid range format")
	}

	startPart := strings.TrimSpace(parts[0])
	endPart := strings.TrimSpace(parts[1])

	if startPart == "" {
		suffix, err := strconv.ParseInt(endPart, 10, 64)
		if err != nil || suffix <= 0 {
			return byteRange{}, fmt.Errorf("invalid suffix range")
		}
		if suffix > size {
			suffix = size
		}
		return byteRange{start: size - suffix, end: size - 1}, nil
	}

	start, err := strconv.ParseInt(startPart, 10, 64)
	if err != nil || start < 0 {
		return byteRange{}, fmt.Errorf("invalid range start")
	}
	end := size - 1
	if endPart != "" {
		parsedEnd, err := strconv.ParseInt(endPart, 10, 64)
		if err != nil || parsedEnd < 0 {
			return byteRange{}, fmt.Errorf("invalid range end")
		}
		end = parsedEnd
	}
	return byteRange{start: start, end: end}, nil
}
