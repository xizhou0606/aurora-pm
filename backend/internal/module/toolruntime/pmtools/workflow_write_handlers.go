package pmtools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
)

const (
	WorkflowActionExecute        = "workflow.action.execute"
	WorkflowNodePlanUpdate       = "workflow.node_plan.update"
	WorkflowOpinionCreate        = "workflow.opinion.create"
	WorkflowOpinionUpdate        = "workflow.opinion.update"
	WorkflowTestReportLinkCreate = "workflow.test_report_link.create"
)

type WorkflowMutationService interface {
	ApplyWorkflowMutation(context.Context, workflow.Mutation) (workflow.MutationResult, error)
}

type workflowWriteHandler struct {
	service   WorkflowMutationService
	operation workflow.MutationOperation
}

func WorkflowWriteHandlerNames() []string {
	return []string{
		WorkflowActionExecute, WorkflowNodePlanUpdate, WorkflowOpinionCreate, WorkflowOpinionUpdate, WorkflowTestReportLinkCreate,
	}
}

func NewWorkflowWriteHandlers(service WorkflowMutationService) (map[string]toolruntime.ToolHandler, error) {
	if service == nil {
		return nil, fmt.Errorf("workflow mutation service is required")
	}
	return map[string]toolruntime.ToolHandler{
		WorkflowActionExecute:        workflowWriteHandler{service: service, operation: workflow.ActionExecute},
		WorkflowNodePlanUpdate:       workflowWriteHandler{service: service, operation: workflow.NodePlanUpdate},
		WorkflowOpinionCreate:        workflowWriteHandler{service: service, operation: workflow.OpinionCreate},
		WorkflowOpinionUpdate:        workflowWriteHandler{service: service, operation: workflow.OpinionUpdate},
		WorkflowTestReportLinkCreate: workflowWriteHandler{service: service, operation: workflow.TestReportLinkCreate},
	}, nil
}

func (handler workflowWriteHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}

func (handler workflowWriteHandler) Apply(ctx context.Context, call toolruntime.MutationCall) (json.RawMessage, error) {
	if handler.service == nil || call.Authorization.WorkspaceID == 0 || call.Authorization.PrincipalID == 0 ||
		call.ChangeSetID == "" || call.ApplyKey == "" {
		return nil, workflow.ErrMutationInvalid
	}
	command, err := decodeWorkflowMutation(call.Input, handler.operation)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(call.Input)
	command.WorkspaceID, command.PrincipalID = call.Authorization.WorkspaceID, call.Authorization.PrincipalID
	command.ApplyKey, command.InputDigest = call.ApplyKey, hex.EncodeToString(digest[:])
	result, err := handler.service.ApplyWorkflowMutation(ctx, command)
	if err != nil {
		if !errors.Is(err, workflow.ErrMutationInvalid) && !errors.Is(err, workflow.ErrMutationNotFound) &&
			!errors.Is(err, workflow.ErrMutationConflict) && !errors.Is(err, workflow.ErrMutationUnavailable) &&
			!errors.Is(err, workflow.ErrRequirementLockConfirmationRequired) &&
			!errors.Is(err, workflow.ErrRequirementsGuardBlocked) {
			return nil, toolruntime.RetryableApplyError(err)
		}
		return nil, err
	}
	return json.Marshal(result)
}

