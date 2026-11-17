package feishu

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"math"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
)

const documentCoverSourceMaxPixels = 8 * 1024 * 1024

func normalizeNativeDocumentCover(
	content []byte,
	contentType string,
	offsetX float64,
	offsetY float64,
) (DocumentCover, error) {
	configuration, format, err := image.DecodeConfig(bytes.NewReader(content))
	if err != nil || !documentCoverImageFormatMatches(contentType, format) ||
		validateDocumentCoverSourceDimensions(configuration.Width, configuration.Height) != nil {
		return DocumentCover{}, fmt.Errorf("document cover image dimensions are invalid")
	}
	source, decodedFormat, err := image.Decode(bytes.NewReader(content))
	if err != nil || decodedFormat != format ||
		validateDocumentCoverSourceDimensions(source.Bounds().Dx(), source.Bounds().Dy()) != nil {
		return DocumentCover{}, fmt.Errorf("decode document cover image failed")
	}
	crop := documentCoverCrop(source.Bounds(), offsetX, offsetY)
	target := image.NewRGBA(image.Rect(0, 0, documentCoverRenderWidth, documentCoverRenderHeight))
	xdraw.ApproxBiLinear.Scale(target, target.Bounds(), source, crop, xdraw.Src, nil)
	var output bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&output, target); err != nil {
		return DocumentCover{}, fmt.Errorf("encode normalized document cover: %w", err)
	}
	if err := validateRenderedDocumentCover(output.Bytes()); err != nil {
		return DocumentCover{}, err
	}
	return DocumentCover{Content: output.Bytes(), ContentType: "image/png"}, nil
}

func validateDocumentCoverSourceDimensions(width int, height int) error {
	if width <= 0 || height <= 0 || width > documentCoverRenderMaxHeight ||
		height > documentCoverRenderMaxHeight ||
		int64(width)*int64(height) > documentCoverSourceMaxPixels {
		return fmt.Errorf("document cover source exceeds pixel limit")
	}
	return nil
}

func documentCoverImageFormatMatches(contentType string, format string) bool {
	want := map[string]string{
		"image/gif": "gif", "image/jpeg": "jpeg", "image/png": "png", "image/webp": "webp",
	}
	return want[contentType] == format
}

func documentCoverCrop(bounds image.Rectangle, offsetX float64, offsetY float64) image.Rectangle {
	width, height := bounds.Dx(), bounds.Dy()
	if int64(width)*documentCoverRenderHeight > int64(height)*documentCoverRenderWidth {
		cropWidth := int(int64(height) * documentCoverRenderWidth / documentCoverRenderHeight)
		left := int(math.Round(float64(width-cropWidth) * (0.5 + offsetX)))
		return image.Rect(bounds.Min.X+left, bounds.Min.Y, bounds.Min.X+left+cropWidth, bounds.Max.Y)
	}
	cropHeight := max(1, int(int64(width)*documentCoverRenderHeight/documentCoverRenderWidth))
	top := int(math.Round(float64(height-cropHeight) * (0.5 - offsetY)))
	return image.Rect(bounds.Min.X, bounds.Min.Y+top, bounds.Max.X, bounds.Min.Y+top+cropHeight)
}
