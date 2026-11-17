package designreview

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	xdraw "golang.org/x/image/draw"
)

type ImageComparison struct {
	PNG             []byte
	DifferenceRatio float64
}

func NormalizePNG(raw []byte) ([]byte, image.Config, error) {
	decoded, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, image.Config{}, fmt.Errorf("decode evidence: %w", err)
	}
	bounds := decoded.Bounds()
	if bounds.Dx() < 1 || bounds.Dy() < 1 || bounds.Dx() > 7680 || bounds.Dy() > 7680 {
		return nil, image.Config{}, ErrInvalid
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	draw.Draw(canvas, canvas.Bounds(), decoded, bounds.Min, draw.Src)
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		return nil, image.Config{}, err
	}
	return output.Bytes(), image.Config{ColorModel: color.NRGBAModel, Width: bounds.Dx(), Height: bounds.Dy()}, nil
}

func NormalizePNGTo(raw []byte, width, height int) ([]byte, image.Config, error) {
	if width < 1 || height < 1 || width > 7680 || height > 7680 {
		return nil, image.Config{}, ErrInvalid
	}
	normalized, config, err := NormalizePNG(raw)
	if err != nil || config.Width == width && config.Height == height {
		return normalized, config, err
	}
	decoded, err := png.Decode(bytes.NewReader(normalized))
	if err != nil {
		return nil, image.Config{}, fmt.Errorf("decode normalized evidence: %w", err)
	}
	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))
	xdraw.CatmullRom.Scale(canvas, canvas.Bounds(), decoded, decoded.Bounds(), xdraw.Src, nil)
	output, err := EncodePNG(canvas)
	if err != nil {
		return nil, image.Config{}, err
	}
	return output, image.Config{ColorModel: color.NRGBAModel, Width: width, Height: height}, nil
}

func CropNormalized(source image.Image, region Region) (image.Image, error) {
	if source == nil || !region.Valid() {
		return nil, ErrInvalid
	}
	bounds := source.Bounds()
	x0 := bounds.Min.X + int(math.Floor(region.X*float64(bounds.Dx())))
	y0 := bounds.Min.Y + int(math.Floor(region.Y*float64(bounds.Dy())))
	x1 := bounds.Min.X + int(math.Ceil((region.X+region.Width)*float64(bounds.Dx())))
	y1 := bounds.Min.Y + int(math.Ceil((region.Y+region.Height)*float64(bounds.Dy())))
	if x1-x0 < 16 || y1-y0 < 16 {
		return nil, ErrInvalid
	}
	rect := image.Rect(0, 0, x1-x0, y1-y0)
	result := image.NewNRGBA(rect)
	draw.Draw(result, rect, source, image.Pt(x0, y0), draw.Src)
	return result, nil
}

func ComposeIOSTopbar(source image.Image, background color.NRGBA, darkIcons bool) (image.Image, error) {
	if source == nil || source.Bounds().Dx() < 240 {
		return nil, ErrInvalid
	}
	const logicalHeight = 44
	bounds := source.Bounds()
	result := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()+logicalHeight))
	draw.Draw(result, image.Rect(0, 0, bounds.Dx(), logicalHeight), &image.Uniform{C: background}, image.Point{}, draw.Src)
	draw.Draw(result, image.Rect(0, logicalHeight, bounds.Dx(), result.Bounds().Dy()), source, bounds.Min, draw.Src)
	ink := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	if darkIcons {
		ink = color.NRGBA{A: 255}
	}
	// Compact deterministic stand-ins for signal, Wi-Fi and battery glyphs.
	for index := 0; index < 4; index++ {
		x := bounds.Dx() - 72 + index*4
		draw.Draw(result, image.Rect(x, 29-index*3, x+2, 32), &image.Uniform{C: ink}, image.Point{}, draw.Src)
	}
	draw.Draw(result, image.Rect(bounds.Dx()-48, 22, bounds.Dx()-32, 30), &image.Uniform{C: ink}, image.Point{}, draw.Src)
	draw.Draw(result, image.Rect(bounds.Dx()-27, 20, bounds.Dx()-8, 31), &image.Uniform{C: ink}, image.Point{}, draw.Src)
	return result, nil
}

func EncodePNG(source image.Image) ([]byte, error) {
	if source == nil {
		return nil, ErrInvalid
	}
	var output bytes.Buffer
	if err := png.Encode(&output, source); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func Heatmap(design, implementation image.Image, threshold uint8) (ImageComparison, error) {
	if design == nil || implementation == nil || design.Bounds().Size() != implementation.Bounds().Size() {
		return ImageComparison{}, ErrInvalid
	}
	bounds := design.Bounds()
	output := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	var changed int64
	total := int64(bounds.Dx() * bounds.Dy())
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			dr, dg, db, _ := design.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			ir, ig, ib, _ := implementation.At(implementation.Bounds().Min.X+x, implementation.Bounds().Min.Y+y).RGBA()
			delta := (abs32(int64(dr)-int64(ir)) + abs32(int64(dg)-int64(ig)) + abs32(int64(db)-int64(ib))) / (3 * 257)
			if delta > int64(threshold) {
				changed++
				alpha := uint8(min64(220, 70+delta))
				output.SetNRGBA(x, y, color.NRGBA{R: 239, G: 68, B: 68, A: alpha})
			} else {
				output.SetNRGBA(x, y, color.NRGBA{A: 0})
			}
		}
	}
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, output); err != nil {
		return ImageComparison{}, err
	}
	return ImageComparison{PNG: encoded.Bytes(), DifferenceRatio: float64(changed) / float64(total)}, nil
}

func abs32(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
