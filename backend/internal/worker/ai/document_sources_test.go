package aiworker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai/docsource"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/document"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/database"
	"gorm.io/gorm"
)

type workerDocumentSource struct {
	calls    atomic.Int32
	changeAt int32
	denied   atomic.Bool
}

func (f *workerDocumentSource) Kind() string { return docsource.Internal }
func (f *workerDocumentSource) Read(context.Context, docsource.ReadContext, docsource.SourceRef) (docsource.Snapshot, error) {
	calls := f.calls.Add(1)
	revision := "1"
	if f.changeAt > 0 && calls >= f.changeAt {
		revision = "2"
	}
	return docsource.Snapshot{Type: docsource.Internal, Content: "私有正文只进模型内存", Available: true, Revision: revision, ContentHash: "hash", Unexpanded: []string{"image"}}, nil
}
func (f *workerDocumentSource) ReadDocumentSource(context.Context, uint, uint) (requirement.DocumentSource, error) {
	return requirement.DocumentSource{RequirementID: 1, Version: 1, ProjectID: 1, ResourceIDs: []string{"doc"}}, nil
}
func (f *workerDocumentSource) FindActiveUserByID(context.Context, uint) (identity.User, error) {
	return identity.User{ID: 11}, nil
}
func (f *workerDocumentSource) VerifyWorkspaceMembership(context.Context, uint, uint) error {
	if f.denied.Load() {
		return docsource.ErrUnavailable
	}
	return nil
}
func (f *workerDocumentSource) RequireProjectAccess(context.Context, uint, uint, uint) error {
	return nil
}
func (f *workerDocumentSource) Authorize(context.Context, uint, string, string) (identity.AuthorizationResult, error) {
	return identity.AuthorizationAllowed, nil
}

func TestDocumentSourcesReachModelAndRevalidateBeforePublication(t *testing.T) {
	for _, test := range []struct {
		name     string
		changeAt int32
		revoke   bool
		failed   bool
	}{{name: "success"}, {name: "edit during model", changeAt: 2, failed: true}, {name: "edit in commit window", changeAt: 3, failed: true}, {name: "revoke during model", revoke: true, failed: true}} {
		t.Run(test.name, func(t *testing.T) {
			source := &workerDocumentSource{changeAt: test.changeAt}
			registry, _ := docsource.NewRegistry(source)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				if !strings.Contains(string(raw), "私有正文只进模型内存") || strings.Contains(string(raw), docsource.ManifestKey) || !strings.Contains(string(raw), "requirement_ids") {
					t.Errorf("unexpected runtime payload %s", raw)
				}
				if test.revoke {
					source.denied.Store(true)
				}
				_, _ = w.Write([]byte(`{"data":{"output":{"summary":"分析完成","changes":[]},"usage":{}}}`))
			}))
			defer server.Close()
			cfg := config.Config{DatabaseDriver: "sqlite", DatabaseDSN: "file:" + strings.ReplaceAll(t.Name(), " ", "_") + "?mode=memory&cache=shared", AI: config.AI{Enabled: true, RuntimeBaseURL: server.URL, Model: "test", RequestTimeout: time.Second, InternalSecret: "test-internal-secret-at-least-32-characters"}}
			db, err := database.Open(cfg)
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := db.DB()
			defer sqlDB.Close()
			if err = database.Migrate(db); err != nil {
				t.Fatal(err)
			}
			req := docsource.RequirementRef{ID: 1, Version: 1, ProjectID: 1}
			m := docsource.Manifest{Version: 1, Context: docsource.ReadContext{WorkspaceID: 1, UserID: 11, FactUserID: 11, Method: "POST", Route: "/test"}, Requirements: []docsource.RequirementRef{req}, Sources: []docsource.SourceRef{{Kind: docsource.Internal, ResourceID: "doc", ConverterVersion: document.AnalysisProjectionVersion, Requirements: []docsource.RequirementRef{req}}}}
			raw, _ := json.Marshal(map[string]any{"capability": "requirement.completeness", "subject": map[string]any{"id": 1}, docsource.ManifestKey: m})
			run := aigorm.AIRun{WorkspaceID: 1, SubjectType: "requirement", SubjectID: 1, SubjectVersion: 1, Capability: "requirement.completeness", Status: "queued", InputJSON: string(raw), RequestedByUser: 11}
			if err = db.Create(&run).Error; err != nil {
				t.Fatal(err)
			}
			if err = aigorm.SaveDocumentSources(db, run, &m); err != nil {
				t.Fatal(err)
			}
			worker := New(db, cfg, nil)
			worker.ConfigureDocumentSources(func(*gorm.DB) (*docsource.Service, error) {
				return &docsource.Service{Registry: registry, Users: source, Actions: source, Requirements: source, Memberships: source, Projects: source}, nil
			})
			_, runErr := worker.RunOnce(context.Background())
			if test.failed != (runErr != nil) {
				t.Fatalf("run err=%v failed=%v", runErr, test.failed)
			}
			if err = db.First(&run, run.ID).Error; err != nil {
				t.Fatal(err)
			}
			var artifacts int64
			db.Model(&aigorm.AIArtifact{}).Where("run_id = ?", run.ID).Count(&artifacts)
			if test.failed && (run.Status != "failed" || artifacts != 0) {
				t.Fatalf("published stale result: %s artifacts=%d", run.Status, artifacts)
			}
			if !test.failed && (run.Status != "completed" || artifacts != 1 || !strings.Contains(run.OutputJSON, "未展开")) {
				t.Fatalf("missing result/disclosure: %+v", run)
			}
			var rows []aigorm.AIRunSource
			db.Where("run_id = ?", run.ID).Find(&rows)
			stored, _ := json.Marshal(rows)
			if strings.Contains(run.InputJSON+string(stored), "私有正文") {
				t.Fatal("body persisted")
			}
		})
	}
}
