package workflowgorm

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

// intakeIntakeLockStore mirrors the production app/api adapter: it bridges the
// workflow mutation transaction onto the project-module plan-unit row.
type intakeIntakeLockStore struct{ db *gorm.DB }

func (store intakeIntakeLockStore) RequirementIntakeLocked(_ context.Context, workspaceID, planUnitID uint) (bool, error) {
	var unit projectgorm.PlanUnit
	if err := store.db.Select("requirements_locked_at").
		Where("workspace_id = ? AND id = ?", workspaceID, planUnitID).First(&unit).Error; err != nil {
		return false, err
	}
	return unit.RequirementsLockedAt != nil, nil
}

func (store intakeIntakeLockStore) MarkRequirementIntakeLocked(
	_ context.Context, workspaceID, planUnitID, lockedByUserID uint, lockedAt time.Time,
) (bool, error) {
	return projectgorm.MarkPlanUnitRequirementsLockedTx(store.db, workspaceID, planUnitID, lockedByUserID, lockedAt)
}

// intakePlanRequirements mirrors the production adapter over the requirement
// module scope reader.
type intakePlanRequirements struct{ db *gorm.DB }

func (reader intakePlanRequirements) PlanUnitRequirementRefs(_ context.Context, workspaceID, planUnitID uint) ([]workflow.PlanRequirementRef, error) {
	raw, err := requirementgorm.PlanUnitRequirementWorkflowRefs(reader.db, workspaceID, planUnitID)
	if err != nil {
		return nil, err
	}
	refs := make([]workflow.PlanRequirementRef, 0, len(raw))
	for _, ref := range raw {
		item := workflow.PlanRequirementRef{RequirementID: ref.RequirementID, Status: ref.Status}
		if ref.WorkflowInstanceID != nil {
			item.WorkflowInstanceID = *ref.WorkflowInstanceID
		}
		if ref.TemplateVersionID != nil {
			item.TemplateVersionID = *ref.TemplateVersionID
		}
		refs = append(refs, item)
	}
	return refs, nil
}

