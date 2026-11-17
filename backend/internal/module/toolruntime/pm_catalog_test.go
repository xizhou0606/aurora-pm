package toolruntime

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var pmCatalogSnapshotNames = strings.Fields(`
bi.screen.get
document.access_request.context
document.access_request.create
document.access_request.decide
document.access_request.list
document.access_scope.get
document.asset.delete
document.asset.list
document.binding.create
document.binding.delete
document.binding.list
document.binding.preview
document.bitable.access_request.context
document.comment.delete_own
document.comment.edit_own
document.comment.reply
document.comment_thread.create
document.comment_thread.list
document.comment_thread.reopen
document.comment_thread.resolve
document.content.replace
document.export
document.get
document.node.archive
document.node.copy
document.node.create
document.node.deletion_impact
document.node.move
document.node.preference.update
document.node.rename
document.node.shortcut.create
document.node.tree
document.owner.transfer
document.resource.policy.get
document.resource.policy.update
document.share_link.list
document.share_link.revoke
document.share_subject.search
document.space.get
project.access.effective
project.board.get
project.delivery_label.create
project.delivery_label.delete
project.delivery_label.list
project.delivery_label.update
project.participant.list
project.plan_unit.create
project.plan_unit.delete
project.plan_unit.get
project.plan_unit.list
project.plan_unit.update
project.requirement.batch_move
project.requirement.create
project.requirement.delete
project.requirement.get
project.requirement.list
project.requirement.role_owners.assign
project.requirement.update
project.scope_options.list
project.set.audit.list
project.set.create
project.set.delete
project.set.get
project.set.list
project.set.transfer_owner
project.set.update
quality.bug.activity.list
quality.bug.assign
quality.bug.batch_assign
quality.bug.batch_create
quality.bug.batch_delete
quality.bug.create
quality.bug.delete
quality.bug.get
quality.bug.list
quality.bug.status.update
quality.bug.update
quality.material.upload
task.activity.list
task.attachment.delete
task.attachment.list
task.batch_create
task.batch_delete
task.batch_update
task.children.batch_create
task.client_option.list
task.clone
task.comment.create
task.comment.delete
task.comment.list
task.create
task.delete
task.due_conflict.list
task.get
task.git_link.list
task.list
task.related.list
task.stats.assignee_status
task.stats.delivery
task.stats.due_completion
task.stats.overview
task.status.batch_update
task.status.update
task.testing.context.get
task.testing.definition.get
task.testing.definition.update
task.testing.execution.create
task.testing.execution_run.list
task.testing.execution_run.update
task.update
task.watch.update
task.workflow.kind.list
task.workflow.status.list
task.workflow.transition.list
team.member.list
user.me
user.search
workbench.todo.feed
workbench.todo.summary
workflow.action.execute
workflow.instance.get
workflow.instance.list
workflow.node_plan.update
workflow.opinion.create
workflow.opinion.list
workflow.opinion.update
workflow.template.get
workflow.template.list
workflow.test_case_artifact.list
workflow.test_report_artifact.list
workflow.test_report_link.create
workspace.member.list
`)

var pmActivationSnapshotNames = []string{
	"project.set.list", "project.plan_unit.list", "project.access.effective", "project.board.get",
	"project.scope_options.list", "project.set.audit.list", "project.delivery_label.list",
	"project.requirement.list", "project.participant.list",
	"project.set.get", "project.plan_unit.get", "project.requirement.get", "task.get",
	"task.list", "task.stats.overview", "task.stats.delivery", "task.stats.due_completion", "task.stats.assignee_status",
	"task.comment.list", "task.activity.list", "task.attachment.list", "task.git_link.list", "task.related.list", "task.due_conflict.list",
	"workflow.template.list", "workflow.template.get", "workflow.instance.list", "workflow.instance.get",
	"quality.bug.list", "quality.bug.get", "quality.bug.activity.list",
	"team.member.list", "user.me", "user.search", "workbench.todo.feed", "workbench.todo.summary",
	"workspace.member.list", "bi.screen.get",
}

