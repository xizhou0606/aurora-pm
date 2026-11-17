package requirementcover

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishu"
	requirementcovermodule "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/cover"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
)

var coverWorkerTestSequence atomic.Uint64

type testCoverReader struct {
	cover feishu.DocumentCover
	err   error
	urls  []string
}

func (r *testCoverReader) ReadCover(_ context.Context, rawURL string) (feishu.DocumentCover, error) {
	r.urls = append(r.urls, rawURL)
	if r.err != nil {
		return feishu.DocumentCover{}, r.err
	}
	return r.cover, nil
}

type testCoverStore struct {
	objects map[string][]byte
	removed []string
}

func (s *testCoverStore) PutObject(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
	content, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if s.objects == nil {
		s.objects = map[string][]byte{}
	}
	s.objects[key] = content
	return nil
}

func (s *testCoverStore) RemoveObject(_ context.Context, key string) error {
	delete(s.objects, key)
	s.removed = append(s.removed, key)
	return nil
}

func coverWorkerFixture(t *testing.T, documentURL string) (*Worker, *testCoverReader, *testCoverStore, requirementgorm.Requirement) {
	t.Helper()
	cfg := config.Config{
		DatabaseDriver: "sqlite", DatabaseDSN: "file:requirement_cover_worker_" + strconv.FormatUint(coverWorkerTestSequence.Add(1), 10) + "?mode=memory&cache=shared",
		Feishu:                  config.Feishu{Enabled: true, DomainAllowlist: []string{".feishu.cn"}},
		RequirementCoverEnabled: true,
		ObjectStorage:           config.ObjectStorage{Enabled: true, GlobalPrefix: "pm"},
		CoverRenderer: config.CoverRenderer{
			BaseURL: "http://pm-cover-renderer:8092", InternalToken: "renderer-token-that-is-at-least-32-characters",
		},
	}
	db, err := database.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(db); err != nil {
		t.Fatal(err)
	}
	requirement := requirementgorm.Requirement{
		WorkspaceID: 7, TeamID: 1, ProjectSetID: 2, Title: "封面异步生成", ProductDocURL: documentURL,
		Priority: "normal", Status: "backlog", Version: 1,
	}
	if err := db.Create(&requirement).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(func(tx *gorm.DB) error { return requirementcovermodule.EnqueueTx(tx, cfg, requirement) }); err != nil {
		t.Fatal(err)
	}
	reader := &testCoverReader{cover: feishu.DocumentCover{Content: testCoverPNG(t), ContentType: "image/png"}}
	store := &testCoverStore{objects: map[string][]byte{}}
	worker, err := NewWorker(db, cfg, reader, store)
	if err != nil {
		t.Fatal(err)
	}
	worker.workerID = "test-worker"
	worker.now = func() time.Time { return time.Now().UTC().Add(time.Minute) }
	return worker, reader, store, requirement
}

func TestWorkerPublishesCompressedCoverAndCleansSupersededObject(t *testing.T) {
	worker, reader, store, requirement := coverWorkerFixture(t, "https://acme.feishu.cn/docx/docx-token")
	oldKey := "pm/workspace/7/requirement-covers/requirement/1/old.jpg"
	var cover requirementgorm.RequirementCover
	if err := worker.db.Where("workspace_id = ? AND requirement_id = ?", requirement.WorkspaceID, requirement.ID).First(&cover).Error; err != nil {
		t.Fatal(err)
	}
	if err := worker.db.Model(&requirementgorm.RequirementCover{}).Where("id = ?", cover.ID).Update("object_key", oldKey).Error; err != nil {
		t.Fatal(err)
	}
	store.objects[oldKey] = []byte("old")

	processed, err := worker.RunOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("RunOnce() processed=%v err=%v", processed, err)
	}
	if len(reader.urls) != 1 || reader.urls[0] != "https://acme.feishu.cn/docx/docx-token" {
		t.Fatalf("reader urls=%v", reader.urls)
	}
	if err := worker.db.First(&cover, cover.ID).Error; err != nil {
		t.Fatal(err)
	}
	if cover.Status != requirementcovermodule.StatusReady || cover.ContentType != "image/jpeg" || cover.Width != thumbnailWidth || cover.Height != thumbnailHeight {
		t.Fatalf("published cover=%+v", cover)
	}
	content := store.objects[cover.ObjectKey]
	decoded, format, err := image.Decode(bytes.NewReader(content))
	if err != nil || format != "jpeg" || decoded.Bounds().Dx() != thumbnailWidth || decoded.Bounds().Dy() != thumbnailHeight {
		t.Fatalf("thumbnail format=%q bounds=%v err=%v", format, decoded.Bounds(), err)
	}
	if _, exists := store.objects[oldKey]; exists {
		t.Fatal("superseded cover object was not removed")
	}
}

func TestWorkerRetriesProviderFailureWithoutLeakingProviderDetail(t *testing.T) {
	worker, reader, store, requirement := coverWorkerFixture(t, "https://acme.feishu.cn/wiki/wiki-token")
	reader.err = errors.New("provider returned credential details")

	processed, err := worker.RunOnce(context.Background())
	if err != nil || !processed {
		t.Fatalf("RunOnce() processed=%v err=%v", processed, err)
	}
	var cover requirementgorm.RequirementCover
	if err := worker.db.Where("workspace_id = ? AND requirement_id = ?", requirement.WorkspaceID, requirement.ID).First(&cover).Error; err != nil {
		t.Fatal(err)
	}
	if cover.Status != requirementcovermodule.StatusPending || cover.Attempts != 1 || cover.LastError != "cover generation failed" || !cover.NextAttemptAt.After(time.Now().UTC()) {
		t.Fatalf("retry cover=%+v", cover)
	}
	if len(store.objects) != 0 {
		t.Fatalf("failure stored unexpected objects=%v", store.objects)
	}
}

