package feishu

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	documentCoverRenderWidth     = 1280
	documentCoverRenderHeight    = 640
	documentCoverRenderMaxHeight = 4096
	documentCoverRenderMaxBytes  = 8 * 1024 * 1024
	documentCoverRenderTimeout   = 8 * time.Second
	documentCoverRenderASLimit   = 256 * 1024 * 1024
	documentCoverRenderFileLimit = 16 * 1024 * 1024
)

var globalDocumentCoverRenderSlot = make(chan struct{}, 1)

type DocumentCoverRenderer interface {
	RenderFirstPage(context.Context, []byte) ([]byte, error)
}

type documentCoverRenderer = DocumentCoverRenderer

type documentCoverCommandRunner interface {
	Run(context.Context, string, []string, string) error
}

type popplerDocumentCoverRenderer struct {
	executable string
	command    documentCoverCommandRunner
	timeout    time.Duration
	slot       chan struct{}
	readyOnce  sync.Once
	readyErr   error
}

type execDocumentCoverCommandRunner struct{}

func newPopplerDocumentCoverRenderer() documentCoverRenderer {
	return NewPopplerDocumentCoverRenderer()
}

func NewPopplerDocumentCoverRenderer() DocumentCoverRenderer {
	return &popplerDocumentCoverRenderer{
		executable: "pdftoppm", command: execDocumentCoverCommandRunner{},
		timeout: documentCoverRenderTimeout, slot: globalDocumentCoverRenderSlot,
	}
}

func (r *popplerDocumentCoverRenderer) RenderFirstPage(ctx context.Context, content []byte) ([]byte, error) {
	if len(content) == 0 || int64(len(content)) > documentCoverMaxBytes ||
		strings.ToLower(http.DetectContentType(content)) != "application/pdf" {
		return nil, fmt.Errorf("document cover PDF input is invalid")
	}
	slot := r.slot
	if slot == nil {
		slot = globalDocumentCoverRenderSlot
	}
	select {
	case slot <- struct{}{}:
		defer func() { <-slot }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	renderCtx, cancel := context.WithTimeout(ctx, r.timeoutValue())
	defer cancel()

	workDir, err := os.MkdirTemp(documentCoverRendererTempDir(), "pm-cover-render-")
	if err != nil {
		return nil, fmt.Errorf("create document cover render directory: %w", err)
	}
	defer os.RemoveAll(workDir)
	inputPath := filepath.Join(workDir, "source.pdf")
	outputPrefix := filepath.Join(workDir, "cover")
	if err := os.WriteFile(inputPath, content, 0o600); err != nil {
		return nil, fmt.Errorf("write document cover PDF: %w", err)
	}
	arguments := []string{
		"-png", "-f", "1", "-l", "1", "-singlefile",
		"-scale-to-x", strconv.Itoa(documentCoverRenderWidth), "-scale-to-y", "-1",
		"-x", "0", "-y", "0", "-W", strconv.Itoa(documentCoverRenderWidth),
		"-H", strconv.Itoa(documentCoverRenderHeight),
		inputPath, outputPrefix,
	}
	if err := r.commandValue().Run(renderCtx, r.executableValue(), arguments, workDir); err != nil {
		if renderCtx.Err() != nil {
			return nil, fmt.Errorf("render document cover timed out: %w", renderCtx.Err())
		}
		return nil, fmt.Errorf("render document cover failed: %w", err)
	}
	pageImage, err := readBoundedRegularFile(outputPrefix+".png", documentCoverRenderMaxBytes)
	if err != nil {
		return nil, fmt.Errorf("read rendered document cover: %w", err)
	}
	coverImage, err := cropRenderedDocumentCover(pageImage)
	if err != nil {
		return nil, err
	}
	return coverImage, nil
}

func (r *popplerDocumentCoverRenderer) CheckReady() error {
	r.readyOnce.Do(func() {
		r.readyErr = r.checkReady()
	})
	return r.readyErr
}

func (r *popplerDocumentCoverRenderer) checkReady() error {
	if _, err := exec.LookPath(r.executableValue()); err != nil {
		return fmt.Errorf("document cover renderer executable is unavailable: %w", err)
	}
	if _, _, err := boundedDocumentCoverCommand(r.executableValue(), nil); err != nil {
		return err
	}
	workDir, err := os.MkdirTemp(documentCoverRendererTempDir(), "pm-cover-ready-")
	if err != nil {
		return fmt.Errorf("document cover renderer temporary directory is unavailable: %w", err)
	}
	if err := os.RemoveAll(workDir); err != nil {
		return fmt.Errorf("clean document cover renderer temporary directory: %w", err)
	}
	cover, err := r.RenderFirstPage(context.Background(), documentCoverReadinessPDF())
	if err != nil {
		return fmt.Errorf("document cover renderer readiness probe failed: %w", err)
	}
	return validateRenderedDocumentCover(cover)
}

func documentCoverReadinessPDF() []byte {
	content := "0.15 0.35 0.75 rg\n0 0 612 792 re f\n"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content),
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

func documentCoverRendererTempDir() string {
	if value := strings.TrimSpace(os.Getenv("PM_COVER_RENDERER_TMPDIR")); value != "" {
		return value
	}
	return strings.TrimSpace(os.Getenv("PM_COVER_RENDER_TMPDIR"))
}

func (execDocumentCoverCommandRunner) Run(
	ctx context.Context,
	executable string,
	arguments []string,
	workDir string,
) error {
	commandExecutable, commandArguments, err := boundedDocumentCoverCommand(executable, arguments)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, commandExecutable, commandArguments...)
	command.Dir = workDir
	command.Env = []string{
		"HOME=" + workDir, "TMPDIR=" + workDir, "LC_ALL=C", "PATH=" + os.Getenv("PATH"),
	}
	command.Stdin = nil
	command.Stdout = io.Discard
	command.Stderr = io.Discard
	command.WaitDelay = time.Second
	return command.Run()
}