func TestPMCatalogMatchesAcceptedSourceSnapshot(t *testing.T) {
	catalog, err := NewPMCatalog(PMActivation{})
	if err != nil {
		t.Fatal(err)
	}
	definitions := catalog.Definitions()
	if len(definitions) != 132 {
		t.Fatalf("definitions=%d, want 132", len(definitions))
	}
	names := make([]string, 0, len(definitions))
	for _, definition := range definitions {
		names = append(names, definition.Name)
		if definition.Lifecycle != LifecycleDraft {
			t.Fatalf("%s is %s without a handler activation", definition.Name, definition.Lifecycle)
		}
		assertObjectSchema(t, definition.Name+" input", definition.InputSchema)
		assertObjectSchema(t, definition.Name+" output", definition.OutputSchema)
	}
	if !reflect.DeepEqual(names, pmCatalogSnapshotNames) {
		t.Fatalf("canonical operation snapshot drifted\ngot:  %v\nwant: %v", names, pmCatalogSnapshotNames)
	}

	wantToolsets := strings.Fields(`
pm_native_access_read pm_native_document_read pm_native_document_write pm_native_project_read pm_native_project_write
pm_native_quality_read pm_native_quality_write pm_native_task_read
pm_native_task_testing pm_native_task_write pm_native_user_read
pm_native_workbench_read pm_native_workflow_read pm_native_workflow_write
`)
	if got := PMToolsets(); !reflect.DeepEqual(got, wantToolsets) {
		t.Fatalf("toolsets=%v, want %v", got, wantToolsets)
	}
}

func TestPMSourceLedgerPinsAuditOnlyLineage(t *testing.T) {
	ledger := PMSourceLedger()
	if len(ledger) != 132 {
		t.Fatalf("ledger=%d, want 132", len(ledger))
	}
	seen := make(map[string]struct{}, len(ledger))
	ready := 0
	for _, record := range ledger {
		if record.SourceCommit != PMCatalogSourceCommit || record.SourceOperation != "native."+record.Name {
			t.Fatalf("invalid source lineage: %+v", record)
		}
		if record.SourceMethod == "" || record.SourcePath == "" || record.SourceRisk == "" {
			t.Fatalf("incomplete source record: %+v", record)
		}
		if _, duplicate := seen[record.Name]; duplicate {
			t.Fatalf("duplicate source record %s", record.Name)
		}
		seen[record.Name] = struct{}{}
		switch record.MigrationStatus {
		case "ready_for_implementation":
			ready++
		default:
			t.Fatalf("unknown migration status: %+v", record)
		}
	}
	if ready != 132 {
		t.Fatalf("migration ledger ready=%d, want 132", ready)
	}
}

func TestPMCatalogSourceCommitIsRecordedInMigrationLedger(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "migrations", "source", "source.json"))
	if err != nil {
		t.Fatal(err)
	}
	var source struct {
		Ports []struct {
			Commit string `json:"source_commit"`
		} `json:"post_extraction_ports"`
	}
	if err := json.Unmarshal(raw, &source); err != nil {
		t.Fatal(err)
	}
	for _, port := range source.Ports {
		if port.Commit == PMCatalogSourceCommit {
			return
		}
	}
	t.Fatalf("backend/docs/migrations/source/source.json does not record PM catalog source %s", PMCatalogSourceCommit)
}

