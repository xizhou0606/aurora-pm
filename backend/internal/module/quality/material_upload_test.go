package quality

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
)

type uploadScope struct{}

func (uploadScope) FactUserID(context.Context, uint) (uint, error) { return 91, nil }

func (uploadScope) ResolveMutationScope(_ context.Context, w, id uint) (requirement.MutationScope, error) {
	if w != 7 || id != 9 {
		return requirement.MutationScope{}, ErrBugMutationNotFound
	}
	return requirement.MutationScope{WorkspaceID: 7, ProjectSetID: 8}, nil
}

type uploadAccess struct {
	project.AccessChecker
	deny bool
}

func (a *uploadAccess) RequireProjectAccess(_ context.Context, _ uint, _ uint, fact uint) error {
	if a.deny || fact != 91 {
		return project.ErrProjectAccessNotFound
	}
	return nil
}

type uploadStorage struct {
	keys []string
	body []byte
}

func (s *uploadStorage) PutObject(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
	s.keys = append(s.keys, key)
	s.body, _ = io.ReadAll(body)
	return nil
}

func TestMaterialUploadValidatesScopeFileAndUsesStableKey(t *testing.T) {
	storage, access := &uploadStorage{}, &uploadAccess{}
	s := &MaterialUploadService{Users: uploadScope{}, Scopes: uploadScope{}, Access: access, Store: storage, Prefix: "pm", PublicURL: func(key string) (string, bool) { return "https://cdn.example.test/" + key, true }}
	data := []byte("image bytes")
	input := MaterialUploadInput{RequirementID: 9, Name: "../proof.png", Size: int64(len(data)), SHA256: fmt.Sprintf("%x", sha256.Sum256(data)), ContentType: "image/png"}
	first, err := s.Upload(context.Background(), 7, 41, "cs-1", input, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Upload(context.Background(), 7, 41, "cs-1", input, bytes.NewReader(data))
	if err != nil || first != second || first.Name != "proof.png" || !bytes.Equal(storage.body, data) {
		t.Fatalf("replay=%+v %v", second, err)
	}
	for _, tc := range []struct {
		workspace uint
		deny      bool
		data      []byte
	}{{8, false, data}, {7, true, data}, {7, false, []byte("wrong bytes")}, {7, false, append(data, 'x')}} {
		access.deny = tc.deny
		_, err := s.Upload(context.Background(), tc.workspace, 41, "cs-2", input, bytes.NewReader(tc.data))
		if err == nil || len(storage.keys) != 2 {
			t.Fatalf("invalid upload reached storage: %v", err)
		}
	}
}
