package httpapi

import (
	"net/http"
	"testing"
	"time"

	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func TestBatchCreateTaskChildrenInheritsScopeAndCreatesAtomically(t *testing.T) {
	fix := setupTeamUserAssignmentFixture(t)
	due := time.Date(2026, 8, 14, 23, 59, 59, 0, time.UTC)
	assigneeID := fix.users["member"]
	parent := taskgorm.WorkTask{
		ID: 701, WorkspaceID: fix.workspaceID, TeamID: fix.teamID,
		ProjectSetID: fix.projectID, ProjectID: fix.planID, RequirementID: fix.requirement,
		Kind: taskgorm.WorkTaskKindTask, TaskKey: "ASG-701", Title: "Existing parent",
		Status: "draft", Priority: "high", AssigneeID: &assigneeID, DueAt: &due,
	}
	if err := fix.app.db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	response := fix.app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/701/children/batch", fix.token, fix.workspaceID,
		map[string]any{"children": []map[string]any{
			{"client_id": "child-a", "title": "Child A"},
			{"client_id": "child-b", "title": "Child B", "description": "Detailed child",
				"status": "in_dev", "priority": "urgent", "reporter_user_id": assigneeID},
		}},
	)
	requireStatus(t, response, http.StatusCreated)
	data := decodeData(t, response).(map[string]any)
	if data["parent_id"] != float64(parent.ID) || data["created_count"] != float64(2) || data["atomic"] != true {
		t.Fatalf("children batch response=%s", response.Body.String())
	}
	created := data["created"].([]any)
	if created[0].(map[string]any)["client_id"] != "child-a" ||
		created[1].(map[string]any)["client_id"] != "child-b" {
		t.Fatalf("client mapping=%s", response.Body.String())
	}
	var children []taskgorm.WorkTask
	if err := fix.app.db.Where("parent_id = ?", parent.ID).Order("id ASC").Find(&children).Error; err != nil {
		t.Fatal(err)
	}
	if len(children) != 2 {
		t.Fatalf("created children=%d", len(children))
	}
	if children[0].RequirementID != parent.RequirementID || children[0].ProjectSetID != parent.ProjectSetID ||
		children[0].Kind != taskgorm.WorkTaskKindSubtask || children[0].AssigneeID == nil ||
		*children[0].AssigneeID != assigneeID || children[0].DueAt == nil ||
		!children[0].DueAt.Equal(due) || children[0].Priority != "high" ||
		children[1].Priority != "urgent" || children[1].Description != "Detailed child" ||
		children[1].Status != "in_dev" || children[1].ReporterUserID == nil ||
		*children[1].ReporterUserID != assigneeID {
		t.Fatalf("child inheritance invalid: %+v", children)
	}
	assertBatchMutationCounts(t, fix.app, 2)
}

func TestBatchCreateTaskChildrenRejectsInvalidInputWithoutWrites(t *testing.T) {
	fix := setupTeamUserAssignmentFixture(t)
	parent := taskgorm.WorkTask{
		ID: 702, WorkspaceID: fix.workspaceID, TeamID: fix.teamID,
		ProjectSetID: fix.projectID, ProjectID: fix.planID, RequirementID: fix.requirement,
		Kind: taskgorm.WorkTaskKindTask, TaskKey: "ASG-702", Title: "Existing parent",
		Status: "draft", Priority: "normal",
	}
	if err := fix.app.db.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	response := fix.app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/702/children/batch", fix.token, fix.workspaceID,
		map[string]any{"children": []map[string]any{
			{"client_id": "same", "title": "Valid"},
			{"client_id": "same", "title": "Duplicate"},
		}},
	)
	requireStatus(t, response, http.StatusUnprocessableEntity)
	var count int64
	fix.app.db.Model(&taskgorm.WorkTask{}).Where("parent_id = ?", parent.ID).Count(&count)
	if count != 0 {
		t.Fatalf("invalid batch wrote %d children", count)
	}
	assertBatchMutationCounts(t, fix.app, 0)
}

func TestTaskDeletionImpactReturnsCompleteTreeAndRelatedCounts(t *testing.T) {
	fix := setupTeamUserAssignmentFixture(t)
	root := taskgorm.WorkTask{
		ID: 710, WorkspaceID: fix.workspaceID, TeamID: fix.teamID,
		ProjectSetID: fix.projectID, ProjectID: fix.planID, RequirementID: fix.requirement,
		Kind: taskgorm.WorkTaskKindTask, TaskKey: "ASG-710", Title: "Root",
		Status: "draft", Priority: "normal",
	}
	child := root
	child.ID, child.TaskKey, child.Title, child.Kind, child.ParentID =
		711, "ASG-711", "Child", taskgorm.WorkTaskKindSubtask, &root.ID
	grandchild := root
	grandchild.ID, grandchild.TaskKey, grandchild.Title, grandchild.Kind, grandchild.ParentID =
		712, "ASG-712", "Grandchild", taskgorm.WorkTaskKindSubtask, &child.ID
	for _, row := range []any{&root, &child, &grandchild} {
		if err := fix.app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []any{
		&taskgorm.WorkTaskComment{WorkspaceID: fix.workspaceID, WorkTaskID: child.ID, UserID: 1, Body: "comment"},
		&taskgorm.WorkTaskActivity{WorkspaceID: fix.workspaceID, WorkTaskID: root.ID, UserID: 1, Action: "created"},
		&taskgorm.WorkTaskWatcher{WorkspaceID: fix.workspaceID, WorkTaskID: grandchild.ID, UserID: 1},
		&taskgorm.WorkTaskAttachment{WorkspaceID: fix.workspaceID, WorkTaskID: child.ID, UploaderUserID: 1, Filename: "a.txt", Size: 1, ObjectKey: "a", StorageBackend: "s3"},
	} {
		if err := fix.app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	bug := qualitygorm.QualityBug{
		WorkspaceID: fix.workspaceID, TeamID: fix.teamID, ProjectSetID: fix.projectID,
		ProjectID: fix.planID, RequirementID: fix.requirement, BugKey: "BUG-710",
		Title: "reference", Status: "triage", Priority: "medium", SeverityKey: "p2",
		SourceExecutionID: &grandchild.ID,
	}
	if err := fix.app.db.Create(&bug).Error; err != nil {
		t.Fatal(err)
	}
	response := fix.app.request(t, http.MethodGet,
		"/api/v1/platform/tasks/710/deletion-impact", fix.token, fix.workspaceID, nil)
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	if data["affected_count"] != float64(3) || data["cascade_required"] != true ||
		len(data["affected_ids"].([]any)) != 3 || len(data["descendants"].([]any)) != 2 {
		t.Fatalf("impact tree=%s", response.Body.String())
	}
	rootImpact := data["root"].(map[string]any)
	if rootImpact["team_id"] != float64(fix.teamID) ||
		rootImpact["project_set_id"] != float64(fix.projectID) ||
		rootImpact["requirement_id"] != float64(fix.requirement) {
		t.Fatalf("impact scope=%v body=%s", rootImpact, response.Body.String())
	}
	related := data["related"].(map[string]any)
	for _, key := range []string{"comments", "attachments", "activities", "watchers", "quality_source_references"} {
		if related[key] != float64(1) {
			t.Fatalf("related[%s]=%v body=%s", key, related[key], response.Body.String())
		}
	}
}
