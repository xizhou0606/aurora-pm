package workflowgorm

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
)

const gateGuardConfig = `{"requirements_guard":{"schema_version":1,"source":"template_nodes","scope":"current_plan_requirements","bindings":[{"requirement_template_version_id":22,"condition":{"all_of":[{"node_id":"dev","success_action":"complete"}]}}],"aggregate":{"mode":"all"},"on_satisfied":"auto_complete","empty_scope":"block","missing_binding":"block"}}`

func newGateReconcileFixture(t *testing.T) (*DeliveryReconcileRepository, *workflow.DeliveryReconcileService, *gorm.DB) {
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
		&auditgorm.AuditLog{}, &auditgorm.OutboxEvent{},
	); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	definition := `{"nodes":[{"id":"start","type":"start","name":"开始"},{"id":"dev-gate","type":"requirement_gate","name":"等待开发完成","config":` + gateGuardConfig + `},{"id":"end","type":"end","name":"结束"}],"edges":[{"id":"e1","source":"start","target":"dev-gate"},{"id":"e2","source":"dev-gate","target":"end"}]}`
	template := WorkflowTemplate{ID: 11, WorkspaceID: 1, Name: "gate-flow", SubjectType: "plan_unit", PublishedVersion: 1}
	version := WorkflowTemplateVersion{ID: 21, WorkspaceID: 1, TemplateID: 11, Version: 1, Definition: definition, DefinitionHash: "hash", PublishedByUser: 7, PublishedAt: now}
	instance := WorkflowInstance{ID: 31, WorkspaceID: 1, TemplateID: 11, TemplateVersionID: 21, SubjectType: "plan_unit", SubjectID: 41, Status: "running", CurrentNodeIDs: `["dev-gate"]`, NodePlansJSON: `{}`, StartedByUserID: 7, StartedAt: now}
	token := WorkflowToken{WorkspaceID: 1, InstanceID: 31, NodeID: "dev-gate", Status: "active"}
	projectSet := projectgorm.ProjectSet{ID: 51, WorkspaceID: 1, TeamID: 61, Name: "AI PM"}
	planUnit := projectgorm.PlanUnit{ID: 41, WorkspaceID: 1, TeamID: 61, ProjectSetID: 51, Title: "V1", Type: "version"}
	for _, row := range []any{&template, &version, &instance, &token, &projectSet, &planUnit} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository, err := NewDeliveryReconcileRepository(db, func(tx *gorm.DB) (MutationDependencies, error) {
		return MutationDependencies{PlanRequirements: intakePlanRequirements{db: tx}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := workflow.NewDeliveryReconcileService(repository)
	if err != nil {
		t.Fatal(err)
	}
	return repository, service, db
}

func TestReconcileAutoAdvancesSatisfiedGate(t *testing.T) {
	_, service, db := newGateReconcileFixture(t)
	seedIntakeRequirementEvidence(t, db, false)
	// 证据未满足：不推进，token 保持。
	advanced, err := service.ReconcilePlanUnitGates(context.Background(), 1, 41)
	if err != nil || advanced != 0 {
		t.Fatalf("unsatisfied gate must not advance, advanced=%d err=%v", advanced, err)
	}
	var token WorkflowToken
	if err := db.Where("instance_id = ? AND node_id = ?", 31, "dev-gate").First(&token).Error; err != nil || token.Status != "active" {
		t.Fatalf("gate token must stay active, got %s err=%v", token.Status, err)
	}
	// 证据满足后重算：gate 自动完成，流程推进到 end 并结束实例。
	seedIntakeRequirementEvidenceDevRun(t, db)
	advanced, err = service.ReconcilePlanUnitGates(context.Background(), 1, 41)
	if err != nil || advanced != 1 {
		t.Fatalf("satisfied gate must auto-advance once, advanced=%d err=%v", advanced, err)
	}
	var instance WorkflowInstance
	if err := db.First(&instance, 31).Error; err != nil {
		t.Fatal(err)
	}
	if instance.Status != "completed" {
		t.Fatalf("instance must complete after gate pass, got %s %s", instance.Status, instance.CurrentNodeIDs)
	}
	var run WorkflowNodeRun
	if err := db.Where("instance_id = ? AND node_id = ? AND status = ?", 31, "dev-gate", "completed").First(&run).Error; err != nil {
		t.Fatalf("gate run missing: %v", err)
	}
	if run.OutputJSON != `{"system_actor":true}` {
		t.Fatalf("gate run must record the system actor, got %s", run.OutputJSON)
	}
	var unit projectgorm.PlanUnit
	if err := db.First(&unit, 41).Error; err != nil {
		t.Fatal(err)
	}
	// 完成的项目/版本流程按既有 SubjectStageStatus 映射为 released。
	if unit.Status != "released" {
		t.Fatalf("plan unit status must sync, got %s", unit.Status)
	}
	var audits int64
	db.Model(&auditgorm.AuditLog{}).Where("resource = ? AND action = ?", "workflow_instance", "auto_completed").Count(&audits)
	if audits != 1 {
		t.Fatalf("system audit row missing, got %d", audits)
	}
	// 幂等：重复重算不再推进。
	advanced, err = service.ReconcilePlanUnitGates(context.Background(), 1, 41)
	if err != nil || advanced != 0 {
		t.Fatalf("repeat reconcile must be a no-op, advanced=%d err=%v", advanced, err)
	}
}

// 需求阶段驱动：人工节点按名称等待所有需求达到阶段，由消费者自动推进。
func TestReconcileAutoAdvancesStageDrivenNode(t *testing.T) {
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
		&auditgorm.AuditLog{}, &auditgorm.OutboxEvent{},
	); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	definition := `{"nodes":[{"id":"start","type":"start","name":"开始"},{"id":"dev-node","type":"human_task","name":"开发与实施"},{"id":"end","type":"end","name":"结束"}],"edges":[{"id":"e1","source":"start","target":"dev-node"},{"id":"e2","source":"dev-node","target":"end"}]}`
	template := WorkflowTemplate{ID: 11, WorkspaceID: 1, Name: "stage-flow", SubjectType: "plan_unit", PublishedVersion: 1}
	version := WorkflowTemplateVersion{ID: 21, WorkspaceID: 1, TemplateID: 11, Version: 1, Definition: definition, DefinitionHash: "hash", PublishedByUser: 7, PublishedAt: now}
	instance := WorkflowInstance{ID: 31, WorkspaceID: 1, TemplateID: 11, TemplateVersionID: 21, SubjectType: "plan_unit", SubjectID: 41, Status: "running", CurrentNodeIDs: `["dev-node"]`, NodePlansJSON: `{}`, StartedByUserID: 7, StartedAt: now}
	token := WorkflowToken{WorkspaceID: 1, InstanceID: 31, NodeID: "dev-node", Status: "active"}
	projectSet := projectgorm.ProjectSet{ID: 51, WorkspaceID: 1, TeamID: 61, Name: "AI PM"}
	planUnit := projectgorm.PlanUnit{ID: 41, WorkspaceID: 1, TeamID: 61, ProjectSetID: 51, Title: "V1", Type: "version"}
	// 需求已完成（status=done）：阶段 development 达标。
	requirement := requirementgorm.Requirement{ID: 81, WorkspaceID: 1, TeamID: 61, ProjectSetID: 51, Title: "联动需求", Status: "done", Version: 1}
	requirement.ScheduledPlanUnitID = uintPointer(41)
	for _, row := range []any{&template, &version, &instance, &token, &projectSet, &planUnit, &requirement} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	repository, err := NewDeliveryReconcileRepository(db, func(tx *gorm.DB) (MutationDependencies, error) {
		return MutationDependencies{PlanRequirements: intakePlanRequirements{db: tx}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := workflow.NewDeliveryReconcileService(repository)
	if err != nil {
		t.Fatal(err)
	}
	advanced, err := service.ReconcilePlanUnitGates(context.Background(), 1, 41)
	if err != nil || advanced != 1 {
		t.Fatalf("stage-driven node must auto-advance, advanced=%d err=%v", advanced, err)
	}
	var done WorkflowInstance
	if err := db.First(&done, 31).Error; err != nil {
		t.Fatal(err)
	}
	if done.Status != "completed" {
		t.Fatalf("instance must complete, got %s %s", done.Status, done.CurrentNodeIDs)
	}
}
