package pmtools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	qualitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/quality/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	toolruntimegorm "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime/gorm"
	"gorm.io/gorm"
)

type qualityMutationCapture struct {
	command quality.BugMutation
	result  quality.BugMutationResult
}

func (capture *qualityMutationCapture) ApplyBugMutation(_ context.Context, command quality.BugMutation) (quality.BugMutationResult, error) {
	capture.command = command
	return capture.result, nil
}

func TestQualityWriteHandlerUsesServerScopeAndStrictInput(t *testing.T) {
	bug := quality.BugSnapshot{ID: 9, WorkspaceID: 7, TeamID: 2, ProjectSetID: 3, PlanUnitID: 4, RequirementID: 5, BugKey: "BUG-9", Title: "Bug", Status: "triage", Priority: "medium"}
	capture := &qualityMutationCapture{result: quality.BugMutationResult{Bug: &bug}}
	handlers, err := NewQualityWriteHandlers(capture)
	if err != nil {
		t.Fatal(err)
	}
	handler := handlers[QualityBugUpdate].(toolruntime.IdempotentMutationHandler)
	_, err = handler.Apply(context.Background(), toolruntime.MutationCall{
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input:         json.RawMessage(`{"id":9,"title":"Bug","defect_domain":"ui","assignee_user_id":null}`),
		ChangeSetID:   "changeset_12345678", ApplyKey: "apply_12345678",
	})
	if err != nil || capture.command.WorkspaceID != 7 || capture.command.PrincipalID != 3 ||
		capture.command.ChangeSetID != "changeset_12345678" || capture.command.ApplyKey != "apply_12345678" ||
		!capture.command.Values.Fields["assignee_user_id"] || capture.command.Values.AssigneeUserID != nil ||
		capture.command.Values.DefectDomain != quality.BugDefectDomainUI {
		t.Fatalf("command=%+v err=%v", capture.command, err)
	}
	_, err = handler.Apply(context.Background(), toolruntime.MutationCall{
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input:         json.RawMessage(`{"id":9,"unknown":true}`), ChangeSetID: "changeset_12345678", ApplyKey: "apply_12345678",
	})
	if !errors.Is(err, quality.ErrBugMutationInvalid) {
		t.Fatalf("unknown field error=%v", err)
	}
}

func TestQualityWriteHandlersAndCatalogActivateExactlyEightStrictMutations(t *testing.T) {
	capture := &qualityMutationCapture{}
	handlers, err := NewQualityWriteHandlers(capture)
	if err != nil {
		t.Fatal(err)
	}
	names := QualityWriteHandlerNames()
	if len(names) != 8 || len(handlers) != 8 {
		t.Fatalf("names=%v handlers=%d", names, len(handlers))
	}
	catalog, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: names})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		handler, exists := handlers[name]
		if !exists {
			t.Fatalf("handler %s missing", name)
		}
		if _, ok := handler.(toolruntime.IdempotentMutationHandler); !ok {
			t.Fatalf("handler %s is not idempotent mutation handler", name)
		}
		if _, invokeErr := handler.Invoke(context.Background(), toolruntime.ToolCall{}); !errors.Is(invokeErr, toolruntime.ErrChangeSetRequired) {
			t.Fatalf("handler %s Invoke error=%v", name, invokeErr)
		}
		definition, found := catalog.Find(name)
		if !found || definition.Lifecycle != toolruntime.LifecycleActive || definition.ChangeSet != toolruntime.ChangeSetRequired || definition.Risk == toolruntime.RiskRead {
			t.Fatalf("definition %s=%+v found=%v", name, definition, found)
		}
		for label, raw := range map[string]json.RawMessage{"input": definition.InputSchema, "output": definition.OutputSchema} {
			var schema map[string]any
			if json.Unmarshal(raw, &schema) != nil || schema["additionalProperties"] != false {
				t.Fatalf("%s %s schema is not strict: %s", name, label, raw)
			}
		}
	}
}

