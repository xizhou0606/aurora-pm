package httpapi

import (
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"net/http"
	"testing"
)

func TestRequirementWorkspaceMembersCanManageAllFields(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "admin", "super_admin", "")
	productOwner := app.createUser(t, 2, "product", "user", "")
	designOwner := app.createUser(t, 3, "design", "user", "")
	otherOwner := app.createUser(t, 4, "other", "user", "")
	workspace := identitygorm.Workspace{ID: 1, Name: "研发", Slug: "dev", Enabled: true}
	team := identitygorm.Team{ID: 10, WorkspaceID: workspace.ID, Name: "平台"}
	project := projectgorm.ProjectSet{
		ID: 20, WorkspaceID: workspace.ID, TeamID: team.ID,
		Name: "PM", Code: "PM", OwnerUserID: &admin.ID, Status: "active",
	}
	unit := projectgorm.PlanUnit{
		ID: 30, WorkspaceID: workspace.ID, TeamID: team.ID,
		ProjectSetID: project.ID, Title: "V1", Type: "version", Status: "planning",
	}
	rows := []any{&workspace, &team, &project, &unit}
	for _, user := range []identitygorm.User{admin, productOwner, designOwner, otherOwner} {
		rows = append(rows, &identitygorm.WorkspaceMember{
			WorkspaceID: workspace.ID, UserID: user.ID, Role: "member", Enabled: true,
		})
		rows = append(rows, &identitygorm.TeamMember{
			WorkspaceID: workspace.ID, TeamID: team.ID, UserID: user.ID, Role: "member",
		})
	}
	for _, row := range rows {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	adminToken := app.token(t, admin)
	workflowTemplateID := createPublishedRequirementWorkflow(t, app, adminToken, workspace.ID)
	createRequirement := func(title string, productOwnerID uint) uint {
		rec := app.request(t, http.MethodPost, "/api/v1/platform/product-requirements-new",
			adminToken, workspace.ID, map[string]any{
				"version_task_new_id":   unit.ID,
				"title":                 title,
				"description":           "",
				"product_doc_url":       "",
				"product_owner_user_id": productOwnerID,
				"design_url":            "https://design.example/initial",
				"design_owner_user_id":  designOwner.ID,
				"developer_user_ids":    []uint{},
				"acceptance_criteria":   "",
				"priority":              "p1",
				"workflow_template_id":  workflowTemplateID,
			})
		requireStatus(t, rec, http.StatusCreated)
		return uint(decodeData(t, rec).(map[string]any)["id"].(float64))
	}
	ownedID := createRequirement("支付链路", productOwner.ID)
	otherID := createRequirement("其它需求", otherOwner.ID)
	var beforePatch requirementgorm.Requirement
	if err := app.db.First(&beforePatch, ownedID).Error; err != nil {
		t.Fatal(err)
	}

	productToken := app.token(t, productOwner)
	fullProductPatch := app.request(t, http.MethodPatch,
		"/api/v1/platform/product-requirements-new/"+uintString(ownedID),
		productToken, workspace.ID, map[string]any{
			"title":                 "支付链路改版",
			"description":           "",
			"product_doc_url":       "",
			"product_owner_user_id": productOwner.ID,
			"design_url":            "https://design.example/initial",
			"design_owner_user_id":  designOwner.ID,
			"developer_user_ids":    []uint{},
			"acceptance_criteria":   "失败原因可追踪",
			"priority":              "p2",
			"expected_version":      beforePatch.Version,
		})
	requireStatus(t, fullProductPatch, http.StatusOK)
	productDesignUpdate := app.request(t, http.MethodPatch,
		"/api/v1/platform/product-requirements-new/"+uintString(ownedID),
		productToken, workspace.ID, map[string]any{
			"title":            "工作区成员更新",
			"design_url":       "https://design.example/member",
			"expected_version": beforePatch.Version + 1,
		})
	requireStatus(t, productDesignUpdate, http.StatusOK)
	var updated requirementgorm.Requirement
	if err := app.db.First(&updated, ownedID).Error; err != nil {
		t.Fatal(err)
	}
	if updated.Title != "工作区成员更新" ||
		updated.DesignURL != "https://design.example/member" {
		t.Fatalf("workspace member field patch not persisted: %+v", updated)
	}
	if updated.Version != beforePatch.Version+2 {
		t.Fatalf("requirement version=%d, want %d after two content patches", updated.Version, beforePatch.Version+2)
	}

	designToken := app.token(t, designOwner)
	fullDesignPatch := app.request(t, http.MethodPatch,
		"/api/v1/platform/product-requirements-new/"+uintString(ownedID),
		designToken, workspace.ID, map[string]any{
			"title":                 "支付链路改版",
			"description":           "",
			"product_doc_url":       "",
			"product_owner_user_id": productOwner.ID,
			"design_url":            "https://design.example/v2",
			"design_owner_user_id":  designOwner.ID,
			"developer_user_ids":    []uint{},
			"acceptance_criteria":   "失败原因可追踪",
			"priority":              "p2",
			"expected_version":      beforePatch.Version + 2,
		})
	requireStatus(t, fullDesignPatch, http.StatusOK)
	designPriorityUpdate := app.request(t, http.MethodPatch,
		"/api/v1/platform/product-requirements-new/"+uintString(ownedID),
		designToken, workspace.ID, map[string]any{
			"priority": "p1", "expected_version": beforePatch.Version + 3,
		})
	requireStatus(t, designPriorityUpdate, http.StatusOK)

	// Deletion cascades every requirement-owned scope, including the mandatory
	// creation workflow runtime.
	deleteOther := app.request(t, http.MethodDelete,
		"/api/v1/platform/product-requirements-new/"+uintString(otherID),
		productToken, workspace.ID, nil)
	requireStatus(t, deleteOther, http.StatusOK)
	deleteOwned := app.request(t, http.MethodDelete,
		"/api/v1/platform/product-requirements-new/"+uintString(ownedID),
		productToken, workspace.ID, nil)
	requireStatus(t, deleteOwned, http.StatusOK)
}
