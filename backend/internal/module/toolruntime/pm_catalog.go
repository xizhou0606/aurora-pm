package toolruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

const (
	PMCatalogSourceCommit = "6da4b880aa9215135afa8a798077749cbd91525f"
	PMCatalogVersion      = "1.1.0"
)

var ErrPMActivationInvalid = errors.New("PM tool activation is invalid")

// PMActivation is the composition-root contract between executable handlers
// and the catalog. A PM operation is published only when it is both eligible
// for the current release and explicitly backed by a registered handler.
type PMActivation struct {
	ExecutableToolNames []string
}

type PMSourceRecord struct {
	Name            string `json:"name"`
	Toolset         string `json:"toolset"`
	SourceCommit    string `json:"source_commit"`
	SourceOperation string `json:"source_operation"`
	SourceMethod    string `json:"source_method"`
	SourcePath      string `json:"source_path"`
	SourceRisk      string `json:"source_risk"`
	MigrationStatus string `json:"migration_status"`
}

type pmOperationSpec struct {
	name, toolset, method, path, summary, sourceRisk string
	version                                          string
	fields                                           []pmCatalogField
	activationEligible                               bool
}

// NewPMCatalog builds the canonical PM operation catalog. Definitions remain
// draft unless the composition root proves that an eligible handler exists.
func NewPMCatalog(activation PMActivation) (*Catalog, error) {
	specs := pmOperationSpecs()
	requested := make(map[string]struct{}, len(activation.ExecutableToolNames))
	eligible := make(map[string]bool, len(specs))
	for _, spec := range specs {
		eligible[spec.name] = spec.activationEligible
	}
	for _, name := range activation.ExecutableToolNames {
		if _, duplicate := requested[name]; duplicate {
			return nil, fmt.Errorf("%w: duplicate handler %s", ErrPMActivationInvalid, name)
		}
		if allowed, exists := eligible[name]; !exists || !allowed {
			return nil, fmt.Errorf("%w: %s is not activation-eligible", ErrPMActivationInvalid, name)
		}
		requested[name] = struct{}{}
	}

	definitions := make([]Definition, 0, len(specs))
	for _, spec := range specs {
		lifecycle := LifecycleDraft
		if _, active := requested[spec.name]; active {
			lifecycle = LifecycleActive
		}
		risk, changeSet := pmRiskContract(spec.sourceRisk)
		version := spec.version
		if version == "" {
			version = PMCatalogVersion
		}
		definitions = append(definitions, Definition{
			Name: spec.name, Version: version, Summary: spec.summary,
			Lifecycle: lifecycle, InputSchema: pmInputSchema(spec.name, spec.fields, risk != RiskRead),
			OutputSchema: pmOutputSchema(spec.name), Authorization: AuthorizationContract{
				IdentityRequired: true, WorkspaceRequired: true, GrantRequired: true,
				GrantScope: pmGrantScope(spec.toolset, risk),
			},
			Risk: risk, ChangeSet: changeSet, UAT: UATWithinGrantWindow,
			AuditEvent: "tool." + spec.name,
		})
	}
	return NewCatalog(definitions)
}

func PMSourceLedger() []PMSourceRecord {
	specs := pmOperationSpecs()
	result := make([]PMSourceRecord, 0, len(specs))
	for _, spec := range specs {
		result = append(result, PMSourceRecord{
			Name: spec.name, Toolset: spec.toolset, SourceCommit: PMCatalogSourceCommit,
			SourceOperation: "native." + spec.name, SourceMethod: spec.method,
			SourcePath: spec.path, SourceRisk: spec.sourceRisk, MigrationStatus: "ready_for_implementation",
		})
	}
	return result
}

func PMToolsets() []string {
	set := map[string]struct{}{}
	for _, record := range PMSourceLedger() {
		set[record.Toolset] = struct{}{}
	}
	result := make([]string, 0, len(set))
	for toolset := range set {
		result = append(result, toolset)
	}
	sort.Strings(result)
	return result
}

func pmOperationSpecs() []pmOperationSpec {
	result := make([]pmOperationSpec, 0, 90)
	result = append(result, pmAccessOperationSpecs()...)
	result = append(result, pmDocumentOperationSpecs()...)
	result = append(result, pmDocumentAccessOperationSpecs()...)
	result = append(result, pmDocumentAccessWriteOperationSpecs()...)
	result = append(result, pmDocumentWriteOperationSpecs()...)
	result = append(result, pmProjectOperationSpecs()...)
	result = append(result, pmTaskOperationSpecs()...)
	result = append(result, pmQualityOperationSpecs()...)
	result = append(result, pmWorkbenchOperationSpecs()...)
	result = append(result, pmTestingOperationSpecs()...)
	result = append(result, pmWorkflowOperationSpecs()...)
	sort.Slice(result, func(i, j int) bool { return result[i].name < result[j].name })
	return result
}

func pmSpec(name, toolset, method, path, summary, sourceRisk string, fields ...pmCatalogField) pmOperationSpec {
	return pmOperationSpec{name: name, toolset: toolset, method: method, path: path, summary: summary, sourceRisk: sourceRisk, fields: fields}
}

func pmActivatable(spec pmOperationSpec) pmOperationSpec {
	spec.activationEligible = true
	return spec
}

func pmRiskContract(sourceRisk string) (RiskLevel, ChangeSetPolicy) {
	switch sourceRisk {
	case "L0":
		return RiskRead, ChangeSetForbidden
	case "L1", "L2":
		return RiskMutation, ChangeSetRequired
	default:
		return RiskHighImpact, ChangeSetRequired
	}
}

func pmGrantScope(toolset string, risk RiskLevel) string {
	domain := map[string]string{
		"pm_native_document_read": "document", "pm_native_document_write": "document",
		"pm_native_access_read": "workspace", "pm_native_user_read": "identity",
		"pm_native_project_read": "project", "pm_native_project_write": "project",
		"pm_native_task_read": "task", "pm_native_task_write": "task",
		"pm_native_quality_read": "quality", "pm_native_quality_write": "quality",
		"pm_native_workbench_read": "workbench", "pm_native_task_testing": "task",
		"pm_native_workflow_read": "workflow", "pm_native_workflow_write": "workflow",
	}[toolset]
	action := "write"
	if risk == RiskRead {
		action = "read"
	}
	return domain + ":" + action
}

