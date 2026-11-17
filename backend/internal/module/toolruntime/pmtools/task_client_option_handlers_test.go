package pmtools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type taskClientOptionQueriesFake struct {
	workspaceID uint
	actorUserID uint
}

func (fake *taskClientOptionQueriesFake) List(_ context.Context, workspaceID, actorUserID uint) ([]task.ClientOption, error) {
	fake.workspaceID, fake.actorUserID = workspaceID, actorUserID
	return []task.ClientOption{{ID: 9, Name: "桌面端", UsageCount: 2}}, nil
}

func TestTaskClientOptionHandlerUsesAuthorizationFactsAndReturnsDirectory(t *testing.T) {
	fake := &taskClientOptionQueriesFake{}
	handlers, err := NewTaskClientOptionHandlers(fake)
	if err != nil {
		t.Fatal(err)
	}
	if len(handlers) != 1 || len(TaskClientOptionHandlerNames()) != 1 {
		t.Fatalf("handlers=%v names=%v", handlers, TaskClientOptionHandlerNames())
	}
	raw, err := handlers[TaskClientOptionList].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7, PrincipalID: 11},
		Input:         json.RawMessage(`{}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.workspaceID != 7 || fake.actorUserID != 11 {
		t.Fatalf("workspace=%d actor=%d", fake.workspaceID, fake.actorUserID)
	}
	var output struct {
		Items []task.ClientOption `json:"items"`
	}
	if err := json.Unmarshal(raw, &output); err != nil || len(output.Items) != 1 || output.Items[0].ID != 9 {
		t.Fatalf("output=%s err=%v", raw, err)
	}
}
