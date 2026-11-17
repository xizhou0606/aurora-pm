package workflowgorm

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

type mutationSubjectFake struct{ subject workflow.SubjectContext }

func (fake mutationSubjectFake) ResolveWorkflowSubject(context.Context, uint, string, uint) (workflow.SubjectContext, error) {
	return fake.subject, nil
}

type mutationUserFake struct{}

func (mutationUserFake) VerifyActiveWorkspaceUsers(_ context.Context, _ uint, ids []uint) error {
	for _, id := range ids {
		if id == 99 {
			return workflow.ErrMutationConflict
		}
	}
	return nil
}

type mutationReportFake struct{ calls *int }

func (fake mutationReportFake) CreateWorkflowTestReportLink(
	_ context.Context, workspaceID uint, subject workflow.SubjectContext, principalID uint, _, name, rawURL string,
) (workflow.TestReportLink, error) {
	*fake.calls++
	return workflow.TestReportLink{
		ID: 71, WorkspaceID: workspaceID, PlanUnitID: subject.PlanUnitID, ProjectSetID: subject.ProjectSetID,
		Kind: "link", Name: name, URL: rawURL, UploaderUserID: principalID, CreatedAt: time.Now().UTC(),
	}, nil
}

type mutationScheduleFake struct{ calls *int }

func (fake mutationScheduleFake) ConfirmWorkflowSchedule(
	_ context.Context, _ uint, _ workflow.SubjectContext, _, _ *time.Time,
) error {
	*fake.calls++
	return nil
}

type mutationEffectFake struct{ calls *int }

func (fake mutationEffectFake) RecordWorkflowAction(context.Context, workflow.ActionEffect) error {
	*fake.calls++
	return nil
}

type mutationAssignmentFake struct{ calls *int }

func (fake mutationAssignmentFake) RecordSubjectAssignment(context.Context, workflow.SubjectAssignment) error {
	*fake.calls++
	return nil
}

type mutationInvitationFake struct{ calls *int }

func (fake mutationInvitationFake) RecordPreReviewInvitation(context.Context, workflow.PreReviewInvitation) error {
	*fake.calls++
	return nil
}

type mutationAIFake struct{}

func (mutationAIFake) PrepareWorkflowAIAction(context.Context, workflow.AIActionRequest) (workflow.AIActionResult, error) {
	return workflow.AIActionResult{}, workflow.ErrMutationConflict
}

type mutationDesignFake struct{}

func (mutationDesignFake) AttachWorkflowDesign(context.Context, uint, uint, string) error { return nil }

func TestMutationRepositoryOpinionReplayAndOwnership(t *testing.T) {
	repository, db, deps := newWorkflowMutationFixture(t, "pre-review", "approval", "产品预审")
	command := workflow.Mutation{
		Operation: workflow.OpinionCreate, WorkspaceID: 1, PrincipalID: 7, InstanceID: 31,
		NodeID: "pre-review", Body: "同意", InviteUserIDs: []uint{8, 8},
		ApplyKey: "workflow-opinion-create-0001", InputDigest: mutationDigest("a"),
	}
	first, err := repository.ApplyWorkflowMutation(context.Background(), command)
	if err != nil || first.Opinion == nil || first.Opinion.AuthorUserID != 7 || len(first.InvitedUserIDs) != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	if *deps.invitationCalls != 1 {
		t.Fatalf("invitation calls=%d", *deps.invitationCalls)
	}
	second, err := repository.ApplyWorkflowMutation(context.Background(), command)
	if err != nil || second.Opinion == nil || second.Opinion.ID != first.Opinion.ID {
		t.Fatalf("replay=%+v err=%v", second, err)
	}
	if *deps.invitationCalls != 1 {
		t.Fatalf("replay invitation calls=%d", *deps.invitationCalls)
	}
	assertWorkflowMutationCounts(t, db, 1, 1, 1)

	update := workflow.Mutation{
		Operation: workflow.OpinionUpdate, WorkspaceID: 1, PrincipalID: 8, InstanceID: 31,
		NodeID: "pre-review", OpinionID: first.Opinion.ID, Body: "越权修改",
		ApplyKey: "workflow-opinion-update-0001", InputDigest: mutationDigest("b"),
	}
	if _, err := repository.ApplyWorkflowMutation(context.Background(), update); !errors.Is(err, workflow.ErrMutationConflict) {
		t.Fatalf("foreign update err=%v", err)
	}
	assertWorkflowMutationCounts(t, db, 1, 1, 1)
}

