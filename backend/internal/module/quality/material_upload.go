package quality

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
)

var ErrMaterialStorageUnavailable = errors.New("material storage unavailable")
var ErrMaterialFileMismatch = errors.New("material file mismatch")

const MaterialMaxBytes int64 = 200 * 1024 * 1024

type MaterialObjectStore interface {
	PutObject(context.Context, string, io.Reader, int64, string) error
}
type MaterialUploadInput struct {
	RequirementID uint   `json:"requirement_id"`
	Name          string `json:"name"`
	Size          int64  `json:"size"`
	SHA256        string `json:"sha256"`
	ContentType   string `json:"content_type"`
}
type UploadedMaterial struct {
	URL                    string `json:"url"`
	ObjectKey              string `json:"object_key"`
	ObjectStorageProfileID uint   `json:"object_storage_profile_id"`
	Name                   string `json:"name"`
	Size                   int64  `json:"size"`
	ContentType            string `json:"content_type"`
}
type MaterialUserResolver interface {
	FactUserID(context.Context, uint) (uint, error)
}

type MaterialUploadService struct {
	Users     MaterialUserResolver
	Scopes    requirement.MutationScopeResolver
	Access    project.AccessChecker
	Store     MaterialObjectStore
	Prefix    string
	PublicURL func(string) (string, bool)
}

var materialDigest = regexp.MustCompile(`^[a-f0-9]{64}$`)
var materialExtension = regexp.MustCompile(`^\.[a-zA-Z0-9]{1,16}$`)

func (s *MaterialUploadService) Upload(ctx context.Context, workspace, actor uint, key string, input MaterialUploadInput, source io.ReadSeeker) (UploadedMaterial, error) {
	if workspace == 0 || actor == 0 || key == "" || source == nil || input.Size <= 0 || input.Size > MaterialMaxBytes || !materialDigest.MatchString(input.SHA256) || strings.TrimSpace(input.Name) == "" {
		return UploadedMaterial{}, ErrBugMutationInvalid
	}
	if s.Scopes == nil || s.Access == nil || s.Store == nil || s.PublicURL == nil {
		return UploadedMaterial{}, ErrMaterialStorageUnavailable
	}
	if input.RequirementID != 0 {
		if s.Users == nil {
			return UploadedMaterial{}, ErrBugMutationNotFound
		}
		fact, err := s.Users.FactUserID(ctx, actor)
		if err != nil || fact == 0 {
			return UploadedMaterial{}, ErrBugMutationNotFound
		}
		scope, err := s.Scopes.ResolveMutationScope(ctx, workspace, input.RequirementID)
		if err != nil || scope.WorkspaceID != workspace {
			return UploadedMaterial{}, ErrBugMutationNotFound
		}
		if err := s.Access.RequireProjectAccess(ctx, workspace, scope.ProjectSetID, fact); err != nil {
			return UploadedMaterial{}, ErrBugMutationNotFound
		}
	}
	hash := sha256.New()
	count, err := io.Copy(hash, io.LimitReader(source, input.Size+1))
	if err != nil || count != input.Size || hex.EncodeToString(hash.Sum(nil)) != input.SHA256 {
		return UploadedMaterial{}, ErrMaterialFileMismatch
	}
	if _, err := source.Seek(0, io.SeekStart); err != nil {
		return UploadedMaterial{}, ErrMaterialFileMismatch
	}
	name := filepath.Base(strings.ReplaceAll(strings.TrimSpace(input.Name), "\\", "/"))
	ext := strings.ToLower(filepath.Ext(name))
	if !materialExtension.MatchString(ext) {
		ext = ""
	}
	// A recovered apply writes the identical bytes to the identical key. No
	// cross-workspace or cross-actor key can be supplied by a caller.
	operation := sha256.Sum256([]byte(key))
	objectKey := strings.Trim(s.Prefix, "/") + fmt.Sprintf("/workspace/%d/bug_materials/tools/%d/%x%s", workspace, actor, operation, ext)
	objectKey = strings.TrimLeft(objectKey, "/")
	url, ok := s.PublicURL(objectKey)
	if !ok {
		return UploadedMaterial{}, ErrMaterialStorageUnavailable
	}
	contentType := strings.TrimSpace(input.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if err := s.Store.PutObject(ctx, objectKey, source, input.Size, contentType); err != nil {
		return UploadedMaterial{}, ErrMaterialStorageUnavailable
	}
	return UploadedMaterial{URL: url, ObjectKey: objectKey, Name: name, Size: input.Size, ContentType: contentType}, nil
}
