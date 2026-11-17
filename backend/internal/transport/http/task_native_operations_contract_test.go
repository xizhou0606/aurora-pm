package httpapi

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func newNativeTaskOperationsTestApp(t *testing.T, mutate func(*config.Config)) *testApp {
	t.Helper()
	app := newTestApp(t, mutate)
	engine := app.engine.(*gin.Engine)
	mounted := false
	for _, route := range engine.Routes() {
		if route.Method == http.MethodPost && route.Path == "/api/v1/platform/tasks/:id/clone" {
			mounted = true
			break
		}
	}
	if !mounted {
		server := &Server{
			cfg: app.cfg, db: app.db, tokens: app.tokens, authCache: map[string]cachedIdentity{},
		}
		scoped := engine.Group("/api/v1/platform")
		scoped.Use(server.authenticate, server.requireWorkspace)
		server.mountTaskNativeOperationRoutes(scoped)
	}
	return app
}

func seedNativeTaskOperationScopes(t *testing.T, app *testApp) (identitygorm.User, string) {
	t.Helper()
	admin := app.createUser(t, 1, "native-task-admin", "super_admin", "")
	worker := app.createUser(t, 2, "native-task-worker", "developer", "")
	seedNativeWorkbenchScopes(t, app, admin)
	for _, row := range []any{
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: worker.ID, Role: "member", Enabled: true},
		&identitygorm.TeamMember{WorkspaceID: 1, TeamID: 10, UserID: worker.ID, Role: "member"},
		&requirementgorm.Requirement{ID: 14, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11,
			PlanUnitID: 12, Title: "Requirement A2", Priority: "high", Status: "active"},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now().UTC()
	adminID, workerID := admin.ID, worker.ID
	parentID := uint(101)
	tasks := []taskgorm.WorkTask{
		{ID: 101, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: taskgorm.WorkTaskKindTask, TaskKey: "A-101", Title: "Source",
			Description: "source description", Status: "in_progress", Priority: "high",
			AssigneeID: &workerID, ReporterUserID: &adminID,
			EstimatedMinutes: 120, StartsAt: timePtr(now.Add(-72 * time.Hour)), DueAt: timePtr(now.Add(-48 * time.Hour))},
		{ID: 102, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: taskgorm.WorkTaskKindSubtask, ParentID: &parentID,
			TaskKey: "A-102", Title: "Child", Status: "todo", Priority: "normal",
			AssigneeID: &workerID, ReporterUserID: &adminID, DueAt: timePtr(now.Add(48 * time.Hour))},
		{ID: 103, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: taskgorm.WorkTaskKindTask, TaskKey: "A-103", Title: "Done",
			Status: "done", Priority: "normal", AssigneeID: &adminID, ReporterUserID: &adminID,
			DueAt: timePtr(now.Add(-48 * time.Hour)), CompletedAt: timePtr(now.Add(-72 * time.Hour))},
		{ID: 201, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, ProjectID: 22,
			RequirementID: 23, Kind: taskgorm.WorkTaskKindTask, TaskKey: "B-201", Title: "Other Workspace",
			Status: "todo", Priority: "normal", AssigneeID: &adminID, ReporterUserID: &adminID},
	}
	if err := app.db.Create(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	return admin, app.token(t, admin)
}

func TestNativeTaskCloneCopiesOnlyTaskFields(t *testing.T) {
	app := newNativeTaskOperationsTestApp(t, nil)
	_, token := seedNativeTaskOperationScopes(t, app)
	for _, row := range []any{
		&taskgorm.WorkTaskComment{WorkspaceID: 1, WorkTaskID: 101, UserID: 1, Body: "do not copy"},
		&taskgorm.WorkTaskAttachment{WorkspaceID: 1, WorkTaskID: 101, UploaderUserID: 1,
			Filename: "source.txt", ContentType: "text/plain", Size: 10, ObjectKey: "source/101"},
		&taskgorm.TaskTestDefinition{WorkspaceID: 1, WorkTaskID: 101, Version: 1,
			CasesJSON:       `[{"key":"TC-1","title":"source","steps":[{"action":"a"}]}]`,
			CreatedByUserID: 1, UpdatedByUserID: 1},
	} {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	response := app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/101/clone", token, 1, map[string]any{
			"title_suffix": " - clone", "target_requirement_id": 14,
		})
	requireStatus(t, response, http.StatusCreated)
	data := decodeData(t, response).(map[string]any)
	cloneID := uint(data["id"].(float64))
	clonedFrom, _ := data["cloned_from"].(map[string]any)
	if cloneID == 101 || data["task_key"] == "A-101" || data["title"] != "Source- clone" ||
		data["requirement_id"] != float64(14) || clonedFrom["task_key"] != "A-101" ||
		clonedFrom["title"] != "Source" {
		t.Fatalf("clone response=%s", response.Body.String())
	}
	var clone taskgorm.WorkTask
	if err := app.db.First(&clone, cloneID).Error; err != nil {
		t.Fatal(err)
	}
	if clone.Description != "source description" || clone.Priority != "high" ||
		clone.Status != "draft" || clone.CompletedAt != nil ||
		clone.AssigneeID == nil || *clone.AssigneeID != 2 {
		t.Fatalf("clone fields=%+v", clone)
	}
	for name, target := range map[string]any{
		"children":    &taskgorm.WorkTask{},
		"comments":    &taskgorm.WorkTaskComment{},
		"attachments": &taskgorm.WorkTaskAttachment{},
		"testing":     &taskgorm.TaskTestDefinition{},
	} {
		var count int64
		query := app.db.Model(target).Where("workspace_id = ?", 1)
		if name == "children" {
			query = query.Where("parent_id = ?", cloneID)
		} else {
			query = query.Where("work_task_id = ?", cloneID)
		}
		if err := query.Count(&count).Error; err != nil || count != 0 {
			t.Fatalf("clone copied %s count=%d err=%v", name, count, err)
		}
	}
	var activity taskgorm.WorkTaskActivity
	if err := app.db.Where("workspace_id = ? AND work_task_id = ? AND action = ?", 1, cloneID, "cloned").
		First(&activity).Error; err != nil || !strings.Contains(activity.Detail, "source_task_id=101") {
		t.Fatalf("clone activity=%+v err=%v", activity, err)
	}
	var auditCount, outboxCount int64
	app.db.Model(&auditgorm.AuditLog{}).Where("resource = ? AND resource_id = ?", "task", cloneID).Count(&auditCount)
	app.db.Model(&auditgorm.OutboxEvent{}).Where("aggregate_type = ? AND aggregate_id = ?", "work_task", cloneID).Count(&outboxCount)
	if auditCount != 1 || outboxCount != 1 {
		t.Fatalf("clone mutation audit=%d outbox=%d", auditCount, outboxCount)
	}
	legacy := app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/101/clone", token, 1, map[string]any{"project_id": 12})
	requireStatus(t, legacy, http.StatusBadRequest)
}

