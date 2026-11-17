package feishu

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

type documentCoverRendererFunc func(context.Context, []byte) ([]byte, error)

func (f documentCoverRendererFunc) RenderFirstPage(ctx context.Context, content []byte) ([]byte, error) {
	return f(ctx, content)
}

type recordingDocumentCoverCommandRunner struct {
	t          *testing.T
	page       []byte
	executable string
	arguments  []string
	workDir    string
	input      []byte
}

func (r *recordingDocumentCoverCommandRunner) Run(
	ctx context.Context,
	executable string,
	arguments []string,
	workDir string,
) error {
	r.t.Helper()
	if _, ok := ctx.Deadline(); !ok {
		r.t.Fatal("render command context has no deadline")
	}
	r.executable = executable
	r.arguments = append([]string(nil), arguments...)
	r.workDir = workDir
	input, err := os.ReadFile(arguments[len(arguments)-2])
	if err != nil {
		return err
	}
	r.input = input
	return os.WriteFile(arguments[len(arguments)-1]+".png", r.page, 0o600)
}

func TestPopplerDocumentCoverRendererCommandAndTopCropContract(t *testing.T) {
	pdf := testMultiPagePDF(t)
	page := testTopBottomPNG(t, documentCoverRenderWidth, 800)
	runner := &recordingDocumentCoverCommandRunner{t: t, page: page}
	renderer := &popplerDocumentCoverRenderer{
		executable: "/usr/bin/pdftoppm", command: runner,
		timeout: time.Second, slot: make(chan struct{}, 1),
	}
	cover, err := renderer.RenderFirstPage(context.Background(), pdf)
	if err != nil {
		t.Fatal(err)
	}
	wantArguments := []string{
		"-png", "-f", "1", "-l", "1", "-singlefile",
		"-scale-to-x", "1280", "-scale-to-y", "-1",
		"-x", "0", "-y", "0", "-W", "1280", "-H", "640",
	}
	if runner.executable != "/usr/bin/pdftoppm" ||
		!reflect.DeepEqual(runner.arguments[:len(wantArguments)], wantArguments) {
		t.Fatalf("command=%q arguments=%v", runner.executable, runner.arguments)
	}
	if !bytes.Equal(runner.input, pdf) || filepath.Dir(runner.arguments[len(runner.arguments)-2]) != runner.workDir ||
		filepath.Dir(runner.arguments[len(runner.arguments)-1]) != runner.workDir {
		t.Fatalf("command input or output escaped work directory: %v", runner.arguments)
	}
	if _, err := os.Stat(runner.workDir); !os.IsNotExist(err) {
		t.Fatalf("temporary render directory still exists: %v", err)
	}
	assertPNGDimensions(t, cover, documentCoverRenderWidth, documentCoverRenderHeight)
	image := decodePNG(t, cover)
	r, _, b, _ := image.At(640, documentCoverRenderHeight-1).RGBA()
	if r <= b {
		t.Fatal("cover was not cropped from the top of the page")
	}
}

func TestBoundedDocumentCoverCommandUsesPrlimitOnLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("prlimit wrapping is Linux-specific")
	}
	if _, lookErr := exec.LookPath("prlimit"); lookErr != nil {
		t.Skip("prlimit is unavailable on this runner")
	}
	executable, arguments, err := boundedDocumentCoverCommand("pdftoppm", []string{"-png", "source.pdf", "cover"})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(arguments, " ")
	if filepath.Base(executable) != "prlimit" ||
		!strings.Contains(joined, "--as=268435456") ||
		!strings.Contains(joined, "--fsize=16777216") ||
		!strings.Contains(joined, "-- pdftoppm -png source.pdf cover") {
		t.Fatalf("bounded command=%q %q", executable, joined)
	}
}

func TestCropRenderedDocumentCoverPadsShortPageWhite(t *testing.T) {
	page := testSolidPNG(t, color.RGBA{G: 255, A: 255}, documentCoverRenderWidth, 320)
	cover, err := cropRenderedDocumentCover(page)
	if err != nil {
		t.Fatal(err)
	}
	image := decodePNG(t, cover)
	r, g, b, _ := image.At(640, 500).RGBA()
	if r != 0xffff || g != 0xffff || b != 0xffff {
		t.Fatalf("padded pixel=(%d,%d,%d), want white", r, g, b)
	}
}

func testSolidPNG(t *testing.T, fill color.Color, width, height int) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: fill}, image.Point{}, draw.Src)
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func testTopBottomPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{B: 255, A: 255}}, image.Point{}, draw.Src)
	draw.Draw(canvas, image.Rect(0, 0, width, documentCoverRenderHeight),
		&image.Uniform{C: color.RGBA{R: 255, A: 255}}, image.Point{}, draw.Src)
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func assertPNGDimensions(t *testing.T, content []byte, width, height int) {
	t.Helper()
	configuration, err := png.DecodeConfig(bytes.NewReader(content))
	if err != nil || configuration.Width != width || configuration.Height != height {
		t.Fatalf("PNG dimensions=%dx%d err=%v", configuration.Width, configuration.Height, err)
	}
}

func decodePNG(t *testing.T, content []byte) image.Image {
	t.Helper()
	decoded, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		t.Fatal(err)
	}
	return decoded
}

func testMultiPagePDF(t *testing.T) []byte {
	t.Helper()
	firstContent := "1 0 0 rg\n0 0 612 792 re f\n"
	secondContent := "0 0 1 rg\n0 0 612 792 re f\n"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R 5 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(firstContent), firstContent),
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 6 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(secondContent), secondContent),
	}
	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = pdf.Len()
		_, _ = fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xrefOffset := pdf.Len()
	_, _ = fmt.Fprintf(&pdf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		_, _ = fmt.Fprintf(&pdf, "%010d 00000 n \n", offset)
	}
	_, _ = fmt.Fprintf(&pdf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(offsets), xrefOffset)
	return pdf.Bytes()
}