func pmOutputSchema(name string) json.RawMessage {
	properties := map[string]any{}
	required := []string(nil)
	additionalProperties := true
	switch name {
	case "user.me":
		properties = map[string]any{
			"id":           map[string]any{"type": "integer", "minimum": 1},
			"username":     map[string]any{"type": "string"},
			"display_name": map[string]any{"type": "string"},
		}
		required = []string{"id", "username", "display_name"}
		additionalProperties = false
	case "quality.bug.create", "quality.bug.update", "quality.bug.assign", "quality.bug.status.update":
		bug := pmQualityBugOutputSchema()
		properties, _ = bug["properties"].(map[string]any)
		required, _ = bug["required"].([]string)
		additionalProperties = false
	case "quality.bug.delete":
		properties = map[string]any{"id": pmPositiveOutputSchema(), "deleted": map[string]any{"type": "boolean", "const": true}}
		required = []string{"id", "deleted"}
		additionalProperties = false
	case "quality.material.upload":
		properties = map[string]any{"url": map[string]any{"type": "string"}, "object_key": map[string]any{"type": "string"}, "object_storage_profile_id": pmCountOutputSchema(), "name": map[string]any{"type": "string"}, "size": pmPositiveOutputSchema(), "content_type": map[string]any{"type": "string"}}
		required = []string{"url", "object_key", "object_storage_profile_id", "name", "size", "content_type"}
		additionalProperties = false
	case "quality.bug.batch_create":
		properties = map[string]any{
			"created": map[string]any{"type": "array", "items": map[string]any{
				"type": "object", "properties": map[string]any{
					"client_id": map[string]any{"type": "string", "minLength": 1}, "bug": pmQualityBugOutputSchema(),
				}, "required": []string{"client_id", "bug"}, "additionalProperties": false,
			}},
			"created_count": pmCountOutputSchema(), "atomic": map[string]any{"type": "boolean", "const": true},
		}
		required = []string{"created", "created_count", "atomic"}
		additionalProperties = false
	case "quality.bug.batch_assign":
		properties = map[string]any{
			"updated":       map[string]any{"type": "array", "items": pmQualityBugOutputSchema()},
			"updated_count": pmCountOutputSchema(),
			"atomic":        map[string]any{"type": "boolean", "const": true},
		}
		required = []string{"updated", "updated_count", "atomic"}
		additionalProperties = false
	case "quality.bug.batch_delete":
		properties = map[string]any{
			"deleted_ids":   map[string]any{"type": "array", "items": pmPositiveOutputSchema()},
			"deleted_count": pmCountOutputSchema(), "atomic": map[string]any{"type": "boolean", "const": true},
		}
		required = []string{"deleted_ids", "deleted_count", "atomic"}
		additionalProperties = false
	case "project.delivery_label.create", "project.delivery_label.update":
		properties = map[string]any{
			"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(),
			"project_set_id": pmPositiveOutputSchema(), "plan_unit_id": pmPositiveOutputSchema(),
			"client_key": map[string]any{"type": "string"}, "name": map[string]any{"type": "string", "minLength": 1},
			"system_key": map[string]any{"type": "string"}, "sort_order": pmCountOutputSchema(),
			"deleted": map[string]any{"type": "boolean"},
		}
		required = []string{"id", "workspace_id", "project_set_id", "plan_unit_id", "name", "sort_order", "deleted"}
		additionalProperties = false
	case "project.delivery_label.delete":
		properties = map[string]any{"id": pmPositiveOutputSchema(), "deleted": map[string]any{"type": "boolean", "const": true}}
		required = []string{"id", "deleted"}
		additionalProperties = false
	case "project.set.delete":
		properties = map[string]any{"id": pmPositiveOutputSchema(), "deleted": map[string]any{"type": "boolean", "const": true}}
		required = []string{"id", "deleted"}
		additionalProperties = false
	case "project.plan_unit.delete":
		properties = map[string]any{"id": pmPositiveOutputSchema(), "deleted": map[string]any{"type": "boolean", "const": true}}
		required = []string{"id", "deleted"}
		additionalProperties = false
	case "project.set.create", "project.set.update", "project.set.transfer_owner":
		nullable := func(value map[string]any) map[string]any {
			return map[string]any{"anyOf": []any{value, map[string]any{"type": "null"}}}
		}
		properties = map[string]any{
			"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(), "team_id": pmPositiveOutputSchema(),
			"name": map[string]any{"type": "string", "minLength": 1}, "code": map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"}, "owner_user_id": nullable(pmPositiveOutputSchema()),
			"planned_start_at": nullable(map[string]any{"type": "string", "format": "date-time"}),
			"planned_end_at":   nullable(map[string]any{"type": "string", "format": "date-time"}),
			"status":           map[string]any{"type": "string", "enum": []string{"active", "archived"}},
		}
		required = []string{
			"id", "workspace_id", "team_id", "name", "code", "description", "owner_user_id",
			"planned_start_at", "planned_end_at", "status",
		}
		additionalProperties = false
	case "project.plan_unit.create", "project.plan_unit.update":
		nullable := func(value map[string]any) map[string]any {
			return map[string]any{"anyOf": []any{value, map[string]any{"type": "null"}}}
		}
		properties = map[string]any{
			"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(), "team_id": pmPositiveOutputSchema(),
			"project_set_id": pmPositiveOutputSchema(), "title": map[string]any{"type": "string", "minLength": 1}, "type": map[string]any{"type": "string"},
			"goal": map[string]any{"type": "string"}, "owner_user_id": nullable(pmPositiveOutputSchema()),
			"planned_start_at": nullable(map[string]any{"type": "string", "format": "date-time"}), "planned_end_at": nullable(map[string]any{"type": "string", "format": "date-time"}),
			"risk_level": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"},
		}
		required = []string{"id", "workspace_id", "team_id", "project_set_id", "title", "type", "goal", "owner_user_id", "planned_start_at", "planned_end_at", "risk_level", "status"}
		additionalProperties = false
	case "project.requirement.create", "project.requirement.update":
		requirementSchema := pmRequirementMutationOutputSchema()
		properties, _ = requirementSchema["properties"].(map[string]any)
		required, _ = requirementSchema["required"].([]string)
		additionalProperties = false
	case "project.requirement.delete":
		properties = map[string]any{"id": pmPositiveOutputSchema(), "deleted": map[string]any{"type": "boolean", "const": true}}
		required = []string{"id", "deleted"}
		additionalProperties = false
	case "project.requirement.batch_move":
		properties = map[string]any{
			"items":       map[string]any{"type": "array", "minItems": 1, "items": pmRequirementMutationOutputSchema()},
			"moved_count": pmCountOutputSchema(), "affected_task_count": pmCountOutputSchema(),
			"affected_quality_bug_count": pmCountOutputSchema(),
		}
		required = []string{"items", "moved_count", "affected_task_count", "affected_quality_bug_count"}
		additionalProperties = false
	case "task.create", "task.update":
		taskSchema := pmTaskMutationSnapshotOutputSchema()
		properties, _ = taskSchema["properties"].(map[string]any)
		required, _ = taskSchema["required"].([]string)
		additionalProperties = false
	case "task.clone":
		properties = map[string]any{
			"task": pmTaskMutationSnapshotOutputSchema(),
			"cloned_from": map[string]any{
				"type": "object", "properties": map[string]any{
					"id": pmPositiveOutputSchema(), "task_key": map[string]any{"type": "string", "minLength": 1},
					"title": map[string]any{"type": "string", "minLength": 1},
				}, "required": []string{"id", "task_key", "title"}, "additionalProperties": false,
			},
		}
		required = []string{"task", "cloned_from"}
		additionalProperties = false
	case "task.delete":
		properties = map[string]any{"deleted": map[string]any{"type": "boolean", "const": true}, "deleted_count": pmPositiveOutputSchema()}
		required = []string{"deleted", "deleted_count"}
		additionalProperties = false
	case "task.batch_create":
		properties = map[string]any{
			"created":       map[string]any{"type": "array", "minItems": 1, "items": pmBatchCreatedTaskOutputSchema(true)},
			"created_count": pmPositiveOutputSchema(), "atomic": map[string]any{"type": "boolean", "const": true},
		}
		required = []string{"created", "created_count", "atomic"}
		additionalProperties = false
	case "task.children.batch_create":
		properties = map[string]any{
			"created":       map[string]any{"type": "array", "minItems": 1, "items": pmBatchCreatedTaskOutputSchema(false)},
			"created_count": pmPositiveOutputSchema(), "parent_id": pmPositiveOutputSchema(),
			"atomic": map[string]any{"type": "boolean", "const": true},
		}
		required = []string{"created", "created_count", "parent_id", "atomic"}
		additionalProperties = false
	case "task.batch_delete":
		properties = map[string]any{
			"deleted_ids": pmPositiveIDListOutputSchema(), "deleted_count": pmPositiveOutputSchema(),
			"affected_ids": pmPositiveIDListOutputSchema(), "atomic": map[string]any{"type": "boolean", "const": true},
		}
		required = []string{"deleted_ids", "deleted_count", "affected_ids", "atomic"}
		additionalProperties = false
	case "task.batch_update":
		properties = map[string]any{
			"updated_ids": pmPositiveIDListOutputSchema(), "updated_count": pmPositiveOutputSchema(),
			"affected_ids": pmPositiveIDListOutputSchema(), "auto_completed_ancestor_ids": pmPositiveIDListOutputSchema(),
			"due_propagated_descendant_ids": pmPositiveIDListOutputSchema(), "atomic": map[string]any{"type": "boolean", "const": true},
		}
		required = []string{"updated_ids", "updated_count", "affected_ids", "auto_completed_ancestor_ids", "due_propagated_descendant_ids", "atomic"}
		additionalProperties = false
	case "bi.screen.get":
		array := func() map[string]any { return map[string]any{"type": "array"} }
		object := func() map[string]any { return map[string]any{"type": "object"} }
		properties = map[string]any{
			"period":       map[string]any{"type": "string", "enum": []string{"week", "month"}},
			"workspace_id": pmPositiveOutputSchema(), "workspace_name": map[string]any{"type": "string"},
			"team_count": pmCountOutputSchema(), "scope": object(), "overview_metrics": object(),
			"urgency_distribution": array(), "due_distribution": array(), "priority_distribution": array(),
			"type_distribution": array(), "status_progress": array(), "completion_trend": array(),
			"version_progress": array(), "requirement_progress": array(), "version_rankings": array(),
			"owner_board": array(), "milestone_options": array(), "quality_severity_distribution": array(),
			"quality_status_distribution": array(), "quality_trend": array(), "version_quality_risks": array(),
			"quality_owner_load": array(), "team_data": object(),
			"generated_at": map[string]any{"type": "string", "format": "date-time"},
			"summary":      object(), "kpi": object(), "teams": array(), "projects": array(), "risks": array(),
			"risks_by_team": array(), "risks_by_kind": array(), "release_train": array(), "sparklines": array(),
			"light_distribution": object(), "status_mix": array(), "data_quality": object(),
			"project_total": pmCountOutputSchema(), "project_exception_count": pmCountOutputSchema(),
			"pm_pipeline": object(), "team_clusters": array(), "pm_narrative": object(),
		}
		required = []string{
			"period", "workspace_id", "workspace_name", "team_count", "scope", "overview_metrics",
			"urgency_distribution", "due_distribution", "priority_distribution", "type_distribution", "status_progress",
			"completion_trend", "version_progress", "requirement_progress", "version_rankings", "owner_board",
			"milestone_options", "quality_severity_distribution", "quality_status_distribution", "quality_trend",
			"version_quality_risks", "quality_owner_load", "team_data", "generated_at", "summary", "kpi", "teams",
			"projects", "risks", "risks_by_team", "risks_by_kind", "release_train", "sparklines", "light_distribution",
			"status_mix", "data_quality", "project_total", "project_exception_count", "pm_pipeline", "team_clusters", "pm_narrative",
		}
		additionalProperties = false
	case "workspace.member.list":
		properties = map[string]any{
			"items": map[string]any{
				"type": "array", "items": map[string]any{
					"type": "object", "properties": map[string]any{
						"id":           map[string]any{"type": "integer", "minimum": 1},
						"username":     map[string]any{"type": "string"},
						"display_name": map[string]any{"type": "string"},
					},
					"required": []string{"id", "username", "display_name"}, "additionalProperties": false,
				},
			},
			"total": map[string]any{"type": "integer", "minimum": 0},
		}
		required = []string{"items", "total"}
		additionalProperties = false
	case "project.set.get", "project.plan_unit.get", "project.requirement.get":
		properties = map[string]any{"id": map[string]any{"type": "integer"}, "workspace_id": map[string]any{"type": "integer"}}
		required = []string{"id", "workspace_id"}
	case "task.get":
		properties = map[string]any{
			"id": map[string]any{"type": "integer"}, "workspace_id": map[string]any{"type": "integer"},
			"task_key": map[string]any{"type": "string"}, "title": map[string]any{"type": "string"},
			"children": map[string]any{"type": "array"},
		}
		required = []string{"id", "workspace_id", "task_key", "title", "children"}
	case "project.set.list", "project.plan_unit.list", "project.requirement.list", "project.set.audit.list",
		"project.delivery_label.list", "workflow.template.list", "workflow.instance.list",
		"task.list", "task.comment.list", "task.activity.list", "task.attachment.list",
		"task.git_link.list", "task.related.list", "task.due_conflict.list",
		"quality.bug.list", "quality.bug.activity.list", "user.search", "team.member.list":
		properties = map[string]any{"items": map[string]any{"type": "array"}, "total": map[string]any{"type": "integer", "minimum": 0}}
		required = []string{"items", "total"}
	case "project.board.get":
		properties = map[string]any{"project": map[string]any{"type": "object"}}
		required = []string{"project"}
	case "project.scope_options.list":
		properties = map[string]any{
			"project_sets": map[string]any{"type": "array"}, "projects": map[string]any{"type": "array"},
			"requirements": map[string]any{"type": "array"},
		}
		required = []string{"project_sets", "projects", "requirements"}
	case "workbench.todo.summary":
		properties = map[string]any{
			"summary": map[string]any{"type": "object"}, "generated_at": map[string]any{"type": "string", "format": "date-time"},
		}
		required = []string{"summary", "generated_at"}
	case "workbench.todo.feed":
		properties = map[string]any{"items": map[string]any{"type": "array"}, "total": map[string]any{"type": "integer", "minimum": 0}}
		required = []string{"items", "total"}
	case "project.participant.list":
		properties = map[string]any{
			"project_set": map[string]any{"type": "object"}, "participants": map[string]any{"type": "array"},
			"total": pmCountOutputSchema(), "authorization": map[string]any{"type": "object"},
		}
		required = []string{"project_set", "participants", "total", "authorization"}
	case "quality.bug.get":
		properties = map[string]any{
			"id": map[string]any{"type": "integer", "minimum": 1}, "workspace_id": map[string]any{"type": "integer", "minimum": 1},
			"bug_key": map[string]any{"type": "string"}, "title": map[string]any{"type": "string"},
			"status": map[string]any{"type": "string"}, "priority": map[string]any{"type": "string"},
			"defect_domain": map[string]any{"type": "string", "enum": []string{"functional", "ui"}},
		}
		required = []string{"id", "workspace_id", "bug_key", "title", "status", "priority", "defect_domain"}
	case "task.stats.overview":
		countMap := map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer", "minimum": 0}}
		properties = map[string]any{
			"total": pmCountOutputSchema(), "completed": pmCountOutputSchema(), "incomplete": pmCountOutputSchema(), "overdue": pmCountOutputSchema(),
			"due_soon": pmCountOutputSchema(), "due_soon_days": pmCountOutputSchema(), "by_kind": countMap,
			"by_kind_completed": countMap, "by_status": countMap,
		}
		required = []string{"total", "completed", "incomplete", "overdue", "due_soon", "due_soon_days", "by_kind", "by_kind_completed", "by_status"}
	case "task.stats.delivery":
		countMap := map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer", "minimum": 0}}
		properties = map[string]any{
			"total": pmCountOutputSchema(), "completed": pmCountOutputSchema(), "open": pmCountOutputSchema(), "overdue": pmCountOutputSchema(),
			"completion_rate": map[string]any{"type": "number", "minimum": 0}, "by_status": countMap, "by_priority": countMap,
		}
		required = []string{"total", "completed", "open", "overdue", "completion_rate", "by_status", "by_priority"}
	case "task.stats.due_completion":
		properties = map[string]any{
			"total": pmCountOutputSchema(), "overdue": pmCountOutputSchema(), "due_today": pmCountOutputSchema(), "due_soon": pmCountOutputSchema(),
			"later": pmCountOutputSchema(), "open_without_due": pmCountOutputSchema(), "completed_on_time": pmCountOutputSchema(),
			"completed_late": pmCountOutputSchema(), "completed_without_due": pmCountOutputSchema(),
			"as_of": map[string]any{"type": "string"}, "timezone": map[string]any{"type": "string"},
		}
		required = []string{"total", "overdue", "due_today", "due_soon", "later", "open_without_due", "completed_on_time", "completed_late", "completed_without_due", "as_of", "timezone"}
	case "task.stats.assignee_status":
		properties = map[string]any{
			"items": map[string]any{"type": "array"}, "total_assignees": pmCountOutputSchema(),
		}
		required = []string{"items", "total_assignees"}
	case "project.access.effective":
		properties = map[string]any{
			"effective": map[string]any{"type": "boolean"},
			"actor": map[string]any{
				"type": "object", "properties": map[string]any{"user_id": map[string]any{"type": "integer"}},
				"required": []string{"user_id"}, "additionalProperties": false,
			},
			"workspace": map[string]any{
				"type": "object", "properties": map[string]any{
					"id": map[string]any{"type": "integer"}, "effective": map[string]any{"type": "boolean"},
				}, "required": []string{"id", "effective"}, "additionalProperties": false,
			},
			"project_set": map[string]any{
				"type": "object", "properties": map[string]any{
					"id": map[string]any{"type": "integer"}, "code": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"},
					"owner_user_id":  map[string]any{"anyOf": []any{map[string]any{"type": "integer"}, map[string]any{"type": "null"}}},
					"actor_is_owner": map[string]any{"type": "boolean"},
				}, "required": []string{"id", "name", "owner_user_id", "actor_is_owner"}, "additionalProperties": false,
			},
			"access_basis": map[string]any{
				"type": "object", "properties": map[string]any{
					"model": map[string]any{"type": "string"}, "workspace_is_data_context": map[string]any{"type": "boolean"},
					"project_owner_is_informational": map[string]any{"type": "boolean"}, "legacy_project_permission_used": map[string]any{"type": "boolean"},
				}, "required": []string{"model", "workspace_is_data_context", "project_owner_is_informational", "legacy_project_permission_used"}, "additionalProperties": false,
			},
		}
		required = []string{"effective", "actor", "workspace", "project_set", "access_basis"}
	case "workflow.template.get":
		properties = map[string]any{
			"id": map[string]any{"type": "integer"}, "workspace_id": map[string]any{"type": "integer"},
			"definition": map[string]any{"type": "object"},
		}
		required = []string{"id", "workspace_id", "definition"}
	case "workflow.instance.get":
		properties = map[string]any{
			"id": map[string]any{"type": "integer"}, "workspace_id": map[string]any{"type": "integer"},
			"current_node_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"node_plans":       map[string]any{"type": "object"},
		}
		required = []string{"id", "workspace_id", "current_node_ids", "node_plans"}
	case "workflow.opinion.list":
		properties = pmWorkflowAuxListOutputProperties(map[string]any{
			"type": "object", "properties": map[string]any{
				"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(),
				"instance_id": pmPositiveOutputSchema(), "node_id": map[string]any{"type": "string", "minLength": 1},
				"author_user_id": pmPositiveOutputSchema(), "body": map[string]any{"type": "string", "minLength": 1},
				"created_at": map[string]any{"type": "string"}, "updated_at": map[string]any{"type": "string"},
				"edited": map[string]any{"type": "boolean"},
			}, "required": []string{"id", "workspace_id", "instance_id", "node_id", "author_user_id", "body", "created_at", "updated_at", "edited"},
			"additionalProperties": false,
		})
		required = pmWorkflowAuxListOutputRequired()
		additionalProperties = false
	case "workflow.test_case_artifact.list":
		properties = pmWorkflowAuxListOutputProperties(map[string]any{
			"type": "object", "properties": map[string]any{
				"id": pmPositiveOutputSchema(), "requirement_id": pmPositiveOutputSchema(),
				"instance_id": pmPositiveOutputSchema(), "node_id": map[string]any{"type": "string", "minLength": 1},
				"kind":   map[string]any{"type": "string", "enum": []string{"uploaded", "ai_generated"}},
				"source": map[string]any{"type": "string"}, "name": map[string]any{"type": "string", "minLength": 1},
				"content_type": map[string]any{"type": "string"}, "size": pmCountOutputSchema(),
				"url": map[string]any{"type": "string"}, "origin_url": map[string]any{"type": "string"},
				"ai_run_id":        map[string]any{"anyOf": []any{pmPositiveOutputSchema(), map[string]any{"type": "null"}}},
				"uploader_user_id": pmPositiveOutputSchema(), "created_at": map[string]any{"type": "string"},
			}, "required": []string{"id", "requirement_id", "instance_id", "node_id", "kind", "source", "name", "content_type", "size", "url", "origin_url", "ai_run_id", "uploader_user_id", "created_at"},
			"additionalProperties": false,
		})
		required = pmWorkflowAuxListOutputRequired()
		additionalProperties = false
	case "workflow.test_report_artifact.list":
		properties = pmWorkflowAuxListOutputProperties(map[string]any{
			"type": "object", "properties": map[string]any{
				"id": pmPositiveOutputSchema(), "plan_unit_id": pmPositiveOutputSchema(), "project_new_id": pmPositiveOutputSchema(),
				"instance_id": pmPositiveOutputSchema(), "node_id": map[string]any{"type": "string", "minLength": 1},
				"kind":   map[string]any{"type": "string", "const": "test_report"},
				"source": map[string]any{"type": "string", "enum": []string{"file", "link"}},
				"name":   map[string]any{"type": "string", "minLength": 1}, "content_type": map[string]any{"type": "string"},
				"size": pmCountOutputSchema(), "url": map[string]any{"type": "string"},
				"uploader_user_id": pmPositiveOutputSchema(), "created_at": map[string]any{"type": "string"},
			}, "required": []string{"id", "plan_unit_id", "project_new_id", "instance_id", "node_id", "kind", "source", "name", "content_type", "size", "url", "uploader_user_id", "created_at"},
			"additionalProperties": false,
		})
		required = pmWorkflowAuxListOutputRequired()
		additionalProperties = false
	case "workflow.action.execute":
		properties = map[string]any{
			"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(),
			"instance_id": pmPositiveOutputSchema(), "node_id": map[string]any{"type": "string", "minLength": 1},
			"status":           map[string]any{"type": "string", "enum": []string{"running", "completed", "rejected"}},
			"action":           map[string]any{"type": "string"},
			"current_node_ids": map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}},
		}
		required = []string{"id", "workspace_id", "instance_id", "node_id", "status", "action", "current_node_ids"}
		additionalProperties = false
	case "workflow.node_plan.update":
		properties = map[string]any{
			"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(),
			"instance_id": pmPositiveOutputSchema(), "status": map[string]any{"type": "string"},
			"node_plans": map[string]any{"type": "object"},
		}
		required = []string{"id", "workspace_id", "instance_id", "status", "node_plans"}
		additionalProperties = false
	case "workflow.opinion.create":
		properties = map[string]any{
			"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(),
			"instance_id": pmPositiveOutputSchema(), "node_id": map[string]any{"type": "string", "minLength": 1},
			"opinion":          pmWorkflowOpinionOutputSchema(),
			"invited_user_ids": map[string]any{"type": "array", "items": pmPositiveOutputSchema()},
		}
		required = []string{"id", "workspace_id", "instance_id", "node_id"}
		additionalProperties = false
	case "workflow.opinion.update":
		properties = map[string]any{
			"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(),
			"instance_id": pmPositiveOutputSchema(), "node_id": map[string]any{"type": "string", "minLength": 1},
			"opinion": pmWorkflowOpinionOutputSchema(),
		}
		required = []string{"id", "workspace_id", "instance_id", "node_id", "opinion"}
		additionalProperties = false
	case "workflow.test_report_link.create":
		properties = map[string]any{
			"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(),
			"instance_id": pmPositiveOutputSchema(), "node_id": map[string]any{"type": "string", "minLength": 1},
			"test_report": map[string]any{
				"type": "object", "properties": map[string]any{
					"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(), "plan_unit_id": pmPositiveOutputSchema(),
					"project_set_id": pmPositiveOutputSchema(), "kind": map[string]any{"type": "string", "const": "link"},
					"name": map[string]any{"type": "string", "minLength": 1}, "url": map[string]any{"type": "string", "minLength": 1},
					"uploader_user_id": pmPositiveOutputSchema(), "created_at": map[string]any{"type": "string"},
				}, "required": []string{"id", "workspace_id", "plan_unit_id", "project_set_id", "kind", "name", "url", "uploader_user_id", "created_at"},
				"additionalProperties": false,
			},
		}
		required = []string{"id", "workspace_id", "instance_id", "node_id", "test_report"}
		additionalProperties = false
	case "task.testing.definition.get":
		properties = map[string]any{
			"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(), "work_task_id": pmPositiveOutputSchema(),
			"version": pmPositiveOutputSchema(), "purpose": map[string]any{"type": "string"}, "preconditions": map[string]any{"type": "string"},
			"environment_guidance": map[string]any{"type": "string"}, "cases": map[string]any{"type": "array"},
			"created_by_user_id": pmPositiveOutputSchema(), "updated_by_user_id": pmPositiveOutputSchema(),
			"created_at": map[string]any{"type": "string"}, "updated_at": map[string]any{"type": "string"},
			"task": map[string]any{"type": "object"},
		}
		required = []string{"id", "workspace_id", "work_task_id", "version", "cases", "created_by_user_id", "updated_by_user_id", "created_at", "updated_at", "task"}
		additionalProperties = false
	case "task.testing.definition.update":
		definition := pmTaskTestingDefinitionOutputSchema()
		properties, _ = definition["properties"].(map[string]any)
		required, _ = definition["required"].([]string)
		additionalProperties = false
	case "task.testing.execution.create":
		properties = map[string]any{
			"execution": pmTaskTestingExecutionOutputSchema(),
			"runs":      map[string]any{"type": "array", "minItems": 1, "items": pmTaskTestingRunOutputSchema()},
		}
		required = []string{"execution", "runs"}
		additionalProperties = false
	case "task.testing.execution_run.update":
		properties = map[string]any{
			"run": pmTaskTestingRunOutputSchema(), "execution": pmTaskTestingExecutionOutputSchema(),
		}
		required = []string{"run", "execution"}
		additionalProperties = false
	case "task.comment.create":
		properties = map[string]any{
			"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(), "work_task_id": pmPositiveOutputSchema(),
			"user_id": pmPositiveOutputSchema(), "body": map[string]any{"type": "string", "minLength": 1, "maxLength": 20000},
			"created_at": map[string]any{"type": "string", "format": "date-time"},
		}
		required = []string{"id", "workspace_id", "work_task_id", "user_id", "body", "created_at"}
		additionalProperties = false
	case "task.comment.delete", "task.attachment.delete":
		properties = map[string]any{"id": pmPositiveOutputSchema(), "deleted": map[string]any{"type": "boolean", "const": true}}
		required = []string{"id", "deleted"}
		additionalProperties = false
	case "task.watch.update":
		properties = map[string]any{"watched": map[string]any{"type": "boolean"}, "work_task_id": pmPositiveOutputSchema()}
		required = []string{"watched", "work_task_id"}
		additionalProperties = false
	case "task.testing.context.get":
		properties = map[string]any{
			"task": map[string]any{"type": "object"}, "project": map[string]any{"type": "object"},
			"requirement": map[string]any{"type": "object"},
			"definition":  map[string]any{"anyOf": []any{map[string]any{"type": "object"}, map[string]any{"type": "null"}}},
			"executions":  map[string]any{"type": "object"},
		}
		required = []string{"task", "project", "requirement", "definition", "executions"}
		additionalProperties = false
	case "task.testing.execution_run.list":
		properties = map[string]any{
			"task": map[string]any{"type": "object"}, "execution": map[string]any{"type": "object"},
			"items": map[string]any{"type": "array"}, "total": pmCountOutputSchema(),
		}
		required = []string{"task", "execution", "items", "total"}
		additionalProperties = false
	case "task.client_option.list":
		properties = map[string]any{
			"items": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": pmPositiveOutputSchema(), "name": map[string]any{"type": "string", "minLength": 1},
						"usage_count": pmCountOutputSchema(),
					},
					"required": []string{"id", "name", "usage_count"}, "additionalProperties": false,
				},
			},
			"total": pmCountOutputSchema(),
		}
		required = []string{"items", "total"}
		additionalProperties = false
	case "task.workflow.kind.list", "task.workflow.status.list":
		properties = map[string]any{"items": map[string]any{"type": "array"}, "total": pmCountOutputSchema()}
		required = []string{"items", "total"}
		additionalProperties = false
	case "task.workflow.transition.list":
		properties = map[string]any{
			"enabled": map[string]any{"type": "boolean"}, "enforce": map[string]any{"type": "boolean"},
			"standard": map[string]any{"type": "boolean"}, "workflow_profile": map[string]any{"type": "string"},
			"transitions": map[string]any{"type": "array"},
		}
		required = []string{"enabled", "enforce", "standard", "workflow_profile", "transitions"}
		additionalProperties = false
	}
	schema := map[string]any{"type": "object", "properties": properties, "additionalProperties": additionalProperties}
	if len(required) > 0 {
		schema["required"] = required
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		panic(err)
	}
	return encoded
}

