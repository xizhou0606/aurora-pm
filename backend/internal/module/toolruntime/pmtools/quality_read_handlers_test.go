package pmtools

import (
	"context"
	"encoding/json"
	"reflect"
	"sort"
	"testing"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

type qualityReadFake struct {
	workspace uint
	filter    quality.BugReadFilter
	bugID     uint
}

func (fake *qualityReadFake) ListBugs(_ context.Context, workspaceID uint, filter quality.BugReadFilter) (quality.BugReadPage, error) {
	fake.workspace, fake.filter = workspaceID, filter
	return quality.BugReadPage{Items: []quality.BugSnapshot{{ID: 1, WorkspaceID: workspaceID, BugKey: "BUG-1", Title: "Bug", Status: "triage", Priority: "high"}}, Total: 1}, nil
}
func (fake *qualityReadFake) GetBug(_ context.Context, workspaceID, bugID uint) (quality.BugSnapshot, error) {
	fake.workspace, fake.bugID = workspaceID, bugID
	return quality.BugSnapshot{ID: bugID, WorkspaceID: workspaceID, BugKey: "BUG-1", Title: "Bug", Status: "triage", Priority: "high"}, nil
}
func (fake *qualityReadFake) ListBugActivities(_ context.Context, workspaceID, bugID uint, _, _ int) (quality.BugActivityPage, error) {
	fake.workspace, fake.bugID = workspaceID, bugID
	return quality.BugActivityPage{Items: []quality.BugActivitySnapshot{}, Total: 0}, nil
}

func TestQualityReadHandlersExposeCanonicalNamesAndServerWorkspace(t *testing.T) {
	fake := &qualityReadFake{}
	handlers, err := NewQualityReadHandlers(fake)
	if err != nil {
		t.Fatal(err)
	}
	got, want := make([]string, 0, len(handlers)), QualityReadHandlerNames()
	for name := range handlers {
		got = append(got, name)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) || len(got) != 3 {
		t.Fatalf("handlers=%v want=%v", got, want)
	}
	raw, err := handlers[QualityBugList].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7},
		Input:         json.RawMessage(`{"q":"login","status":"triage,reopened","project_new_id":3,"defect_domain":"ui","limit":25}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if fake.workspace != 7 || fake.filter.PlanUnitID != 3 || fake.filter.Limit != 25 || len(fake.filter.Statuses) != 1 ||
		len(fake.filter.DefectDomains) != 1 || fake.filter.DefectDomains[0] != quality.BugDefectDomainUI {
		t.Fatalf("workspace=%d filter=%+v", fake.workspace, fake.filter)
	}
	var page quality.BugReadPage
	if err := json.Unmarshal(raw, &page); err != nil || page.Total != 1 || page.Items[0].WorkspaceID != 7 {
		t.Fatalf("output=%s err=%v", raw, err)
	}
}

func TestQualityReadHandlersRejectUnknownInputAndUseID(t *testing.T) {
	fake := &qualityReadFake{}
	handlers, _ := NewQualityReadHandlers(fake)
	if _, err := handlers[QualityBugList].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7}, Input: json.RawMessage(`{"unknown":true}`),
	}); err == nil {
		t.Fatal("expected unknown input rejection")
	}
	if _, err := handlers[QualityBugGet].Invoke(context.Background(), toolruntime.ToolCall{
		Authorization: toolruntime.ExecutionAuthorization{WorkspaceID: 7}, Input: json.RawMessage(`{"id":9}`),
	}); err != nil || fake.bugID != 9 || fake.workspace != 7 {
		t.Fatalf("bugID=%d workspace=%d err=%v", fake.bugID, fake.workspace, err)
	}
}
