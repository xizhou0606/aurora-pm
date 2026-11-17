package taskgorm

import (
	"context"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

func TestTodoReadScopesAssignedTasksAndMentionsByWorkspaceAndPrincipal(t *testing.T) {
	repository, db := newToolReadRepository(t)
	now := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	owner, other := uint(9), uint(10)
	rows := []WorkTask{
		{ID: 101, WorkspaceID: 1, TeamID: 1, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: "task", TaskKey: "A-101", Title: "Owned overdue", Status: "in_dev", Priority: "high", AssigneeID: &owner, DueAt: timePointer(now.Add(-time.Hour)), UpdatedAt: now},
		{ID: 102, WorkspaceID: 1, TeamID: 1, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: "task", TaskKey: "A-102", Title: "Other task", Status: "draft", Priority: "normal", AssigneeID: &other, UpdatedAt: now},
		{ID: 103, WorkspaceID: 2, TeamID: 2, ProjectSetID: 21, ProjectID: 22, RequirementID: 23, Kind: "task", TaskKey: "B-103", Title: "Foreign owned", Status: "draft", Priority: "urgent", AssigneeID: &owner, UpdatedAt: now},
		{ID: 104, WorkspaceID: 1, TeamID: 1, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: "task", TaskKey: "A-104", Title: "Completed", Status: "completed", Priority: "urgent", AssigneeID: &owner, UpdatedAt: now},
		{ID: 105, WorkspaceID: 1, TeamID: 1, ProjectSetID: 11, ProjectID: 12, RequirementID: 13, Kind: "task", TaskKey: "A-105", Title: "Deleting", Status: "in_dev", Priority: "urgent", AssigneeID: &owner, MutationState: workTaskStateDeleting, UpdatedAt: now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	comments := []WorkTaskComment{
		{WorkspaceID: 1, WorkTaskID: 102, UserID: other, Body: `{"type":"doc","content":[{"type":"mention","attrs":{"user_id":9}}]}`, CreatedAt: now},
		{WorkspaceID: 2, WorkTaskID: 103, UserID: other, Body: "@owner foreign", CreatedAt: now},
	}
	if err := db.Create(&comments).Error; err != nil {
		t.Fatal(err)
	}
	page, err := repository.ListPrincipalTodos(context.Background(), 1, owner, task.TodoFilter{Limit: 20, Now: now})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != 101 {
		t.Fatalf("assigned page=%+v err=%v", page, err)
	}
	mentions, err := repository.ListPrincipalMentions(context.Background(), 1, owner, "owner", task.TodoFilter{Limit: 20, Now: now})
	if err != nil || mentions.Total != 1 || len(mentions.Items) != 1 || mentions.Items[0].ID != 102 || mentions.Items[0].MentionAt == nil {
		t.Fatalf("mention page=%+v err=%v", mentions, err)
	}
}