func pmRequirementMutationOutputSchema() map[string]any {
	nullable := func(value map[string]any) map[string]any {
		return map[string]any{"anyOf": []any{value, map[string]any{"type": "null"}}}
	}
	properties := map[string]any{
		"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(), "team_id": pmPositiveOutputSchema(),
		"project_new_id": pmPositiveOutputSchema(), "version_task_new_id": pmPositiveOutputSchema(), "version": pmPositiveOutputSchema(),
		"workflow_instance_id": nullable(pmPositiveOutputSchema()), "title": map[string]any{"type": "string", "minLength": 1},
		"description": map[string]any{"type": "string"}, "product_doc_url": map[string]any{"type": "string"},
		"design_url": map[string]any{"type": "string"}, "acceptance_criteria": map[string]any{"type": "string"},
		"priority": map[string]any{"type": "string", "enum": []string{"p0", "p1", "p2", "p3"}}, "status": map[string]any{"type": "string", "minLength": 1},
		"product_owner_user_id": nullable(pmPositiveOutputSchema()), "design_owner_user_id": nullable(pmPositiveOutputSchema()),
		"developer_user_ids": map[string]any{"type": "array", "items": pmPositiveOutputSchema()}, "deleted": map[string]any{"type": "boolean"},
	}
	required := []string{
		"id", "workspace_id", "team_id", "project_new_id", "version_task_new_id", "version", "workflow_instance_id", "title",
		"description", "product_doc_url", "design_url", "acceptance_criteria", "priority", "status", "product_owner_user_id",
		"design_owner_user_id", "developer_user_ids", "deleted",
	}
	return map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}
}

