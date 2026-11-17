package pmtools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
	"gorm.io/gorm"
)

type projectMutationCapture struct {
	command     project.DeliveryLabelMutation
	result      project.DeliveryLabelMutationResult
	setCommand  project.SetMutation
	setResult   project.SetMutationResult
	planCommand project.PlanMutation
	planResult  project.PlanMutationResult
}

func (capture *projectMutationCapture) ApplyPlanUnit(_ context.Context, command project.PlanMutation) (project.PlanMutationResult, error) {
	capture.planCommand = command
	return capture.planResult, nil
}
func (capture *projectMutationCapture) ApplyPlanUnitDeletion(_ context.Context, command project.PlanMutation) (project.PlanMutationResult, error) {
	capture.planCommand = command
	return capture.planResult, nil
}
func (capture *projectMutationCapture) ApplyPlanUnitCreation(_ context.Context, command project.PlanMutation) (project.PlanMutationResult, error) {
	capture.planCommand = command
	return capture.planResult, nil
}

type retryDeletionCapture struct{ calls int }

func (capture *retryDeletionCapture) ApplyProjectSetDeletion(_ context.Context, command project.SetMutation) (project.SetMutationResult, error) {
	capture.calls++
	if capture.calls == 1 {
		return project.SetMutationResult{}, project.ErrMutationRetryable
	}
	return project.SetMutationResult{ID: command.ID, Deleted: true}, nil
}

func (capture *projectMutationCapture) ApplyDeliveryLabel(_ context.Context, command project.DeliveryLabelMutation) (project.DeliveryLabelMutationResult, error) {
	capture.command = command
	return capture.result, nil
}

func (capture *projectMutationCapture) ApplyProjectSet(_ context.Context, command project.SetMutation) (project.SetMutationResult, error) {
	capture.setCommand = command
	return capture.setResult, nil
}

func (capture *projectMutationCapture) ApplyProjectSetDeletion(_ context.Context, command project.SetMutation) (project.SetMutationResult, error) {
	capture.setCommand = command
	return capture.setResult, nil
}

func TestProjectWriteHandlersUseServerAuthorizationAndChangeSetApplyKey(t *testing.T) {
	capture := &projectMutationCapture{result: project.DeliveryLabelMutationResult{ID: 9, WorkspaceID: 7, Name: "Release"}}
	handlers, err := NewProjectWriteHandlers(capture, capture, capture, capture, capture, capture)
	if err != nil {
		t.Fatal(err)
	}
	handler := handlers[ProjectDeliveryLabelCreate].(toolruntime.IdempotentMutationHandler)
	output, err := handler.Apply(context.Background(), toolruntime.MutationCall{
		Definition:    toolruntime.Definition{Name: ProjectDeliveryLabelCreate},
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input:         json.RawMessage(`{"id":8,"name":"Release","sort_order":2}`),
		ChangeSetID:   "changeset_12345678", ApplyKey: "changeset_12345678",
	})
	if err != nil || capture.command.WorkspaceID != 7 || capture.command.ID != 8 ||
		capture.command.ApplyKey != "changeset_12345678" || len(capture.command.InputDigest) != 64 {
		t.Fatalf("command=%+v output=%s err=%v", capture.command, output, err)
	}
	var result project.DeliveryLabelMutationResult
	if json.Unmarshal(output, &result) != nil || result.ID != 9 {
		t.Fatalf("output=%s", output)
	}
}

func TestProjectDeliveryLabelDeleteUsesStrictPublicOutput(t *testing.T) {
	capture := &projectMutationCapture{result: project.DeliveryLabelMutationResult{ID: 9, Deleted: true}}
	handlers, err := NewProjectWriteHandlers(capture, capture, capture, capture, capture, capture)
	if err != nil {
		t.Fatal(err)
	}
	handler := handlers[ProjectDeliveryLabelDelete].(toolruntime.IdempotentMutationHandler)
	output, err := handler.Apply(context.Background(), toolruntime.MutationCall{
		Definition:    toolruntime.Definition{Name: ProjectDeliveryLabelDelete},
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input:         json.RawMessage(`{"id":9}`),
		ChangeSetID:   "changeset_12345678", ApplyKey: "changeset_12345678",
	})
	if err != nil || string(output) != `{"id":9,"deleted":true}` {
		t.Fatalf("output=%s err=%v", output, err)
	}
}

