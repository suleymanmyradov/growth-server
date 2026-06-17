package imageproc

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"

	"github.com/disintegration/imaging"
)

const (
	// ArticleCoverMaxWidth is the maximum width for article cover images.
	ArticleCoverMaxWidth = 1200
	// ArticleCoverMaxHeight is the maximum height for article cover images.
	ArticleCoverMaxHeight = 630
	// ArticleCoverQuality is the JPEG quality for article cover images (1-100).
	ArticleCoverQuality = 85
)

// ProcessArticleCover takes an image reader, decodes it, resizes it to fit
// within ArticleCoverMaxWidth x ArticleCoverMaxHeight while preserving aspect
// ratio, and returns the bytes as a JPEG.
func ProcessArticleCover(r io.Reader) ([]byte, string, error) {
	src, format, err := image.Decode(r)
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Only resize if the image is larger than the max dimensions.
	if width > ArticleCoverMaxWidth || height > ArticleCoverMaxHeight {
		src = imaging.Fit(src, ArticleCoverMaxWidth, ArticleCoverMaxHeight, imaging.Lanczos)
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: ArticleCoverQuality}); err != nil {
		return nil, "", fmt.Errorf("encode jpeg: %w", err)
	}

	// Return processed bytes, content type, and any decode error.
	_ = format // could be used for logging if needed
	return buf.Bytes(), "image/jpeg", nil
}