func pmQualityBugOutputSchema() map[string]any {
	nullablePositive := map[string]any{"anyOf": []any{pmPositiveOutputSchema(), map[string]any{"type": "null"}}}
	nullableTime := map[string]any{"anyOf": []any{map[string]any{"type": "string", "format": "date-time"}, map[string]any{"type": "null"}}}
	properties := map[string]any{
		"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(), "team_id": pmPositiveOutputSchema(),
		"project_set_id": pmPositiveOutputSchema(), "project_new_id": pmPositiveOutputSchema(), "requirement_id": pmPositiveOutputSchema(),
		"bug_key": map[string]any{"type": "string", "minLength": 1}, "title": map[string]any{"type": "string", "minLength": 1, "maxLength": 160},
		"status": map[string]any{"type": "string"}, "priority": map[string]any{"type": "string"},
		"bug_type": map[string]any{"type": "string"}, "defect_domain": map[string]any{"type": "string", "enum": []string{"functional", "ui"}},
		"severity_key": map[string]any{"type": "string"}, "due_at": nullableTime,
		"found_environment": map[string]any{"type": "string"}, "operating_system": map[string]any{"type": "string"},
		"browser": map[string]any{"type": "string"}, "found_version": map[string]any{"type": "string"},
		"reproduction_materials": pmQualityMaterials("", "").schema, "resolution_materials": pmQualityMaterials("", "").schema,
		"reproduction_steps": map[string]any{"type": "string"}, "expected_result": map[string]any{"type": "string"},
		"actual_result": map[string]any{"type": "string"}, "resolution_reason": map[string]any{"type": "string"},
		"resolution_note": map[string]any{"type": "string"}, "assignee_user_id": nullablePositive,
		"reporter_user_id": nullablePositive, "reviewer_user_id": nullablePositive,
		"source_execution_id": nullablePositive, "source_test_id": nullablePositive,
		"resolved_at": nullableTime, "reviewed_at": nullableTime, "completed_at": nullableTime, "canceled_at": nullableTime,
		"status_changed_at": map[string]any{"type": "string", "format": "date-time"},
		"created_at":        map[string]any{"type": "string", "format": "date-time"}, "updated_at": map[string]any{"type": "string", "format": "date-time"},
	}
	return map[string]any{
		"type": "object", "properties": properties,
		"required":             []string{"id", "workspace_id", "team_id", "project_set_id", "project_new_id", "requirement_id", "bug_key", "title", "status", "priority", "defect_domain", "status_changed_at", "created_at", "updated_at"},
		"additionalProperties": false,
	}
}

