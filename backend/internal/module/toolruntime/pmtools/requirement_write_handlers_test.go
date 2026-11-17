package pmtools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
	"gorm.io/gorm"
)

type requirementMutationCapture struct {
	command requirement.Mutation
	result  requirement.ApplyResult
	calls   int
}

func (capture *requirementMutationCapture) ApplyRequirementMutation(_ context.Context, command requirement.Mutation) (requirement.ApplyResult, error) {
	capture.command = command
	capture.calls++
	return capture.result, nil
}

func TestRequirementWriteHandlerUsesServerFactsAndDecodesBatchCASItems(t *testing.T) {
	capture := &requirementMutationCapture{result: requirement.ApplyResult{Batch: &requirement.BatchMoveResult{
		Items: []requirement.MutationResult{{ID: 5}}, MovedCount: 1,
	}}}
	handlers, err := NewRequirementWriteHandlers(capture)
	if err != nil {
		t.Fatal(err)
	}
	handler := handlers[ProjectRequirementBatchMove].(toolruntime.IdempotentMutationHandler)
	_, err = handler.Apply(context.Background(), toolruntime.MutationCall{
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input:         json.RawMessage(`{"items":[{"requirement_id":5,"expected_version":2}],"version_task_new_id":31,"confirm_subject_schedule":true}`),
		ChangeSetID:   "changeset_requirement_batch", ApplyKey: "apply_requirement_batch",
	})
	if err != nil || capture.command.WorkspaceID != 7 || capture.command.PrincipalID != 3 ||
		capture.command.ChangeSetID != "changeset_requirement_batch" || capture.command.ApplyKey != "apply_requirement_batch" ||
		len(capture.command.InputDigest) != 64 || len(capture.command.Items) != 1 || capture.command.Items[0].RequirementID != 5 ||
		capture.command.Items[0].ExpectedVersion != 2 || !capture.command.ConfirmSubjectSchedule {
		t.Fatalf("command=%+v err=%v", capture.command, err)
	}
}

func TestRequirementRoleOwnersHandlerIsActivatedAndDecodesAllOwnerGroups(t *testing.T) {
	capture := &requirementMutationCapture{result: requirement.ApplyResult{Requirement: &requirement.MutationResult{ID: 5}}}
	handlers, err := NewRequirementWriteHandlers(capture)
	if err != nil {
		t.Fatal(err)
	}
	handler, ok := handlers[ProjectRequirementRoleOwnersAssign].(toolruntime.IdempotentMutationHandler)
	if !ok {
		t.Fatalf("role owner handler missing: %v", RequirementWriteHandlerNames())
	}
	_, err = handler.Apply(context.Background(), toolruntime.MutationCall{
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input:         json.RawMessage(`{"id":5,"expected_version":2,"product_owner_user_ids":[11],"design_owner_user_ids":[12],"frontend_owner_user_ids":[13],"backend_owner_user_ids":[14],"tester_user_ids":[15],"developer_user_ids":[16]}`),
		ChangeSetID:   "changeset_requirement_owners", ApplyKey: "apply_requirement_owners",
	})
	if err != nil || capture.command.Operation != requirement.MutationUpdate || !capture.command.ProductOwnerUserIDsSet ||
		!capture.command.DesignOwnerUserIDsSet || !capture.command.FrontendOwnerUserIDsSet ||
		!capture.command.BackendOwnerUserIDsSet || !capture.command.TesterUserIDsSet || !capture.command.DeveloperUserIDsSet ||
		capture.command.ProductOwnerUserIDs[0] != 11 || capture.command.DesignOwnerUserIDs[0] != 12 ||
		capture.command.FrontendOwnerUserIDs[0] != 13 || capture.command.BackendOwnerUserIDs[0] != 14 ||
		capture.command.TesterUserIDs[0] != 15 || capture.command.DeveloperUserIDs[0] != 16 {
		t.Fatalf("command=%+v err=%v", capture.command, err)
	}
}

func TestRequirementDeleteUsesStrictPublicOutput(t *testing.T) {
	capture := &requirementMutationCapture{result: requirement.ApplyResult{Requirement: &requirement.MutationResult{ID: 5, Deleted: true}}}
	handlers, _ := NewRequirementWriteHandlers(capture)
	output, err := handlers[ProjectRequirementDelete].(toolruntime.IdempotentMutationHandler).Apply(context.Background(), toolruntime.MutationCall{
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7}, Input: json.RawMessage(`{"id":5}`),
		ChangeSetID: "changeset_requirement_delete", ApplyKey: "apply_requirement_delete",
	})
	if err != nil || string(output) != `{"id":5,"deleted":true}` {
		t.Fatalf("output=%s err=%v", output, err)
	}
}

func TestRequirementCreateRunsPendingConfirmSucceededThroughApplication(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(toolruntimegorm.ChangeSetMigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	instanceID := uint(41)
	ownerID := uint(3)
	capture := &requirementMutationCapture{result: requirement.ApplyResult{Requirement: &requirement.MutationResult{
		ID: 13, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, PlanUnitID: 12, Version: 1,
		WorkflowInstanceID: &instanceID, Title: "Checkout", Priority: "p1", Status: "grooming",
		ProductOwnerUserID: &ownerID, DeveloperUserIDs: []uint{},
	}}}
	handlers, _ := NewRequirementWriteHandlers(capture)
	source, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: []string{ProjectRequirementCreate}})
	if err != nil {
		t.Fatal(err)
	}
	definition, found := source.Find(ProjectRequirementCreate)
	if !found {
		t.Fatal("requirement create definition missing")
	}
	catalog, err := toolruntime.NewCatalog([]toolruntime.Definition{definition})
	if err != nil {
		t.Fatal(err)
	}
	changeSets, _ := toolruntime.NewChangeSetService(toolruntimegorm.NewChangeSetRepository(db), toolruntime.DefaultChangeSetPolicy())
	application, err := toolruntime.NewApplication(catalog, projectWriteAuthorizer{tool: ProjectRequirementCreate},
		map[string]toolruntime.ToolHandler{ProjectRequirementCreate: handlers[ProjectRequirementCreate]}, toolruntime.WithChangeSetService(changeSets))
	if err != nil {
		t.Fatal(err)
	}
	authorization := toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7, GrantID: 5,
		ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI}
	pending, err := application.Invoke(context.Background(), toolruntime.Invocation{Name: ProjectRequirementCreate,
		Input:          json.RawMessage(`{"version_task_new_id":12,"title":"Checkout","workflow_template_id":9,"priority":"p1"}`),
		IdempotencyKey: "requirement-create-tool-0001", Authorization: authorization})
	if err != nil || pending.ChangeSet == nil || pending.ChangeSet.Status != toolruntime.ChangeSetPending {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	succeeded, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if err != nil || succeeded.Status != toolruntime.ChangeSetSucceeded || capture.calls != 1 || capture.command.ChangeSetID != pending.ChangeSet.ID {
		t.Fatalf("succeeded=%+v calls=%d command=%+v err=%v", succeeded, capture.calls, capture.command, err)
	}
	if succeeded.Result == nil || !json.Valid(succeeded.Result) {
		t.Fatalf("result=%s", succeeded.Result)
	}
}
