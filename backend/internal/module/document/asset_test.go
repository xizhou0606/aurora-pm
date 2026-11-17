package document

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestDocumentAssetBindingPolicyUsesCanonicalActionAsUpperBound(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{
		string(ActionAssetsView),
		string(ActionAssetsUpload),
		string(ActionAssetsDownload),
		string(ActionAssetsDelete),
	})
	storage := &stubAssetStorage{}
	service.ConfigureAssetStorage(storage, "pm/native")
	payload := []byte("protected attachment")
	asset, err := service.UploadAsset(
		context.Background(),
		Principal{UserID: 5, WorkspaceID: 7},
		11,
		AssetUpload{
			FileName: "decision.txt", ContentType: "text/plain; charset=utf-8",
			SizeBytes: int64(len(payload)), Body: bytes.NewReader(payload),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	assets, err := service.ListAssets(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11)
	if err != nil || len(assets) != 1 || assets[0].ID != asset.ID {
		t.Fatalf("developer assets=%+v err=%v", assets, err)
	}

	opened, body, err := service.OpenAsset(
		context.Background(), Principal{UserID: 6, WorkspaceID: 7}, 11, asset.ID,
	)
	if err != nil {
		t.Fatal(err)
	}
	readBack, readErr := io.ReadAll(body)
	_ = body.Close()
	if readErr != nil || opened.ID != asset.ID || !bytes.Equal(readBack, payload) {
		t.Fatalf("tester opened=%+v body=%q err=%v", opened, readBack, readErr)
	}
	if _, err := service.UploadAsset(
		context.Background(), Principal{UserID: 6, WorkspaceID: 7}, 11,
		AssetUpload{FileName: "denied.txt", SizeBytes: 1, Body: bytes.NewReader([]byte("x"))},
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("tester upload err=%v", err)
	}
	if err := service.DeleteAsset(
		context.Background(), Principal{UserID: 6, WorkspaceID: 7}, 11, asset.ID,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("tester delete err=%v", err)
	}
	if _, err := service.ListAssets(
		context.Background(), Principal{UserID: 9, WorkspaceID: 7}, 11,
	); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unbound participant list err=%v", err)
	}
	if _, err := service.ListAssets(
		context.Background(), Principal{UserID: 5, WorkspaceID: 8}, 11,
	); !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("cross-workspace list err=%v", err)
	}
}

func TestDocumentAssetScopeAllDoesNotReplaceTheCanonicalAction(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionScopeAll), string(ActionAssetsView)})
	service.ConfigureAssetStorage(&stubAssetStorage{}, "pm/native")
	_, err := service.UploadAsset(
		context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11,
		AssetUpload{FileName: "decision.txt", SizeBytes: 1, Body: bytes.NewReader([]byte("x"))},
	)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("scope-all upload without action err=%v", err)
	}
}

func TestDocumentAssetUploadNormalizesMetadataAndKeepsFilenameOutOfObjectKey(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionScopeAll), string(ActionAssetsUpload)})
	storage := &stubAssetStorage{}
	service.ConfigureAssetStorage(storage, "/pm/native/")
	payload := []byte("attachment")
	asset, err := service.UploadAsset(
		context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11,
		AssetUpload{
			FileName: "../../Customer Decision.TXT", ContentType: "TEXT/PLAIN; CHARSET=utf-8",
			SizeBytes: int64(len(payload)), Body: bytes.NewReader(payload),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256(payload)
	if asset.FileName != "Customer Decision.TXT" || asset.ContentType != "text/plain; charset=utf-8" ||
		asset.SizeBytes != int64(len(payload)) || asset.ContentHash != hex.EncodeToString(wantHash[:]) {
		t.Fatalf("asset=%+v", asset)
	}
	if !strings.HasPrefix(asset.ObjectKey, "pm/native/workspace/7/documents/11/assets/") ||
		strings.Contains(asset.ObjectKey, "Customer") || !strings.HasSuffix(asset.ObjectKey, ".txt") {
		t.Fatalf("unsafe object key=%q", asset.ObjectKey)
	}
}

func TestDocumentAssetUploadRejectsInvalidMetadataAndSizeMismatch(t *testing.T) {
	tests := []struct {
		name  string
		input AssetUpload
	}{
		{name: "parent filename", input: AssetUpload{FileName: "..", SizeBytes: 1, Body: bytes.NewReader([]byte("x"))}},
		{name: "control character", input: AssetUpload{FileName: "bad\nname.txt", SizeBytes: 1, Body: bytes.NewReader([]byte("x"))}},
		{name: "filename too long", input: AssetUpload{FileName: strings.Repeat("a", 256), SizeBytes: 1, Body: bytes.NewReader([]byte("x"))}},
		{name: "invalid content type", input: AssetUpload{FileName: "safe.txt", ContentType: "not a mime", SizeBytes: 1, Body: bytes.NewReader([]byte("x"))}},
		{name: "empty file", input: AssetUpload{FileName: "safe.txt", SizeBytes: 0, Body: bytes.NewReader(nil)}},
		{name: "too large", input: AssetUpload{FileName: "safe.txt", SizeBytes: DocumentAssetMaxBytes + 1, Body: bytes.NewReader([]byte("x"))}},
		{name: "declared size mismatch", input: AssetUpload{FileName: "safe.txt", SizeBytes: 2, Body: bytes.NewReader([]byte("x"))}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service, _ := newPolicyTestService(t, []string{string(ActionScopeAll), string(ActionAssetsUpload)})
			storage := &stubAssetStorage{}
			service.ConfigureAssetStorage(storage, "pm/native")
			_, err := service.UploadAsset(
				context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11, test.input,
			)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("err=%v", err)
			}
			if len(storage.objects) != 0 {
				t.Fatalf("uploaded objects=%v", storage.objects)
			}
		})
	}
}

