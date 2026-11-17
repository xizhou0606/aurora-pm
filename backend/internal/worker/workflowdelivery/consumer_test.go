package workflowdelivery

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	auditgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/audit/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	workflowgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/workflow/gorm"
	"gorm.io/gorm"
)

const consumerGateDefinition = `{"nodes":[{"id":"start","type":"start","name":"开始"},{"id":"dev-gate","type":"requirement_gate","name":"等待开发完成","config":{"requirements_guard":{"schema_version":1,"source":"template_nodes","scope":"current_plan_requirements","bindings":[{"requirement_template_version_id":22,"condition":{"all_of":[{"node_id":"dev","success_action":"complete"}]}}],"aggregate":{"mode":"all"},"on_satisfied":"auto_complete","empty_scope":"block","missing_binding":"block"}}},{"id":"end","type":"end","name":"结束"}],"edges":[{"id":"e1","source":"start","target":"dev-gate"},{"id":"e2","source":"dev-gate","target":"end"}]}`

func newConsumerFixture(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared&_pragma=foreign_keys(1)"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&workflowgorm.WorkflowTemplate{}, &workflowgorm.WorkflowTemplateVersion{}, &workflowgorm.WorkflowInstance{},
		&workflowgorm.WorkflowToken{}, &workflowgorm.WorkflowNodeRun{}, &workflowgorm.WorkflowApproval{},
		&workflowgorm.WorkflowNodeOpinion{}, &workflowgorm.ToolMutationApply{}, &workflowgorm.WorkflowOpinionInvitation{},
		&workflowgorm.WorkflowJob{},
		&projectgorm.ProjectSet{}, &projectgorm.PlanUnit{}, &requirementgorm.Requirement{},
		&auditgorm.AuditLog{}, &auditgorm.OutboxEvent{},
	); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	template := workflowgorm.WorkflowTemplate{ID: 11, WorkspaceID: 1, Name: "gate-flow", SubjectType: "plan_unit", PublishedVersion: 1}
	version := workflowgorm.WorkflowTemplateVersion{ID: 21, WorkspaceID: 1, TemplateID: 11, Version: 1, Definition: consumerGateDefinition, DefinitionHash: "hash", PublishedByUser: 7, PublishedAt: now}
	instance := workflowgorm.WorkflowInstance{ID: 31, WorkspaceID: 1, TemplateID: 11, TemplateVersionID: 21, SubjectType: "plan_unit", SubjectID: 41, Status: "running", CurrentNodeIDs: `["dev-gate"]`, NodePlansJSON: `{}`, StartedByUserID: 7, StartedAt: now}
	token := workflowgorm.WorkflowToken{WorkspaceID: 1, InstanceID: 31, NodeID: "dev-gate", Status: "active"}
	projectSet := projectgorm.ProjectSet{ID: 51, WorkspaceID: 1, TeamID: 61, Name: "AI PM"}
	planUnit := projectgorm.PlanUnit{ID: 41, WorkspaceID: 1, TeamID: 61, ProjectSetID: 51, Title: "V1", Type: "version"}
	requirement := requirementgorm.Requirement{ID: 81, WorkspaceID: 1, TeamID: 61, ProjectSetID: 51, Title: "联动需求", Status: "development", Version: 1}
	requirement.ScheduledPlanUnitID = new(uint)
	*requirement.ScheduledPlanUnitID = 41
	for _, row := range []any{&template, &version, &instance, &token, &projectSet, &planUnit, &requirement} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestConsumerAdvancesGateFromOutboxEvent(t *testing.T) {
	db := newConsumerFixture(t)
	// 需求开发证据已满足 + 一条重算事件待消费。
	now := time.Now().UTC()
	if err := db.Create(&workflowgorm.WorkflowNodeRun{
		WorkspaceID: 1, InstanceID: 32, NodeID: "dev", NodeType: "human_task",
		Status: "completed", StartedAt: now, CompletedAt: &now,
	}).Error; err != nil {
		t.Fatal(err)
	}
	// 需求实例绑定让证据读取生效。
	requirementInstance := workflowgorm.WorkflowInstance{
		ID: 32, WorkspaceID: 1, TemplateID: 12, TemplateVersionID: 22,
		SubjectType: "requirement", SubjectID: 81, Status: "running",
		CurrentNodeIDs: `["dev"]`, NodePlansJSON: "{}", StartedByUserID: 7, StartedAt: now,
	}
	requirementVersion := workflowgorm.WorkflowTemplateVersion{
		ID: 22, WorkspaceID: 1, TemplateID: 12, Version: 1,
		Definition:     `{"nodes":[{"id":"start","type":"start","name":"开始"},{"id":"dev","type":"human_task","name":"开发"},{"id":"end","type":"end","name":"结束"}],"edges":[{"id":"e1","source":"start","target":"dev"},{"id":"e2","source":"dev","target":"end"}]}`,
		DefinitionHash: "req-hash", PublishedByUser: 7, PublishedAt: now,
	}
	requirementTemplate := workflowgorm.WorkflowTemplate{ID: 12, WorkspaceID: 1, Name: "req", SubjectType: "requirement", PublishedVersion: 1}
	if err := db.Create(&requirementVersion).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&requirementTemplate).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&requirementInstance).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&requirementgorm.Requirement{}).Where("id = ?", 81).
		Update("workflow_instance_id", 32).Error; err != nil {
		t.Fatal(err)
	}
	event := auditgorm.OutboxEvent{
		WorkspaceID: 1, AggregateType: "plan_unit", AggregateID: 41,
		EventType:  "pm.delivery.reconcile_requested",
		Payload:    `{"plan_unit_id":41,"reason":"requirement_node_action:completed"}`,
		OccurredAt: now,
	}
	if err := db.Create(&event).Error; err != nil {
		t.Fatal(err)
	}
	consumer, err := NewConsumer(db)
	if err != nil {
		t.Fatal(err)
	}
	processed, err := consumer.ConsumeOnce(context.Background())
	if err != nil || processed != 1 {
		t.Fatalf("processed=%d err=%v", processed, err)
	}
	var instance workflowgorm.WorkflowInstance
	if err := db.First(&instance, 31).Error; err != nil {
		t.Fatal(err)
	}
	if instance.Status != "completed" {
		t.Fatalf("gate must auto-advance via the consumer, got %s %s", instance.Status, instance.CurrentNodeIDs)
	}
	var published auditgorm.OutboxEvent
	if err := db.First(&published, event.ID).Error; err != nil {
		t.Fatal(err)
	}
	if published.PublishedAt == nil {
		t.Fatal("event must be marked published after a successful reconcile")
	}
}