func TestProjectSetWriteHandlerUsesServerPrincipalAndPreservesNullablePatch(t *testing.T) {
	capture := &projectMutationCapture{setResult: project.SetMutationResult{
		ID: 9, WorkspaceID: 7, TeamID: 8, Name: "Project", Code: "PM-9", Status: "active",
	}}
	handlers, err := NewProjectWriteHandlers(capture, capture, capture, capture, capture, capture)
	if err != nil {
		t.Fatal(err)
	}
	handler := handlers[ProjectSetUpdate].(toolruntime.IdempotentMutationHandler)
	output, err := handler.Apply(context.Background(), toolruntime.MutationCall{
		Definition:    toolruntime.Definition{Name: ProjectSetUpdate},
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input:         json.RawMessage(`{"id":9,"owner_user_id":null,"planned_end_at":"2026-09-01T08:00:00+08:00"}`),
		ChangeSetID:   "changeset_12345678", ApplyKey: "changeset_12345678",
	})
	if err != nil || capture.setCommand.ActorUserID != 3 || capture.setCommand.WorkspaceID != 7 ||
		!capture.setCommand.OwnerSet || capture.setCommand.OwnerUserID != nil || !capture.setCommand.PlannedEndSet ||
		capture.setCommand.PlannedEndAt == nil || len(capture.setCommand.InputDigest) != 64 {
		t.Fatalf("command=%+v output=%s err=%v", capture.setCommand, output, err)
	}
}

func TestProjectPlanUpdateUsesServerPrincipalAndStrictResult(t *testing.T) {
	capture := &projectMutationCapture{planResult: project.PlanMutationResult{ID: 12, WorkspaceID: 7, TeamID: 8, ProjectSetID: 11, Title: "Renamed", Type: "version", Status: "planning"}}
	handlers, err := NewProjectWriteHandlers(capture, capture, capture, capture, capture, capture)
	if err != nil {
		t.Fatal(err)
	}
	handler := handlers[ProjectPlanUnitUpdate].(toolruntime.IdempotentMutationHandler)
	output, err := handler.Apply(context.Background(), toolruntime.MutationCall{Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input: json.RawMessage(`{"id":12,"title":"Renamed","owner_user_id":null}`), ChangeSetID: "changeset_12345678", ApplyKey: "changeset_12345678"})
	if err != nil || capture.planCommand.ActorUserID != 3 || capture.planCommand.WorkspaceID != 7 || !capture.planCommand.OwnerSet || capture.planCommand.OwnerUserID != nil || len(capture.planCommand.InputDigest) != 64 {
		t.Fatalf("command=%+v output=%s err=%v", capture.planCommand, output, err)
	}
}

func TestProjectSetDeleteReleasesRetryableClaimAndConfirmsAgain(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(toolruntimegorm.ChangeSetMigrationRecords()...); err != nil {
		t.Fatal(err)
	}
	capture := &projectMutationCapture{}
	retry := &retryDeletionCapture{}
	handlers, err := NewProjectWriteHandlers(capture, capture, retry, capture, capture, capture)
	if err != nil {
		t.Fatal(err)
	}
	source, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: []string{ProjectSetDelete}})
	if err != nil {
		t.Fatal(err)
	}
	definition, found := source.Find(ProjectSetDelete)
	if !found {
		t.Fatal("project set delete definition is missing")
	}
	catalog, _ := toolruntime.NewCatalog([]toolruntime.Definition{definition})
	changeSets, _ := toolruntime.NewChangeSetService(toolruntimegorm.NewChangeSetRepository(db), toolruntime.DefaultChangeSetPolicy())
	application, err := toolruntime.NewApplication(catalog, projectWriteAuthorizer{tool: ProjectSetDelete},
		map[string]toolruntime.ToolHandler{ProjectSetDelete: handlers[ProjectSetDelete]}, toolruntime.WithChangeSetService(changeSets))
	if err != nil {
		t.Fatal(err)
	}
	authorization := toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7, GrantID: 5,
		ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI}
	pending, err := application.Invoke(context.Background(), toolruntime.Invocation{Name: ProjectSetDelete,
		Input: json.RawMessage(`{"id":11}`), IdempotencyKey: "delete-project-tool-0001", Authorization: authorization})
	if err != nil || pending.ChangeSet == nil {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	confirmed, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if !errors.Is(err, toolruntime.ErrChangeSetInProgress) || confirmed.Status != toolruntime.ChangeSetConfirmed {
		t.Fatalf("confirmed=%+v err=%v", confirmed, err)
	}
	succeeded, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if err != nil || succeeded.Status != toolruntime.ChangeSetSucceeded || string(succeeded.Result) != `{"id":11,"deleted":true}` {
		t.Fatalf("succeeded=%+v err=%v", succeeded, err)
	}
	if retry.calls != 2 {
		t.Fatalf("apply calls=%d", retry.calls)
	}
}

