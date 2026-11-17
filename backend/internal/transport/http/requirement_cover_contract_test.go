package httpapi

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/integration/feishu"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/cover"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

type memoryRequirementCoverStore struct {
	objects map[string][]byte
	err     error
}

func (s *memoryRequirementCoverStore) PutObject(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
	if s.err != nil {
		return s.err
	}
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

func (s *memoryRequirementCoverStore) GetObject(_ context.Context, key string) (io.ReadCloser, error) {
	content, ok := s.objects[key]
	if s.err != nil || !ok {
		return nil, io.EOF
	}
	return io.NopCloser(bytes.NewReader(content)), nil
}

func (s *memoryRequirementCoverStore) RemoveObject(_ context.Context, key string) error {
	delete(s.objects, key)
	return nil
}

func TestRequirementCoverProxyReturnsPersistedWorkspaceScopedImage(t *testing.T) {
	app, token, requirementID, _ := newRequirementCoverFixture(t)
	cover := testRequirementCoverJPEG(t)
	store := &memoryRequirementCoverStore{objects: map[string][]byte{"pm/workspace/1/requirement-covers/requirement/101/cover.jpg": cover}}
	requirement := requirementgorm.Requirement{}
	if err := app.db.First(&requirement, requirementID).Error; err != nil {
		t.Fatal(err)
	}
	normalized, err := feishu.NormalizeDocumentURL(requirement.ProductDocURL, []string{"feishu.cn"})
	if err != nil {
		t.Fatal(err)
	}
	if err := app.db.Create(&requirementgorm.RequirementCover{
		WorkspaceID: 1, RequirementID: requirementID, SourceHash: requirementcover.SourceHash(normalized),
		Generation: 3, Status: requirementcover.StatusReady, ObjectKey: "pm/workspace/1/requirement-covers/requirement/101/cover.jpg",
		ContentType: "image/jpeg", Size: int64(len(cover)), Width: 640, Height: 320,
	}).Error; err != nil {
		t.Fatal(err)
	}
	app.rebuildEngine(t, store, nil)

	response := app.request(t, http.MethodGet,
		"/api/v1/platform/requirements/"+uintString(requirementID)+"/cover", token, 1, nil)
	requireStatus(t, response, http.StatusOK)
	if !bytes.Equal(response.Body.Bytes(), cover) {
		t.Fatalf("cover response bytes=%d want=%d", response.Body.Len(), len(cover))
	}
	if response.Header().Get("Content-Type") != "image/jpeg" ||
		response.Header().Get("Content-Disposition") != `inline; filename="requirement-cover.jpg"` ||
		response.Header().Get("X-Content-Type-Options") != "nosniff" ||
		response.Header().Get("Cache-Control") != "private, max-age=600" ||
		response.Header().Get("Vary") != "Origin, Authorization, X-Workspace-ID" ||
		response.Header().Get("ETag") == "" {
		t.Fatalf("cover response headers=%v", response.Header())
	}
}

func TestRequirementCoverProxyFailsClosed(t *testing.T) {
	tests := []struct {
		name        string
		documentURL string
		wantStatus  int
		wantCode    string
	}{
		{
			name:        "no persisted cover",
			documentURL: "https://acme.feishu.cn/docx/docx-token", wantStatus: http.StatusNotFound,
			wantCode: "REQUIREMENT_COVER_UNAVAILABLE",
		},
		{
			name:        "requirement has no document",
			documentURL: "", wantStatus: http.StatusNotFound,
			wantCode: "REQUIREMENT_COVER_UNAVAILABLE",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, token, requirementID, _ := newRequirementCoverFixture(t)
			if err := app.db.Model(&requirementgorm.Requirement{}).Where("id = ?", requirementID).
				Update("product_doc_url", test.documentURL).Error; err != nil {
				t.Fatal(err)
			}
			response := app.request(t, http.MethodGet,
				"/api/v1/platform/requirements/"+uintString(requirementID)+"/cover", token, 1, nil)
			requireStatus(t, response, test.wantStatus)
			if !strings.Contains(response.Body.String(), `"code":"`+test.wantCode+`"`) {
				t.Fatalf("cover error response=%s", response.Body.String())
			}
		})
	}
}

