package httpapi

import (
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"net/http"
	"testing"
)

func TestTaskBatchDeleteOrdersChildrenAndKeepsWorkspaceIsolation(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "batch-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	parent := taskgorm.WorkTask{
		ID: 901, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "task", TaskKey: "A-901", Title: "Parent",
		Status: "draft", Priority: "normal",
	}
	child := taskgorm.WorkTask{
		ID: 902, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "subtask", TaskKey: "A-902", Title: "Child",
		Status: "draft", Priority: "normal", ParentID: &parent.ID,
	}
	foreign := taskgorm.WorkTask{
		ID: 903, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, ProjectID: 22,
		RequirementID: 23, Kind: "task", TaskKey: "B-903", Title: "Foreign",
		Status: "draft", Priority: "normal",
	}
	bug := qualitygorm.QualityBug{
		ID: 904, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, BugKey: "A-B-904", Title: "References child",
		Status: "triage", Priority: "medium", SeverityKey: "p2",
		SourceExecutionID: &child.ID, SourceTestID: &child.ID,
	}
	for _, row := range []any{&parent, &child, &foreign, &bug} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	response := app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-delete", token, 1,
		map[string]any{"ids": []uint{parent.ID, child.ID, foreign.ID}})
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	if data["deleted"] != float64(2) || data["failed"] != float64(1) {
		t.Fatalf("batch result=%s", response.Body.String())
	}
	var localCount, foreignCount int64
	app.db.Model(&taskgorm.WorkTask{}).Where("id IN ?", []uint{parent.ID, child.ID}).Count(&localCount)
	app.db.Model(&taskgorm.WorkTask{}).Where("id = ?", foreign.ID).Count(&foreignCount)
	if localCount != 0 || foreignCount != 1 {
		t.Fatalf("local=%d foreign=%d", localCount, foreignCount)
	}
	var storedBug qualitygorm.QualityBug
	if err := app.db.First(&storedBug, bug.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedBug.SourceExecutionID != nil || storedBug.SourceTestID != nil {
		t.Fatalf("quality source refs=%v/%v", storedBug.SourceExecutionID, storedBug.SourceTestID)
	}
}

func TestTaskBatchDeleteAtomicRejectsIncompleteTreeWithoutWrites(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "atomic-batch-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	parent := taskgorm.WorkTask{
		ID: 911, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "task", TaskKey: "A-911", Title: "Parent",
		Status: "draft", Priority: "normal",
	}
	child := parent
	child.ID, child.Kind, child.TaskKey, child.Title, child.ParentID =
		912, "subtask", "A-912", "Child", &parent.ID
	for _, row := range []*taskgorm.WorkTask{&parent, &child} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	response := app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-delete", token, 1,
		map[string]any{"ids": []uint{parent.ID}, "atomic": true})
	requireStatus(t, response, http.StatusConflict)
	var retained int64
	app.db.Model(&taskgorm.WorkTask{}).Where("id IN ?", []uint{parent.ID, child.ID}).Count(&retained)
	if retained != 2 {
		t.Fatalf("incomplete atomic batch retained=%d", retained)
	}
}

func TestTaskBatchDeleteCascadeExpandsSelectedParentTree(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "cascade-batch-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	parent := taskgorm.WorkTask{
		ID: 931, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "task", TaskKey: "A-931", Title: "Parent",
		Status: "draft", Priority: "normal",
	}
	child := parent
	child.ID, child.Kind, child.TaskKey, child.Title, child.ParentID =
		932, "subtask", "A-932", "Child", &parent.ID
	grandchild := child
	grandchild.ID, grandchild.TaskKey, grandchild.Title, grandchild.ParentID =
		933, "A-933", "Grandchild", &child.ID
	for _, row := range []*taskgorm.WorkTask{&parent, &child, &grandchild} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	response := app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-delete", token, 1,
		map[string]any{"ids": []uint{parent.ID}, "cascade": true})
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	if data["deleted"] != float64(3) || data["failed"] != float64(0) {
		t.Fatalf("cascade batch result=%s", response.Body.String())
	}
	var remaining int64
	app.db.Model(&taskgorm.WorkTask{}).Where("id IN ?", []uint{parent.ID, child.ID, grandchild.ID}).Count(&remaining)
	if remaining != 0 {
		t.Fatalf("cascade batch left %d tasks", remaining)
	}
}

func TestTaskBatchDeleteAtomicDeletesCompleteTreeWithAuditAndOutbox(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "atomic-batch-admin", "super_admin", "")
	seedNativeWorkbenchScopes(t, app, admin)
	token := app.token(t, admin)
	parent := taskgorm.WorkTask{
		ID: 921, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: "task", TaskKey: "A-921", Title: "Parent",
		Status: "draft", Priority: "normal",
	}
	child := parent
	child.ID, child.Kind, child.TaskKey, child.Title, child.ParentID =
		922, "subtask", "A-922", "Child", &parent.ID
	for _, row := range []*taskgorm.WorkTask{&parent, &child} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := app.db.Create(&taskgorm.WorkTaskComment{
		WorkspaceID: 1, WorkTaskID: child.ID, UserID: admin.ID, Body: "dependent",
	}).Error; err != nil {
		t.Fatal(err)
	}

	response := app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-delete", token, 1,
		map[string]any{"ids": []uint{parent.ID, child.ID}, "atomic": true})
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	if data["deleted"] != float64(2) || data["failed"] != float64(0) || data["atomic"] != true {
		t.Fatalf("atomic batch response=%s", response.Body.String())
	}
	var tasks, comments, audits, outbox int64
	app.db.Model(&taskgorm.WorkTask{}).Where("id IN ?", []uint{parent.ID, child.ID}).Count(&tasks)
	app.db.Model(&taskgorm.WorkTaskComment{}).Where("work_task_id IN ?", []uint{parent.ID, child.ID}).Count(&comments)
	app.db.Model(&auditgorm.AuditLog{}).Where("resource_id IN ?", []uint{parent.ID, child.ID}).Count(&audits)
	app.db.Model(&auditgorm.OutboxEvent{}).Where("aggregate_id IN ?", []uint{parent.ID, child.ID}).Count(&outbox)
	if tasks != 0 || comments != 0 || audits != 2 || outbox != 2 {
		t.Fatalf("tasks=%d comments=%d audits=%d outbox=%d", tasks, comments, audits, outbox)
	}
}