func TestMutationRepositoryNodePlanCASAndReportActiveScope(t *testing.T) {
	repository, db, deps := newWorkflowMutationFixture(t, "test-report", "human_task", "测试报告")
	start := time.Date(2026, 8, 28, 9, 0, 0, 0, time.UTC)
	plan := workflow.Mutation{
		Operation: workflow.NodePlanUpdate, WorkspaceID: 1, PrincipalID: 7, InstanceID: 31,
		ApplyKey: "workflow-node-plan-update-0001", InputDigest: mutationDigest("c"), ConfirmSchedule: true,
		NodePlans: []workflow.NodePlanMutation{{
			NodeID: "test-report", PlannedStartAt: workflow.OptionalTime{Set: true, Value: &start},
			ExpectedStartAt: workflow.OptionalTime{Set: true, Value: nil}, AssigneeUserIDSet: true, AssigneeUserID: 7,
		}},
	}
	if _, err := repository.ApplyWorkflowMutation(context.Background(), plan); err != nil {
		t.Fatalf("plan: %v", err)
	}
	if *deps.scheduleCalls != 1 {
		t.Fatalf("schedule calls=%d", *deps.scheduleCalls)
	}
	if *deps.effectCalls != 1 {
		t.Fatalf("effect calls=%d", *deps.effectCalls)
	}
	if *deps.assignmentCalls != 1 {
		t.Fatalf("assignment calls=%d", *deps.assignmentCalls)
	}
	if _, err := repository.ApplyWorkflowMutation(context.Background(), plan); err != nil || *deps.scheduleCalls != 1 || *deps.effectCalls != 1 {
		t.Fatalf("plan replay schedule=%d effects=%d err=%v", *deps.scheduleCalls, *deps.effectCalls, err)
	}
	if *deps.assignmentCalls != 1 {
		t.Fatalf("plan replay assignment calls=%d", *deps.assignmentCalls)
	}
	stale := plan
	stale.ApplyKey, stale.InputDigest = "workflow-node-plan-update-0002", mutationDigest("d")
	if _, err := repository.ApplyWorkflowMutation(context.Background(), stale); !errors.Is(err, workflow.ErrMutationConflict) {
		t.Fatalf("stale err=%v", err)
	}

	report := workflow.Mutation{
		Operation: workflow.TestReportLinkCreate, WorkspaceID: 1, PrincipalID: 7, InstanceID: 31,
		NodeID: "test-report", URL: "https://reports.example.test/run/71",
		ApplyKey: "workflow-test-report-link-0001", InputDigest: mutationDigest("e"),
	}
	if _, err := repository.ApplyWorkflowMutation(context.Background(), report); err != nil {
		t.Fatalf("report: %v", err)
	}
	if *deps.reportCalls != 1 {
		t.Fatalf("report calls=%d", *deps.reportCalls)
	}
	var ledgerCount int64
	if err := db.Model(&ToolMutationApply{}).Count(&ledgerCount).Error; err != nil || ledgerCount != 2 {
		t.Fatalf("ledgers=%d err=%v", ledgerCount, err)
	}
}

