package bitable

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
)

type assetMemory struct{ puts, removes int }

func (m *assetMemory) PutObject(context.Context, string, io.Reader, int64, string) error {
	m.puts++
	return nil
}
func (m *assetMemory) GetObject(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewBufferString("asset")), nil
}
func (m *assetMemory) RemoveObject(context.Context, string) error { m.removes++; return nil }

type assetProvider struct {
	testProvider
	mode string
}

func (p *assetProvider) AppendAttachment(_ context.Context, _ Scope, in AssetAppend) (Operation, error) {
	switch p.mode {
	case "unknown":
		return Operation{}, ErrUnknownOutcome
	case "conflict":
		return Operation{}, ErrConflict
	case "replay":
		in.Asset.ID = "previous-upload"
	}
	return Operation{State: "committed", Records: []Record{{Values: map[string]any{in.FieldID: []AttachmentRef{in.Asset.AttachmentRef}}}}}, nil
}
func (p *assetProvider) GetAttachment(context.Context, Scope, string, string, string) (Asset, error) {
	return Asset{}, ErrNotFound
}
func TestUploadCleanupPreservesCommittedOrAmbiguousBytes(t *testing.T) {
	for _, test := range []struct {
		mode    string
		removes int
		err     error
	}{{"committed", 0, nil}, {"replay", 1, nil}, {"conflict", 1, ErrConflict}, {"unknown", 0, ErrUnknownOutcome}} {
		t.Run(test.mode, func(t *testing.T) {
			provider := &assetProvider{mode: test.mode}
			storage := &assetMemory{}
			policy := &testPolicy{writes: true}
			service, _ := NewService(testStore{1}, nil, policy, provider)
			configureTestAccess(service, policy)
			service.ConfigureAssetStorage(storage)
			contract := CellContract{RecordID: "row", FieldID: "attachment", BaseRevision: 1, LeaseID: "550e8400-e29b-41d4-a716-446655440002", FencingToken: 1, ConnectionID: "connection-1", EditorInstanceID: "editor-1"}
			_, err := service.Upload(context.Background(), Principal{UserID: 1, WorkspaceID: 1}, "550e8400-e29b-41d4-a716-446655440000", "table-1", Upload{RecordID: "row", FieldID: "attachment", OperationID: "550e8400-e29b-41d4-a716-446655440001", Name: "tiny.png", CellContract: contract, ExpectedSchemaRevision: 1, Size: 5, Body: bytes.NewBufferString("asset")})
			if !errors.Is(err, test.err) || storage.puts != 1 || storage.removes != test.removes {
				t.Fatalf("err=%v puts=%d removes=%d", err, storage.puts, storage.removes)
			}
		})
	}
}
