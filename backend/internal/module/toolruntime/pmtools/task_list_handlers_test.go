package pmtools

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type taskReadQueriesFake struct {
	workspace uint
	filter    task.ListFilter
	taskID    uint
	dueAt     time.Time
}

func (fake *taskReadQueriesFake) capture(workspaceID uint) error {
	fake.workspace = workspaceID
	if workspaceID != 7 {
		return errors.New("unexpected workspace")
	}
	return nil
}

func (fake *taskReadQueriesFake) ListTasks(_ context.Context, workspaceID uint, filter task.ListFilter) (task.ListResult, error) {
	fake.filter = filter
	return task.ListResult{Items: []task.ListItem{{ID: 1, WorkspaceID: workspaceID}}, Total: 1}, fake.capture(workspaceID)
}
func (fake *taskReadQueriesFake) ListRelated(_ context.Context, workspaceID, taskID uint) ([]task.RelatedItem, error) {
	fake.taskID = taskID
	return []task.RelatedItem{}, fake.capture(workspaceID)
}
func (fake *taskReadQueriesFake) ListGitLinks(_ context.Context, workspaceID, taskID uint) ([]any, error) {
	fake.taskID = taskID
	return []any{}, fake.capture(workspaceID)
}
func (fake *taskReadQueriesFake) ListActivities(_ context.Context, workspaceID, taskID uint, _, _ int) ([]task.ActivitySnapshot, int64, error) {
	fake.taskID = taskID
	return []task.ActivitySnapshot{}, 0, fake.capture(workspaceID)
}
func (fake *taskReadQueriesFake) ListComments(_ context.Context, workspaceID, taskID uint) ([]task.CommentSnapshot, error) {
	fake.taskID = taskID
	return []task.CommentSnapshot{}, fake.capture(workspaceID)
}
func (fake *taskReadQueriesFake) ListAttachments(_ context.Context, workspaceID, taskID uint) ([]task.AttachmentSnapshot, error) {
	fake.taskID = taskID
	return []task.AttachmentSnapshot{}, fake.capture(workspaceID)
}
func (fake *taskReadQueriesFake) ListDueConflicts(_ context.Context, workspaceID, taskID uint, dueAt time.Time) ([]task.DueConflict, error) {
	fake.taskID, fake.dueAt = taskID, dueAt
	return []task.DueConflict{}, fake.capture(workspaceID)
}
func (fake *taskReadQueriesFake) OverviewStats(_ context.Context, workspaceID uint, filter task.ListFilter) (task.OverviewStats, error) {
	fake.filter = filter
	return task.OverviewStats{ByKind: map[string]int64{}, ByKindCompleted: map[string]int64{}, ByStatus: map[string]int64{}}, fake.capture(workspaceID)
}
func (fake *taskReadQueriesFake) DeliveryStats(_ context.Context, workspaceID uint, filter task.ListFilter) (task.DeliveryStats, error) {
	fake.filter = filter
	return task.DeliveryStats{ByStatus: map[string]int64{}, ByPriority: map[string]int64{}}, fake.capture(workspaceID)
}
func (fake *taskReadQueriesFake) DueCompletionStats(_ context.Context, workspaceID uint, filter task.ListFilter) (task.DueCompletionStats, error) {
	fake.filter = filter
	return task.DueCompletionStats{Timezone: "UTC"}, fake.capture(workspaceID)
}
func (fake *taskReadQueriesFake) AssigneeStatusStats(_ context.Context, workspaceID uint, filter task.ListFilter) (task.AssigneeStatusStats, error) {
	fake.filter = filter
	return task.AssigneeStatusStats{Items: []task.AssigneeStatusItem{}}, fake.capture(workspaceID)
}

func TestTaskReadHandlersExposeIndependentActivationContract(t *testing.T) {
	handlers, err := NewTaskReadHandlers(&taskReadQueriesFake{})
	if err != nil {
		t.Fatal(err)
	}
	want := TaskReadHandlerNames()
	got := make([]string, 0, len(handlers))
	for name := range handlers {
		got = append(got, name)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) || len(got) != 11 {
		t.Fatalf("handlers=%v want=%v", got, want)
	}
}

func TestTaskReadHandlersUseServerWorkspaceAndCanonicalFilter(t *testing.T) {
	fake := &taskReadQueriesFake{}
	handlers, err := NewTaskReadHandlers(fake)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := handlers[TaskList].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7},
		Input:         json.RawMessage(`{"ids":[1,2],"status":"draft,in_dev","project_new_id":9,"limit":25,"offset":2}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.workspace != 7 || fake.filter.PlanUnitID != 9 || fake.filter.Limit != 25 || fake.filter.Offset != 2 || len(fake.filter.Statuses) != 1 {
		t.Fatalf("workspace=%d filter=%+v", fake.workspace, fake.filter)
	}
	var output task.ListResult
	if err := json.Unmarshal(raw, &output); err != nil || output.Total != 1 || output.Items == nil || output.Items[0].WorkspaceID != 7 {
		t.Fatalf("output=%s err=%v", raw, err)
	}
}

func TestTaskReferenceHandlersReturnStableObjectEnvelopes(t *testing.T) {
	fake := &taskReadQueriesFake{}
	handlers, err := NewTaskReadHandlers(fake)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{TaskRelatedList, TaskGitLinkList, TaskActivityList, TaskCommentList, TaskAttachmentList} {
		raw, invokeErr := handlers[name].Invoke(context.Background(), toolruntime.ToolCall{
			Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7},
			Input:         json.RawMessage(`{"id":42}`),
		})
		if invokeErr != nil {
			t.Fatalf("%s: %v", name, invokeErr)
		}
		var output struct {
			Items []json.RawMessage `json:"items"`
			Total int64             `json:"total"`
		}
		if err := json.Unmarshal(raw, &output); err != nil || output.Items == nil || output.Total != 0 || fake.taskID != 42 {
			t.Fatalf("%s output=%s err=%v task=%d", name, raw, err, fake.taskID)
		}
	}
}

func TestTaskDueConflictHandlerRequiresRFC3339AndWorkspace(t *testing.T) {
	fake := &taskReadQueriesFake{}
	handlers, err := NewTaskReadHandlers(fake)
	if err != nil {
		t.Fatal(err)
	}
	_, err = handlers[TaskDueConflictList].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7},
		Input:         json.RawMessage(`{"id":42,"due_at":"2026-08-30"}`),
	})
	if !errors.Is(err, ErrReadHandlersInvalid) {
		t.Fatalf("date-only err=%v", err)
	}
	_, err = handlers[TaskDueConflictList].Invoke(context.Background(), toolruntime.ToolCall{
		Input: json.RawMessage(`{"id":42,"due_at":"2026-08-30T10:00:00Z"}`),
	})
	if !errors.Is(err, ErrReadHandlersInvalid) || fake.workspace != 0 {
		t.Fatalf("missing workspace err=%v workspace=%d", err, fake.workspace)
	}
}
