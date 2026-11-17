package workflowgorm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

type subjectStartAIStub struct{ calls int }

func (stub *subjectStartAIStub) StartSubjectAI(context.Context, workflow.SubjectAIStart) (uint, error) {
	stub.calls++
	return 99, nil
}

type subjectStartAssignmentStub struct{ calls int }

func (stub *subjectStartAssignmentStub) RecordSubjectAssignment(context.Context, workflow.SubjectAssignment) error {
	stub.calls++
	return nil
}

type subjectStartEffectStub struct{ calls int }

func (stub *subjectStartEffectStub) RecordWorkflowAction(context.Context, workflow.ActionEffect) error {
	stub.calls++
	return nil
}

func TestSubjectStartCreatesWorkflowOnceAndReplaysDurableResult(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&WorkflowTemplate{}, &WorkflowTemplateVersion{}, &WorkflowInstance{}, &WorkflowToken{},
		&WorkflowNodeRun{}, &WorkflowJob{}, &ToolMutationApply{}); err != nil {
		t.Fatal(err)
	}
	definition := `{"nodes":[{"id":"start","type":"start","name":"开始"},{"id":"review","type":"human_task","name":"设计确认"},{"id":"end","type":"end","name":"结束"}],"edges":[{"id":"a","source":"start","target":"review"},{"id":"b","source":"review","target":"end"}]}`
	template := WorkflowTemplate{ID: 10, WorkspaceID: 7, Name: "Plan", SubjectType: "plan_unit", PublishedVersion: 1}
	version := WorkflowTemplateVersion{ID: 11, WorkspaceID: 7, TemplateID: 10, Version: 1, Definition: definition, DefinitionHash: strings.Repeat("a", 64)}
	if err := db.Create(&template).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&version).Error; err != nil {
		t.Fatal(err)
	}
	ai, assignments, effects := &subjectStartAIStub{}, &subjectStartAssignmentStub{}, &subjectStartEffectStub{}
	repository, err := NewSubjectStartRepository(db, func(*gorm.DB) (SubjectStartDependencies, error) {
		return SubjectStartDependencies{AI: ai, Assignments: assignments, Effects: effects}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	command := workflow.SubjectStartCommand{WorkspaceID: 7, PrincipalID: 9, ApplyKey: "plan-create-0001",
		InputDigest: strings.Repeat("b", 64), TemplateID: 10, SubjectType: "plan_unit", SubjectID: 12,
		SubjectVersion: 1, ProjectSetID: 13, TeamID: 14, SubjectTitle: "V1", SubjectSnapshot: json.RawMessage(`{"id":12}`)}
	result, err := repository.StartSubject(context.Background(), command)
	if err != nil || result.InstanceID == 0 || result.SubjectStatus != "grooming" || len(result.CurrentNodeIDs) != 1 || result.CurrentNodeIDs[0] != "review" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	replayed, err := repository.StartSubject(context.Background(), command)
	if err != nil || replayed.InstanceID != result.InstanceID {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	var instances, tokens, startRuns, ledgers int64
	db.Model(&WorkflowInstance{}).Count(&instances)
	db.Model(&WorkflowToken{}).Where("assignee_user_id = ? AND status = ?", 9, "active").Count(&tokens)
	db.Model(&WorkflowNodeRun{}).Where("node_type = ? AND status = ?", "start", "completed").Count(&startRuns)
	db.Model(&ToolMutationApply{}).Where("apply_key = ? AND result_json <> ''", command.ApplyKey).Count(&ledgers)
	if instances != 1 || tokens != 1 || startRuns != 1 || ledgers != 1 || assignments.calls != 1 || effects.calls != 1 || ai.calls != 0 {
		t.Fatalf("instances=%d tokens=%d runs=%d ledgers=%d assignment=%d effects=%d ai=%d",
			instances, tokens, startRuns, ledgers, assignments.calls, effects.calls, ai.calls)
	}
}
