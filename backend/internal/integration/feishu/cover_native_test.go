package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestDocumentCoverMediaDownloadIsRateAndConcurrencyLimited(t *testing.T) {
	if documentCoverMediaStartInterval < time.Second/5 {
		t.Fatalf("production media start interval=%v", documentCoverMediaStartInterval)
	}
	pngContent := testSolidPNG(t, color.RGBA{R: 255, A: 255}, 32, 32)
	var mu sync.Mutex
	starts := make([]time.Time, 0, 5)
	active, maxActive := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/open-apis/auth/v3/tenant_access_token/internal":
			_ = json.NewEncoder(writer).Encode(map[string]any{
				"code": 0, "tenant_access_token": "tenant-token", "expire": 7200,
			})
		default:
			mu.Lock()
			starts = append(starts, time.Now())
			active++
			maxActive = max(maxActive, active)
			mu.Unlock()
			time.Sleep(45 * time.Millisecond)
			mu.Lock()
			active--
			mu.Unlock()
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(pngContent)
		}
	}))
	defer server.Close()

	httpClient := newBoundedFeishuHTTPClient(server.Client())
	reader := &SDKReader{
		cfg: config.Feishu{},
		client: lark.NewClient("cover-media-"+server.URL, "app-secret",
			lark.WithOpenBaseUrl(server.URL), lark.WithOAuthBaseUrl(server.URL),
			lark.WithHttpClient(httpClient)),
		httpClient:            httpClient,
		coverMediaCoordinator: newDocumentCoverMediaCoordinator(2, 20*time.Millisecond),
	}
	const downloads = 5
	errorsByCall := make(chan error, downloads)
	var wait sync.WaitGroup
	for index := range downloads {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := reader.downloadDocumentNativeCover(
				context.Background(), fmt.Sprintf("cover-%d", index), nil, nil)
			errorsByCall <- err
		}()
	}
	wait.Wait()
	close(errorsByCall)
	for err := range errorsByCall {
		if err != nil {
			t.Fatal(err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(starts) != downloads || maxActive > 2 {
		t.Fatalf("media starts=%d max active=%d", len(starts), maxActive)
	}
	for index := 1; index < len(starts); index++ {
		if elapsed := starts[index].Sub(starts[index-1]); elapsed < 15*time.Millisecond {
			t.Fatalf("media start interval=%v", elapsed)
		}
	}
}

func TestDocumentCoverMediaLimiterCancellationDoesNotCreateDebt(t *testing.T) {
	coordinator := newDocumentCoverMediaCoordinator(1, time.Second)
	initialNextStart := time.Now().Add(time.Second)
	coordinator.nextStart = initialNextStart
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := coordinator.waitForStart(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wait error=%v", err)
	}
	if !coordinator.nextStart.Equal(initialNextStart) {
		t.Fatalf("cancelled media wait changed next start from %v to %v",
			initialNextStart, coordinator.nextStart)
	}
}

func TestDocumentCoverMediaLimiterQueueIsCancellable(t *testing.T) {
	coordinator := newDocumentCoverMediaCoordinator(2, time.Second)
	gate := coordinator.startGateValue()
	gate <- struct{}{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	started := time.Now()
	if err := coordinator.waitForStart(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued wait error=%v", err)
	}
	<-gate
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("queued cancellation took %v", elapsed)
	}
}
