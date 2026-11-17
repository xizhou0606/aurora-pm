package document

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"
)

const DocumentAssetMaxBytes int64 = 32 * 1024 * 1024

func safeAssetExtension(name string) string {
	extension := strings.ToLower(path.Ext(name))
	if len(extension) < 2 || len(extension) > 16 {
		return ".bin"
	}
	for _, value := range extension[1:] {
		if (value < 'a' || value > 'z') && (value < '0' || value > '9') {
			return ".bin"
		}
	}
	return extension
}

type AssetUpload struct {
	FileName    string
	ContentType string
	SizeBytes   int64
	Body        io.Reader
}

type assetCountingReader struct {
	reader    io.Reader
	bytesRead int64
}

func (reader *assetCountingReader) Read(buffer []byte) (int, error) {
	count, err := reader.reader.Read(buffer)
	reader.bytesRead += int64(count)
	return count, err
}

func (service *Service) assetDocument(
	ctx context.Context,
	principal Principal,
	documentID uint,
	action string,
	requiredRole objectRole,
) (Document, error) {
	state, err := service.accessState(ctx, principal)
	if err != nil {
		return Document{}, err
	}
	if !state.has(action) {
		return Document{}, ErrForbidden
	}
	body, err := service.store.GetDocument(ctx, principal.WorkspaceID, documentID)
	if err != nil {
		return Document{}, err
	}
	role, err := service.documentRole(ctx, principal, state, documentID)
	if err != nil {
		return Document{}, err
	}
	if role < requiredRole {
		return Document{}, ErrForbidden
	}
	return body, nil
}

func (service *Service) ListAssets(ctx context.Context, principal Principal, documentID uint) ([]Asset, error) {
	if _, err := service.assetDocument(ctx, principal, documentID, string(ActionAssetsView), roleViewer); err != nil {
		return nil, err
	}
	return service.store.ListAssets(ctx, principal.WorkspaceID, documentID)
}

// ValidateCollaborationAssetReferences is the narrow Go-authority seam used
// before a Yjs candidate is accepted. It validates active document-owned asset
// facts without exposing the document store to the collaboration module.
func (service *Service) ValidateCollaborationAssetReferences(
	ctx context.Context,
	principal Principal,
	documentID uint,
	assetIDs []uint,
) error {
	if _, err := service.assetDocument(ctx, principal, documentID, string(ActionContentUpdate), roleEditor); err != nil {
		return err
	}
	seen := make(map[uint]struct{}, len(assetIDs))
	for _, assetID := range assetIDs {
		if assetID == 0 {
			return ErrInvalidInput
		}
		if _, exists := seen[assetID]; exists {
			continue
		}
		seen[assetID] = struct{}{}
		if _, err := service.store.GetAsset(ctx, principal.WorkspaceID, documentID, assetID); err != nil {
			return err
		}
	}
	return nil
}