func pmTaskMutationSnapshotOutputSchema() map[string]any {
	reference := map[string]any{
		"type": "object", "properties": map[string]any{
			"id": pmPositiveOutputSchema(), "task_key": map[string]any{"type": "string", "minLength": 1},
			"title": map[string]any{"type": "string", "minLength": 1},
		}, "required": []string{"id", "task_key", "title"}, "additionalProperties": false,
	}
	properties := map[string]any{
		"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(), "team_id": pmPositiveOutputSchema(),
		"project_set_id": pmPositiveOutputSchema(), "project_new_id": pmPositiveOutputSchema(), "requirement_id": pmPositiveOutputSchema(),
		"kind":     map[string]any{"type": "string", "enum": []string{"task", "subtask"}},
		"task_key": map[string]any{"type": "string", "minLength": 1}, "title": map[string]any{"type": "string", "minLength": 1},
		"description": map[string]any{"type": "string"}, "status": map[string]any{"type": "string"}, "priority": map[string]any{"type": "string"},
		"assignee_id": pmPositiveOutputSchema(), "reporter_user_id": pmPositiveOutputSchema(), "parent_id": pmPositiveOutputSchema(),
		"estimated_minutes": pmCountOutputSchema(), "starts_at": map[string]any{"type": "string", "format": "date-time"},
		"due_at": map[string]any{"type": "string", "format": "date-time"}, "completed_at": map[string]any{"type": "string", "format": "date-time"},
		"created_at": map[string]any{"type": "string", "format": "date-time"}, "updated_at": map[string]any{"type": "string", "format": "date-time"},
		"parent": reference, "children": map[string]any{"type": "array", "items": reference},
	}
	return map[string]any{
		"type": "object", "properties": properties,
		"required": []string{
			"id", "workspace_id", "team_id", "project_set_id", "project_new_id", "requirement_id", "kind", "task_key", "title",
			"status", "priority", "estimated_minutes", "created_at", "updated_at", "children",
		},
		"additionalProperties": false,
	}
}

