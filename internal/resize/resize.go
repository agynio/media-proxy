package resize

import (
	"bytes"
	"image"
	"mime"
	"strings"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp"
)

type Result struct {
	Bytes       []byte
	ContentType string
}

func Image(data []byte, contentType string, maxSize int) (Result, error) {
	if maxSize <= 0 {
		return Result{Bytes: data, ContentType: contentType}, nil
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return Result{Bytes: data, ContentType: contentType}, nil
	}

	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return Result{Bytes: data, ContentType: contentType}, nil
	}

	bounds := decoded.Bounds()
	if bounds.Dx() <= maxSize && bounds.Dy() <= maxSize {
		return Result{Bytes: data, ContentType: contentType}, nil
	}

	resized := imaging.Fit(decoded, maxSize, maxSize, imaging.CatmullRom)
	format, outputType := outputFormat(strings.ToLower(mediaType))

	var buf bytes.Buffer
	if err := imaging.Encode(&buf, resized, format); err != nil {
		return Result{}, err
	}

	return Result{Bytes: buf.Bytes(), ContentType: outputType}, nil
}

func outputFormat(contentType string) (imaging.Format, string) {
	switch contentType {
	case "image/jpeg", "image/jpg":
		return imaging.JPEG, "image/jpeg"
	case "image/png", "image/gif":
		return imaging.PNG, "image/png"
	default:
		// imaging does not encode webp; fall back to png for other formats.
		return imaging.PNG, "image/png"
	}
}
