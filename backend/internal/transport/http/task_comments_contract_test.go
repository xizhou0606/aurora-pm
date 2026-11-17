package httpapi

import (
	"fmt"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestTaskCommentsMentionsWorkspaceAccessAndIsolation(t *testing.T) {
	app := newTestApp(t, nil)
	admin := app.createUser(t, 1, "admin", "super_admin", "")
	other := app.createUser(t, 2, "other", "user", "")
	readonly := app.createUser(t, 3, "readonly", "readonly", "")
	viewer := app.createUser(t, 4, "viewer", "user", "")
	if err := app.db.Model(&admin).Update("external_user_id", 77).Error; err != nil {
		t.Fatal(err)
	}
	seedTaskCommentContract(t, app, admin, other, readonly, viewer)
	adminToken, otherToken, readonlyToken := app.token(t, admin), app.token(t, other), app.token(t, readonly)
	viewerToken := app.token(t, viewer)

	mentioned := app.request(t, http.MethodPost, "/api/v1/platform/tasks/101/comments", otherToken, 1,
		map[string]any{"body": "@admin 请看这个任务"})
	requireStatus(t, mentioned, http.StatusCreated)
	mentionedRow := decodeData(t, mentioned).(map[string]any)
	mentionedID := uint(mentionedRow["id"].(float64))
	if mentionedRow["user_id"] != float64(2) || mentionedRow["body"] != "@admin 请看这个任务" {
		t.Fatalf("created comment contract=%v", mentionedRow)
	}

	adminComment := app.request(t, http.MethodPost, "/api/v1/platform/tasks/101/comments", adminToken, 1,
		map[string]any{"body": "shared fact author"})
	requireStatus(t, adminComment, http.StatusCreated)
	adminCommentID := uint(decodeData(t, adminComment).(map[string]any)["id"].(float64))
	var stored taskgorm.WorkTaskComment
	if err := app.db.First(&stored, adminCommentID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.WorkspaceID != 1 || stored.UserID != admin.ID {
		t.Fatalf("comment did not preserve internal user id: %+v", stored)
	}

	list := app.request(t, http.MethodGet, "/api/v1/platform/tasks/101/comments", otherToken, 1, nil)
	requireStatus(t, list, http.StatusOK)
	comments := decodeData(t, list).([]any)
	if len(comments) != 2 || comments[0].(map[string]any)["username"] != "other" {
		t.Fatalf("comment list identity=%s", list.Body.String())
	}
	for _, raw := range comments {
		if _, exists := raw.(map[string]any)["can_delete"]; exists {
			t.Fatalf("retired comment authorization field leaked: %s", list.Body.String())
		}
	}

	readonlyPost := app.request(t, http.MethodPost, "/api/v1/platform/tasks/101/comments", readonlyToken, 1,
		map[string]any{"body": "readonly role is no longer restricted"})
	requireStatus(t, readonlyPost, http.StatusCreated)
	viewerPost := app.request(t, http.MethodPost, "/api/v1/platform/tasks/101/comments", viewerToken, 1,
		map[string]any{"body": "viewer label is no longer restricted"})
	requireStatus(t, viewerPost, http.StatusCreated)
	memberDelete := app.request(t, http.MethodDelete,
		fmt.Sprintf("/api/v1/platform/tasks/101/comments/%d", adminCommentID), otherToken, 1, nil)
	requireStatus(t, memberDelete, http.StatusNoContent)
	wrongParent := app.request(t, http.MethodDelete,
		fmt.Sprintf("/api/v1/platform/tasks/104/comments/%d", adminCommentID), adminToken, 1, nil)
	requireStatus(t, wrongParent, http.StatusNotFound)
	foreign := app.request(t, http.MethodGet, "/api/v1/platform/tasks/201/comments", adminToken, 1, nil)
	requireStatus(t, foreign, http.StatusNotFound)
	aliases := app.request(t, http.MethodGet,
		"/api/v1/platform/tasks?statuses=draft&priorities=normal&assignee_ids=2,999&filter_kinds=subtask&parent_id=101",
		otherToken, 1, nil)
	requireStatus(t, aliases, http.StatusOK)
	aliasRows := decodeData(t, aliases).(map[string]any)
	if aliasRows["total"] != float64(1) || aliasRows["items"].([]any)[0].(map[string]any)["id"] != float64(106) {
		t.Fatalf("legacy task filter aliases=%s", aliases.Body.String())
	}

	now := time.Now().UTC()
	directComments := []taskgorm.WorkTaskComment{
		{WorkspaceID: 1, WorkTaskID: 104, UserID: 2,
			Body:      `{"type":"doc","content":[{"type":"mention","attrs":{"id":"1"}}]}`,
			CreatedAt: now.Add(-time.Hour)},
		{WorkspaceID: 1, WorkTaskID: 109, UserID: 2,
			Body: "@admin behind more than 500 comments", CreatedAt: now.Add(-2 * time.Hour)},
		{WorkspaceID: 1, WorkTaskID: 110, UserID: admin.ID,
			Body: "@admin self mention follows legacy behavior", CreatedAt: now.Add(-30 * time.Minute)},
		{WorkspaceID: 1, WorkTaskID: 105, UserID: 2, Body: "@admin old", CreatedAt: now.AddDate(0, 0, -15)},
		{WorkspaceID: 1, WorkTaskID: 102, UserID: 2, Body: "@admin terminal", CreatedAt: now},
		{WorkspaceID: 2, WorkTaskID: 201, UserID: 2, Body: "@admin foreign secret", CreatedAt: now},
	}
	if err := app.db.Create(&directComments).Error; err != nil {
		t.Fatal(err)
	}
	noise := make([]taskgorm.WorkTaskComment, 501)
	for index := range noise {
		noise[index] = taskgorm.WorkTaskComment{
			WorkspaceID: 1, WorkTaskID: 108, UserID: 2,
			Body: "ordinary update without a mention", CreatedAt: now.Add(-time.Minute),
		}
	}
	if err := app.db.CreateInBatches(&noise, 100).Error; err != nil {
		t.Fatal(err)
	}
	summaryRec := app.request(t, http.MethodGet,
		"/api/v1/platform/workbench/todos/summary?domains=task&include_mentions=1", adminToken, 1, nil)
	requireStatus(t, summaryRec, http.StatusOK)
	summary := decodeData(t, summaryRec).(map[string]any)["summary"].(map[string]any)
	if summary["mentions"] != float64(4) {
		t.Fatalf("mention summary=%s", summaryRec.Body.String())
	}
	withoutMentions := app.request(t, http.MethodGet,
		"/api/v1/platform/workbench/todos/summary?domains=task", adminToken, 1, nil)
	if decodeData(t, withoutMentions).(map[string]any)["summary"].(map[string]any)["mentions"] != float64(0) {
		t.Fatalf("mention summary should be opt-in: %s", withoutMentions.Body.String())
	}
	feed := app.request(t, http.MethodGet,
		"/api/v1/platform/workbench/todos/feed?type=mention&limit=10", adminToken, 1, nil)
	requireStatus(t, feed, http.StatusOK)
	if !strings.Contains(feed.Body.String(), `"total":4`) ||
		!strings.Contains(feed.Body.String(), `"mention_at"`) ||
		strings.Contains(feed.Body.String(), "foreign secret") || strings.Contains(feed.Body.String(), "old task") {
		t.Fatalf("mention feed isolation/window=%s", feed.Body.String())
	}

	deletedAlias := app.request(t, http.MethodDelete,
		fmt.Sprintf("/api/v1/platform/tasks/comments/%d", mentionedID), otherToken, 1, nil)
	requireStatus(t, deletedAlias, http.StatusNoContent)

	for _, row := range []taskgorm.WorkTaskComment{
		{WorkspaceID: 1, WorkTaskID: 108, UserID: 2, Body: "task delete cleanup"},
		{WorkspaceID: 1, WorkTaskID: 107, UserID: 2, Body: "requirement delete cleanup"},
	} {
		if err := app.db.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
	deletedTask := app.request(t, http.MethodDelete, "/api/v1/platform/tasks/108", adminToken, 1, nil)
	requireStatus(t, deletedTask, http.StatusOK)
	deletedRequirement := app.request(t, http.MethodDelete,
		"/api/v1/platform/product-requirements-new/14", adminToken, 1, nil)
	requireStatus(t, deletedRequirement, http.StatusOK)
	var leftover int64
	app.db.Model(&taskgorm.WorkTaskComment{}).Where("work_task_id IN ?", []uint{107, 108}).Count(&leftover)
	if leftover != 0 {
		t.Fatalf("task/requirement delete left %d comments", leftover)
	}
}

func seedTaskCommentContract(
	t *testing.T, app *testApp, admin identitygorm.User, other identitygorm.User, readonly identitygorm.User, viewer identitygorm.User,
) {
	t.Helper()
	ownerFact := admin.ID
	assigneeFact := uint(2)
	rows := []any{
		&identitygorm.Workspace{ID: 1, Name: "A", Slug: "comments-a", Enabled: true},
		&identitygorm.Workspace{ID: 2, Name: "B", Slug: "comments-b", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: admin.ID, Role: "owner", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: other.ID, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: readonly.ID, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 1, UserID: viewer.ID, Role: "member", Enabled: true},
		&identitygorm.WorkspaceMember{WorkspaceID: 2, UserID: admin.ID, Role: "owner", Enabled: true},
		&identitygorm.Team{ID: 10, WorkspaceID: 1, Name: "Team A"},
		&identitygorm.Team{ID: 20, WorkspaceID: 2, Name: "Team B"},
		&projectgorm.ProjectSet{ID: 11, WorkspaceID: 1, TeamID: 10, Name: "Project A", Code: "A", OwnerUserID: &ownerFact},
		&projectgorm.ProjectSet{ID: 21, WorkspaceID: 2, TeamID: 20, Name: "Project B", Code: "B", OwnerUserID: &ownerFact},
		&projectgorm.PlanUnit{ID: 12, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, Title: "V1"},
		&projectgorm.PlanUnit{ID: 22, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, Title: "V2"},
		&requirementgorm.Requirement{ID: 13, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, PlanUnitID: 12, Title: "Req A"},
		&requirementgorm.Requirement{ID: 14, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, PlanUnitID: 12, Title: "Req cleanup"},
		&requirementgorm.Requirement{ID: 23, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, PlanUnitID: 22, Title: "Req B"},
		&projectgorm.ProjectPermission{WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, UserID: 2, AccessLevel: "commenter"},
		&projectgorm.ProjectPermission{WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, UserID: 3, AccessLevel: "commenter"},
		&projectgorm.ProjectPermission{WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, UserID: 4, AccessLevel: "viewer"},
		&taskgorm.WorkTask{ID: 101, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-1", Title: "mentioned text task", Status: "draft", AssigneeID: &assigneeFact},
		&taskgorm.WorkTask{ID: 102, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-2", Title: "terminal task", Status: "completed"},
		&taskgorm.WorkTask{ID: 104, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-4", Title: "mentioned tiptap task", Status: "draft"},
		&taskgorm.WorkTask{ID: 105, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-5", Title: "old task", Status: "draft"},
		&taskgorm.WorkTask{ID: 106, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "subtask", TaskKey: "A-6", Title: "child task", Status: "draft",
			Priority: "normal", AssigneeID: &assigneeFact, ParentID: uintPtr(101)},
		&taskgorm.WorkTask{ID: 107, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 14, Kind: "task", TaskKey: "A-7", Title: "requirement cleanup", Status: "draft"},
		&taskgorm.WorkTask{ID: 108, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-8", Title: "task cleanup", Status: "draft"},
		&taskgorm.WorkTask{ID: 109, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-9", Title: "deep scan mention", Status: "draft"},
		&taskgorm.WorkTask{ID: 110, WorkspaceID: 1, TeamID: 10, ProjectSetID: 11, ProjectID: 12,
			RequirementID: 13, Kind: "task", TaskKey: "A-10", Title: "self mention", Status: "draft"},
		&taskgorm.WorkTask{ID: 201, WorkspaceID: 2, TeamID: 20, ProjectSetID: 21, ProjectID: 22,
			RequirementID: 23, Kind: "task", TaskKey: "B-1", Title: "foreign secret", Status: "draft"},
	}
	for _, row := range rows {
		if err := app.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
}