func pmBatchCreatedTaskOutputSchema(withChildren bool) map[string]any {
	properties := map[string]any{
		"client_id": map[string]any{"type": "string", "minLength": 1}, "id": pmPositiveOutputSchema(),
		"task_key": map[string]any{"type": "string", "minLength": 1},
	}
	if withChildren {
		properties["children"] = map[string]any{"type": "array", "items": pmBatchCreatedTaskOutputSchema(false)}
	}
	return map[string]any{
		"type": "object", "properties": properties, "required": []string{"client_id", "id", "task_key"},
		"additionalProperties": false,
	}
}

func pmPositiveIDListOutputSchema() map[string]any {
	return map[string]any{"type": "array", "items": pmPositiveOutputSchema()}
}

func pmTaskTestingDefinitionOutputSchema() map[string]any {
	step := map[string]any{"type": "object", "properties": map[string]any{
		"action": map[string]any{"type": "string", "minLength": 1}, "expected_result": map[string]any{"type": "string"},
	}, "required": []string{"action"}, "additionalProperties": false}
	testCase := map[string]any{"type": "object", "properties": map[string]any{
		"key": map[string]any{"type": "string", "minLength": 1}, "title": map[string]any{"type": "string", "minLength": 1},
		"steps": map[string]any{"type": "array", "minItems": 1, "items": step}, "expected_result": map[string]any{"type": "string"},
	}, "required": []string{"key", "title", "steps"}, "additionalProperties": false}
	properties := map[string]any{
		"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(), "work_task_id": pmPositiveOutputSchema(),
		"version": pmPositiveOutputSchema(), "purpose": map[string]any{"type": "string"},
		"preconditions": map[string]any{"type": "string"}, "environment_guidance": map[string]any{"type": "string"},
		"cases":              map[string]any{"type": "array", "minItems": 1, "items": testCase},
		"created_by_user_id": pmPositiveOutputSchema(), "updated_by_user_id": pmPositiveOutputSchema(),
		"created_at": map[string]any{"type": "string", "format": "date-time"}, "updated_at": map[string]any{"type": "string", "format": "date-time"},
	}
	return map[string]any{"type": "object", "properties": properties, "required": []string{
		"id", "workspace_id", "work_task_id", "version", "cases", "created_by_user_id", "updated_by_user_id", "created_at", "updated_at",
	}, "additionalProperties": false}
}

