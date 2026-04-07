package resize

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestImageResizeDownsample(t *testing.T) {
	input := encodePNG(t, 100, 50)

	result, err := Image(input, "image/png", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ContentType != "image/png" {
		t.Fatalf("unexpected content type: %s", result.ContentType)
	}
	if len(result.Bytes) == 0 {
		t.Fatalf("expected resized bytes")
	}

	decoded, _, err := image.Decode(bytes.NewReader(result.Bytes))
	if err != nil {
		t.Fatalf("failed to decode resized image: %v", err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() > 30 || bounds.Dy() > 30 {
		t.Fatalf("expected resized image to fit within 30x30, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestImageResizeNoopWhenSmaller(t *testing.T) {
	input := encodePNG(t, 10, 10)

	result, err := Image(input, "image/png", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(result.Bytes, input) {
		t.Fatalf("expected no-op resize to return original bytes")
	}
	if result.ContentType != "image/png" {
		t.Fatalf("unexpected content type: %s", result.ContentType)
	}
}

func TestImageResizeInvalidImageFallback(t *testing.T) {
	input := []byte("not-a-real-image")

	result, err := Image(input, "image/png", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(result.Bytes, input) {
		t.Fatalf("expected invalid image to return original bytes")
	}
	if result.ContentType != "image/png" {
		t.Fatalf("unexpected content type: %s", result.ContentType)
	}
}

func TestImageResizeSizeZero(t *testing.T) {
	input := encodePNG(t, 100, 50)

	result, err := Image(input, "image/png", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(result.Bytes, input) {
		t.Fatalf("expected size=0 to return original bytes")
	}
	if result.ContentType != "image/png" {
		t.Fatalf("unexpected content type: %s", result.ContentType)
	}
}

func encodePNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("failed to encode png: %v", err)
	}
	return buf.Bytes()
}
