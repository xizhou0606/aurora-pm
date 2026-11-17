package documentgorm

import (
	"context"
	"errors"
	"testing"
	"time"

	document "github.com/xizhou0606/aurora-pm/backend/internal/module/document"
)

func TestRepositoryAssetsAreWorkspaceScopedAndSoftArchived(t *testing.T) {
	database := openDocumentDB(t)
	repository := NewRepository(database)
	now := time.Now().UTC()
	created, err := repository.CreateAsset(context.Background(), document.Asset{
		WorkspaceID: 7,
		DocumentID:  11,
		ObjectKey:   "pm/native/workspace/7/documents/11/assets/asset.pdf",
		FileName:    "decision.pdf",
		ContentType: "application/pdf",
		SizeBytes:   2048,
		ContentHash: "hash",
		CreatedBy:   5,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil || created.ID == 0 {
		t.Fatalf("created=%+v err=%v", created, err)
	}
	assets, err := repository.ListAssets(context.Background(), 7, 11)
	if err != nil || len(assets) != 1 || assets[0].ID != created.ID {
		t.Fatalf("assets=%+v err=%v", assets, err)
	}
	otherWorkspace, err := repository.ListAssets(context.Background(), 8, 11)
	if err != nil || len(otherWorkspace) != 0 {
		t.Fatalf("other workspace assets=%+v err=%v", otherWorkspace, err)
	}
	if _, err := repository.GetAsset(context.Background(), 8, 11, created.ID); !errors.Is(err, document.ErrAssetNotFound) {
		t.Fatalf("cross-workspace get err=%v", err)
	}
	if err := repository.ArchiveAsset(context.Background(), 7, 11, created.ID, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.GetAsset(context.Background(), 7, 11, created.ID); !errors.Is(err, document.ErrAssetNotFound) {
		t.Fatalf("archived get err=%v", err)
	}
	if err := repository.ArchiveAsset(context.Background(), 7, 11, created.ID, 5); !errors.Is(err, document.ErrAssetNotFound) {
		t.Fatalf("replayed archive err=%v", err)
	}
}
