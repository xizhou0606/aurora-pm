package pmtools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const (
	ProjectRequirementCreate           = "project.requirement.create"
	ProjectRequirementUpdate           = "project.requirement.update"
	ProjectRequirementRoleOwnersAssign = "project.requirement.role_owners.assign"
	ProjectRequirementDelete           = "project.requirement.delete"
	ProjectRequirementBatchMove        = "project.requirement.batch_move"
)

type RequirementMutationService interface {
	ApplyRequirementMutation(context.Context, requirement.Mutation) (requirement.ApplyResult, error)
}
type requirementWriteHandler struct {
	service   RequirementMutationService
	operation requirement.MutationOperation
}

func RequirementWriteHandlerNames() []string {
	return []string{ProjectRequirementCreate, ProjectRequirementUpdate, ProjectRequirementRoleOwnersAssign, ProjectRequirementDelete, ProjectRequirementBatchMove}
}
func NewRequirementWriteHandlers(service RequirementMutationService) (map[string]toolruntime.ToolHandler, error) {
	if service == nil {
		return nil, fmt.Errorf("requirement mutation service required")
	}
	return map[string]toolruntime.ToolHandler{
		ProjectRequirementCreate:           requirementWriteHandler{service, requirement.MutationCreate},
		ProjectRequirementUpdate:           requirementWriteHandler{service, requirement.MutationUpdate},
		ProjectRequirementRoleOwnersAssign: requirementWriteHandler{service, requirement.MutationUpdate},
		ProjectRequirementDelete:           requirementWriteHandler{service, requirement.MutationDelete},
		ProjectRequirementBatchMove:        requirementWriteHandler{service, requirement.MutationBatchMove},
	}, nil
}
func (handler requirementWriteHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}
func (handler requirementWriteHandler) Apply(ctx context.Context, call toolruntime.MutationCall) (json.RawMessage, error) {
	if handler.service == nil || call.Authorization.WorkspaceID == 0 || call.Authorization.PrincipalID == 0 || call.ApplyKey == "" || call.ChangeSetID == "" {
		return nil, requirement.ErrMutationInvalid
	}
	command, err := decodeRequirementMutation(call.Input, handler.operation)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(call.Input)
	command.WorkspaceID, command.PrincipalID, command.ApplyKey, command.InputDigest = call.Authorization.WorkspaceID, call.Authorization.PrincipalID, call.ApplyKey, hex.EncodeToString(sum[:])
	command.ChangeSetID = call.ChangeSetID
	result, err := handler.service.ApplyRequirementMutation(ctx, command)
	if err != nil {
		return nil, err
	}
	if result.Batch != nil {
		return json.Marshal(result.Batch)
	}
	if result.Requirement == nil {
		return nil, requirement.ErrMutationConflict
	}
	if handler.operation == requirement.MutationDelete {
		return json.Marshal(struct {
			ID      uint `json:"id"`
			Deleted bool `json:"deleted"`
		}{result.Requirement.ID, true})
	}
	return json.Marshal(result.Requirement)
}
func decodeRequirementMutation(input json.RawMessage, operation requirement.MutationOperation) (requirement.Mutation, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(input, &fields) != nil {
		return requirement.Mutation{}, requirement.ErrMutationInvalid
	}
	c := requirement.Mutation{Operation: operation}
	for name, target := range map[string]*uint{"id": &c.ID, "expected_version": &c.ExpectedVersion, "version_task_new_id": &c.PlanUnitID, "workflow_template_id": &c.WorkflowTemplateID} {
		if raw, ok := fields[name]; ok && json.Unmarshal(raw, target) != nil {
			return c, requirement.ErrMutationInvalid
		}
	}
	for name, target := range map[string]*string{"title": &c.Title, "description": &c.Description, "product_doc_url": &c.ProductDocURL, "design_url": &c.DesignURL, "acceptance_criteria": &c.AcceptanceCriteria, "priority": &c.Priority} {
		if raw, ok := fields[name]; ok && json.Unmarshal(raw, target) != nil {
			return c, requirement.ErrMutationInvalid
		}
	}
	_, c.TitleSet = fields["title"]
	_, c.DescriptionSet = fields["description"]
	_, c.ProductDocURLSet = fields["product_doc_url"]
	_, c.DesignURLSet = fields["design_url"]
	_, c.AcceptanceCriteriaSet = fields["acceptance_criteria"]
	_, c.PrioritySet = fields["priority"]
	for name, setting := range map[string]struct {
		set    *bool
		target **uint
	}{"product_owner_user_id": {&c.ProductOwnerSet, &c.ProductOwnerUserID}, "design_owner_user_id": {&c.DesignOwnerSet, &c.DesignOwnerUserID}} {
		if raw, ok := fields[name]; ok {
			*setting.set = true
			if string(raw) != "null" {
				var id uint
				if json.Unmarshal(raw, &id) != nil || id == 0 {
					return c, requirement.ErrMutationInvalid
				}
				*setting.target = &id
			}
		}
	}
	if raw, ok := fields["developer_user_ids"]; ok {
		c.DeveloperUserIDsSet = true
		if json.Unmarshal(raw, &c.DeveloperUserIDs) != nil {
			return c, requirement.ErrMutationInvalid
		}
	}
	for name, setting := range map[string]struct {
		set    *bool
		target *[]uint
	}{
		"product_owner_user_ids":  {&c.ProductOwnerUserIDsSet, &c.ProductOwnerUserIDs},
		"design_owner_user_ids":   {&c.DesignOwnerUserIDsSet, &c.DesignOwnerUserIDs},
		"frontend_owner_user_ids": {&c.FrontendOwnerUserIDsSet, &c.FrontendOwnerUserIDs},
		"backend_owner_user_ids":  {&c.BackendOwnerUserIDsSet, &c.BackendOwnerUserIDs},
		"tester_user_ids":         {&c.TesterUserIDsSet, &c.TesterUserIDs},
	} {
		if raw, ok := fields[name]; ok {
			*setting.set = true
			if json.Unmarshal(raw, setting.target) != nil {
				return c, requirement.ErrMutationInvalid
			}
		}
	}
	if raw, ok := fields["items"]; ok {
		if json.Unmarshal(raw, &c.Items) != nil {
			return c, requirement.ErrMutationInvalid
		}
	}
	if raw, ok := fields["confirm_subject_schedule"]; ok {
		if json.Unmarshal(raw, &c.ConfirmSubjectSchedule) != nil {
			return c, requirement.ErrMutationInvalid
		}
	}
	return c, nil
}

var _ toolruntime.IdempotentMutationHandler = requirementWriteHandler{}