func TestQualityWriteHandlersApplyAllEightOperations(t *testing.T) {
	bug := quality.BugSnapshot{ID: 9, WorkspaceID: 7, TeamID: 2, ProjectSetID: 3, PlanUnitID: 4, RequirementID: 5, BugKey: "BUG-9", Title: "Bug", Status: "triage", Priority: "medium"}
	cases := []struct {
		name      string
		input     json.RawMessage
		operation quality.BugMutationOperation
		result    quality.BugMutationResult
	}{
		{QualityBugCreate, json.RawMessage(`{"requirement_id":5,"title":"Bug"}`), quality.BugCreate, quality.BugMutationResult{Bug: &bug}},
		{QualityBugUpdate, json.RawMessage(`{"id":9,"title":"Updated"}`), quality.BugUpdate, quality.BugMutationResult{Bug: &bug}},
		{QualityBugAssign, json.RawMessage(`{"id":9,"assignee_user_id":3}`), quality.BugUpdate, quality.BugMutationResult{Bug: &bug}},
		{QualityBugStatusUpdate, json.RawMessage(`{"id":9,"status":"in_progress"}`), quality.BugUpdate, quality.BugMutationResult{Bug: &bug}},
		{QualityBugDelete, json.RawMessage(`{"id":9}`), quality.BugDelete, quality.BugMutationResult{DeletedID: 9, Deleted: true}},
		{QualityBugBatchCreate, json.RawMessage(`{"bugs":[{"client_id":"one","requirement_id":5,"title":"Bug"}]}`), quality.BugBatchCreate, quality.BugMutationResult{Created: []quality.BatchBugMutationResult{{ClientID: "one", Bug: bug}}, CreatedCount: 1, Atomic: true}},
		{QualityBugBatchAssign, json.RawMessage(`{"ids":[9],"assignee_user_id":3}`), quality.BugBatchAssign, quality.BugMutationResult{Updated: []quality.BugSnapshot{bug}, UpdatedCount: 1, Atomic: true}},
		{QualityBugBatchDelete, json.RawMessage(`{"ids":[9]}`), quality.BugBatchDelete, quality.BugMutationResult{DeletedIDs: []uint{9}, DeletedCount: 1, Atomic: true}},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			capture := &qualityMutationCapture{result: item.result}
			handlers, err := NewQualityWriteHandlers(capture)
			if err != nil {
				t.Fatal(err)
			}
			handler := handlers[item.name].(toolruntime.IdempotentMutationHandler)
			if _, err := handler.Apply(context.Background(), toolruntime.MutationCall{
				Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
				Input:         item.input, ChangeSetID: "changeset_12345678", ApplyKey: "apply_12345678",
			}); err != nil {
				t.Fatal(err)
			}
			if capture.command.Operation != item.operation || capture.command.ChangeSetID == "" || capture.command.ApplyKey == "" {
				t.Fatalf("command=%+v", capture.command)
			}
		})
	}
}

func TestQualityStatusUpdateRejectsNonWorkflowFields(t *testing.T) {
	bug := quality.BugSnapshot{ID: 9, WorkspaceID: 7, BugKey: "BUG-9", Title: "Bug", Status: "triage"}
	capture := &qualityMutationCapture{result: quality.BugMutationResult{Bug: &bug}}
	handlers, err := NewQualityWriteHandlers(capture)
	if err != nil {
		t.Fatal(err)
	}
	handler := handlers[QualityBugStatusUpdate].(toolruntime.IdempotentMutationHandler)
	_, err = handler.Apply(context.Background(), toolruntime.MutationCall{
		Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
		Input:         json.RawMessage(`{"id":9,"status":"in_progress","severity_key":"p0"}`),
		ChangeSetID:   "changeset_12345678", ApplyKey: "apply_12345678",
	})
	if !errors.Is(err, quality.ErrBugMutationInvalid) {
		t.Fatalf("non-workflow field error=%v", err)
	}
}

func TestQualityAssignmentToolsUseStrictAssignmentOnlyCommands(t *testing.T) {
	capture := &qualityMutationCapture{result: quality.BugMutationResult{Bug: &quality.BugSnapshot{ID: 9}}}
	handlers, err := NewQualityWriteHandlers(capture)
	if err != nil {
		t.Fatal(err)
	}
	call := func(name string, input json.RawMessage) error {
		_, err := handlers[name].(toolruntime.IdempotentMutationHandler).Apply(
			context.Background(),
			toolruntime.MutationCall{
				Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7},
				Input:         input, ChangeSetID: "changeset_12345678", ApplyKey: "apply_12345678",
			},
		)
		return err
	}
	if err := call(QualityBugAssign, json.RawMessage(`{"id":9,"assignee_user_id":4,"severity_key":"p0"}`)); !errors.Is(err, quality.ErrBugMutationInvalid) {
		t.Fatalf("single assignment accepted unrelated fields: %v", err)
	}
	capture.result = quality.BugMutationResult{Updated: []quality.BugSnapshot{{ID: 9}}, UpdatedCount: 1, Atomic: true}
	if err := call(QualityBugBatchAssign, json.RawMessage(`{"ids":[9],"assignee_user_id":4}`)); err != nil {
		t.Fatal(err)
	}
	if capture.command.Operation != quality.BugBatchAssign || len(capture.command.IDs) != 1 ||
		capture.command.Values.AssigneeUserID == nil || *capture.command.Values.AssigneeUserID != 4 ||
		!capture.command.Values.Fields["assignee_user_id"] {
		t.Fatalf("batch assignment command=%+v", capture.command)
	}
}