func TestNativeTaskBatchUpdateIsAtomicAndRejectsLegacyFields(t *testing.T) {
	app := newNativeTaskOperationsTestApp(t, nil)
	_, token := seedNativeTaskOperationScopes(t, app)
	response := app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/batch-update", token, 1, map[string]any{
			"ids": []uint{101, 103}, "atomic": true,
			"updates": map[string]any{"priority": "urgent", "assignee_id": 2, "estimated_minutes": 90},
		})
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	if data["updated_count"] != float64(2) || data["atomic"] != true {
		t.Fatalf("batch response=%s", response.Body.String())
	}
	var updated []taskgorm.WorkTask
	if err := app.db.Where("id IN ?", []uint{101, 103}).Order("id").Find(&updated).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range updated {
		if row.Priority != "urgent" || row.AssigneeID == nil || *row.AssigneeID != 2 || row.EstimatedMinutes != 90 {
			t.Fatalf("batch row=%+v", row)
		}
	}
	legacy := app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/batch-update", token, 1, map[string]any{
			"ids": []uint{101}, "atomic": true, "updates": map[string]any{"milestone_id": 9},
		})
	requireStatus(t, legacy, http.StatusBadRequest)
	crossWorkspace := app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/batch-update", token, 1, map[string]any{
			"ids": []uint{101, 201}, "atomic": true, "updates": map[string]any{"priority": "low"},
		})
	requireStatus(t, crossWorkspace, http.StatusNotFound)
}

