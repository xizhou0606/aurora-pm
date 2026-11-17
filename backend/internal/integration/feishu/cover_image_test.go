package feishu

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"testing"
)

func TestNormalizeNativeDocumentCoverMakesAnimatedImageStaticAndBounded(t *testing.T) {
	palette := color.Palette{color.RGBA{R: 255, A: 255}, color.RGBA{G: 255, A: 255}}
	first := image.NewPaletted(image.Rect(0, 0, 4, 2), palette)
	second := image.NewPaletted(image.Rect(0, 0, 4, 2), palette)
	for index := range second.Pix {
		second.Pix[index] = 1
	}
	var animated bytes.Buffer
	if err := gif.EncodeAll(&animated, &gif.GIF{Image: []*image.Paletted{first, second}, Delay: []int{1, 1}}); err != nil {
		t.Fatal(err)
	}
	cover, err := normalizeNativeDocumentCover(animated.Bytes(), "image/gif", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	configuration, err := png.DecodeConfig(bytes.NewReader(cover.Content))
	if err != nil || cover.ContentType != "image/png" ||
		configuration.Width != documentCoverRenderWidth || configuration.Height != documentCoverRenderHeight {
		t.Fatalf("normalized cover type=%q config=%+v err=%v", cover.ContentType, configuration, err)
	}
	decoded, err := png.Decode(bytes.NewReader(cover.Content))
	if err != nil {
		t.Fatal(err)
	}
	r, g, _, _ := decoded.At(640, 320).RGBA()
	if r <= g {
		t.Fatalf("normalized cover did not preserve the first static frame: red=%d green=%d", r, g)
	}
}

func TestValidateDocumentCoverSourceDimensionsRejectsPixelAndDimensionBombs(t *testing.T) {
	for _, dimensions := range [][2]int{{0, 10}, {4097, 1}, {3000, 3000}} {
		if err := validateDocumentCoverSourceDimensions(dimensions[0], dimensions[1]); err == nil {
			t.Fatalf("dimensions %v unexpectedly accepted", dimensions)
		}
	}
	if err := validateDocumentCoverSourceDimensions(1920, 1080); err != nil {
		t.Fatalf("ordinary cover rejected: %v", err)
	}
}