type projectWriteAuthorizer struct{ tool string }

func (authorizer projectWriteAuthorizer) AuthorizeGrant(context.Context, toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant([]string{authorizer.tool}, []string{"project:write"})
}

type validProjectReferences struct{}

func (validProjectReferences) ProjectReferencesValid(context.Context, uint, uint, []uint) (bool, error) {
	return true, nil
}

type stubDeletionImpact struct{}

func (stubDeletionImpact) InspectProjectDeletionImpact(context.Context, uint, uint) (project.DeletionImpact, error) {
	return project.DeletionImpact{}, nil
}

type stubDeletionSteps struct{}

func (stubDeletionSteps) RunProjectDeletionStep(context.Context, project.DeletionStep, project.DeletionSaga) error {
	return nil
}

type stubPlanDeletionStore struct{}

func (*stubPlanDeletionStore) FindPlanDeletion(context.Context, string) (project.PlanDeletionSaga, bool, error) {
	return project.PlanDeletionSaga{}, false, nil
}
func (*stubPlanDeletionStore) BeginPlanDeletion(_ context.Context, c project.PlanMutation) (project.PlanDeletionSaga, error) {
	return project.PlanDeletionSaga{WorkspaceID: c.WorkspaceID, PlanUnitID: c.ID, ApplyKey: c.ApplyKey, InputDigest: c.InputDigest}, nil
}
func (*stubPlanDeletionStore) RejectPlanDeletion(context.Context, string) error { return nil }
func (*stubPlanDeletionStore) AdvancePlanDeletion(_ context.Context, _ string, step int) (project.PlanDeletionSaga, error) {
	return project.PlanDeletionSaga{NextStep: step + 1}, nil
}
func (*stubPlanDeletionStore) CompletePlanDeletion(context.Context, string) (project.PlanMutationResult, error) {
	return project.PlanMutationResult{ID: 1, Deleted: true}, nil
}

type stubPlanDeletionImpact struct{}

func (stubPlanDeletionImpact) InspectPlanUnitDeletion(context.Context, uint, uint) (project.PlanDeletionImpact, error) {
	return project.PlanDeletionImpact{}, nil
}

type stubPlanWorkflow struct{}

func (stubPlanWorkflow) DeletePlanUnitScaffolding(context.Context, uint, uint) error { return nil }

type stubPlanWorkflowStart struct{ calls int }

func (starter *stubPlanWorkflowStart) StartPlanUnitWorkflow(_ context.Context, command project.PlanWorkflowStartCommand) (project.PlanWorkflowStartResult, error) {
	starter.calls++
	return project.PlanWorkflowStartResult{InstanceID: command.PlanUnitID + 100, SubjectStatus: "grooming"}, nil
}