func TestMutationRepositoryActionTransitionIsCASAndReplaySafe(t *testing.T) {
	repository, db, deps := newWorkflowMutationFixture(t, "review", "approval", "审批")
	command := workflow.Mutation{
		Operation: workflow.ActionExecute, WorkspaceID: 1, PrincipalID: 7, InstanceID: 31,
		NodeID: "review", Action: "approve", Comment: "通过",
		ApplyKey: "workflow-action-approve-0001", InputDigest: mutationDigest("action"),
	}
	first, err := repository.ApplyWorkflowMutation(context.Background(), command)
	if err != nil || first.Status != "completed" || len(first.CurrentNodeIDs) != 0 || first.Action != "approve" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	second, err := repository.ApplyWorkflowMutation(context.Background(), command)
	if err != nil || second.Status != first.Status || *deps.effectCalls != 1 {
		t.Fatalf("replay=%+v effects=%d err=%v", second, *deps.effectCalls, err)
	}
	var instance WorkflowInstance
	if err := db.First(&instance, 31).Error; err != nil || instance.Status != "completed" || instance.CurrentNodeIDs != "[]" {
		t.Fatalf("instance=%+v err=%v", instance, err)
	}
	for model, want := range map[any]int64{
		&WorkflowNodeRun{}: 2, &WorkflowApproval{}: 1, &ToolMutationApply{}: 1,
	} {
		var got int64
		if err := db.Model(model).Count(&got).Error; err != nil || got != want {
			t.Fatalf("count %T=%d/%d err=%v", model, got, want, err)
		}
	}

	conflicting := command
	conflicting.ApplyKey, conflicting.InputDigest = "workflow-action-approve-0002", mutationDigest("other")
	if _, err := repository.ApplyWorkflowMutation(context.Background(), conflicting); !errors.Is(err, workflow.ErrMutationConflict) {
		t.Fatalf("stale action err=%v", err)
	}
	var ledgers int64
	if err := db.Model(&ToolMutationApply{}).Count(&ledgers).Error; err != nil || ledgers != 1 {
		t.Fatalf("ledgers=%d err=%v", ledgers, err)
	}
}

func TestMutationRepositoryConditionUsesExactBranchAndWorkspaceAssignee(t *testing.T) {
	repository, db, deps := newWorkflowMutationFixture(t, "choice", "condition", "选择")
	definition := `{"nodes":[{"id":"start","type":"start","name":"开始"},{"id":"choice","type":"condition","name":"选择"},{"id":"left","type":"human_task","name":"左"},{"id":"right","type":"human_task","name":"右"},{"id":"end","type":"end","name":"结束"}],"edges":[{"id":"e1","source":"start","target":"choice"},{"id":"e2","source":"choice","target":"left","condition":"context.decision == true"},{"id":"e3","source":"choice","target":"right","condition":"context.decision == false"},{"id":"e4","source":"left","target":"end"},{"id":"e5","source":"right","target":"end"}]}`
	if err := db.Model(&WorkflowTemplateVersion{}).Where("id = ?", 21).Update("definition", definition).Error; err != nil {
		t.Fatal(err)
	}
	command := workflow.Mutation{
		Operation: workflow.ActionExecute, WorkspaceID: 1, PrincipalID: 7, InstanceID: 31,
		NodeID: "choice", Action: "condition_yes", NextAssignees: map[string]uint{"left": 8},
		ApplyKey: "workflow-action-condition-0001", InputDigest: mutationDigest("condition"),
	}
	result, err := repository.ApplyWorkflowMutation(context.Background(), command)
	if err != nil || len(result.CurrentNodeIDs) != 1 || result.CurrentNodeIDs[0] != "left" || *deps.effectCalls != 1 {
		t.Fatalf("result=%+v effects=%d err=%v", result, *deps.effectCalls, err)
	}
	if *deps.assignmentCalls != 1 {
		t.Fatalf("assignment calls=%d", *deps.assignmentCalls)
	}
	var token WorkflowToken
	if err := db.Where("instance_id = ? AND node_id = ? AND status = ?", 31, "left", "active").First(&token).Error; err != nil || token.AssigneeUserID == nil || *token.AssigneeUserID != 8 {
		t.Fatalf("left token=%+v err=%v", token, err)
	}
	var rightTokens int64
	if err := db.Model(&WorkflowToken{}).Where("instance_id = ? AND node_id = ?", 31, "right").Count(&rightTokens).Error; err != nil || rightTokens != 0 {
		t.Fatalf("right tokens=%d err=%v", rightTokens, err)
	}
}

