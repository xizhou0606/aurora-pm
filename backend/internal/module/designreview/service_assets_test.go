package designreview

import (
	"bytes"
	"context"
	"errors"
	"image"
	"io"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
)

type assetBatchRepository struct {
	Repository
	detail    DesignReviewDetail
	saved     []AssetRecord
	saveError error
	completed []string
}

func (repository *assetBatchRepository) CompleteObjectCleanup(_ context.Context, _ uint, objectKey string) error {
	repository.completed = append(repository.completed, objectKey)
	return nil
}

func (repository *assetBatchRepository) GetReview(context.Context, uint, string) (DesignReviewDetail, error) {
	return repository.detail, nil
}

func (repository *assetBatchRepository) SaveAssets(_ context.Context, records []AssetRecord) ([]string, error) {
	repository.saved = append([]AssetRecord(nil), records...)
	if repository.saveError != nil {
		return nil, repository.saveError
	}
	return []string{"old/original", "old/normalized"}, nil
}

func (repository *assetBatchRepository) SaveAsset(_ context.Context, record AssetRecord) (*string, error) {
	repository.saved = []AssetRecord{record}
	return nil, repository.saveError
}

type assetObjectStore struct {
	putKeys       []string
	removedKeys   []string
	failPutNumber int
}

func (store *assetObjectStore) PutObject(_ context.Context, key string, _ io.Reader, _ int64, _ string) error {
	store.putKeys = append(store.putKeys, key)
	if store.failPutNumber > 0 && len(store.putKeys) == store.failPutNumber {
		return errors.New("object upload failed")
	}
	return nil
}

func (store *assetObjectStore) GetObject(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("not implemented")
}

func (store *assetObjectStore) RemoveObject(_ context.Context, key string) error {
	store.removedKeys = append(store.removedKeys, key)
	return nil
}

func TestOriginalUploadPersistsOriginalAndNormalizedAsOneBatch(t *testing.T) {
	repository := &assetBatchRepository{detail: uploadReviewDetail()}
	objects := &assetObjectStore{}
	service := uploadAssetService(repository, objects)

	asset, err := service.UploadAsset(context.Background(), 7, testActor(42), repository.detail.ID, "implementation_original", "image/png", bytes.NewReader(testPNG(t)))
	if err != nil {
		t.Fatal(err)
	}
	if asset.Kind != "implementation_original" || len(repository.saved) != 2 {
		t.Fatalf("asset=%+v saved=%+v", asset, repository.saved)
	}
	if repository.saved[0].Kind != "implementation_original" || repository.saved[1].Kind != "implementation_normalized" {
		t.Fatalf("saved kinds=%s,%s", repository.saved[0].Kind, repository.saved[1].Kind)
	}
	if len(objects.putKeys) != 2 || len(objects.removedKeys) != 2 || objects.removedKeys[0] != "old/original" || objects.removedKeys[1] != "old/normalized" {
		t.Fatalf("puts=%v removals=%v", objects.putKeys, objects.removedKeys)
	}
	if len(repository.completed) != 2 || repository.completed[0] != "old/original" || repository.completed[1] != "old/normalized" {
		t.Fatalf("completed cleanup jobs=%v", repository.completed)
	}
}

func TestImplementationUploadNormalizesToReviewViewport(t *testing.T) {
	repository := &assetBatchRepository{detail: uploadReviewDetail()}
	objects := &assetObjectStore{}
	service := uploadAssetService(repository, objects)

	_, err := service.UploadAsset(context.Background(), 7, testActor(42), repository.detail.ID, "implementation_original", "image/png", bytes.NewReader(testPNGSize(t, 4, 3)))
	if err != nil {
		t.Fatal(err)
	}
	if len(repository.saved) != 2 || repository.saved[0].Width == nil || repository.saved[0].Height == nil ||
		*repository.saved[0].Width != 4 || *repository.saved[0].Height != 3 {
		t.Fatalf("original asset metadata=%+v", repository.saved)
	}
	if repository.saved[1].Width == nil || repository.saved[1].Height == nil ||
		*repository.saved[1].Width != 2 || *repository.saved[1].Height != 2 {
		t.Fatalf("normalized asset metadata=%+v", repository.saved)
	}
}

func TestDOMNormalizedUploadNormalizesToReviewViewport(t *testing.T) {
	repository := &assetBatchRepository{detail: uploadReviewDetail()}
	objects := &assetObjectStore{}
	service := uploadAssetService(repository, objects)

	_, err := service.UploadAsset(context.Background(), 7, testActor(42), repository.detail.ID, "implementation_normalized", "image/png", bytes.NewReader(testPNGSize(t, 4, 3)))
	if err != nil {
		t.Fatal(err)
	}
	if len(repository.saved) != 1 || repository.saved[0].Width == nil || repository.saved[0].Height == nil ||
		*repository.saved[0].Width != 2 || *repository.saved[0].Height != 2 || repository.saved[0].ContentType != "image/png" {
		t.Fatalf("normalized asset metadata=%+v", repository.saved)
	}
}