func newIntakeMutationFixture(t *testing.T, nodeConfig string) (*MutationRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&WorkflowTemplate{}, &WorkflowTemplateVersion{}, &WorkflowInstance{}, &WorkflowToken{},
		&WorkflowNodeRun{}, &WorkflowApproval{}, &WorkflowNodeOpinion{}, &ToolMutationApply{}, &WorkflowOpinionInvitation{},
		&WorkflowJob{},
		&projectgorm.ProjectSet{}, &projectgorm.PlanUnit{},
		&requirementgorm.Requirement{},
	); err != nil {
		t.Fatal(err)
	}
	configSuffix := ""
	if nodeConfig != "" {
		configSuffix = `,"config":` + nodeConfig
	}
	definition := `{"nodes":[{"id":"start","type":"start","name":"开始"},{"id":"dev-gate","type":"human_task","name":"开发与实施"` + configSuffix + `},{"id":"end","type":"end","name":"结束"}],"edges":[{"id":"e1","source":"start","target":"dev-gate"},{"id":"e2","source":"dev-gate","target":"end"}]}`
	now := time.Now().UTC()
	template := WorkflowTemplate{ID: 11, WorkspaceID: 1, Name: "test", SubjectType: "plan_unit", PublishedVersion: 1}
	version := WorkflowTemplateVersion{ID: 21, WorkspaceID: 1, TemplateID: 11, Version: 1, Definition: definition, DefinitionHash: "hash", PublishedByUser: 7, PublishedAt: now}
	instance := WorkflowInstance{ID: 31, WorkspaceID: 1, TemplateID: 11, TemplateVersionID: 21, SubjectType: "plan_unit", SubjectID: 41, Status: "running", CurrentNodeIDs: `["dev-gate"]`, NodePlansJSON: `{}`, StartedByUserID: 7, StartedAt: now}
	token := WorkflowToken{WorkspaceID: 1, InstanceID: 31, NodeID: "dev-gate", Status: "active", AssigneeUserID: uintPointer(7)}
	projectSet := projectgorm.ProjectSet{ID: 51, WorkspaceID: 1, TeamID: 61, Name: "AI PM"}
	planUnit := projectgorm.PlanUnit{ID: 41, WorkspaceID: 1, TeamID: 61, ProjectSetID: 51, Title: "V1", Type: "version"}
	for _, row := range []any{&template, &version, &instance, &token, &projectSet, &planUnit} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	effects := 0
	repository, err := NewMutationRepository(db, func(tx *gorm.DB) (MutationDependencies, error) {
		return MutationDependencies{
			Subjects: mutationSubjectFake{subject: workflow.SubjectContext{
				ProjectSetID: 51, PlanUnitID: 41, TeamID: 61,
				SubjectID: 41, SubjectType: "plan_unit", SubjectTitle: "测试版本",
			}},
			Users: mutationUserFake{}, Reports: mutationReportFake{calls: &effects},
			Schedule: mutationScheduleFake{calls: &effects}, Effects: mutationEffectFake{calls: &effects},
			Assignments: mutationAssignmentFake{calls: &effects},
			Invitations: mutationInvitationFake{calls: &effects},
			AI:          mutationAIFake{}, Designs: mutationDesignFake{},
			IntakeLocks:      intakeIntakeLockStore{db: tx},
			IntakeEffects:    intakeEffectFake{},
			PlanRequirements: intakePlanRequirements{db: tx},
		}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return repository, db
}

type intakeEffectFake struct{}

func (intakeEffectFake) RecordRequirementIntakeLocked(context.Context, workflow.RequirementIntakeEffect) error {
	return nil
}

func seedIntakeRequirementEvidence(t *testing.T, db *gorm.DB, devCompleted bool) {
	t.Helper()
	now := time.Now().UTC()
	requirementTemplate := WorkflowTemplate{ID: 12, WorkspaceID: 1, Name: "req-flow", SubjectType: "requirement", PublishedVersion: 1}
	requirementVersion := WorkflowTemplateVersion{ID: 22, WorkspaceID: 1, TemplateID: 12, Version: 1,
		Definition:     `{"nodes":[{"id":"start","type":"start","name":"开始"},{"id":"dev","type":"human_task","name":"开发"},{"id":"end","type":"end","name":"结束"}],"edges":[{"id":"e1","source":"start","target":"dev"},{"id":"e2","source":"dev","target":"end"}]}`,
		DefinitionHash: "req-hash", PublishedByUser: 7, PublishedAt: now}
	requirementInstance := WorkflowInstance{ID: 32, WorkspaceID: 1, TemplateID: 12, TemplateVersionID: 22,
		SubjectType: "requirement", SubjectID: 81, Status: "running", CurrentNodeIDs: `["dev"]`,
		NodePlansJSON: "{}", StartedByUserID: 7, StartedAt: now}
	requirement := requirementgorm.Requirement{ID: 81, WorkspaceID: 1, TeamID: 61, ProjectSetID: 51,
		WorkflowInstanceID: uintPointer(32), Title: "联动需求", Status: "development", Version: 1}
	requirement.ScheduledPlanUnitID = uintPointer(41)
	startRun := WorkflowNodeRun{WorkspaceID: 1, InstanceID: 32, NodeID: "start", NodeType: "start", Status: "completed", StartedAt: now, CompletedAt: &now}
	for _, row := range []any{&requirementTemplate, &requirementVersion, &requirementInstance, &requirement, &startRun} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	if devCompleted {
		run := WorkflowNodeRun{WorkspaceID: 1, InstanceID: 32, NodeID: "dev", NodeType: "human_task", Status: "completed", StartedAt: now, CompletedAt: &now}
		if err := db.Create(&run).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestMutationIntakeLockRequiresExplicitConfirmation(t *testing.T) {
	repository, db := newIntakeMutationFixture(t, `{"requirement_intake_lock":true}`)
	command := workflow.Mutation{
		Operation: workflow.ActionExecute, WorkspaceID: 1, PrincipalID: 7, InstanceID: 31,
		NodeID: "dev-gate", Action: "complete", ApplyKey: "intake-0001", InputDigest: mutationDigest("a"),
	}
	if _, err := repository.ApplyWorkflowMutation(context.Background(), command); err != workflow.ErrRequirementLockConfirmationRequired {
		t.Fatalf("expected confirmation required, got %v", err)
	}
	var unit projectgorm.PlanUnit
	if err := db.First(&unit, 41).Error; err != nil || unit.RequirementsLockedAt != nil {
		t.Fatalf("plan unit must stay unlocked, err=%v unit=%+v", err, unit)
	}
	confirmed := command
	confirmed.ApplyKey, confirmed.InputDigest = "intake-0002", mutationDigest("b")
	confirmed.ConfirmRequirementLock = true
	if _, err := repository.ApplyWorkflowMutation(context.Background(), confirmed); err != nil {
		t.Fatalf("confirmed completion failed: %v", err)
	}
	if err := db.First(&unit, 41).Error; err != nil || unit.RequirementsLockedAt == nil || unit.RequirementsLockedByUserID == nil || *unit.RequirementsLockedByUserID != 7 {
		t.Fatalf("plan unit must be locked by user 7, err=%v unit=%+v", err, unit)
	}
}

func TestMutationIntakeLockFlagRejectedOnPlainNodes(t *testing.T) {
	repository, _ := newIntakeMutationFixture(t, "")
	command := workflow.Mutation{
		Operation: workflow.ActionExecute, WorkspaceID: 1, PrincipalID: 7, InstanceID: 31,
		NodeID: "dev-gate", Action: "complete", ConfirmRequirementLock: true,
		ApplyKey: "intake-plain-0001", InputDigest: mutationDigest("c"),
	}
	if _, err := repository.ApplyWorkflowMutation(context.Background(), command); err != workflow.ErrMutationInvalid {
		t.Fatalf("flag on plain node must be invalid, got %v", err)
	}
}

func TestMutationRequirementsGuardBlocksUntilEvidenceSatisfied(t *testing.T) {
	repository, db := newIntakeMutationFixture(t, `{"requirements_guard":{"schema_version":1,"source":"template_nodes","scope":"current_plan_requirements","bindings":[{"requirement_template_version_id":22,"condition":{"all_of":[{"node_id":"dev","success_action":"complete"}]}}],"aggregate":{"mode":"all"},"on_satisfied":"enable_action","empty_scope":"block","missing_binding":"block"}}`)
	seedIntakeRequirementEvidence(t, db, false)
	command := workflow.Mutation{
		Operation: workflow.ActionExecute, WorkspaceID: 1, PrincipalID: 7, InstanceID: 31,
		NodeID: "dev-gate", Action: "complete", ApplyKey: "guard-0001", InputDigest: mutationDigest("d"),
	}
	if _, err := repository.ApplyWorkflowMutation(context.Background(), command); err == nil {
		t.Fatal("guard without evidence must block the completion")
	} else {
		var blocked *workflow.GuardBlockedError
		if !asGuardBlocked(err, &blocked) {
			t.Fatalf("expected guard blocked error, got %v", err)
		}
		if blocked.Evaluation.State != workflow.GuardUnsatisfied || blocked.Evaluation.TotalCount != 1 {
			t.Fatalf("unexpected evaluation: %+v", blocked.Evaluation)
		}
	}
	seedIntakeRequirementEvidenceDevRun(t, db)
	if _, err := repository.ApplyWorkflowMutation(context.Background(), command); err != nil {
		t.Fatalf("guard with satisfied evidence must pass, got %v", err)
	}
}

func asGuardBlocked(err error, target **workflow.GuardBlockedError) bool {
	if blocked, ok := err.(*workflow.GuardBlockedError); ok {
		*target = blocked
		return true
	}
	return false
}

func seedIntakeRequirementEvidenceDevRun(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC()
	run := WorkflowNodeRun{WorkspaceID: 1, InstanceID: 32, NodeID: "dev", NodeType: "human_task", Status: "completed", StartedAt: now, CompletedAt: &now}
	if err := db.Create(&run).Error; err != nil {
		t.Fatal(err)
	}
}