type workflowMutationFixtureDependencies struct {
	reportCalls     *int
	scheduleCalls   *int
	effectCalls     *int
	assignmentCalls *int
	invitationCalls *int
}

func newWorkflowMutationFixture(
	t *testing.T, nodeID, nodeType, nodeName string,
) (*MutationRepository, *gorm.DB, workflowMutationFixtureDependencies) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&WorkflowTemplate{}, &WorkflowTemplateVersion{}, &WorkflowInstance{}, &WorkflowToken{},
		&WorkflowNodeRun{}, &WorkflowApproval{}, &WorkflowNodeOpinion{}, &ToolMutationApply{}, &WorkflowOpinionInvitation{},
		&WorkflowJob{},
	); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	template := WorkflowTemplate{ID: 11, WorkspaceID: 1, Name: "test", SubjectType: "plan_unit", PublishedVersion: 1}
	definition := `{"nodes":[{"id":"start","type":"start","name":"开始"},{"id":"` + nodeID + `","type":"` + nodeType + `","name":"` + nodeName + `"},{"id":"end","type":"end","name":"结束"}],"edges":[{"id":"e1","source":"start","target":"` + nodeID + `"},{"id":"e2","source":"` + nodeID + `","target":"end"}]}`
	version := WorkflowTemplateVersion{ID: 21, WorkspaceID: 1, TemplateID: 11, Version: 1, Definition: definition, DefinitionHash: "hash", PublishedByUser: 7, PublishedAt: now}
	instance := WorkflowInstance{ID: 31, WorkspaceID: 1, TemplateID: 11, TemplateVersionID: 21, SubjectType: "plan_unit", SubjectID: 41, Status: "running", CurrentNodeIDs: `["` + nodeID + `"]`, NodePlansJSON: `{}`, StartedByUserID: 7, StartedAt: now}
	token := WorkflowToken{WorkspaceID: 1, InstanceID: 31, NodeID: nodeID, Status: "active", AssigneeUserID: uintPointer(7)}
	for _, row := range []any{&template, &version, &instance, &token} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	reportCalls, scheduleCalls, effectCalls, assignmentCalls, invitationCalls := 0, 0, 0, 0, 0
	repository, err := NewMutationRepository(db, func(*gorm.DB) (MutationDependencies, error) {
		return MutationDependencies{
			Subjects: mutationSubjectFake{subject: workflow.SubjectContext{
				ProjectSetID: 51, PlanUnitID: 41, TeamID: 61,
				SubjectID: 41, SubjectType: "plan_unit", SubjectTitle: "测试版本",
			}},
			Users: mutationUserFake{}, Reports: mutationReportFake{calls: &reportCalls},
			Schedule: mutationScheduleFake{calls: &scheduleCalls}, Effects: mutationEffectFake{calls: &effectCalls},
			Assignments: mutationAssignmentFake{calls: &assignmentCalls},
			Invitations: mutationInvitationFake{calls: &invitationCalls},
			AI:          mutationAIFake{}, Designs: mutationDesignFake{},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return repository, db, workflowMutationFixtureDependencies{
		reportCalls: &reportCalls, scheduleCalls: &scheduleCalls, effectCalls: &effectCalls,
		assignmentCalls: &assignmentCalls, invitationCalls: &invitationCalls,
	}
}

func assertWorkflowMutationCounts(t *testing.T, db *gorm.DB, opinions, invites, ledgers int64) {
	t.Helper()
	for model, want := range map[any]int64{&WorkflowNodeOpinion{}: opinions, &WorkflowOpinionInvitation{}: invites, &ToolMutationApply{}: ledgers} {
		var got int64
		if err := db.Model(model).Count(&got).Error; err != nil || got != want {
			t.Fatalf("count %T=%d/%d err=%v", model, got, want, err)
		}
	}
}

func mutationDigest(value string) string {
	for len(value) < 64 {
		value += "0"
	}
	return value[:64]
}

func uintPointer(value uint) *uint { return &value }