func TestProjectPlanCreateRunsPendingConfirmAndDurableReplay(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	records := []any{&projectgorm.ProjectSet{}, &projectgorm.PlanUnit{}, &projectgorm.ToolMutationApply{}, &projectgorm.PlanCreationSaga{}}
	records = append(records, toolruntimegorm.ChangeSetMigrationRecords()...)
	if err := db.AutoMigrate(records...); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&projectgorm.ProjectSet{ID: 11, WorkspaceID: 7, TeamID: 8, Name: "Project", Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	starter := &stubPlanWorkflowStart{}
	creation, err := project.NewPlanCreationService(projectgorm.NewMutationRepository(db), validProjectReferences{}, starter)
	if err != nil {
		t.Fatal(err)
	}
	capture := &projectMutationCapture{}
	handlers, err := NewProjectWriteHandlers(capture, capture, capture, capture, capture, creation)
	if err != nil {
		t.Fatal(err)
	}
	source, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: []string{ProjectPlanUnitCreate}})
	if err != nil {
		t.Fatal(err)
	}
	definition, found := source.Find(ProjectPlanUnitCreate)
	if !found {
		t.Fatal("project plan create definition is missing")
	}
	catalog, err := toolruntime.NewCatalog([]toolruntime.Definition{definition})
	if err != nil {
		t.Fatal(err)
	}
	changeSets, err := toolruntime.NewChangeSetService(toolruntimegorm.NewChangeSetRepository(db), toolruntime.DefaultChangeSetPolicy())
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(catalog, projectWriteAuthorizer{tool: ProjectPlanUnitCreate},
		map[string]toolruntime.ToolHandler{ProjectPlanUnitCreate: handlers[ProjectPlanUnitCreate]}, toolruntime.WithChangeSetService(changeSets))
	if err != nil {
		t.Fatal(err)
	}
	authorization := toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7, GrantID: 5,
		ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI}
	pending, err := application.Invoke(context.Background(), toolruntime.Invocation{Name: ProjectPlanUnitCreate,
		Input:          json.RawMessage(`{"project_new_id":11,"title":"V1","type":"version","workflow_template_id":20}`),
		IdempotencyKey: "project-plan-create-0001", Authorization: authorization})
	if err != nil || pending.ChangeSet == nil || pending.ChangeSet.Status != toolruntime.ChangeSetPending {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	succeeded, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if err != nil || succeeded.Status != toolruntime.ChangeSetSucceeded {
		t.Fatalf("succeeded=%+v err=%v", succeeded, err)
	}
	replayed, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if err != nil || replayed.Status != toolruntime.ChangeSetSucceeded || string(replayed.Result) != string(succeeded.Result) {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	var plans, projectApplies, sagas int64
	db.Model(&projectgorm.PlanUnit{}).Where("workspace_id = ? AND mutation_state = ''", 7).Count(&plans)
	db.Model(&projectgorm.ToolMutationApply{}).Where("workspace_id = ? AND result_json <> ''", 7).Count(&projectApplies)
	db.Model(&projectgorm.PlanCreationSaga{}).Where("workspace_id = ? AND status = ?", 7, "succeeded").Count(&sagas)
	if plans != 1 || projectApplies != 1 || sagas != 1 || starter.calls != 1 {
		t.Fatalf("plans=%d applies=%d sagas=%d starts=%d", plans, projectApplies, sagas, starter.calls)
	}
}

func TestProjectDeliveryLabelCreateRunsPendingConfirmAndIdempotentReplay(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	records := []any{
		&projectgorm.ProjectSet{}, &projectgorm.PlanUnit{}, &projectgorm.DeliveryNodeLabel{},
		&projectgorm.DeliveryNode{}, &projectgorm.ToolMutationApply{},
	}
	records = append(records, toolruntimegorm.ChangeSetMigrationRecords()...)
	if err := db.AutoMigrate(records...); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&projectgorm.ProjectSet{ID: 11, WorkspaceID: 7, TeamID: 101, Name: "Project", Status: "active"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&projectgorm.PlanUnit{ID: 12, WorkspaceID: 7, TeamID: 101, ProjectSetID: 11, Title: "V1", Type: "version", Status: "planning"}).Error; err != nil {
		t.Fatal(err)
	}
	repository := projectgorm.NewMutationRepository(db)
	service, err := project.NewMutationService(repository)
	if err != nil {
		t.Fatal(err)
	}
	setService, err := project.NewSetMutationService(repository, validProjectReferences{})
	if err != nil {
		t.Fatal(err)
	}
	deletionService, err := project.NewDeletionService(repository, stubDeletionImpact{}, stubDeletionSteps{})
	if err != nil {
		t.Fatal(err)
	}
	planService, err := project.NewPlanMutationService(repository, validProjectReferences{})
	if err != nil {
		t.Fatal(err)
	}
	planDeletion, _ := project.NewPlanDeletionService(&stubPlanDeletionStore{}, stubPlanDeletionImpact{}, stubPlanWorkflow{})
	handlers, err := NewProjectWriteHandlers(service, setService, deletionService, planService, planDeletion, &projectMutationCapture{})
	if err != nil {
		t.Fatal(err)
	}
	source, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: []string{ProjectDeliveryLabelCreate}})
	if err != nil {
		t.Fatal(err)
	}
	definition, found := source.Find(ProjectDeliveryLabelCreate)
	if !found {
		t.Fatal("delivery label create definition is missing")
	}
	catalog, err := toolruntime.NewCatalog([]toolruntime.Definition{definition})
	if err != nil {
		t.Fatal(err)
	}
	changeSets, err := toolruntime.NewChangeSetService(
		toolruntimegorm.NewChangeSetRepository(db), toolruntime.DefaultChangeSetPolicy(),
	)
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(
		catalog, projectWriteAuthorizer{tool: ProjectDeliveryLabelCreate},
		map[string]toolruntime.ToolHandler{ProjectDeliveryLabelCreate: handlers[ProjectDeliveryLabelCreate]},
		toolruntime.WithChangeSetService(changeSets),
	)
	if err != nil {
		t.Fatal(err)
	}
	authorization := toolruntime.ExecutionAuthorization{
		PrincipalID: 3, WorkspaceID: 7, GrantID: 5, ClientID: "pm-cli",
		Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
	}
	pending, err := application.Invoke(context.Background(), toolruntime.Invocation{
		Name: ProjectDeliveryLabelCreate, Input: json.RawMessage(`{"id":12,"name":"Release","sort_order":2}`),
		IdempotencyKey: "delivery-label-create-0001", Authorization: authorization,
	})
	if err != nil || pending.ChangeSet == nil || pending.ChangeSet.Status != toolruntime.ChangeSetPending {
		t.Fatalf("pending=%+v err=%v", pending, err)
	}
	succeeded, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if err != nil || succeeded.Status != toolruntime.ChangeSetSucceeded {
		t.Fatalf("succeeded=%+v err=%v", succeeded, err)
	}
	replayed, err := application.ConfirmChangeSet(context.Background(), pending.ChangeSet.ID, authorization)
	if err != nil || replayed.Status != toolruntime.ChangeSetSucceeded || string(replayed.Result) != string(succeeded.Result) {
		t.Fatalf("replayed=%+v err=%v", replayed, err)
	}
	var labels, applies int64
	if err := db.Model(&projectgorm.DeliveryNodeLabel{}).Where("workspace_id = ?", 7).Count(&labels).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Model(&projectgorm.ToolMutationApply{}).Where("workspace_id = ?", 7).Count(&applies).Error; err != nil {
		t.Fatal(err)
	}
	if labels != 1 || applies != 1 {
		t.Fatalf("labels=%d applies=%d", labels, applies)
	}

	setSource, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: []string{ProjectSetCreate}})
	if err != nil {
		t.Fatal(err)
	}
	setDefinition, found := setSource.Find(ProjectSetCreate)
	if !found {
		t.Fatal("project set create definition is missing")
	}
	setCatalog, err := toolruntime.NewCatalog([]toolruntime.Definition{setDefinition})
	if err != nil {
		t.Fatal(err)
	}
	setApplication, err := toolruntime.NewApplication(
		setCatalog, projectWriteAuthorizer{tool: ProjectSetCreate},
		map[string]toolruntime.ToolHandler{ProjectSetCreate: handlers[ProjectSetCreate]},
		toolruntime.WithChangeSetService(changeSets),
	)
	if err != nil {
		t.Fatal(err)
	}
	setPending, err := setApplication.Invoke(context.Background(), toolruntime.Invocation{
		Name: ProjectSetCreate, Input: json.RawMessage(`{"team_id":101,"name":"New Project"}`),
		IdempotencyKey: "project-set-create-0001", Authorization: authorization,
	})
	if err != nil || setPending.ChangeSet == nil {
		t.Fatalf("set pending=%+v err=%v", setPending, err)
	}
	setSucceeded, err := setApplication.ConfirmChangeSet(context.Background(), setPending.ChangeSet.ID, authorization)
	if err != nil || setSucceeded.Status != toolruntime.ChangeSetSucceeded {
		t.Fatalf("set succeeded=%+v err=%v", setSucceeded, err)
	}
	var sets int64
	if err := db.Model(&projectgorm.ProjectSet{}).Where("workspace_id = ? AND name = ?", 7, "New Project").Count(&sets).Error; err != nil || sets != 1 {
		t.Fatalf("sets=%d err=%v", sets, err)
	}
}
