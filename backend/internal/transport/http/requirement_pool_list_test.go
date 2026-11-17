package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
)

type requirementPoolListEnvelope struct {
	Data    []map[string]any       `json:"data"`
	Users   []listUserProfile      `json:"users"`
	Total   int64                  `json:"total"`
	Summary requirementPoolSummary `json:"summary"`
}

func decodeRequirementPoolList(
	t *testing.T,
	rec *httptest.ResponseRecorder,
) requirementPoolListEnvelope {
	t.Helper()
	var envelope requirementPoolListEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode requirement pool list: %v body=%s", err, rec.Body.String())
	}
	return envelope
}

func TestRequirementPoolListFiltersPaginationAndSummary(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	owner := app.createUser(t, 77, "pool-owner", "user", "")
	if err := app.db.Model(&identitygorm.User{}).Where("id = ?", owner.ID).Updates(map[string]any{
		"external_user_id": 707,
		"display_name":     "需求负责人",
		"avatar_url":       "https://cdn.example.test/pool-owner.png",
	}).Error; err != nil {
		t.Fatal(err)
	}
	ownerFactID := owner.ID
	otherProject := projectgorm.ProjectSet{
		ID: 13, WorkspaceID: workspace.ID, TeamID: project.TeamID, Name: "Other",
	}
	if err := app.db.Create(&otherProject).Error; err != nil {
		t.Fatal(err)
	}
	requirements := []requirementgorm.Requirement{
		{
			WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID,
			Title: "Checkout search", Description: "Payment flow", Status: "development", Priority: "p1", Version: 1,
		},
		{
			WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID, PlanUnitID: unit.ID,
			Title: "Acceptance", Status: "acceptance", Priority: "p1", Version: 1,
			ProductOwnerUserID: &ownerFactID,
		},
		{
			WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID, PlanUnitID: unit.ID,
			Title: "Backlog", Status: "backlog", Priority: "p3", Version: 1,
		},
		{
			WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: otherProject.ID,
			Title: "Other project", Status: "development", Priority: "p2", Version: 1,
		},
	}
	for index := range requirements {
		if err := app.db.Create(&requirements[index]).Error; err != nil {
			t.Fatal(err)
		}
	}

	filtered := app.request(
		t,
		http.MethodGet,
		"/api/v1/platform/requirements?project_new_id=11&priority=p1&limit=1&offset=0",
		token,
		workspace.ID,
		nil,
	)
	requireStatus(t, filtered, http.StatusOK)
	page := decodeRequirementPoolList(t, filtered)
	if page.Total != 2 || len(page.Data) != 1 {
		t.Fatalf("filtered page total=%d rows=%d", page.Total, len(page.Data))
	}
	if len(page.Users) != 1 || page.Users[0].ID != ownerFactID ||
		page.Users[0].DisplayName != "需求负责人" ||
		page.Users[0].AvatarURL != "https://cdn.example.test/pool-owner.png" {
		t.Fatalf("filtered page users=%+v", page.Users)
	}
	if page.Summary.Total != 2 || page.Summary.Unscheduled != 1 || page.Summary.Delivery != 2 {
		t.Fatalf("filtered summary=%+v", page.Summary)
	}

	searched := app.request(
		t,
		http.MethodGet,
		"/api/v1/platform/requirements?version_task_new_id=none&status=development&q=payment",
		token,
		workspace.ID,
		nil,
	)
	requireStatus(t, searched, http.StatusOK)
	searchPage := decodeRequirementPoolList(t, searched)
	if searchPage.Total != 1 || len(searchPage.Data) != 1 || searchPage.Data[0]["title"] != "Checkout search" {
		t.Fatalf("search page=%+v", searchPage)
	}

	listedAll := app.request(
		t,
		http.MethodGet,
		"/api/v1/platform/requirements?project_new_id=11&limit=all&offset=1",
		token,
		workspace.ID,
		nil,
	)
	requireStatus(t, listedAll, http.StatusOK)
	allPage := decodeRequirementPoolList(t, listedAll)
	if allPage.Total != 3 || len(allPage.Data) != 3 {
		t.Fatalf("limit=all should return every matching row total=%d rows=%d", allPage.Total, len(allPage.Data))
	}
	if allPage.Data[0]["title"] != "Backlog" || allPage.Data[2]["title"] != "Checkout search" {
		t.Fatalf("pool list should return newest created first titles=%v %v %v", allPage.Data[0]["title"], allPage.Data[1]["title"], allPage.Data[2]["title"])
	}

	multiScope := app.request(
		t,
		http.MethodGet,
		"/api/v1/platform/requirements?project_new_id=11,13",
		token,
		workspace.ID,
		nil,
	)
	requireStatus(t, multiScope, http.StatusOK)
	multiPage := decodeRequirementPoolList(t, multiScope)
	if multiPage.Total != 4 || len(multiPage.Data) != 4 {
		t.Fatalf("multi project filter total=%d rows=%d", multiPage.Total, len(multiPage.Data))
	}

	invalid := app.request(
		t,
		http.MethodGet,
		"/api/v1/platform/requirements?priority=critical",
		token,
		workspace.ID,
		nil,
	)
	requireStatus(t, invalid, http.StatusBadRequest)
}