func TestPMCatalogActivationRequiresEligibleExecutableHandlers(t *testing.T) {
	catalog, err := NewPMCatalog(PMActivation{ExecutableToolNames: pmActivationSnapshotNames})
	if err != nil {
		t.Fatal(err)
	}
	active := make([]string, 0, len(pmActivationSnapshotNames))
	for _, definition := range catalog.Definitions() {
		if definition.Lifecycle == LifecycleActive {
			active = append(active, definition.Name)
		}
	}
	if !reflect.DeepEqual(active, pmActivationSnapshotNamesSorted()) {
		t.Fatalf("active=%v", active)
	}
	mcp, err := catalog.Manifest(DeliveryMCP)
	if err != nil {
		t.Fatal(err)
	}
	cli, err := catalog.Manifest(DeliveryCLI)
	if err != nil {
		t.Fatal(err)
	}
	if len(mcp) != len(pmActivationSnapshotNames) || len(cli) != len(pmActivationSnapshotNames) {
		t.Fatalf("published manifests MCP=%d CLI=%d, want %d/%d", len(mcp), len(cli), len(pmActivationSnapshotNames), len(pmActivationSnapshotNames))
	}
	if err := ValidateDeliveryParity(mcp, cli); err != nil {
		t.Fatalf("MCP/CLI parity: %v", err)
	}
}

func TestPMCatalogRejectsUnsafeActivation(t *testing.T) {
	for name, activation := range map[string][]string{
		"unknown":   {"hello.world"},
		"duplicate": {"task.get", "task.get"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewPMCatalog(PMActivation{ExecutableToolNames: activation}); !errors.Is(err, ErrPMActivationInvalid) {
				t.Fatalf("err=%v, want ErrPMActivationInvalid", err)
			}
		})
	}
}

func TestPMActivationAndApplicationHandlerRegistrationStayInLockstep(t *testing.T) {
	catalog, err := NewPMCatalog(PMActivation{ExecutableToolNames: pmActivationSnapshotNames})
	if err != nil {
		t.Fatal(err)
	}
	handlers := make(map[string]ToolHandler, len(pmActivationSnapshotNames))
	for _, name := range pmActivationSnapshotNames {
		handlers[name] = executionHandler()
	}
	if _, err := NewApplication(catalog, &executorAuthorizer{}, handlers); err != nil {
		t.Fatalf("complete handler registration rejected: %v", err)
	}
	delete(handlers, "task.get")
	if _, err := NewApplication(catalog, &executorAuthorizer{}, handlers); !errors.Is(err, ErrToolUnavailable) {
		t.Fatalf("incomplete handler registration err=%v, want ErrToolUnavailable", err)
	}
}

func TestPMActiveInputSchemasMatchHandlerContracts(t *testing.T) {
	catalog, err := NewPMCatalog(PMActivation{ExecutableToolNames: pmActivationSnapshotNames})
	if err != nil {
		t.Fatal(err)
	}
	wantRequired := map[string][]string{
		"project.set.list": nil, "project.plan_unit.list": {"project_new_id"}, "project.access.effective": {"project_set_id"},
		"project.board.get": {"project_new_id"}, "project.scope_options.list": nil,
		"project.set.audit.list": {"id"}, "project.delivery_label.list": {"id"},
		"project.requirement.list": nil, "project.participant.list": {"id"},
		"project.set.get": {"id"}, "project.plan_unit.get": {"id"}, "project.requirement.get": {"id"},
		"task.get": nil, "task.list": nil, "task.stats.overview": nil, "task.stats.delivery": nil,
		"task.stats.due_completion": nil, "task.stats.assignee_status": nil,
		"task.comment.list": {"id"}, "task.activity.list": {"id"}, "task.attachment.list": {"id"},
		"task.git_link.list": {"id"}, "task.related.list": {"id"}, "task.due_conflict.list": {"id", "due_at"},
		"workflow.template.list": nil, "workflow.template.get": {"id"},
		"workflow.instance.list": {"subject_type", "subject_id"}, "workflow.instance.get": {"id"},
		"team.member.list": {"id"}, "user.me": nil, "user.search": nil, "workspace.member.list": {"id"},
		"workbench.todo.feed": nil, "workbench.todo.summary": nil,
		"bi.screen.get": nil,
	}
	for name, want := range wantRequired {
		definition, found := catalog.Find(name)
		if !found || definition.Lifecycle != LifecycleActive {
			t.Fatalf("active definition missing: %s", name)
		}
		var schema struct {
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(definition.InputSchema, &schema); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(schema.Required, want) {
			t.Fatalf("%s required=%v, want %v", name, schema.Required, want)
		}
	}
}

func TestPMTaskGetRequiresTaskReferenceOrID(t *testing.T) {
	catalog, err := NewPMCatalog(PMActivation{ExecutableToolNames: []string{"task.get"}})
	if err != nil {
		t.Fatal(err)
	}
	definition, _ := catalog.Find("task.get")
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
		AnyOf      []struct {
			Required []string `json:"required"`
		} `json:"anyOf"`
	}
	if err := json.Unmarshal(definition.InputSchema, &schema); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"task_ref", "id", "project_set_id", "project_new_id", "requirement_id"} {
		if _, exists := schema.Properties[name]; !exists {
			t.Fatalf("task.get input property %s is missing", name)
		}
	}
	wantAnyOf := [][]string{{"task_ref"}, {"id"}}
	gotAnyOf := make([][]string, 0, len(schema.AnyOf))
	for _, choice := range schema.AnyOf {
		gotAnyOf = append(gotAnyOf, choice.Required)
	}
	if !reflect.DeepEqual(gotAnyOf, wantAnyOf) {
		t.Fatalf("task.get anyOf=%v, want %v", gotAnyOf, wantAnyOf)
	}
}