func pmTaskTestingExecutionOutputSchema() map[string]any {
	nullableTime := map[string]any{"anyOf": []any{map[string]any{"type": "string", "format": "date-time"}, map[string]any{"type": "null"}}}
	properties := map[string]any{
		"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(), "work_task_id": pmPositiveOutputSchema(),
		"task_test_definition_id": pmPositiveOutputSchema(), "definition_version": pmPositiveOutputSchema(),
		"environment": map[string]any{"type": "string", "minLength": 1}, "build_version": map[string]any{"type": "string"},
		"executor_user_id": pmPositiveOutputSchema(), "status": map[string]any{"type": "string", "enum": []string{"running", "passed", "failed", "blocked"}}, "summary": map[string]any{"type": "string"},
		"run_counts": map[string]any{"type": "object", "additionalProperties": pmCountOutputSchema()},
		"started_at": nullableTime, "finished_at": nullableTime,
		"created_at": map[string]any{"type": "string", "format": "date-time"}, "updated_at": map[string]any{"type": "string", "format": "date-time"},
	}
	return map[string]any{"type": "object", "properties": properties, "required": []string{
		"id", "workspace_id", "work_task_id", "task_test_definition_id", "definition_version", "environment",
		"executor_user_id", "status", "run_counts", "created_at", "updated_at",
	}, "additionalProperties": false}
}