func TestRequirementPoolListFiltersCurrentNodeAndRoleOwner(t *testing.T) {
	app, token, workspace, project, unit := newAIWorkflowFixture(t)
	uiOwner := uint(88)
	nearOwner := uint(880)
	now := time.Now().UTC()
	running := requirementgorm.Requirement{
		WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID, PlanUnitID: unit.ID,
		Title: "Review node", Status: "development", Priority: "p2", Version: 1,
		DesignOwnerUserID: &uiOwner, DesignOwnerUserIDs: requirementgorm.UintList{uiOwner},
		TesterUserIDs: requirementgorm.UintList{uiOwner},
	}
	idle := requirementgorm.Requirement{
		WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID, PlanUnitID: unit.ID,
		Title: "Idle node", Status: "backlog", Priority: "p3", Version: 1,
		DesignOwnerUserID: &nearOwner, DesignOwnerUserIDs: requirementgorm.UintList{nearOwner},
	}
	completed := requirementgorm.Requirement{
		WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID, PlanUnitID: unit.ID,
		Title: "Completed node", Status: "done", Priority: "normal", Version: 1,
		DesignOwnerUserIDs: requirementgorm.UintList{uiOwner, 9},
		TesterUserIDs:      requirementgorm.UintList{uiOwner, 9},
	}
	underscored := requirementgorm.Requirement{
		WorkspaceID: workspace.ID, TeamID: project.TeamID, ProjectSetID: project.ID, PlanUnitID: unit.ID,
		Title: "Underscored node", Status: "development", Priority: "high", Version: 1,
	}
	for _, row := range []*requirementgorm.Requirement{&running, &idle, &completed, &underscored} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	bind := func(requirement *requirementgorm.Requirement, status, nodes string) {
		t.Helper()
		instance := workflowgorm.WorkflowInstance{
			WorkspaceID: workspace.ID, TemplateID: 1, TemplateVersionID: 1,
			SubjectType: "requirement", SubjectID: requirement.ID, Status: status,
			CurrentNodeIDs: nodes, NodePlansJSON: "{}", StartedByUserID: 1, StartedAt: now,
		}
		if err := app.db.Create(&instance).Error; err != nil {
			t.Fatal(err)
		}
		if err := app.db.Model(requirement).Update("workflow_instance_id", instance.ID).Error; err != nil {
			t.Fatal(err)
		}
	}
	bind(&running, "running", `["review"]`)
	bind(&underscored, "running", `["product_review"]`)
	bind(&completed, "completed", `[]`)

	assertTitles := func(query string, want ...string) {
		t.Helper()
		rec := app.request(t, http.MethodGet, "/api/v1/platform/requirements?"+query, token, workspace.ID, nil)
		requireStatus(t, rec, http.StatusOK)
		page := decodeRequirementPoolList(t, rec)
		got := map[string]bool{}
		for _, row := range page.Data {
			title, _ := row["title"].(string)
			got[title] = true
		}
		if page.Total != int64(len(want)) || len(page.Data) != len(want) {
			t.Fatalf("%s total=%d rows=%d want=%v", query, page.Total, len(page.Data), want)
		}
		for _, title := range want {
			if !got[title] {
				t.Fatalf("%s missing %q in %v", query, title, got)
			}
		}
	}

	assertTitles("design_owner_user_id=88", "Review node", "Completed node")
	assertTitles("design_owner_user_id=88,880", "Review node", "Idle node", "Completed node")
	assertTitles("tester_user_id=88", "Review node", "Completed node")
	assertTitles("current_node_id=review", "Review node")
	assertTitles("current_node_id=product_review", "Underscored node")
	assertTitles("current_node_id=__idle__", "Idle node")
	assertTitles("current_node_id=__completed__", "Completed node")
	assertTitles("current_node_id=review,__idle__", "Review node", "Idle node")
	assertTitles("design_owner_user_id=88&current_node_id=review", "Review node")
	assertTitles("q="+strconv.FormatUint(uint64(running.ID), 10), "Review node")

	invalidNode := app.request(
		t,
		http.MethodGet,
		"/api/v1/platform/requirements?current_node_id=%25review",
		token,
		workspace.ID,
		nil,
	)
	requireStatus(t, invalidNode, http.StatusBadRequest)
}