func TestAssetUploadStoresFactUserID(t *testing.T) {
	repository := &assetBatchRepository{detail: uploadReviewDetail()}
	objects := &assetObjectStore{}
	actions := &effectiveActionReaderStub{codes: []string{
		string(accesscatalog.ActionDesignReviewsReviewsView),
		string(accesscatalog.ActionDesignReviewsReviewsCreate),
	}}
	service := uploadAssetService(repository, objects)
	service.actions = actions

	_, err := service.UploadAsset(context.Background(), 7, ActorIdentity{PrincipalUserID: 109, FactUserID: 1}, repository.detail.ID, "implementation_original", "image/png", bytes.NewReader(testPNG(t)))

	if err != nil {
		t.Fatal(err)
	}
	if len(repository.saved) != 2 || repository.saved[0].ActorUserID != 1 || repository.saved[1].ActorUserID != 1 {
		t.Fatalf("saved assets=%+v", repository.saved)
	}
	for _, userID := range actions.users {
		if userID != 109 {
			t.Fatalf("RBAC action lookup users=%v want only 109", actions.users)
		}
	}
}

func TestOriginalUploadCleansNewObjectsWhenBatchPersistenceFails(t *testing.T) {
	repository := &assetBatchRepository{detail: uploadReviewDetail(), saveError: errors.New("database failed")}
	objects := &assetObjectStore{}
	service := uploadAssetService(repository, objects)

	_, err := service.UploadAsset(context.Background(), 7, testActor(42), repository.detail.ID, "implementation_original", "image/png", bytes.NewReader(testPNG(t)))
	if err == nil {
		t.Fatal("expected batch persistence failure")
	}
	if len(objects.putKeys) != 2 || len(objects.removedKeys) != 2 {
		t.Fatalf("puts=%v removals=%v", objects.putKeys, objects.removedKeys)
	}
	for index, key := range objects.putKeys {
		if objects.removedKeys[index] != key {
			t.Fatalf("new object %q was not cleaned: %v", key, objects.removedKeys)
		}
	}
}

func TestOriginalUploadDoesNotPersistWhenDerivedObjectUploadFails(t *testing.T) {
	repository := &assetBatchRepository{detail: uploadReviewDetail()}
	objects := &assetObjectStore{failPutNumber: 2}
	service := uploadAssetService(repository, objects)

	_, err := service.UploadAsset(context.Background(), 7, testActor(42), repository.detail.ID, "implementation_original", "image/png", bytes.NewReader(testPNG(t)))
	if err == nil {
		t.Fatal("expected derived object upload failure")
	}
	if len(repository.saved) != 0 || len(objects.removedKeys) != 1 || objects.removedKeys[0] != objects.putKeys[0] {
		t.Fatalf("saved=%v puts=%v removals=%v", repository.saved, objects.putKeys, objects.removedKeys)
	}
}

func TestAssetUploadRejectsReviewsThatAlreadyLeftDraft(t *testing.T) {
	repository := &assetBatchRepository{detail: uploadReviewDetail()}
	repository.detail.Status = "queued"
	objects := &assetObjectStore{}
	service := uploadAssetService(repository, objects)

	_, err := service.UploadAsset(context.Background(), 7, testActor(42), repository.detail.ID, "implementation_original", "image/png", bytes.NewReader(testPNG(t)))
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("error=%v, want conflict", err)
	}
	if len(objects.putKeys) != 0 || len(repository.saved) != 0 {
		t.Fatalf("queued review upload had side effects: puts=%v saved=%v", objects.putKeys, repository.saved)
	}
}

func uploadAssetService(repository Repository, objects ObjectStore) *Service {
	return &Service{
		repository: repository,
		projects:   &projectAccessStub{allowed: map[uint]bool{11: true}},
		actions: &effectiveActionReaderStub{codes: []string{
			string(accesscatalog.ActionDesignReviewsReviewsView),
			string(accesscatalog.ActionDesignReviewsReviewsCreate),
		}},
		objects:      objects,
		availability: Availability{ObjectStorage: true},
		now:          func() time.Time { return time.Date(2026, 9, 4, 13, 30, 0, 0, time.UTC) },
	}
}

func uploadReviewDetail() DesignReviewDetail {
	return DesignReviewDetail{
		DesignReviewSummary: DesignReviewSummary{ID: "c1000000-0000-4000-8000-000000000001", ProjectSetID: 11, ImplementationKind: "web", Status: "draft"},
		ViewportWidth:       2, ViewportHeight: 2,
		MobileTopbar: []byte(`{"enabled":false}`),
	}
}

func testPNG(t *testing.T) []byte {
	return testPNGSize(t, 2, 2)
}

func testPNGSize(t *testing.T, width, height int) []byte {
	t.Helper()
	raw, err := EncodePNG(image.NewNRGBA(image.Rect(0, 0, width, height)))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