type qualityWriteAuthorizer struct{ tool string }

type qualityWriteEffectNoop struct{}

func (qualityWriteEffectNoop) RecordBugMutation(context.Context, quality.BugMutationEffect) error {
	return nil
}

func (authorizer qualityWriteAuthorizer) AuthorizeGrant(context.Context, toolruntime.ExecutionAuthorization) (toolruntime.AuthorizedGrant, error) {
	return toolruntime.NewAuthorizedGrant([]string{authorizer.tool}, []string{"quality:write"})
}

func TestQualityBugCreateRunsPendingConfirmSucceededReplayAndMCPCLIApplicationParity(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	records := []any{
		&requirementgorm.Requirement{}, &qualitygorm.QualityBug{}, &qualitygorm.QualityBugActivity{},
		&qualitygorm.QualityBugComment{}, &qualitygorm.ToolMutationApply{},
	}
	records = append(records, toolruntimegorm.ChangeSetMigrationRecords()...)
	if err := db.AutoMigrate(records...); err != nil {
		t.Fatal(err)
	}
	planUnitID := uint(31)
	requirementRow := requirementgorm.Requirement{
		WorkspaceID: 7, TeamID: 11, ProjectSetID: 21, ScheduledPlanUnitID: &planUnitID,
		Version: 2, Title: "Requirement", Priority: "normal", Status: "backlog",
	}
	if err := db.Create(&requirementRow).Error; err != nil {
		t.Fatal(err)
	}
	repository, err := qualitygorm.NewMutationRepository(db, func(tx *gorm.DB) (requirement.MutationScopeResolver, error) {
		return requirement.NewMutationScopeService(requirementgorm.NewMutationScopeRepository(tx))
	}, func(*gorm.DB) (quality.BugMutationEffectSink, error) {
		return qualityWriteEffectNoop{}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	service, err := quality.NewBugMutationService(repository)
	if err != nil {
		t.Fatal(err)
	}
	handlers, err := NewQualityWriteHandlers(service)
	if err != nil {
		t.Fatal(err)
	}
	source, err := toolruntime.NewPMCatalog(toolruntime.PMActivation{ExecutableToolNames: []string{QualityBugCreate}})
	if err != nil {
		t.Fatal(err)
	}
	definition, found := source.Find(QualityBugCreate)
	if !found || definition.Lifecycle != toolruntime.LifecycleActive {
		t.Fatalf("definition=%+v found=%v", definition, found)
	}
	catalog, err := toolruntime.NewCatalog([]toolruntime.Definition{definition})
	if err != nil {
		t.Fatal(err)
	}
	changeSets, err := toolruntime.NewChangeSetService(toolruntimegorm.NewChangeSetRepository(db), toolruntime.DefaultChangeSetPolicy())
	if err != nil {
		t.Fatal(err)
	}
	application, err := toolruntime.NewApplication(
		catalog, qualityWriteAuthorizer{tool: QualityBugCreate},
		map[string]toolruntime.ToolHandler{QualityBugCreate: handlers[QualityBugCreate]},
		toolruntime.WithChangeSetService(changeSets),
	)
	if err != nil {
		t.Fatal(err)
	}
	authorization := toolruntime.ExecutionAuthorization{
		PrincipalID: 41, WorkspaceID: 7, GrantID: 5, ClientID: "pm-cli",
		Channel: toolruntime.DeliveryCLI, Audience: toolruntime.AudienceCLI,
	}
	pending, err := application.Invoke(context.Background(), toolruntime.Invocation{
		Name:           QualityBugCreate,
		Input:          json.RawMessage(`{"requirement_id":` + jsonNumber(requirementRow.ID) + `,"title":"Tool Bug","status":"review","resolution_reason":"已修复","reproduction_materials":[{"url":"https://cdn.example.test/proof.png"}],"reproduction_steps":"{\"type\":\"doc\",\"content\":[{\"type\":\"image\",\"attrs\":{\"src\":\"https://cdn.example.test/proof.png\"}}]}"}`),
		IdempotencyKey: "quality-create-0001", Authorization: authorization,
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
	mcpAuthorization := authorization
	mcpAuthorization.ClientID, mcpAuthorization.Channel, mcpAuthorization.Audience = "codex", toolruntime.DeliveryMCP, toolruntime.AudienceMCP
	mcpPending, err := application.Invoke(context.Background(), toolruntime.Invocation{
		Name:           QualityBugCreate,
		Input:          json.RawMessage(`{"requirement_id":` + jsonNumber(requirementRow.ID) + `,"title":"MCP Tool Bug","status":"review","resolution_reason":"已解决"}`),
		IdempotencyKey: "quality-create-mcp-0001", Authorization: mcpAuthorization,
	})
	if err != nil || mcpPending.ChangeSet == nil || mcpPending.ChangeSet.Status != toolruntime.ChangeSetPending {
		t.Fatalf("MCP pending=%+v err=%v", mcpPending, err)
	}
	mcpSucceeded, err := application.ConfirmChangeSet(context.Background(), mcpPending.ChangeSet.ID, mcpAuthorization)
	if err != nil || mcpSucceeded.Status != toolruntime.ChangeSetSucceeded {
		t.Fatalf("MCP succeeded=%+v err=%v", mcpSucceeded, err)
	}
	var cliBug, mcpBug quality.BugSnapshot
	if json.Unmarshal(succeeded.Result, &cliBug) != nil || json.Unmarshal(mcpSucceeded.Result, &mcpBug) != nil ||
		cliBug.WorkspaceID != mcpBug.WorkspaceID || cliBug.RequirementID != mcpBug.RequirementID || cliBug.Status != mcpBug.Status || cliBug.ResolutionReason != "fixed" || mcpBug.ResolutionReason != "fixed" {
		t.Fatalf("CLI bug=%+v MCP bug=%+v", cliBug, mcpBug)
	}
	if len(cliBug.ReproductionMaterials) != 1 || cliBug.ReproductionMaterials[0].URL != "https://cdn.example.test/proof.png" || cliBug.ReproductionSteps == "" {
		t.Fatalf("materials lost: %+v", cliBug)
	}
	var stored qualitygorm.QualityBug
	if err := db.First(&stored, cliBug.ID).Error; err != nil || stored.ReproductionMaterialsJSON != quality.EncodeMaterials(cliBug.ReproductionMaterials) || stored.ReproductionSteps != cliBug.ReproductionSteps || stored.ResolutionReason != "fixed" {
		t.Fatalf("database materials lost: %+v %v", stored, err)
	}
	var bugs, applies int64
	_ = db.Model(&qualitygorm.QualityBug{}).Count(&bugs).Error
	_ = db.Model(&qualitygorm.ToolMutationApply{}).Count(&applies).Error
	if bugs != 2 || applies != 2 {
		t.Fatalf("bugs=%d applies=%d", bugs, applies)
	}
}

func jsonNumber(value uint) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func TestQualityFlowHandlersCarryEvidenceWithoutOrdinaryFieldUpdates(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		transition  bool
	}{
		{QualityBugStatusUpdate, `{"id":9,"status":"review","resolution_reason":"fixed","resolution_note":"已修复","resolution_materials":[{"url":"https://example.test/fixed.png"}]}`, true},
		{QualityBugStatusUpdate, `{"id":9,"status":"reopened","regression_note":"不行","regression_materials":[{"url":"https://example.test/reopen.png"}]}`, true},
		{QualityBugAssign, `{"id":9,"assignee_user_id":3,"assignment_note":"请处理","assignment_materials":[{"url":"https://example.test/assign.png"}]}`, false},
	} {
		t.Run(tc.input, func(t *testing.T) {
			capture := &qualityMutationCapture{result: quality.BugMutationResult{Bug: &quality.BugSnapshot{ID: 9}}}
			handlers, err := NewQualityWriteHandlers(capture)
			if err != nil {
				t.Fatal(err)
			}
			_, err = handlers[tc.name].(toolruntime.IdempotentMutationHandler).Apply(context.Background(), toolruntime.MutationCall{
				Authorization: toolruntime.ExecutionAuthorization{PrincipalID: 3, WorkspaceID: 7}, Input: json.RawMessage(tc.input), ChangeSetID: "flow-test", ApplyKey: "flow-test",
			})
			if err != nil || capture.command.StatusTransition != tc.transition {
				t.Fatalf("command=%+v err=%v", capture.command, err)
			}
			v := capture.command.Values
			if v.Fields["reproduction_steps"] || v.Fields["actual_result"] || v.Fields["requirement_id"] {
				t.Fatalf("ordinary fields leaked: %+v", v)
			}
			if tc.name == QualityBugAssign && (v.AssignmentNote != "请处理" || len(v.AssignmentMaterials) != 1) {
				t.Fatalf("assignment evidence lost: %+v", v)
			}
			if v.Status == "reopened" && (v.RegressionNote != "不行" || len(v.RegressionMaterials) != 1) {
				t.Fatalf("reopen evidence lost: %+v", v)
			}
		})
	}
}