func TestPMActiveOutputSchemasMatchHandlerContracts(t *testing.T) {
	catalog, err := NewPMCatalog(PMActivation{ExecutableToolNames: pmActivationSnapshotNames})
	if err != nil {
		t.Fatal(err)
	}
	wantRequired := map[string][]string{
		"project.set.list": {"items", "total"}, "project.plan_unit.list": {"items", "total"},
		"project.requirement.list": {"items", "total"}, "project.set.audit.list": {"items", "total"},
		"project.delivery_label.list": {"items", "total"}, "project.board.get": {"project"},
		"project.scope_options.list": {"project_sets", "projects", "requirements"},
		"project.participant.list":   {"project_set", "participants", "total", "authorization"},
		"project.access.effective":   {"effective", "actor", "workspace", "project_set", "access_basis"},
		"project.set.get":            {"id", "workspace_id"}, "project.plan_unit.get": {"id", "workspace_id"},
		"project.requirement.get": {"id", "workspace_id"},
		"task.get":                {"id", "workspace_id", "task_key", "title", "children"}, "task.list": {"items", "total"},
		"task.stats.overview":        {"total", "completed", "incomplete", "overdue", "due_soon", "due_soon_days", "by_kind", "by_kind_completed", "by_status"},
		"task.stats.delivery":        {"total", "completed", "open", "overdue", "completion_rate", "by_status", "by_priority"},
		"task.stats.due_completion":  {"total", "overdue", "due_today", "due_soon", "later", "open_without_due", "completed_on_time", "completed_late", "completed_without_due", "as_of", "timezone"},
		"task.stats.assignee_status": {"items", "total_assignees"},
		"task.comment.list":          {"items", "total"}, "task.activity.list": {"items", "total"},
		"task.attachment.list": {"items", "total"}, "task.git_link.list": {"items", "total"},
		"task.related.list": {"items", "total"}, "task.due_conflict.list": {"items", "total"},
		"workflow.template.list": {"items", "total"}, "workflow.instance.list": {"items", "total"},
		"workflow.template.get": {"id", "workspace_id", "definition"},
		"workflow.instance.get": {"id", "workspace_id", "current_node_ids", "node_plans"},
		"team.member.list":      {"items", "total"}, "user.me": {"id", "username", "display_name"}, "user.search": {"items", "total"},
		"workspace.member.list": {"items", "total"},
		"workbench.todo.feed":   {"items", "total"}, "workbench.todo.summary": {"summary", "generated_at"},
		"bi.screen.get": {"period", "workspace_id", "workspace_name", "team_count", "scope", "overview_metrics",
			"urgency_distribution", "due_distribution", "priority_distribution", "type_distribution", "status_progress",
			"completion_trend", "version_progress", "requirement_progress", "version_rankings", "owner_board",
			"milestone_options", "quality_severity_distribution", "quality_status_distribution", "quality_trend",
			"version_quality_risks", "quality_owner_load", "team_data", "generated_at", "summary", "kpi", "teams",
			"projects", "risks", "risks_by_team", "risks_by_kind", "release_train", "sparklines", "light_distribution",
			"status_mix", "data_quality", "project_total", "project_exception_count", "pm_pipeline", "team_clusters", "pm_narrative"},
	}
	for name, want := range wantRequired {
		definition, _ := catalog.Find(name)
		var schema struct {
			Required []string `json:"required"`
		}
		if err := json.Unmarshal(definition.OutputSchema, &schema); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(schema.Required, want) {
			t.Fatalf("%s output required=%v, want %v", name, schema.Required, want)
		}
	}
}

