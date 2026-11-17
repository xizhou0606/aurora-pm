package taskgorm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"gorm.io/gorm"
)

func TestToolReadRepositoryScopesListsAndReferencesByWorkspace(t *testing.T) {
	repository, db := newToolReadRepository(t)
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	parent := WorkTask{ID: 10, WorkspaceID: 1, TeamID: 1, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: "task", TaskKey: "PM-10", Title: "Parent", Status: "in_dev", Priority: "high", DueAt: timePointer(now.AddDate(0, 0, 4)), CreatedAt: now.Add(-time.Hour), UpdatedAt: now}
	child := WorkTask{ID: 11, WorkspaceID: 1, TeamID: 1, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: "subtask", ParentID: &parent.ID, TaskKey: "PM-11", Title: "Matched child", Status: "draft", Priority: "normal", DueAt: timePointer(now.AddDate(0, 0, 8)), CreatedAt: now, UpdatedAt: now}
	foreign := WorkTask{ID: 20, WorkspaceID: 2, TeamID: 2, ProjectSetID: 21, ProjectID: 22, RequirementID: 23, Kind: "task", TaskKey: "PM-20", Title: "Matched foreign", Status: "draft", Priority: "normal", CreatedAt: now, UpdatedAt: now}
	deleting := WorkTask{ID: 12, WorkspaceID: 1, TeamID: 1, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: "task", TaskKey: "PM-12", Title: "Matched deleting", Status: "draft", Priority: "normal", MutationState: workTaskStateDeleting, CreatedAt: now, UpdatedAt: now}
	if err := db.Create([]WorkTask{parent, child, foreign, deleting}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&WorkTaskActivity{WorkspaceID: 1, WorkTaskID: child.ID, UserID: 9, Action: "created", Detail: `{"field_key":"status","new_value":"draft","summary":"created"}`, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&WorkTaskComment{WorkspaceID: 1, WorkTaskID: child.ID, UserID: 9, Body: "comment", CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&WorkTaskAttachment{WorkspaceID: 1, WorkTaskID: child.ID, UploaderUserID: 9, Filename: "safe.txt", ObjectKey: "private/key", StorageBackend: "s3", Size: 4, CreatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}

	result, err := repository.ListTasks(context.Background(), 1, task.ListFilter{Query: "matched", Limit: 50, AsOf: now})
	if err != nil || result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != child.ID {
		t.Fatalf("list=%+v err=%v", result, err)
	}
	tree, err := repository.ListTasks(context.Background(), 1, task.ListFilter{Query: "matched", Hierarchy: "filtered_tree", Limit: 50, AsOf: now})
	if err != nil || tree.Total != 1 || tree.MatchTotal != 1 || len(tree.Items) != 2 || !tree.Items[0].ContextOnly || !tree.Items[1].FilterMatch {
		t.Fatalf("tree=%+v err=%v", tree, err)
	}
	related, err := repository.ListRelated(context.Background(), 1, child.ID)
	if err != nil || len(related) != 1 || related[0].Relation != "parent" || related[0].Task.ID != parent.ID {
		t.Fatalf("related=%+v err=%v", related, err)
	}
	activities, total, err := repository.ListActivities(context.Background(), 1, child.ID, 40, 0)
	if err != nil || total != 1 || activities[0].FieldKey != "status" || activities[0].Summary != "created" {
		t.Fatalf("activities=%+v total=%d err=%v", activities, total, err)
	}
	comments, err := repository.ListComments(context.Background(), 1, child.ID)
	if err != nil || len(comments) != 1 || comments[0].Body != "comment" {
		t.Fatalf("comments=%+v err=%v", comments, err)
	}
	attachments, err := repository.ListAttachments(context.Background(), 1, child.ID)
	if err != nil || len(attachments) != 1 || attachments[0].Filename != "safe.txt" {
		t.Fatalf("attachments=%+v err=%v", attachments, err)
	}
	conflicts, err := repository.ListDueConflicts(context.Background(), 1, parent.ID, now.AddDate(0, 0, 5))
	if err != nil || len(conflicts) != 1 || conflicts[0].ID != child.ID {
		t.Fatalf("conflicts=%+v err=%v", conflicts, err)
	}
	if _, err := repository.ListRelated(context.Background(), 2, child.ID); !errors.Is(err, task.ErrTaskNotFound) {
		t.Fatalf("cross-workspace err=%v", err)
	}
}

func TestToolReadRepositoryStatsUseSameScopedFilters(t *testing.T) {
	repository, db := newToolReadRepository(t)
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	assignee := uint(5)
	rows := []WorkTask{
		{ID: 1, WorkspaceID: 1, TeamID: 1, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: "task", TaskKey: "A-1", Title: "open overdue", Status: "in_dev", Priority: "high", AssigneeID: &assignee, DueAt: timePointer(now.AddDate(0, 0, -1))},
		{ID: 2, WorkspaceID: 1, TeamID: 1, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: "subtask", TaskKey: "A-2", Title: "done on time", Status: "completed", Priority: "normal", AssigneeID: &assignee, DueAt: timePointer(now.Add(time.Hour)), CompletedAt: timePointer(now)},
		{ID: 3, WorkspaceID: 2, TeamID: 2, ProjectSetID: 21, ProjectID: 22, RequirementID: 23, Kind: "task", TaskKey: "B-1", Title: "foreign", Status: "completed", Priority: "normal"},
	}
	if err := db.Create(rows).Error; err != nil {
		t.Fatal(err)
	}
	filter := task.ListFilter{ProjectSetID: 11, AsOf: now}
	delivery, err := repository.DeliveryStats(context.Background(), 1, filter)
	if err != nil || delivery.Total != 2 || delivery.Completed != 1 || delivery.Overdue != 1 || delivery.CompletionRate != 50 {
		t.Fatalf("delivery=%+v err=%v", delivery, err)
	}
	due, err := repository.DueCompletionStats(context.Background(), 1, filter)
	if err != nil || due.Total != 2 || due.Overdue != 1 || due.CompletedOnTime != 1 || due.Timezone != "UTC" {
		t.Fatalf("due=%+v err=%v", due, err)
	}
	byAssignee, err := repository.AssigneeStatusStats(context.Background(), 1, filter)
	if err != nil || byAssignee.TotalAssignees != 1 || byAssignee.Items[0].Total != 2 {
		t.Fatalf("assignee=%+v err=%v", byAssignee, err)
	}
}

func newToolReadRepository(t *testing.T) (*ReadRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&WorkTask{}, &WorkTaskActivity{}, &WorkTaskComment{}, &WorkTaskAttachment{}); err != nil {
		t.Fatal(err)
	}
	return NewReadRepository(db), db
}

func timePointer(value time.Time) *time.Time { return &value }
