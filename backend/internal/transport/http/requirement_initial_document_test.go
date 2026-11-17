package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/ai/docsource"
	aigorm "github.com/xizhou0606/aurora-pm/backend/internal/module/ai/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	requirement "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/resourceaccess"
	"gorm.io/gorm"
)

type initialLinkAccess struct{ denied bool }

func (a initialLinkAccess) LockResource(context.Context, uint, string) error { return nil }
func (a initialLinkAccess) RequireRequirement(_ context.Context, w, u, p uint, _ bool) error {
	if a.denied || w != 1 || u != 1 || p != 11 {
		return resourceaccess.ErrForbidden
	}
	return nil
}
func (a initialLinkAccess) Resource(_ context.Context, w, u uint, id string, _ bool) (requirement.ResourceLink, error) {
	if id != "selected-doc" {
		return requirement.ResourceLink{}, requirement.ErrResourceLinkNotFound
	}
	return requirement.ResourceLink{ResourceID: id, PolicyRevision: 1, CanManage: true}, nil
}
func (a initialLinkAccess) Candidates(context.Context, uint, uint) ([]requirement.ResourceLink, error) {
	return nil, nil
}
func (a initialLinkAccess) ApplyResourcePolicy(_ context.Context, c requirement.ResourceLinkCommand) error {
	if c.ExpectedPolicyRevision != 1 {
		return requirement.ErrResourceLinkConflict
	}
	return nil
}

func TestRequirementInitialDocumentPrecedesWorkflowAndRollsBackOnFailure(t *testing.T) {
	app, token, workspace, _, unit := newAIWorkflowFixture(t)
	denied := false
	registry, _ := docsource.NewRegistry(httpSourceProvider{})
	app.engine = NewWithDependencies(app.cfg, app.db, Dependencies{
		ProjectAccess: app.projectAccess,
		RequirementResourceLinks: func(tx *gorm.DB) *requirement.ResourceLinkService {
			return requirement.NewResourceLinkService(requirementgorm.NewResourceLinkRepository(tx, func(*gorm.DB) (requirement.ResourceLinkAccess, error) { return initialLinkAccess{denied: denied}, nil }))
		},
		DocumentSources: func(tx *gorm.DB) (*docsource.Service, error) {
			memberships, _ := identity.NewWorkspaceMembershipService(identitygorm.NewWorkspaceMembershipRepository(tx))
			return &docsource.Service{Users: identitygorm.NewLoginRepository(tx), Registry: registry, Requirements: requirementgorm.NewResourceLinkRepository(tx, nil), Actions: httpSourceProvider{}, Memberships: memberships, Projects: app.projectAccess}, nil
		},
	})
	definition := map[string]any{
		"nodes": []map[string]any{
			{"id": "start", "type": "start", "name": "开始"},
			{"id": "input", "type": "form", "name": "补充"},
			{"id": "ai", "type": "ai_check", "name": "检查", "capability": "requirement.completeness"},
			{"id": "review", "type": "approval", "name": "确认"},
			{"id": "end", "type": "end", "name": "结束"},
		},
		"edges": []map[string]any{
			{"id": "e1", "source": "start", "target": "ai"},
			{"id": "e2", "source": "input", "target": "ai", "loop": true},
			{"id": "e3", "source": "ai", "target": "review", "kind": "success"},
			{"id": "e4", "source": "ai", "target": "input", "kind": "failure", "loop": true},
			{"id": "e5", "source": "review", "target": "end", "kind": "approve"},
		},
	}
	response := app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates", token, workspace.ID, map[string]any{"name": "文档原子创建", "subject_type": "requirement", "definition": definition})
	requireStatus(t, response, http.StatusCreated)
	templateID := uint(decodeData(t, response).(map[string]any)["id"].(float64))
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/pm/workflows/templates/"+uintString(templateID)+"/publish", token, workspace.ID, nil), http.StatusOK)
	body := map[string]any{"project_new_id": 11, "version_task_new_id": unit.ID, "workflow_template_id": templateID, "title": "原子关联", "priority": "p1", "internal_document": map[string]any{"resource_id": "selected-doc", "expected_policy_revision": 1, "operation_id": "initial-op"}}
	denied = true
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, body), http.StatusForbidden)
	for _, model := range []any{&requirementgorm.Requirement{}, &requirementgorm.ResourceLinkRecord{}, &aigorm.AIRun{}} {
		var count int64
		app.db.Model(model).Count(&count)
		if count != 0 {
			t.Fatalf("denied creation left %T rows", model)
		}
	}
	denied = false
	selection := body["internal_document"].(map[string]any)
	selection["expected_policy_revision"] = 2
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, body), http.StatusConflict)
	selection["expected_policy_revision"] = 1
	selection["resource_id"] = "another-workspace-document"
	requireStatus(t, app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, body), http.StatusNotFound)
	selection["resource_id"] = "selected-doc"
	for _, model := range []any{&requirementgorm.Requirement{}, &requirementgorm.ResourceLinkRecord{}, &aigorm.AIRun{}} {
		var count int64
		app.db.Model(model).Count(&count)
		if count != 0 {
			t.Fatalf("failed association left %T rows", model)
		}
	}
	response = app.request(t, http.MethodPost, "/api/v1/platform/requirements", token, workspace.ID, body)
	requireStatus(t, response, http.StatusCreated)
	var row requirementgorm.Requirement
	if err := app.db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	var run aigorm.AIRun
	if err := app.db.Where("subject_id = ?", row.ID).First(&run).Error; err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	if err := json.Unmarshal([]byte(run.InputJSON), &input); err != nil {
		t.Fatal(err)
	}
	manifest, err := docsource.DecodeManifest(input)
	if err != nil || len(manifest.Sources) != 1 || manifest.Sources[0].ResourceID != "selected-doc" || run.SubjectVersion != row.Version {
		t.Fatalf("initial run did not capture committed document/version: %+v %v", manifest, err)
	}
}
