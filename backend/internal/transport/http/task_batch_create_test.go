package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func TestBatchCreateTasksCreatesParentsAndChildrenAtomically(t *testing.T) {
	fix := setupTeamUserAssignmentFixture(t)
	startsAt := "2026-08-10T00:00:00+08:00"
	dueAt := "2026-08-12T23:59:59+08:00"
	response := fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-create",
		fix.token, fix.workspaceID, map[string]any{"tasks": []map[string]any{
			{
				"client_id": "parent-a", "requirement_id": fix.requirement,
				"title": "父任务 A", "starts_at": startsAt, "due_at": dueAt,
				"children": []map[string]any{{"client_id": "child-a", "title": "子任务 A"}},
			},
			{
				"client_id": "parent-b", "requirement_id": fix.requirement,
				"title": "父任务 B", "priority": "high", "assignee_id": fix.users["member"],
			},
		}})
	requireStatus(t, response, http.StatusCreated)

	var envelope struct {
		Data struct {
			Created []struct {
				ClientID string `json:"client_id"`
				ID       uint   `json:"id"`
				TaskKey  string `json:"task_key"`
				Children []struct {
					ClientID string `json:"client_id"`
					ID       uint   `json:"id"`
					TaskKey  string `json:"task_key"`
				} `json:"children"`
			} `json:"created"`
			Total int `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Data.Total != 3 || len(envelope.Data.Created) != 2 ||
		len(envelope.Data.Created[0].Children) != 1 {
		t.Fatalf("unexpected batch response: %s", response.Body.String())
	}
	if envelope.Data.Created[0].ClientID != "parent-a" ||
		envelope.Data.Created[0].Children[0].ClientID != "child-a" {
		t.Fatalf("client id mapping missing: %s", response.Body.String())
	}

	var tasks []taskgorm.WorkTask
	if err := fix.app.db.Order("id ASC").Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 3 {
		t.Fatalf("created tasks=%d want=3", len(tasks))
	}
	parent, child := tasks[0], tasks[1]
	if parent.Kind != taskgorm.WorkTaskKindTask || child.Kind != taskgorm.WorkTaskKindSubtask ||
		child.ParentID == nil || *child.ParentID != parent.ID {
		t.Fatalf("parent/child binding invalid: parent=%+v child=%+v", parent, child)
	}
	if parent.Status != "draft" || child.Status != "draft" ||
		parent.Priority != "normal" || child.Priority != "normal" {
		t.Fatalf("defaults invalid: parent=%+v child=%+v", parent, child)
	}
	if parent.AssigneeID == nil || child.AssigneeID == nil ||
		*parent.AssigneeID != fix.users["actor"] || *child.AssigneeID != fix.users["actor"] {
		t.Fatalf("assignee inheritance invalid: parent=%v child=%v", parent.AssigneeID, child.AssigneeID)
	}
	if child.RequirementID != parent.RequirementID || child.ProjectSetID != parent.ProjectSetID ||
		child.ProjectID != parent.ProjectID || child.TeamID != parent.TeamID {
		t.Fatalf("child scope did not inherit parent: parent=%+v child=%+v", parent, child)
	}
	wantStart, _ := time.Parse(time.RFC3339, startsAt)
	wantDue, _ := time.Parse(time.RFC3339, dueAt)
	if child.StartsAt == nil || !child.StartsAt.Equal(wantStart) ||
		child.DueAt == nil || !child.DueAt.Equal(wantDue) {
		t.Fatalf("child dates did not inherit parent: starts=%v due=%v", child.StartsAt, child.DueAt)
	}

	assertBatchMutationCounts(t, fix.app, 3)
}

func TestBatchCreateTasksAcceptsOptionalFieldsAndPreservesPerTaskReporter(t *testing.T) {
	fix := setupTeamUserAssignmentFixture(t)
	reporterID := fix.users["member"]
	response := fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-create",
		fix.token, fix.workspaceID, map[string]any{"tasks": []map[string]any{{
			"client_id": "full-parent", "requirement_id": fix.requirement,
			"title": "完整父任务", "description": "父任务描述", "status": "in_dev",
			"reporter_user_id": reporterID,
			"children": []map[string]any{{
				"client_id": "full-child", "title": "完整子任务",
				"description": "子任务描述", "status": "pending_test",
				"reporter_user_id": reporterID,
			}},
		}}})
	requireStatus(t, response, http.StatusCreated)

	var tasks []taskgorm.WorkTask
	if err := fix.app.db.Order("id ASC").Find(&tasks).Error; err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 {
		t.Fatalf("created tasks=%d want=2", len(tasks))
	}
	parent, child := tasks[0], tasks[1]
	if parent.Description != "父任务描述" || parent.Status != "in_dev" ||
		parent.ReporterUserID == nil || *parent.ReporterUserID != reporterID {
		t.Fatalf("parent optional fields=%+v", parent)
	}
	if child.Description != "子任务描述" || child.Status != "pending_test" ||
		child.ReporterUserID == nil || *child.ReporterUserID != reporterID {
		t.Fatalf("child optional fields=%+v", child)
	}
}

func TestBatchCreateTasksRollsBackOnOutboxFailure(t *testing.T) {
	fix := setupTeamUserAssignmentFixture(t)
	if err := fix.app.db.Exec("DROP TABLE outbox_events").Error; err != nil {
		t.Fatal(err)
	}
	response := fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-create",
		fix.token, fix.workspaceID, map[string]any{"tasks": []map[string]any{{
			"client_id": "rollback-parent", "requirement_id": fix.requirement,
			"title": "应整体回滚", "children": []map[string]any{{
				"client_id": "rollback-child", "title": "也应回滚",
			}},
		}}})
	requireStatus(t, response, http.StatusInternalServerError)
	for table, target := range map[string]any{
		"work_tasks": &taskgorm.WorkTask{}, "work_task_activities": &taskgorm.WorkTaskActivity{},
		"audit_logs": &auditgorm.AuditLog{},
	} {
		var count int64
		if err := fix.app.db.Model(target).Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != 0 {
			t.Fatalf("transaction left %d rows in %s", count, table)
		}
	}
}

func TestBatchCreateTasksRejectsInvalidBatchWithoutWrites(t *testing.T) {
	fix := setupTeamUserAssignmentFixture(t)
	tests := []struct {
		name        string
		body        map[string]any
		clientID    string
		field, code string
	}{
		{"duplicate client ids", map[string]any{"tasks": []map[string]any{{
			"client_id": "same", "requirement_id": fix.requirement, "title": "父任务",
			"children": []map[string]any{{"client_id": "same", "title": "子任务"}},
		}}}, "same", "client_id", "DUPLICATE"},
		{"foreign requirement", map[string]any{"tasks": []map[string]any{{
			"client_id": "foreign", "requirement_id": 999, "title": "越界任务",
		}}}, "foreign", "requirement_id", "INVALID_SCOPE"},
		{"unassigned", map[string]any{"tasks": []map[string]any{{
			"client_id": "nil-user", "requirement_id": fix.requirement, "title": "无经办人",
			"assignee_id": nil,
		}}}, "nil-user", "assignee_id", "REQUIRED"},
		{"invalid priority", map[string]any{"tasks": []map[string]any{{
			"client_id": "bad-priority", "requirement_id": fix.requirement, "title": "优先级错误",
			"priority": "unknown",
		}}}, "bad-priority", "priority", "INVALID_PRIORITY"},
		{"invalid reporter", map[string]any{"tasks": []map[string]any{{
			"client_id": "bad-reporter", "requirement_id": fix.requirement, "title": "报告人错误",
			"reporter_user_id": 0,
		}}}, "bad-reporter", "reporter_user_id", "INVALID_REPORTER"},
		{"invalid start date", map[string]any{"tasks": []map[string]any{{
			"client_id": "bad-start", "requirement_id": fix.requirement, "title": "日期格式错误",
			"starts_at": "not-a-date",
		}}}, "bad-start", "starts_at", "INVALID_DATE"},
		{"invalid date range", map[string]any{"tasks": []map[string]any{{
			"client_id": "bad-date", "requirement_id": fix.requirement, "title": "日期错误",
			"starts_at": "2026-08-12", "due_at": "2026-08-10",
		}}}, "bad-date", "due_at", "INVALID_RANGE"},
		{"negative work", map[string]any{"tasks": []map[string]any{{
			"client_id": "bad-work", "requirement_id": fix.requirement, "title": "工时错误",
			"estimated_minutes": -1,
		}}}, "bad-work", "estimated_minutes", "MIN_VALUE"},
		{"child title", map[string]any{"tasks": []map[string]any{{
			"client_id": "valid-parent", "requirement_id": fix.requirement, "title": "父任务",
			"children": []map[string]any{{"client_id": "empty-child", "title": ""}},
		}}}, "empty-child", "title", "REQUIRED"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-create",
				fix.token, fix.workspaceID, test.body)
			assertBatchValidation(t, response, test.clientID, test.field, test.code)
			var count int64
			if err := fix.app.db.Model(&taskgorm.WorkTask{}).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("invalid batch wrote %d tasks: %s", count, response.Body.String())
			}
		})
	}
}

func TestBatchCreateTasksEnforcesQuantityLimits(t *testing.T) {
	fix := setupTeamUserAssignmentFixture(t)
	parents := make([]map[string]any, maxBatchParentTasks+1)
	for index := range parents {
		parents[index] = map[string]any{
			"client_id":      fmt.Sprintf("parent-%d", index),
			"requirement_id": fix.requirement, "title": fmt.Sprintf("任务 %d", index),
		}
	}
	response := fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-create",
		fix.token, fix.workspaceID, map[string]any{"tasks": parents})
	assertBatchValidation(t, response, "", "tasks", "MAX_PARENT_TASKS")

	children := make([]map[string]any, maxBatchTotalTasks)
	for index := range children {
		children[index] = map[string]any{
			"client_id": fmt.Sprintf("child-%d", index), "title": fmt.Sprintf("子任务 %d", index),
		}
	}
	response = fix.app.request(t, http.MethodPost, "/api/v1/platform/tasks/batch-create",
		fix.token, fix.workspaceID, map[string]any{"tasks": []map[string]any{{
			"client_id": "one-parent", "requirement_id": fix.requirement,
			"title": "父任务", "children": children,
		}}})
	assertBatchValidation(t, response, "", "tasks", "MAX_TOTAL_TASKS")
}

func assertBatchValidation(
	t *testing.T,
	response interface {
		Result() *http.Response
	},
	clientID, field, code string,
) {
	t.Helper()
	result := response.Result()
	defer result.Body.Close()
	if result.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d want=%d", result.StatusCode, http.StatusUnprocessableEntity)
	}
	var envelope struct {
		Code   string                     `json:"code"`
		Error  string                     `json:"error"`
		Fields []taskBatchValidationField `json:"fields"`
	}
	if err := json.NewDecoder(result.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != "TASK_BATCH_VALIDATION_FAILED" || envelope.Error == "" || len(envelope.Fields) == 0 {
		t.Fatalf("invalid validation envelope: %+v", envelope)
	}
	got := envelope.Fields[0]
	if got.ClientID != clientID || got.Field != field || got.Code != code || got.Message == "" {
		t.Fatalf("validation field=%+v want client=%q field=%q code=%q", got, clientID, field, code)
	}
}

func assertBatchMutationCounts(t *testing.T, app *testApp, want int64) {
	t.Helper()
	for table, target := range map[string]any{
		"work_task_activities": &taskgorm.WorkTaskActivity{},
		"audit_logs":           &auditgorm.AuditLog{},
		"outbox_events":        &auditgorm.OutboxEvent{},
	} {
		var count int64
		query := app.db.Model(target)
		if table == "outbox_events" {
			query = query.Where("event_type NOT LIKE ?", "pm.notification.%")
		}
		if err := query.Count(&count).Error; err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if count != want {
			t.Fatalf("%s rows=%d want=%d", table, count, want)
		}
	}
}