type assetReadFailure struct{}

func (assetReadFailure) Read([]byte) (int, error) {
	return 0, errors.New("synthetic storage read failure")
}

type countingAssetStorage struct{ stubAssetStorage }

func (*countingAssetStorage) PutObject(_ context.Context, _ string, body io.Reader, _ int64, _ string) error {
	_, err := io.Copy(io.Discard, body)
	return err
}

func TestDocumentAssetAcceptsExactMaximumSize(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionScopeAll), string(ActionAssetsUpload)})
	service.ConfigureAssetStorage(&countingAssetStorage{}, "pm/test")
	asset, err := service.UploadAsset(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11,
		AssetUpload{FileName: "maximum.txt", SizeBytes: DocumentAssetMaxBytes,
			Body: strings.NewReader(strings.Repeat("x", int(DocumentAssetMaxBytes)))})
	if err != nil || asset.SizeBytes != DocumentAssetMaxBytes {
		t.Fatalf("exact maximum size was rejected: %v", err)
	}
}

func TestDocumentAssetFailedUploadsNeverCreateMetadata(t *testing.T) {
	for _, unavailable := range []bool{true, false} {
		name := "storage-write-failure"
		if unavailable {
			name = "storage-disabled"
		}
		t.Run(name, func(t *testing.T) {
			service, _ := newPolicyTestService(t, []string{string(ActionScopeAll), string(ActionAssetsUpload), string(ActionAssetsView)})
			if !unavailable {
				service.ConfigureAssetStorage(&stubAssetStorage{}, "pm/test")
			}
			principal := Principal{UserID: 5, WorkspaceID: 7}
			_, err := service.UploadAsset(context.Background(), principal, 11, AssetUpload{
				FileName: "failed.txt", SizeBytes: 1, Body: assetReadFailure{},
			})
			if !errors.Is(err, ErrAssetStorageUnavailable) {
				t.Fatalf("unexpected failure: %v", err)
			}
			assets, err := service.ListAssets(context.Background(), principal, 11)
			if err != nil || len(assets) != 0 {
				t.Fatalf("failed upload produced metadata: count=%d error=%v", len(assets), err)
			}
		})
	}
}

func TestDocumentAssetsCrossWorkspaceCannotReadWriteOrDelete(t *testing.T) {
	service, _ := newPolicyTestService(t, []string{string(ActionScopeAll), string(ActionAssetsUpload), string(ActionAssetsDownload), string(ActionAssetsDelete)})
	storage := &stubAssetStorage{}
	service.ConfigureAssetStorage(storage, "pm/test")
	asset, err := service.UploadAsset(context.Background(), Principal{UserID: 5, WorkspaceID: 7}, 11,
		AssetUpload{FileName: "private.txt", SizeBytes: 1, Body: strings.NewReader("x")})
	if err != nil {
		t.Fatal(err)
	}
	other := Principal{UserID: 5, WorkspaceID: 8}
	if _, _, err := service.OpenAsset(context.Background(), other, 11, asset.ID); !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("cross-workspace download: %v", err)
	}
	if _, err := service.UploadAsset(context.Background(), other, 11, AssetUpload{FileName: "forbidden.txt", SizeBytes: 1, Body: strings.NewReader("x")}); !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("cross-workspace upload: %v", err)
	}
	if err := service.DeleteAsset(context.Background(), other, 11, asset.ID); !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("cross-workspace delete: %v", err)
	}
	if len(storage.objects) != 1 {
		t.Fatalf("cross-workspace mutation changed storage")
	}
}
