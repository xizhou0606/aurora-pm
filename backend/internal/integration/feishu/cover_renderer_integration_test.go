package feishu

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestPopplerDocumentCoverRendererRendersOnlyFirstPage(t *testing.T) {
	executable, err := exec.LookPath("pdftoppm")
	if err != nil {
		t.Skip("pdftoppm is not installed")
	}
	renderer := &popplerDocumentCoverRenderer{
		executable: executable, command: execDocumentCoverCommandRunner{},
		timeout: 5 * time.Second, slot: make(chan struct{}, 1),
	}
	cover, err := renderer.RenderFirstPage(context.Background(), testMultiPagePDF(t))
	if err != nil {
		t.Fatal(err)
	}
	assertPNGDimensions(t, cover, documentCoverRenderWidth, documentCoverRenderHeight)
	image := decodePNG(t, cover)
	r, _, b, _ := image.At(documentCoverRenderWidth/2, documentCoverRenderHeight/2).RGBA()
	if r <= b {
		t.Fatal("rendered cover does not contain the red first page")
	}
}

func TestPopplerDocumentCoverRendererReadinessExecutesRealRenderOnce(t *testing.T) {
	executable, err := exec.LookPath("pdftoppm")
	if err != nil {
		t.Skip("pdftoppm is not installed")
	}
	renderer := &popplerDocumentCoverRenderer{
		executable: executable, command: execDocumentCoverCommandRunner{},
		timeout: 5 * time.Second, slot: make(chan struct{}, 1),
	}
	if err := renderer.CheckReady(); err != nil {
		t.Fatal(err)
	}
	if err := renderer.CheckReady(); err != nil {
		t.Fatal(err)
	}
}