func TestEnqueueTxChangesGenerationOnlyWhenDocumentChanges(t *testing.T) {
	worker, _, _, requirement := coverWorkerFixture(t, "https://acme.feishu.cn/docx/first-token")
	var first requirementgorm.RequirementCover
	if err := worker.db.Where("workspace_id = ? AND requirement_id = ?", requirement.WorkspaceID, requirement.ID).First(&first).Error; err != nil {
		t.Fatal(err)
	}
	if err := worker.db.Transaction(func(tx *gorm.DB) error { return requirementcovermodule.EnqueueTx(tx, worker.cfg, requirement) }); err != nil {
		t.Fatal(err)
	}
	var same requirementgorm.RequirementCover
	if err := worker.db.First(&same, first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if same.Generation != first.Generation || same.SourceHash != first.SourceHash {
		t.Fatalf("unchanged document queued again: before=%+v after=%+v", first, same)
	}

	requirement.ProductDocURL = "https://acme.feishu.cn/wiki/second-token"
	if err := worker.db.Model(&requirementgorm.Requirement{}).Where("id = ?", requirement.ID).Update("product_doc_url", requirement.ProductDocURL).Error; err != nil {
		t.Fatal(err)
	}
	if err := worker.db.Transaction(func(tx *gorm.DB) error { return requirementcovermodule.EnqueueTx(tx, worker.cfg, requirement) }); err != nil {
		t.Fatal(err)
	}
	var changed requirementgorm.RequirementCover
	if err := worker.db.First(&changed, first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if changed.Generation != first.Generation+1 || changed.Status != requirementcovermodule.StatusPending || changed.SourceHash == first.SourceHash || changed.Attempts != 0 {
		t.Fatalf("changed document did not replace job: before=%+v after=%+v", first, changed)
	}
}

func TestEnqueueTxLeavesExistingCoverUntouchedWhenFeatureIsDisabled(t *testing.T) {
	worker, _, _, requirement := coverWorkerFixture(t, "https://acme.feishu.cn/docx/first-token")
	var before requirementgorm.RequirementCover
	if err := worker.db.Where("workspace_id = ? AND requirement_id = ?", requirement.WorkspaceID, requirement.ID).First(&before).Error; err != nil {
		t.Fatal(err)
	}

	requirement.ProductDocURL = "https://acme.feishu.cn/wiki/second-token"
	disabled := worker.cfg
	disabled.RequirementCoverEnabled = false
	if err := worker.db.Transaction(func(tx *gorm.DB) error { return requirementcovermodule.EnqueueTx(tx, disabled, requirement) }); err != nil {
		t.Fatal(err)
	}

	var after requirementgorm.RequirementCover
	if err := worker.db.First(&after, before.ID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Generation != before.Generation || after.SourceHash != before.SourceHash || after.Status != before.Status {
		t.Fatalf("disabled feature rewrote existing cover: before=%+v after=%+v", before, after)
	}
}

func TestDisabledWorkerCannotOverwriteANewerDocumentJob(t *testing.T) {
	worker, _, _, requirement := coverWorkerFixture(t, "https://acme.feishu.cn/docx/first-token")
	var original requirementgorm.RequirementCover
	if err := worker.db.Where("workspace_id = ? AND requirement_id = ?", requirement.WorkspaceID, requirement.ID).First(&original).Error; err != nil {
		t.Fatal(err)
	}
	now := worker.now()
	if err := worker.db.Model(&requirementgorm.RequirementCover{}).Where("id = ?", original.ID).Updates(map[string]any{
		"source_hash": "", "generation": original.Generation + 1, "status": requirementcovermodule.StatusProcessing,
		"claimed_at": now, "claimed_by": worker.workerID,
	}).Error; err != nil {
		t.Fatal(err)
	}
	newURL := "https://acme.feishu.cn/wiki/new-token"
	if err := worker.db.Model(&requirementgorm.Requirement{}).Where("id = ?", requirement.ID).Update("product_doc_url", newURL).Error; err != nil {
		t.Fatal(err)
	}
	requirement.ProductDocURL = newURL
	if err := worker.db.Transaction(func(tx *gorm.DB) error { return requirementcovermodule.EnqueueTx(tx, worker.cfg, requirement) }); err != nil {
		t.Fatal(err)
	}
	stale := requirementgorm.RequirementCover{ID: original.ID, WorkspaceID: requirement.WorkspaceID, RequirementID: requirement.ID,
		Generation: original.Generation + 1, SourceHash: "", Status: requirementcovermodule.StatusProcessing, ClaimedBy: worker.workerID}
	if err := worker.completeDisabled(stale); err != nil {
		t.Fatal(err)
	}
	var current requirementgorm.RequirementCover
	if err := worker.db.First(&current, original.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.Status != requirementcovermodule.StatusPending || current.Generation != stale.Generation+1 || current.SourceHash == "" {
		t.Fatalf("newer job was overwritten: %+v", current)
	}
}

func testCoverPNG(t *testing.T) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, 1280, 640))
	for x := 0; x < 1280; x++ {
		for y := 0; y < 640; y++ {
			canvas.SetRGBA(x, y, color.RGBA{R: uint8(x % 255), G: uint8(y % 255), B: 130, A: 255})
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