func boundedDocumentCoverCommand(executable string, arguments []string) (string, []string, error) {
	if runtime.GOOS != "linux" {
		return executable, arguments, nil
	}
	limiter, err := exec.LookPath("prlimit")
	if err != nil {
		return "", nil, fmt.Errorf("document cover resource limiter is unavailable: %w", err)
	}
	limited := []string{
		"--as=" + strconv.Itoa(documentCoverRenderASLimit),
		"--fsize=" + strconv.Itoa(documentCoverRenderFileLimit),
		"--nofile=64", "--nproc=16", "--core=0", "--cpu=10", "--", executable,
	}
	return limiter, append(limited, arguments...), nil
}

func readBoundedRegularFile(path string, maxBytes int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxBytes {
		return nil, fmt.Errorf("rendered document cover file is invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	content, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil || int64(len(content)) > maxBytes {
		return nil, fmt.Errorf("rendered document cover exceeds byte limit")
	}
	return content, nil
}

func validateRenderedDocumentCover(content []byte) error {
	if len(content) == 0 || len(content) > documentCoverRenderMaxBytes ||
		strings.ToLower(http.DetectContentType(content)) != "image/png" {
		return fmt.Errorf("rendered document cover media type is invalid")
	}
	configuration, err := png.DecodeConfig(bytes.NewReader(content))
	if err != nil || configuration.Width != documentCoverRenderWidth ||
		configuration.Height != documentCoverRenderHeight {
		return fmt.Errorf("rendered document cover dimensions are invalid")
	}
	return nil
}

func cropRenderedDocumentCover(content []byte) ([]byte, error) {
	configuration, err := png.DecodeConfig(bytes.NewReader(content))
	if err != nil || configuration.Width != documentCoverRenderWidth || configuration.Height <= 0 ||
		configuration.Height > documentCoverRenderMaxHeight ||
		int64(configuration.Width)*int64(configuration.Height) > int64(documentCoverRenderWidth*documentCoverRenderMaxHeight) {
		return nil, fmt.Errorf("rendered document page dimensions are invalid")
	}
	source, err := png.Decode(bytes.NewReader(content))
	if err != nil {
		return nil, fmt.Errorf("decode rendered document page: %w", err)
	}
	target := image.NewRGBA(image.Rect(0, 0, documentCoverRenderWidth, documentCoverRenderHeight))
	draw.Draw(target, target.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	copyHeight := min(documentCoverRenderHeight, source.Bounds().Dy())
	draw.Draw(target, image.Rect(0, 0, documentCoverRenderWidth, copyHeight),
		source, source.Bounds().Min, draw.Over)
	var output bytes.Buffer
	if err := png.Encode(&output, target); err != nil {
		return nil, fmt.Errorf("encode cropped document cover: %w", err)
	}
	if err := validateRenderedDocumentCover(output.Bytes()); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

func (r *popplerDocumentCoverRenderer) executableValue() string {
	if strings.TrimSpace(r.executable) != "" {
		return r.executable
	}
	return "pdftoppm"
}

func (r *popplerDocumentCoverRenderer) commandValue() documentCoverCommandRunner {
	if r.command != nil {
		return r.command
	}
	return execDocumentCoverCommandRunner{}
}

func (r *popplerDocumentCoverRenderer) timeoutValue() time.Duration {
	if r.timeout > 0 {
		return r.timeout
	}
	return documentCoverRenderTimeout
}