func TestRequirementCoverProxyRejectsInvalidPersistedContract(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		content     []byte
	}{
		{
			name:        "metadata is not the generated jpeg format",
			contentType: "image/png",
			content:     testRequirementCoverJPEG(t),
		},
		{
			name:        "object bytes are not jpeg",
			contentType: "image/jpeg",
			content:     []byte("not an image"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app, token, requirementID, _ := newRequirementCoverFixture(t)
			requirement := requirementgorm.Requirement{}
			if err := app.db.First(&requirement, requirementID).Error; err != nil {
				t.Fatal(err)
			}
			normalized, err := feishu.NormalizeDocumentURL(requirement.ProductDocURL, []string{"feishu.cn"})
			if err != nil {
				t.Fatal(err)
			}
			objectKey := "pm/workspace/1/requirement-covers/requirement/101/cover.jpg"
			if err := app.db.Create(&requirementgorm.RequirementCover{
				WorkspaceID: 1, RequirementID: requirementID, SourceHash: requirementcover.SourceHash(normalized),
				Generation: 1, Status: requirementcover.StatusReady, ObjectKey: objectKey,
				ContentType: test.contentType, Size: int64(len(test.content)), Width: 640, Height: 320,
			}).Error; err != nil {
				t.Fatal(err)
			}
			app.rebuildEngine(t,
				&memoryRequirementCoverStore{objects: map[string][]byte{objectKey: test.content}}, nil)

			response := app.request(t, http.MethodGet,
				"/api/v1/platform/requirements/"+uintString(requirementID)+"/cover", token, 1, nil)
			requireStatus(t, response, http.StatusBadGateway)
			if !strings.Contains(response.Body.String(), `"code":"FEISHU_COVER_UNAVAILABLE"`) {
				t.Fatalf("cover error response=%s", response.Body.String())
			}
		})
	}
}

func TestRequirementCoverProxyDoesNotReadAcrossWorkspaces(t *testing.T) {
	app, token, _, foreignRequirementID := newRequirementCoverFixture(t)

	response := app.request(t, http.MethodGet,
		"/api/v1/platform/requirements/"+uintString(foreignRequirementID)+"/cover", token, 1, nil)
	requireStatus(t, response, http.StatusNotFound)
}

func testRequirementCoverJPEG(t *testing.T) []byte {
	t.Helper()
	var output bytes.Buffer
	if err := jpeg.Encode(&output, image.NewRGBA(image.Rect(0, 0, 640, 320)), &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func newRequirementCoverFixture(
	t *testing.T,
) (*testApp, string, uint, uint) {
	t.Helper()
	app := newTestApp(t, func(cfg *config.Config) {
		cfg.Feishu.Enabled = true
		cfg.Feishu.DomainAllowlist = []string{"feishu.cn"}
		cfg.ObjectStorage.Enabled = true
	})
	user := app.createUser(t, 1, "cover-reader", "super_admin", "")
	workspace := identitygorm.Workspace{ID: 1, Name: "研发", Slug: "dev", Enabled: true}
	foreignWorkspace := identitygorm.Workspace{ID: 2, Name: "外部", Slug: "foreign", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: workspace.ID, Name: "平台"}
	foreignTeam := identitygorm.Team{ID: 20, WorkspaceID: foreignWorkspace.ID, Name: "外部团队"}
	project := projectgorm.ProjectSet{ID: 11, WorkspaceID: workspace.ID, TeamID: team.ID, Name: "PM", Code: "PM", OwnerUserID: &user.ID}
	foreignProject := projectgorm.ProjectSet{ID: 21, WorkspaceID: foreignWorkspace.ID, TeamID: foreignTeam.ID, Name: "Secret", Code: "SECRET", OwnerUserID: &user.ID}
	requirement := requirementgorm.Requirement{
		ID: 101, WorkspaceID: workspace.ID, TeamID: team.ID, ProjectSetID: project.ID,
		Title: "封面需求", ProductDocURL: "https://acme.feishu.cn/wiki/wiki-token", Priority: "normal", Status: "backlog", Version: 1,
	}
	foreignRequirement := requirementgorm.Requirement{
		ID: 202, WorkspaceID: foreignWorkspace.ID, TeamID: foreignTeam.ID, ProjectSetID: foreignProject.ID,
		Title: "外部需求", ProductDocURL: "https://secret.feishu.cn/docx/secret-token", Priority: "normal", Status: "backlog", Version: 1,
	}
	for _, row := range []any{
		&workspace, &foreignWorkspace, &team, &foreignTeam,
		&identitygorm.WorkspaceMember{WorkspaceID: workspace.ID, UserID: user.ID, Role: "owner", Enabled: true},
		&project, &foreignProject, &requirement, &foreignRequirement,
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return app, app.token(t, user), requirement.ID, foreignRequirement.ID
}