func (service *Service) UploadAsset(
	ctx context.Context,
	principal Principal,
	documentID uint,
	input AssetUpload,
) (Asset, error) {
	if _, err := service.assetDocument(ctx, principal, documentID, string(ActionAssetsUpload), roleEditor); err != nil {
		return Asset{}, err
	}
	rawName := strings.TrimSpace(strings.ReplaceAll(input.FileName, "\\", "/"))
	name := strings.TrimSpace(path.Base(rawName))
	if rawName == "" || name == "." || name == ".." || name == "/" ||
		!utf8.ValidString(name) || utf8.RuneCountInString(name) > 255 ||
		strings.IndexFunc(name, unicode.IsControl) >= 0 || input.Body == nil || input.SizeBytes <= 0 || input.SizeBytes > DocumentAssetMaxBytes {
		return Asset{}, ErrInvalidInput
	}
	extension := safeAssetExtension(name)
	rawContentType := strings.TrimSpace(input.ContentType)
	if rawContentType == "" {
		rawContentType = "application/octet-stream"
	}
	mediaType, parameters, mediaTypeErr := mime.ParseMediaType(rawContentType)
	contentType := mime.FormatMediaType(strings.ToLower(mediaType), parameters)
	if mediaTypeErr != nil || contentType == "" || len(contentType) > 255 ||
		strings.IndexFunc(contentType, unicode.IsControl) >= 0 {
		return Asset{}, ErrInvalidInput
	}
	if service.assetStorage == nil {
		return Asset{}, ErrAssetStorageUnavailable
	}
	randomID := make([]byte, 16)
	if _, err := rand.Read(randomID); err != nil {
		return Asset{}, ErrAssetStorageUnavailable
	}
	objectKey := path.Join(
		strings.Trim(service.assetPrefix, "/"),
		fmt.Sprintf("workspace/%d/documents/%d/assets/%s%s", principal.WorkspaceID, documentID, hex.EncodeToString(randomID), extension),
	)
	hash := sha256.New()
	body := &assetCountingReader{reader: input.Body}
	if err := service.assetStorage.PutObject(
		ctx, objectKey, io.TeeReader(body, hash), input.SizeBytes, contentType,
	); err != nil {
		return Asset{}, ErrAssetStorageUnavailable
	}
	if body.bytesRead != input.SizeBytes {
		_ = service.assetStorage.RemoveObject(context.WithoutCancel(ctx), objectKey)
		return Asset{}, ErrInvalidInput
	}
	now := service.now()
	asset, err := service.store.CreateAsset(ctx, Asset{
		WorkspaceID: principal.WorkspaceID, DocumentID: documentID, ObjectKey: objectKey,
		FileName: name, ContentType: contentType, SizeBytes: input.SizeBytes,
		ContentHash: hex.EncodeToString(hash.Sum(nil)), CreatedBy: principal.UserID,
		CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		_ = service.assetStorage.RemoveObject(context.WithoutCancel(ctx), objectKey)
		return Asset{}, err
	}
	_ = service.record(ctx, MutationEffect{
		WorkspaceID: principal.WorkspaceID, PrincipalID: principal.UserID,
		Resource: "document_asset", ResourceID: asset.ID, Action: "asset.uploaded",
		Payload: map[string]any{"document_id": documentID, "content_type": asset.ContentType, "size_bytes": asset.SizeBytes},
	})
	return asset, nil
}

func (service *Service) OpenAsset(
	ctx context.Context,
	principal Principal,
	documentID uint,
	assetID uint,
) (Asset, io.ReadCloser, error) {
	if _, err := service.assetDocument(ctx, principal, documentID, string(ActionAssetsDownload), roleViewer); err != nil {
		return Asset{}, nil, err
	}
	asset, err := service.store.GetAsset(ctx, principal.WorkspaceID, documentID, assetID)
	if err != nil {
		return Asset{}, nil, err
	}
	if service.assetStorage == nil {
		return Asset{}, nil, ErrAssetStorageUnavailable
	}
	body, err := service.assetStorage.GetObject(ctx, asset.ObjectKey)
	if err != nil {
		return Asset{}, nil, ErrAssetStorageUnavailable
	}
	return asset, body, nil
}

func (service *Service) DeleteAsset(
	ctx context.Context,
	principal Principal,
	documentID uint,
	assetID uint,
) error {
	if _, err := service.assetDocument(ctx, principal, documentID, string(ActionAssetsDelete), roleEditor); err != nil {
		return err
	}
	asset, err := service.store.GetAsset(ctx, principal.WorkspaceID, documentID, assetID)
	if err != nil {
		return err
	}
	if !asset.IsReference && !asset.RetainObject {
		if service.assetStorage == nil || service.assetStorage.RemoveObject(ctx, asset.ObjectKey) != nil {
			return ErrAssetStorageUnavailable
		}
	}
	if err := service.store.ArchiveAsset(ctx, principal.WorkspaceID, documentID, assetID, principal.UserID); err != nil {
		return err
	}
	return service.record(ctx, MutationEffect{
		WorkspaceID: principal.WorkspaceID, PrincipalID: principal.UserID,
		Resource: "document_asset", ResourceID: assetID, Action: "asset.deleted",
		Payload: map[string]any{"document_id": documentID},
	})
}
