package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai/docsource"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"gorm.io/gorm"
	"net/http"
	"strings"
	"testing"
)

type httpSourceProvider struct{}

func (httpSourceProvider) Kind() string { return docsource.Internal }
func (httpSourceProvider) Read(_ context.Context, _ docsource.ReadContext, ref docsource.SourceRef) (docsource.Snapshot, error) {
	return docsource.Snapshot{Type: docsource.Internal, ResourceID: ref.ResourceID, Content: "private-body", Revision: "1", ContentHash: "hash", Available: true}, nil
}
func (httpSourceProvider) Authorize(context.Context, uint, string, string) (identity.AuthorizationResult, error) {
	return identity.AuthorizationAllowed, nil
}

func TestAIRunRetryRefreshesSourceAndOperatorWithoutExposingServerManifest(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	requirement := seedTaskPlanRequirement(t, app, workspace, project, unit, 1)
	link := requirementgorm.ResourceLinkRecord{WorkspaceID: workspace.ID, RequirementID: requirement.ID, ResourceID: "old-doc"}
	if err := app.db.Create(&link).Error; err != nil {
		t.Fatal(err)
	}
	registry, _ := docsource.NewRegistry(httpSourceProvider{})
	app.engine = NewWithDependencies(app.cfg, app.db, Dependencies{ProjectAccess: app.projectAccess, DocumentSources: func(tx *gorm.DB) (*docsource.Service, error) {
		memberships, _ := identity.NewWorkspaceMembershipService(identitygorm.NewWorkspaceMembershipRepository(tx))
		return &docsource.Service{Users: identitygorm.NewLoginRepository(tx), Registry: registry, Requirements: requirementgorm.NewResourceLinkRepository(tx, nil), Actions: httpSourceProvider{}, Memberships: memberships, Projects: app.projectAccess}, nil
	}})
	response := app.request(t, http.MethodPost, "/api/v1/platform/ai/runs", token, workspace.ID, map[string]any{"capability": "requirement.completeness", "subject_type": "requirement", "subject_id": requirement.ID})
	if response.Code != http.StatusCreated {
		t.Fatalf("create %d %s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), docsource.ManifestKey) || strings.Contains(response.Body.String(), "private-body") {
		t.Fatal("server metadata or body exposed")
	}
	var run aigorm.AIRun
	if err := app.db.Where("subject_id = ?", requirement.ID).First(&run).Error; err != nil {
		t.Fatal(err)
	}
	originalInput := run.InputJSON
	if err := app.db.Model(&run).Update("status", "failed").Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&requirement).Update("version", 2).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.Model(&link).Update("resource_id", "new-doc").Error; err != nil {
		t.Fatal(err)
	}
	next := app.createUser(t, 2, "retry-user", "super_admin", "")
	if err := app.db.Create(&identitygorm.WorkspaceMember{WorkspaceID: workspace.ID, UserID: next.ID, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	response = app.request(t, http.MethodPost, fmt.Sprintf("/api/v1/platform/ai/runs/%d/retry", run.ID), app.token(t, next), workspace.ID, nil)
	if response.Code != http.StatusOK {
		t.Fatalf("retry %d %s", response.Code, response.Body.String())
	}
	if err := app.db.First(&run, run.ID).Error; err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	if json.Unmarshal([]byte(run.InputJSON), &input) != nil {
		t.Fatal("invalid run input")
	}
	manifest, err := docsource.DecodeManifest(input)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Context.UserID != 2 || manifest.Sources[0].ResourceID != "new-doc" || run.SubjectVersion != 2 || run.RequestedByUser != 2 || run.InputJSON == originalInput {
		t.Fatalf("stale retry %+v", run)
	}
	var count int64
	app.db.Model(&aigorm.AIRun{}).Count(&count)
	if count != 1 {
		t.Fatal("retry broke workflow run identity")
	}
}
