package bitable

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/documentlibrary"
)

const MaxAttachmentBytes int64 = 32 << 20

type AttachmentRef struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Size         int64  `json:"size"`
	MimeType     string `json:"mimeType"`
	PreviewPath  string `json:"previewPath,omitempty"`
	DownloadPath string `json:"downloadPath"`
}
type Asset struct {
	AttachmentRef
	ObjectKey, ContentHash, RecordID, FieldID string
}
type AssetAppend struct {
	RecordID, FieldID, OperationID string
	ExpectedSchemaRevision         uint64
	CellContract                   CellContract
	Asset                          Asset
}
type AssetStore interface {
	AppendAttachment(context.Context, Scope, AssetAppend) (Operation, error)
	GetAttachment(context.Context, Scope, string, string, string) (Asset, error)
}
type AssetStorage interface {
	PutObject(context.Context, string, io.Reader, int64, string) error
	GetObject(context.Context, string) (io.ReadCloser, error)
	RemoveObject(context.Context, string) error
}
type Upload struct {
	RecordID, FieldID, OperationID, Name string
	ExpectedSchemaRevision               uint64
	CellContract                         CellContract
	Size                                 int64
	Body                                 io.Reader
}

func (s *Service) ConfigureAssetStorage(storage AssetStorage) { s.assets = storage }
func (s *Service) Upload(ctx context.Context, p Principal, ref, table string, in Upload) (Operation, error) {
	scope, _, e := s.scope(ctx, p, ref, table, documentlibrary.OperationAssetWrite)
	if e != nil {
		return Operation{}, e
	}
	store, ok := s.provider.(AssetStore)
	if !ok || s.assets == nil {
		return Operation{}, ErrUnavailable
	}
	name := path.Base(strings.ReplaceAll(strings.TrimSpace(in.Name), "\\", "/"))
	in.CellContract.RecordID = in.RecordID
	in.CellContract.FieldID = in.FieldID
	if !ValidOperationID(in.OperationID) || !validID(in.RecordID) || !validID(in.FieldID) || ValidateCellContract(in.CellContract) != nil || in.ExpectedSchemaRevision > MaxSafeRevision || in.Size <= 0 || in.Size > MaxAttachmentBytes || in.Body == nil || name == "." || name == ".." || !utf8.ValidString(name) || len(name) > 255 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return Operation{}, ErrInvalidInput
	}
	data, e := io.ReadAll(io.LimitReader(in.Body, MaxAttachmentBytes+1))
	if e != nil || int64(len(data)) != in.Size {
		return Operation{}, ErrInvalidInput
	}
	hash := sha256.Sum256(data)
	id := uuid.NewString()
	mime := http.DetectContentType(data)
	key := fmt.Sprintf("bitables/workspaces/%d/resources/%s/%s", p.WorkspaceID, ref, id)
	if e = s.assets.PutObject(ctx, key, bytes.NewReader(data), in.Size, mime); e != nil {
		return Operation{}, ErrUnavailable
	}
	a := Asset{AttachmentRef: AttachmentRef{ID: id, Name: name, Size: in.Size, MimeType: mime}, ObjectKey: key, ContentHash: hex.EncodeToString(hash[:]), RecordID: in.RecordID, FieldID: in.FieldID}
	result, e := store.AppendAttachment(ctx, scope, AssetAppend{RecordID: in.RecordID, FieldID: in.FieldID, OperationID: in.OperationID, ExpectedSchemaRevision: in.ExpectedSchemaRevision, CellContract: in.CellContract, Asset: a})
	if e == nil && !operationReferencesAsset(result, in.FieldID, id) || errors.Is(e, ErrInvalidInput) || errors.Is(e, ErrForbidden) || errors.Is(e, ErrConflict) || errors.Is(e, ErrNotFound) || errors.Is(e, ErrBudgetExceeded) {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = s.assets.RemoveObject(cleanup, key)
	}
	// Never remove bytes after an ambiguous database result: a committed cell may
	// already reference them. A deployment lifecycle/reconciler must handle these
	// ambiguous, potentially unreferenced objects; no automatic GC is assumed.
	return result, e
}
func operationReferencesAsset(operation Operation, fieldID, assetID string) bool {
	for _, record := range operation.Records {
		data, err := json.Marshal(record.Values[fieldID])
		var refs []AttachmentRef
		if err == nil && json.Unmarshal(data, &refs) == nil {
			for _, ref := range refs {
				if ref.ID == assetID {
					return true
				}
			}
		}
	}
	return false
}
func (s *Service) OpenAttachment(ctx context.Context, p Principal, ref, table, record, field, id string) (Asset, io.ReadCloser, error) {
	scope, _, e := s.scope(ctx, p, ref, table, documentlibrary.OperationAssetRead)
	if e != nil {
		return Asset{}, nil, e
	}
	store, ok := s.provider.(AssetStore)
	if !ok || s.assets == nil {
		return Asset{}, nil, ErrUnavailable
	}
	if !validID(record) || !validID(field) || !validID(id) {
		return Asset{}, nil, ErrInvalidInput
	}
	a, e := store.GetAttachment(ctx, scope, record, field, id)
	if e != nil {
		return Asset{}, nil, e
	}
	body, e := s.assets.GetObject(ctx, a.ObjectKey)
	if e != nil {
		return Asset{}, nil, ErrUnavailable
	}
	return a, body, nil
}
func AttachmentPaths(ref, table, record, field string, a AttachmentRef) AttachmentRef {
	base := fmt.Sprintf("/api/v1/platform/bitables/%s/tables/%s/records/%s/fields/%s/attachments/%s", ref, table, record, field, a.ID)
	a.DownloadPath = base + "/download"
	a.PreviewPath = ""
	switch a.MimeType {
	case "image/png", "image/jpeg", "image/gif", "image/webp":
		a.PreviewPath = base + "/preview"
	}
	return a
}