func decodeWorkflowMutation(raw json.RawMessage, operation workflow.MutationOperation) (workflow.Mutation, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return workflow.Mutation{}, workflow.ErrMutationInvalid
	}
	command := workflow.Mutation{Operation: operation}
	if !decodeWorkflowUint(fields, "id", &command.InstanceID) || command.InstanceID == 0 {
		return workflow.Mutation{}, workflow.ErrMutationInvalid
	}
	_ = decodeWorkflowString(fields, "nodeId", &command.NodeID)
	switch operation {
	case workflow.ActionExecute:
		if !decodeWorkflowString(fields, "action", &command.Action) {
			return workflow.Mutation{}, workflow.ErrMutationInvalid
		}
		_ = decodeWorkflowString(fields, "node_id", &command.NodeID)
		_ = decodeWorkflowString(fields, "target_node_id", &command.TargetNodeID)
		_ = decodeWorkflowString(fields, "comment", &command.Comment)
		_ = decodeWorkflowString(fields, "design_url", &command.DesignURL)
		if rawValue, exists := fields["confirm_requirement_lock"]; exists && json.Unmarshal(rawValue, &command.ConfirmRequirementLock) != nil {
			return workflow.Mutation{}, workflow.ErrMutationInvalid
		}
		if rawValue, exists := fields["next_node_assignees"]; exists {
			var values []struct {
				NodeID         string `json:"node_id"`
				AssigneeUserID uint   `json:"assignee_user_id"`
			}
			if json.Unmarshal(rawValue, &values) != nil {
				return workflow.Mutation{}, workflow.ErrMutationInvalid
			}
			command.NextAssignees = map[string]uint{}
			for _, value := range values {
				value.NodeID = strings.TrimSpace(value.NodeID)
				if value.NodeID == "" || value.AssigneeUserID == 0 || command.NextAssignees[value.NodeID] != 0 {
					return workflow.Mutation{}, workflow.ErrMutationInvalid
				}
				command.NextAssignees[value.NodeID] = value.AssigneeUserID
			}
		}
	case workflow.NodePlanUpdate:
		if rawValue, exists := fields["confirm_subject_schedule"]; exists && json.Unmarshal(rawValue, &command.ConfirmSchedule) != nil {
			return workflow.Mutation{}, workflow.ErrMutationInvalid
		}
		rawNodes, exists := fields["nodes"]
		var nodeFields []map[string]json.RawMessage
		if !exists || json.Unmarshal(rawNodes, &nodeFields) != nil {
			return workflow.Mutation{}, workflow.ErrMutationInvalid
		}
		command.NodePlans = make([]workflow.NodePlanMutation, 0, len(nodeFields))
		for _, value := range nodeFields {
			node, err := decodeWorkflowNodePlan(value)
			if err != nil {
				return workflow.Mutation{}, err
			}
			command.NodePlans = append(command.NodePlans, node)
		}
	case workflow.OpinionCreate:
		_ = decodeWorkflowString(fields, "body", &command.Body)
		if rawValue, exists := fields["invite_user_ids"]; exists && json.Unmarshal(rawValue, &command.InviteUserIDs) != nil {
			return workflow.Mutation{}, workflow.ErrMutationInvalid
		}
	case workflow.OpinionUpdate:
		if !decodeWorkflowUint(fields, "opinionId", &command.OpinionID) || command.OpinionID == 0 ||
			!decodeWorkflowString(fields, "body", &command.Body) {
			return workflow.Mutation{}, workflow.ErrMutationInvalid
		}
	case workflow.TestReportLinkCreate:
		if !decodeWorkflowString(fields, "url", &command.URL) {
			return workflow.Mutation{}, workflow.ErrMutationInvalid
		}
	default:
		return workflow.Mutation{}, workflow.ErrMutationInvalid
	}
	return command, nil
}

func decodeWorkflowNodePlan(fields map[string]json.RawMessage) (workflow.NodePlanMutation, error) {
	var result workflow.NodePlanMutation
	if !decodeWorkflowString(fields, "node_id", &result.NodeID) {
		return workflow.NodePlanMutation{}, workflow.ErrMutationInvalid
	}
	var err error
	if raw, exists := fields["planned_start_at"]; exists {
		result.PlannedStartAt.Set = true
		result.PlannedStartAt.Value, err = decodeWorkflowNullableTime(raw)
	}
	if err == nil {
		if raw, exists := fields["planned_end_at"]; exists {
			result.PlannedEndAt.Set = true
			result.PlannedEndAt.Value, err = decodeWorkflowNullableTime(raw)
		}
	}
	if err == nil {
		if raw, exists := fields["expected_planned_start_at"]; exists {
			result.ExpectedStartAt.Set = true
			result.ExpectedStartAt.Value, err = decodeWorkflowNullableTime(raw)
		}
	}
	if err == nil {
		if raw, exists := fields["expected_planned_end_at"]; exists {
			result.ExpectedEndAt.Set = true
			result.ExpectedEndAt.Value, err = decodeWorkflowNullableTime(raw)
		}
	}
	if err != nil {
		return workflow.NodePlanMutation{}, err
	}
	if raw, exists := fields["assignee_user_id"]; exists {
		result.AssigneeUserIDSet = true
		if json.Unmarshal(raw, &result.AssigneeUserID) != nil || result.AssigneeUserID == 0 {
			return workflow.NodePlanMutation{}, workflow.ErrMutationInvalid
		}
	}
	return result, nil
}

func decodeWorkflowNullableTime(raw json.RawMessage) (*time.Time, error) {
	if string(raw) == "null" {
		return nil, nil
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return nil, workflow.ErrMutationInvalid
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, workflow.ErrMutationInvalid
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func decodeWorkflowUint(fields map[string]json.RawMessage, name string, target *uint) bool {
	raw, exists := fields[name]
	return exists && json.Unmarshal(raw, target) == nil
}

func decodeWorkflowString(fields map[string]json.RawMessage, name string, target *string) bool {
	raw, exists := fields[name]
	return exists && json.Unmarshal(raw, target) == nil
}

var _ toolruntime.IdempotentMutationHandler = workflowWriteHandler{}