func TestNativeTaskBatchUpdateRollsBackDatabaseFailure(t *testing.T) {
	app := newNativeTaskOperationsTestApp(t, nil)
	_, token := seedNativeTaskOperationScopes(t, app)
	if err := app.db.Exec(`CREATE TRIGGER fail_second_native_task_update
		BEFORE UPDATE ON work_tasks WHEN OLD.id = 103
		BEGIN SELECT RAISE(FAIL, 'forced batch failure'); END`).Error; err != nil {
		t.Fatal(err)
	}
	response := app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/batch-update", token, 1, map[string]any{
			"ids": []uint{101, 103}, "atomic": true, "updates": map[string]any{"priority": "low"},
		})
	requireStatus(t, response, http.StatusInternalServerError)
	var source taskgorm.WorkTask
	if err := app.db.First(&source, 101).Error; err != nil || source.Priority != "high" {
		t.Fatalf("first update was not rolled back: %+v err=%v", source, err)
	}
	var activityCount, outboxCount int64
	app.db.Model(&taskgorm.WorkTaskActivity{}).Where("work_task_id IN ? AND field_key = ?", []uint{101, 103}, "priority").Count(&activityCount)
	app.db.Model(&auditgorm.OutboxEvent{}).Where("aggregate_type = ? AND aggregate_id IN ?", "work_task", []uint{101, 103}).Count(&outboxCount)
	if activityCount != 0 || outboxCount != 0 {
		t.Fatalf("rolled back side effects activity=%d outbox=%d", activityCount, outboxCount)
	}
}

func TestNativeTaskBatchUpdateImpactIncludesAutoCompletedAncestors(t *testing.T) {
	app := newNativeTaskOperationsTestApp(t, nil)
	_, token := seedNativeTaskOperationScopes(t, app)
	response := app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/batch-update/impact", token, 1, map[string]any{
			"ids": []uint{102}, "atomic": true, "updates": map[string]any{"status": "completed"},
		})
	requireStatus(t, response, http.StatusOK)
	data := decodeData(t, response).(map[string]any)
	if data["affected_count"] != float64(2) ||
		!nativeTaskTestIDsEqual(data["requested_ids"], []uint{102}) ||
		!nativeTaskTestIDsEqual(data["auto_completed_ancestor_ids"], []uint{101}) ||
		!nativeTaskTestIDsEqual(data["affected_ids"], []uint{101, 102}) {
		t.Fatalf("impact response=%s", response.Body.String())
	}
	var source, child taskgorm.WorkTask
	if err := app.db.First(&source, 101).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.db.First(&child, 102).Error; err != nil {
		t.Fatal(err)
	}
	if source.Status != "in_progress" || child.Status != "todo" {
		t.Fatalf("impact mutated tasks source=%s child=%s", source.Status, child.Status)
	}

	update := app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/batch-update", token, 1, map[string]any{
			"ids": []uint{102}, "atomic": true, "updates": map[string]any{"status": "completed"},
		})
	requireStatus(t, update, http.StatusOK)
	updateData := decodeData(t, update).(map[string]any)
	if !nativeTaskTestIDsEqual(updateData["auto_completed_ancestor_ids"], []uint{101}) {
		t.Fatalf("batch impact response=%s", update.Body.String())
	}
	if err := app.db.First(&source, 101).Error; err != nil || source.Status != "completed" {
		t.Fatalf("ancestor was not auto-completed: %+v err=%v", source, err)
	}
}

func TestNativeTaskBatchUpdateImpactIncludesDuePropagation(t *testing.T) {
	app := newNativeTaskOperationsTestApp(t, nil)
	_, token := seedNativeTaskOperationScopes(t, app)
	parentID := uint(101)
	descendant := taskgorm.WorkTask{
		ID: 104, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
		RequirementID: 13, Kind: taskgorm.WorkTaskKindSubtask, ParentID: &parentID,
		TaskKey: "A-104", Title: "Unset due child", Status: "todo", Priority: "normal",
	}
	if err := app.db.Create(&descendant).Error; err != nil {
		t.Fatal(err)
	}
	dueAt := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	body := map[string]any{
		"ids": []uint{101}, "atomic": true, "updates": map[string]any{"due_at": dueAt},
	}
	impactResponse := app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/batch-update/impact", token, 1, body)
	requireStatus(t, impactResponse, http.StatusOK)
	impact := decodeData(t, impactResponse).(map[string]any)
	if impact["affected_count"] != float64(2) ||
		!nativeTaskTestIDsEqual(impact["due_propagated_descendant_ids"], []uint{104}) ||
		!nativeTaskTestIDsEqual(impact["affected_ids"], []uint{101, 104}) {
		t.Fatalf("due impact response=%s", impactResponse.Body.String())
	}
	var before taskgorm.WorkTask
	if err := app.db.First(&before, 104).Error; err != nil || before.DueAt != nil {
		t.Fatalf("impact mutated descendant: %+v err=%v", before, err)
	}

	updateResponse := app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/batch-update", token, 1, body)
	requireStatus(t, updateResponse, http.StatusOK)
	update := decodeData(t, updateResponse).(map[string]any)
	if !nativeTaskTestIDsEqual(update["due_propagated_descendant_ids"], []uint{104}) {
		t.Fatalf("due update response=%s", updateResponse.Body.String())
	}
	var after taskgorm.WorkTask
	if err := app.db.First(&after, 104).Error; err != nil || after.DueAt == nil {
		t.Fatalf("due was not propagated: %+v err=%v", after, err)
	}
}

