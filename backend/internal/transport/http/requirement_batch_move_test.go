package httpapi

import (
	"net/http"
	"testing"

	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func TestBatchMoveRequirementsIsAtomicAndMovesChildScopes(t *testing.T) {
	fix := setupRequirementMoveFixture(t)
	secondID, secondVersion := createBatchMoveRequirement(t, fix, "退款链路")
	createTask := fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks",
		fix.app.token(t, fix.actor), fix.workspace.ID,
		map[string]any{"requirement_id": secondID, "title": "退款接口"})
	requireStatus(t, createTask, http.StatusCreated)
	taskID := uint(decodeData(t, createTask).(map[string]any)["id"].(float64))
	reporterID := fix.actor.ID
	bug := qualitygorm.QualityBug{
		WorkspaceID: fix.workspace.ID, TeamID: fix.team.ID,
		ProjectSetID: fix.sourceProject.ID, ProjectID: fix.sourceUnit.ID,
		RequirementID: fix.requirementID, BugKey: "SRCB-BATCH", Title: "批量移动缺陷",
		Status: "triage", Priority: "medium", SeverityKey: "p2", ReporterUserID: &reporterID,
	}
	if err := fix.app.db.Create(&bug).Error; err != nil {
		t.Fatal(err)
	}

	rec := fix.app.request(t, http.MethodPost, "/api/v1/platform/requirements/batch-move",
		fix.app.token(t, fix.actor), fix.workspace.ID, map[string]any{
			"items": []map[string]any{
				{"requirement_id": secondID, "expected_version": secondVersion},
				{"requirement_id": fix.requirementID, "expected_version": fix.requirementVersion},
			},
			"version_task_new_id": fix.crossUnit.ID,
		})
	requireStatus(t, rec, http.StatusOK)
	data := decodeData(t, rec).(map[string]any)
	if data["moved_count"].(float64) != 2 || data["batch_id"] == "" {
		t.Fatalf("batch response=%+v", data)
	}

	for _, id := range []uint{fix.requirementID, secondID} {
		var requirement requirementgorm.Requirement
		if err := fix.app.db.First(&requirement, id).Error; err != nil {
			t.Fatal(err)
		}
		if requirement.ProjectSetID != fix.targetProject.ID || requirement.PlanUnitID != fix.crossUnit.ID {
			t.Fatalf("requirement %d scope=%+v", id, requirement)
		}
		if requirement.WorkflowInstanceID == nil || *requirement.WorkflowInstanceID == 0 {
			t.Fatalf("requirement %d workflow binding was lost", id)
		}
	}
	var task taskgorm.WorkTask
	if err := fix.app.db.First(&task, taskID).Error; err != nil {
		t.Fatal(err)
	}
	if task.ProjectSetID != fix.targetProject.ID || task.ProjectID != fix.crossUnit.ID {
		t.Fatalf("task scope=%+v", task)
	}
	var movedBug qualitygorm.QualityBug
	if err := fix.app.db.First(&movedBug, bug.ID).Error; err != nil {
		t.Fatal(err)
	}
	if movedBug.ProjectSetID != fix.targetProject.ID || movedBug.ProjectID != fix.crossUnit.ID {
		t.Fatalf("bug scope=%+v", movedBug)
	}
}

func TestBatchMoveRequirementsStaleItemRollsBackEveryRequirement(t *testing.T) {
	fix := setupRequirementMoveFixture(t)
	secondID, secondVersion := createBatchMoveRequirement(t, fix, "库存链路")
	rec := fix.app.request(t, http.MethodPost, "/api/v1/platform/requirements/batch-move",
		fix.app.token(t, fix.actor), fix.workspace.ID, map[string]any{
			"items": []map[string]any{
				{"requirement_id": fix.requirementID, "expected_version": fix.requirementVersion},
				{"requirement_id": secondID, "expected_version": secondVersion + 1},
			},
			"version_task_new_id": fix.crossUnit.ID,
		})
	requireStatus(t, rec, http.StatusConflict)
	for _, id := range []uint{fix.requirementID, secondID} {
		var requirement requirementgorm.Requirement
		if err := fix.app.db.First(&requirement, id).Error; err != nil {
			t.Fatal(err)
		}
		if requirement.ProjectSetID != fix.sourceProject.ID || requirement.PlanUnitID != fix.sourceUnit.ID {
			t.Fatalf("stale batch partially moved requirement %d: %+v", id, requirement)
		}
	}
}

func TestBatchMoveRequirementsRejectsDuplicateIDs(t *testing.T) {
	fix := setupRequirementMoveFixture(t)
	rec := fix.app.request(t, http.MethodPost, "/api/v1/platform/requirements/batch-move",
		fix.app.token(t, fix.actor), fix.workspace.ID, map[string]any{
			"items": []map[string]any{
				{"requirement_id": fix.requirementID, "expected_version": fix.requirementVersion},
				{"requirement_id": fix.requirementID, "expected_version": fix.requirementVersion},
			},
			"version_task_new_id": fix.crossUnit.ID,
		})
	requireStatus(t, rec, http.StatusBadRequest)
}

func TestRequirementListCanResolveByTitleWithoutGuessingAcrossWorkspaces(t *testing.T) {
	fix := setupRequirementMoveFixture(t)
	rec := fix.app.request(t, http.MethodGet, "/api/v1/platform/product-requirements-new?q=支付链路",
		fix.app.token(t, fix.actor), fix.workspace.ID, nil)
	requireStatus(t, rec, http.StatusOK)
	data := decodeData(t, rec).([]any)
	if len(data) != 1 || uint(data[0].(map[string]any)["id"].(float64)) != fix.requirementID {
		t.Fatalf("requirement search=%+v", data)
	}
}

func createBatchMoveRequirement(t *testing.T, fix requirementMoveFixture, title string) (uint, uint) {
	t.Helper()
	rec := fix.app.request(t, http.MethodPost, "/api/v1/platform/product-requirements-new",
		fix.app.token(t, fix.actor), fix.workspace.ID, map[string]any{
			"version_task_new_id": fix.sourceUnit.ID, "title": title,
			"product_owner_user_id": fix.productOwner.ID,
			"workflow_template_id":  fix.workflowTemplateID,
			"priority":              "p1",
		})
	requireStatus(t, rec, http.StatusCreated)
	data := decodeData(t, rec).(map[string]any)
	return uint(data["id"].(float64)), uint(data["version"].(float64))
}