func TestPMWorkspaceMemberListOutputSchemaIsPrivacyMinimal(t *testing.T) {
	catalog, err := NewPMCatalog(PMActivation{ExecutableToolNames: []string{"workspace.member.list"}})
	if err != nil {
		t.Fatal(err)
	}
	definition, found := catalog.Find("workspace.member.list")
	if !found || definition.Lifecycle != LifecycleActive {
		t.Fatalf("definition=%+v found=%v", definition, found)
	}
	var schema struct {
		AdditionalProperties bool `json:"additionalProperties"`
		Properties           map[string]struct {
			Items json.RawMessage `json:"items"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(definition.OutputSchema, &schema); err != nil {
		t.Fatal(err)
	}
	if schema.AdditionalProperties {
		t.Fatal("workspace member output allows undeclared top-level fields")
	}
	var item struct {
		AdditionalProperties bool                       `json:"additionalProperties"`
		Properties           map[string]json.RawMessage `json:"properties"`
		Required             []string                   `json:"required"`
	}
	if err := json.Unmarshal(schema.Properties["items"].Items, &item); err != nil {
		t.Fatal(err)
	}
	want := []string{"display_name", "id", "username"}
	got := make([]string, 0, len(item.Properties))
	for name := range item.Properties {
		got = append(got, name)
	}
	sort.Strings(got)
	sort.Strings(item.Required)
	if item.AdditionalProperties || !reflect.DeepEqual(got, want) || !reflect.DeepEqual(item.Required, want) {
		t.Fatalf("item fields=%v required=%v additional=%v", got, item.Required, item.AdditionalProperties)
	}
	for _, forbidden := range []string{"email", "token", "role_id", "group", "permission"} {
		if strings.Contains(strings.ToLower(string(definition.OutputSchema)), forbidden) {
			t.Fatalf("privacy schema contains forbidden field %q: %s", forbidden, definition.OutputSchema)
		}
	}
}

func TestPMWorkflowInstanceGetAcceptsObjectNodePlans(t *testing.T) {
	const name = "workflow.instance.get"
	catalog, err := NewPMCatalog(PMActivation{ExecutableToolNames: []string{name}})
	if err != nil {
		t.Fatal(err)
	}
	authorizer := &executorAuthorizer{
		tools: map[string]bool{name: true}, scopes: map[string]bool{"workflow:read": true},
	}
	handler := ToolHandlerFunc(func(context.Context, ToolCall) (json.RawMessage, error) {
		return json.RawMessage(`{"id":1,"workspace_id":11,"current_node_ids":[],"node_plans":{}}`), nil
	})
	application, err := NewApplication(catalog, authorizer, map[string]ToolHandler{name: handler})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := application.Invoke(context.Background(), Invocation{
		Name: name, Input: json.RawMessage(`{"id":1}`), Authorization: executionAuthorization(),
	}); err != nil {
		t.Fatalf("workflow.instance.get rejected object node_plans: %v", err)
	}
}

func TestPMProjectListAndAccessHandlersMatchExecutionSchemas(t *testing.T) {
	names := []string{"project.set.list", "project.plan_unit.list", "project.access.effective"}
	catalog, err := NewPMCatalog(PMActivation{ExecutableToolNames: names})
	if err != nil {
		t.Fatal(err)
	}
	handlers := map[string]ToolHandler{
		"project.set.list": ToolHandlerFunc(func(context.Context, ToolCall) (json.RawMessage, error) {
			return json.RawMessage(`{"items":[{"id":11,"workspace_id":7}],"total":1}`), nil
		}),
		"project.plan_unit.list": ToolHandlerFunc(func(context.Context, ToolCall) (json.RawMessage, error) {
			return json.RawMessage(`{"items":[{"id":21,"workspace_id":7,"project_new_id":11}],"total":1}`), nil
		}),
		"project.access.effective": ToolHandlerFunc(func(context.Context, ToolCall) (json.RawMessage, error) {
			return json.RawMessage(`{
				"effective":true,
				"actor":{"user_id":9},
				"workspace":{"id":7,"effective":true},
				"project_set":{"id":11,"name":"PM","owner_user_id":9,"actor_is_owner":true},
				"access_basis":{
					"model":"external_grant_plus_pm_authorization",
					"workspace_is_data_context":true,
					"project_owner_is_informational":true,
					"legacy_project_permission_used":false
				}
			}`), nil
		}),
	}
	authorizer := &executorAuthorizer{
		tools: map[string]bool{
			"project.set.list": true, "project.plan_unit.list": true, "project.access.effective": true,
		},
		scopes: map[string]bool{"project:read": true, "workspace:read": true},
	}
	application, err := NewApplication(catalog, authorizer, handlers)
	if err != nil {
		t.Fatal(err)
	}
	inputs := map[string]json.RawMessage{
		"project.set.list":         json.RawMessage(`{"team_id":70,"q":"Alpha","limit":10,"offset":0}`),
		"project.plan_unit.list":   json.RawMessage(`{"project_new_id":11}`),
		"project.access.effective": json.RawMessage(`{"project_set_id":11}`),
	}
	for _, name := range names {
		if _, err := application.Invoke(context.Background(), Invocation{
			Name: name, Input: inputs[name], Authorization: executionAuthorization(),
		}); err != nil {
			t.Fatalf("%s rejected real handler contract: %v", name, err)
		}
	}
}

func TestPMTaskReadHandlersMatchExecutionSchemas(t *testing.T) {
	names := []string{
		"task.list", "task.stats.overview", "task.stats.delivery", "task.stats.due_completion", "task.stats.assignee_status",
		"task.comment.list", "task.activity.list", "task.attachment.list", "task.git_link.list", "task.related.list", "task.due_conflict.list",
	}
	catalog, err := NewPMCatalog(PMActivation{ExecutableToolNames: names})
	if err != nil {
		t.Fatal(err)
	}
	listOutput := json.RawMessage(`{"items":[],"total":0}`)
	outputs := map[string]json.RawMessage{
		"task.list": listOutput,
		"task.stats.overview": json.RawMessage(`{
			"total":0,"completed":0,"incomplete":0,"overdue":0,"due_soon":0,"due_soon_days":7,
			"by_kind":{},"by_kind_completed":{},"by_status":{}
		}`),
		"task.stats.delivery": json.RawMessage(`{
			"total":0,"completed":0,"open":0,"overdue":0,"completion_rate":0,"by_status":{},"by_priority":{}
		}`),
		"task.stats.due_completion": json.RawMessage(`{
			"total":0,"overdue":0,"due_today":0,"due_soon":0,"later":0,"open_without_due":0,
			"completed_on_time":0,"completed_late":0,"completed_without_due":0,
			"as_of":"2026-08-27T00:00:00Z","timezone":"UTC"
		}`),
		"task.stats.assignee_status": json.RawMessage(`{"items":[],"total_assignees":0}`),
		"task.comment.list":          listOutput,
		"task.activity.list":         listOutput,
		"task.attachment.list":       listOutput,
		"task.git_link.list":         listOutput,
		"task.related.list":          listOutput,
		"task.due_conflict.list":     listOutput,
	}
	inputs := map[string]json.RawMessage{
		"task.list":           json.RawMessage(`{"ids":[1,2],"status":"draft,in_dev","project_new_id":9,"limit":25,"offset":0}`),
		"task.stats.overview": json.RawMessage(`{}`), "task.stats.delivery": json.RawMessage(`{}`),
		"task.stats.due_completion": json.RawMessage(`{}`), "task.stats.assignee_status": json.RawMessage(`{}`),
		"task.comment.list": json.RawMessage(`{"id":42}`), "task.activity.list": json.RawMessage(`{"id":42,"limit":40,"offset":0}`),
		"task.attachment.list": json.RawMessage(`{"id":42}`), "task.git_link.list": json.RawMessage(`{"id":42}`),
		"task.related.list":      json.RawMessage(`{"id":42}`),
		"task.due_conflict.list": json.RawMessage(`{"id":42,"due_at":"2026-08-30T10:00:00Z"}`),
	}
	handlers := make(map[string]ToolHandler, len(names))
	tools := make(map[string]bool, len(names))
	for _, name := range names {
		output := outputs[name]
		handlers[name] = ToolHandlerFunc(func(context.Context, ToolCall) (json.RawMessage, error) { return output, nil })
		tools[name] = true
	}
	application, err := NewApplication(catalog, &executorAuthorizer{
		tools: tools, scopes: map[string]bool{"task:read": true},
	}, handlers)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if _, err := application.Invoke(context.Background(), Invocation{
			Name: name, Input: inputs[name], Authorization: executionAuthorization(),
		}); err != nil {
			t.Fatalf("%s rejected real handler contract: %v", name, err)
		}
	}
}

func TestPMQualityReadHandlersMatchExecutionSchemas(t *testing.T) {
	names := []string{"quality.bug.list", "quality.bug.get", "quality.bug.activity.list"}
	catalog, err := NewPMCatalog(PMActivation{ExecutableToolNames: names})
	if err != nil {
		t.Fatal(err)
	}
	listOutput := json.RawMessage(`{"items":[],"total":0}`)
	outputs := map[string]json.RawMessage{
		"quality.bug.list":          listOutput,
		"quality.bug.activity.list": listOutput,
		"quality.bug.get": json.RawMessage(`{
			"id":7,"workspace_id":11,"bug_key":"BUG-7","title":"Login failure","status":"triage","priority":"high","defect_domain":"functional"
		}`),
	}
	inputs := map[string]json.RawMessage{
		"quality.bug.list":          json.RawMessage(`{"project_new_id":3,"status":"triage,reopened","limit":25,"offset":0}`),
		"quality.bug.get":           json.RawMessage(`{"id":7}`),
		"quality.bug.activity.list": json.RawMessage(`{"id":7,"limit":20,"offset":0}`),
	}
	handlers := make(map[string]ToolHandler, len(names))
	tools := make(map[string]bool, len(names))
	for _, name := range names {
		output := outputs[name]
		handlers[name] = ToolHandlerFunc(func(context.Context, ToolCall) (json.RawMessage, error) { return output, nil })
		tools[name] = true
	}
	application, err := NewApplication(catalog, &executorAuthorizer{
		tools: tools, scopes: map[string]bool{"quality:read": true},
	}, handlers)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if _, err := application.Invoke(context.Background(), Invocation{
			Name: name, Input: inputs[name], Authorization: executionAuthorization(),
		}); err != nil {
			t.Fatalf("%s rejected real handler contract: %v", name, err)
		}
	}
}

func assertObjectSchema(t *testing.T, label string, raw json.RawMessage) {
	t.Helper()
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil || schema["type"] != "object" {
		t.Fatalf("%s is not an object JSON Schema: %s", label, raw)
	}
}

func pmActivationSnapshotNamesSorted() []string {
	names := append([]string(nil), pmActivationSnapshotNames...)
	sort.Strings(names)
	return names
}
