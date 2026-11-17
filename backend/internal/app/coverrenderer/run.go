// Package coverrenderer owns pm-cover-renderer process assembly.
package coverrenderer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishu"
)

// Run starts the internal document cover renderer and stops it on termination.
func Run() error {
	token := strings.TrimSpace(os.Getenv("PM_COVER_RENDERER_INTERNAL_TOKEN"))
	if len(token) < 32 {
		return fmt.Errorf("PM_COVER_RENDERER_INTERNAL_TOKEN must contain at least 32 characters")
	}
	address := strings.TrimSpace(os.Getenv("PM_COVER_RENDERER_HTTP_ADDR"))
	if address == "" {
		address = ":8092"
	}
	renderer := feishu.NewPopplerDocumentCoverRenderer()
	server := &http.Server{
		Addr:              address,
		Handler:           feishu.NewDocumentCoverRendererHandler(token, renderer),
		ReadHeaderTimeout: 3 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 * 1024,
	}
	shutdownContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		<-shutdownContext.Done()
		context, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(context)
	}()
	log.Printf("pm-cover-renderer listening on %s", address)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("cover renderer server error: %w", err)
	}
	return nil
}