func TestNativeTaskStatsRespectWorkspaceScopeAndFilters(t *testing.T) {
	app := newNativeTaskOperationsTestApp(t, nil)
	_, token := seedNativeTaskOperationScopes(t, app)
	delivery := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks/stats/delivery?project_set_ids=11", token, 1, nil)
	requireStatus(t, delivery, http.StatusOK)
	deliveryData := decodeData(t, delivery).(map[string]any)
	if deliveryData["total"] != float64(3) || deliveryData["completed"] != float64(1) ||
		deliveryData["overdue"] != float64(1) {
		t.Fatalf("delivery stats=%s", delivery.Body.String())
	}
	filtered := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks/stats/delivery?project_set_ids=11&status=done", token, 1, nil)
	requireStatus(t, filtered, http.StatusOK)
	if decodeData(t, filtered).(map[string]any)["total"] != float64(1) {
		t.Fatalf("filtered delivery=%s", filtered.Body.String())
	}
	contractFiltered := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks/stats/delivery?project_set_ids=11&ids=101,102", token, 1, nil)
	requireStatus(t, contractFiltered, http.StatusOK)
	if decodeData(t, contractFiltered).(map[string]any)["total"] != float64(2) {
		t.Fatalf("contract filtered delivery=%s", contractFiltered.Body.String())
	}
	invalidContractFilter := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks/stats/delivery?project_set_ids=11&ids=bad", token, 1, nil)
	requireStatus(t, invalidContractFilter, http.StatusBadRequest)
	due := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks/stats/due-completion?project_set_ids=11", token, 1, nil)
	requireStatus(t, due, http.StatusOK)
	dueData := decodeData(t, due).(map[string]any)
	if dueData["total"] != float64(3) || dueData["overdue"] != float64(1) ||
		dueData["completed_on_time"] != float64(1) {
		t.Fatalf("due stats=%s", due.Body.String())
	}
	assignee := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks/stats/by-assignee-status?project_set_ids=11", token, 1, nil)
	requireStatus(t, assignee, http.StatusOK)
	assigneeData := decodeData(t, assignee).(map[string]any)
	if assigneeData["total_assignees"] != float64(2) || len(assigneeData["items"].([]any)) != 2 {
		t.Fatalf("assignee stats=%s", assignee.Body.String())
	}
	crossTenant := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks/stats/delivery?project_set_ids=21", token, 1, nil)
	requireStatus(t, crossTenant, http.StatusBadRequest)
}

func TestNativeTaskBatchUpdateRejectsInactivePMUser(t *testing.T) {
	app := newNativeTaskOperationsTestApp(t, nil)
	_, token := seedNativeTaskOperationScopes(t, app)
	response := app.request(t, http.MethodPost,
		"/api/v1/platform/tasks/batch-update", token, 1, map[string]any{
			"ids": []uint{101}, "atomic": true, "updates": map[string]any{"assignee_id": 999},
		})
	requireStatus(t, response, http.StatusBadRequest)
	if !strings.Contains(response.Body.String(), "PM user reference ID 999 is not active") {
		t.Fatalf("invalid PM user response=%s", response.Body.String())
	}
}

func float64Ptr(value float64) *float64 { return &value }

func nativeTaskTestIDsEqual(raw any, expected []uint) bool {
	values, ok := raw.([]any)
	if !ok || len(values) != len(expected) {
		return false
	}
	for index, value := range values {
		if value != float64(expected[index]) {
			return false
		}
	}
	return true
}