func pmTaskTestingRunOutputSchema() map[string]any {
	nullablePositive := map[string]any{"anyOf": []any{pmPositiveOutputSchema(), map[string]any{"type": "null"}}}
	nullableTime := map[string]any{"anyOf": []any{map[string]any{"type": "string", "format": "date-time"}, map[string]any{"type": "null"}}}
	step := map[string]any{"type": "object", "properties": map[string]any{
		"action": map[string]any{"type": "string", "minLength": 1}, "expected_result": map[string]any{"type": "string"},
	}, "required": []string{"action"}, "additionalProperties": false}
	stepResult := map[string]any{"type": "object", "properties": map[string]any{
		"sequence": pmPositiveOutputSchema(), "status": map[string]any{"type": "string", "enum": []string{"passed", "failed", "blocked", "skipped"}},
		"actual_result": map[string]any{"type": "string"}, "failure_reason": map[string]any{"type": "string"},
	}, "required": []string{"sequence", "status"}, "additionalProperties": false}
	evidence := map[string]any{"type": "object", "properties": map[string]any{
		"kind":      map[string]any{"type": "string", "enum": []string{"url", "attachment", "note"}},
		"reference": map[string]any{"type": "string", "minLength": 1}, "label": map[string]any{"type": "string"},
	}, "required": []string{"kind", "reference"}, "additionalProperties": false}
	properties := map[string]any{
		"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(), "work_task_id": pmPositiveOutputSchema(),
		"task_test_execution_id": pmPositiveOutputSchema(), "version": pmPositiveOutputSchema(), "sequence": pmPositiveOutputSchema(),
		"case_key": map[string]any{"type": "string", "minLength": 1}, "title": map[string]any{"type": "string", "minLength": 1},
		"steps": map[string]any{"type": "array", "minItems": 1, "items": step}, "step_results": map[string]any{"type": "array", "items": stepResult},
		"expected_result": map[string]any{"type": "string"}, "status": map[string]any{"type": "string", "enum": []string{"pending", "running", "passed", "failed", "blocked", "skipped"}},
		"actual_result": map[string]any{"type": "string"}, "failure_reason": map[string]any{"type": "string"},
		"evidence": map[string]any{"type": "array", "items": evidence}, "quality_bug_id": nullablePositive, "executed_at": nullableTime,
		"created_at": map[string]any{"type": "string", "format": "date-time"}, "updated_at": map[string]any{"type": "string", "format": "date-time"},
	}
	return map[string]any{"type": "object", "properties": properties, "required": []string{
		"id", "workspace_id", "work_task_id", "task_test_execution_id", "version", "sequence", "case_key", "title",
		"steps", "step_results", "status", "evidence", "created_at", "updated_at",
	}, "additionalProperties": false}
}

func pmCountOutputSchema() map[string]any {
	return map[string]any{"type": "integer", "minimum": 0}
}

func pmPositiveOutputSchema() map[string]any {
	return map[string]any{"type": "integer", "minimum": 1}
}

func pmWorkflowAuxListOutputProperties(itemSchema map[string]any) map[string]any {
	return map[string]any{
		"instance_id": pmPositiveOutputSchema(), "node_id": map[string]any{"type": "string", "minLength": 1},
		"subject_type": map[string]any{"type": "string", "enum": []string{"plan_unit", "requirement"}},
		"subject_id":   pmPositiveOutputSchema(),
		"items":        map[string]any{"type": "array", "items": itemSchema}, "total": pmCountOutputSchema(),
	}
}

func pmWorkflowAuxListOutputRequired() []string {
	return []string{"instance_id", "node_id", "subject_type", "subject_id", "items", "total"}
}

func pmWorkflowOpinionOutputSchema() map[string]any {
	return map[string]any{
		"type": "object", "properties": map[string]any{
			"id": pmPositiveOutputSchema(), "workspace_id": pmPositiveOutputSchema(),
			"instance_id": pmPositiveOutputSchema(), "node_id": map[string]any{"type": "string", "minLength": 1},
			"author_user_id": pmPositiveOutputSchema(), "body": map[string]any{"type": "string", "minLength": 1},
			"created_at": map[string]any{"type": "string"}, "updated_at": map[string]any{"type": "string"},
			"edited": map[string]any{"type": "boolean"},
		},
		"required": []string{
			"id", "workspace_id", "instance_id", "node_id", "author_user_id", "body", "created_at", "updated_at", "edited",
		},
		"additionalProperties": false,
	}
}
