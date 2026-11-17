package pmtools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const (
	ProjectDeliveryLabelCreate = "project.delivery_label.create"
	ProjectDeliveryLabelUpdate = "project.delivery_label.update"
	ProjectDeliveryLabelDelete = "project.delivery_label.delete"
	ProjectSetCreate           = "project.set.create"
	ProjectSetUpdate           = "project.set.update"
	ProjectSetDelete           = "project.set.delete"
	ProjectSetTransferOwner    = "project.set.transfer_owner"
	ProjectPlanUnitCreate      = "project.plan_unit.create"
	ProjectPlanUnitUpdate      = "project.plan_unit.update"
	ProjectPlanUnitDelete      = "project.plan_unit.delete"
)

type ProjectMutationService interface {
	ApplyDeliveryLabel(context.Context, project.DeliveryLabelMutation) (project.DeliveryLabelMutationResult, error)
}

type ProjectSetMutationService interface {
	ApplyProjectSet(context.Context, project.SetMutation) (project.SetMutationResult, error)
}

type ProjectSetDeletionService interface {
	ApplyProjectSetDeletion(context.Context, project.SetMutation) (project.SetMutationResult, error)
}
type ProjectPlanMutationService interface {
	ApplyPlanUnit(context.Context, project.PlanMutation) (project.PlanMutationResult, error)
}
type ProjectPlanDeletionService interface {
	ApplyPlanUnitDeletion(context.Context, project.PlanMutation) (project.PlanMutationResult, error)
}
type ProjectPlanCreationService interface {
	ApplyPlanUnitCreation(context.Context, project.PlanMutation) (project.PlanMutationResult, error)
}

type projectWriteHandler struct {
	service   ProjectMutationService
	operation project.MutationOperation
}

func ProjectWriteHandlerNames() []string {
	return []string{
		ProjectDeliveryLabelCreate, ProjectDeliveryLabelUpdate, ProjectDeliveryLabelDelete,
		ProjectSetCreate, ProjectSetUpdate, ProjectSetDelete, ProjectSetTransferOwner,
		ProjectPlanUnitCreate, ProjectPlanUnitUpdate, ProjectPlanUnitDelete,
	}
}

func NewProjectWriteHandlers(
	service ProjectMutationService,
	setService ProjectSetMutationService,
	deletionService ProjectSetDeletionService,
	planService ProjectPlanMutationService,
	planDeletionService ProjectPlanDeletionService,
	planCreationService ProjectPlanCreationService,
) (map[string]toolruntime.ToolHandler, error) {
	if service == nil || setService == nil || deletionService == nil || planService == nil || planDeletionService == nil || planCreationService == nil {
		return nil, fmt.Errorf("project mutation service is required")
	}
	return map[string]toolruntime.ToolHandler{
		ProjectDeliveryLabelCreate: projectWriteHandler{service: service, operation: project.DeliveryLabelCreate},
		ProjectDeliveryLabelUpdate: projectWriteHandler{service: service, operation: project.DeliveryLabelUpdate},
		ProjectDeliveryLabelDelete: projectWriteHandler{service: service, operation: project.DeliveryLabelDelete},
		ProjectSetCreate:           projectSetWriteHandler{service: setService, operation: project.SetCreate},
		ProjectSetUpdate:           projectSetWriteHandler{service: setService, operation: project.SetUpdate},
		ProjectSetDelete:           projectSetDeleteHandler{service: deletionService},
		ProjectSetTransferOwner:    projectSetWriteHandler{service: setService, operation: project.SetTransferOwner},
		ProjectPlanUnitCreate:      projectPlanCreateHandler{service: planCreationService},
		ProjectPlanUnitUpdate:      projectPlanWriteHandler{service: planService, operation: project.PlanUpdate},
		ProjectPlanUnitDelete:      projectPlanDeleteHandler{service: planDeletionService},
	}, nil
}

func (handler projectWriteHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}

func (handler projectWriteHandler) Apply(ctx context.Context, call toolruntime.MutationCall) (json.RawMessage, error) {
	if handler.service == nil || call.Authorization.WorkspaceID == 0 || call.ChangeSetID == "" || call.ApplyKey == "" {
		return nil, project.ErrMutationInvalid
	}
	var input struct {
		ID        uint   `json:"id"`
		Name      string `json:"name"`
		ClientKey string `json:"client_key"`
		SystemKey string `json:"system_key"`
		SortOrder int    `json:"sort_order"`
	}
	if json.Unmarshal(call.Input, &input) != nil {
		return nil, project.ErrMutationInvalid
	}
	digest := sha256.Sum256(call.Input)
	result, err := handler.service.ApplyDeliveryLabel(ctx, project.DeliveryLabelMutation{
		Operation: handler.operation, WorkspaceID: call.Authorization.WorkspaceID,
		ApplyKey: call.ApplyKey, InputDigest: hex.EncodeToString(digest[:]), ID: input.ID,
		Name: input.Name, ClientKey: input.ClientKey, SystemKey: input.SystemKey, SortOrder: input.SortOrder,
	})
	if err != nil {
		return nil, err
	}
	if handler.operation == project.DeliveryLabelDelete {
		return json.Marshal(struct {
			ID      uint `json:"id"`
			Deleted bool `json:"deleted"`
		}{ID: result.ID, Deleted: result.Deleted})
	}
	return json.Marshal(result)
}

var _ toolruntime.IdempotentMutationHandler = projectWriteHandler{}

type projectSetWriteHandler struct {
	service   ProjectSetMutationService
	operation project.SetMutationOperation
}

func (handler projectSetWriteHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}

func (handler projectSetWriteHandler) Apply(ctx context.Context, call toolruntime.MutationCall) (json.RawMessage, error) {
	if handler.service == nil || call.Authorization.WorkspaceID == 0 || call.Authorization.PrincipalID == 0 ||
		call.ChangeSetID == "" || call.ApplyKey == "" {
		return nil, project.ErrMutationInvalid
	}
	command, err := decodeProjectSetMutation(call.Input, handler.operation)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(call.Input)
	command.WorkspaceID, command.ActorUserID = call.Authorization.WorkspaceID, call.Authorization.PrincipalID
	command.ApplyKey, command.InputDigest = call.ApplyKey, hex.EncodeToString(digest[:])
	result, err := handler.service.ApplyProjectSet(ctx, command)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

func decodeProjectSetMutation(input json.RawMessage, operation project.SetMutationOperation) (project.SetMutation, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(input, &fields) != nil {
		return project.SetMutation{}, project.ErrMutationInvalid
	}
	command := project.SetMutation{Operation: operation}
	if err := decodeProjectSetScalar(fields, &command); err != nil {
		return project.SetMutation{}, err
	}
	if raw, exists := fields["owner_user_id"]; exists {
		command.OwnerSet = true
		if string(raw) != "null" {
			var value uint
			if json.Unmarshal(raw, &value) != nil || value == 0 {
				return project.SetMutation{}, project.ErrMutationInvalid
			}
			command.OwnerUserID = &value
		}
	}
	if raw, exists := fields["planned_start_at"]; exists {
		command.PlannedStartSet = true
		value, err := decodeNullableMutationTime(raw)
		if err != nil {
			return project.SetMutation{}, err
		}
		command.PlannedStartAt = value
	}
	if raw, exists := fields["planned_end_at"]; exists {
		command.PlannedEndSet = true
		value, err := decodeNullableMutationTime(raw)
		if err != nil {
			return project.SetMutation{}, err
		}
		command.PlannedEndAt = value
	}
	return command, nil
}

func decodeProjectSetScalar(fields map[string]json.RawMessage, command *project.SetMutation) error {
	for name, target := range map[string]*uint{
		"id": &command.ID, "team_id": &command.TeamID, "new_owner_user_id": &command.NewOwnerUserID,
	} {
		if raw, exists := fields[name]; exists && json.Unmarshal(raw, target) != nil {
			return project.ErrMutationInvalid
		}
	}
	_, command.TeamIDSet = fields["team_id"]
	for name, target := range map[string]*string{
		"name": &command.Name, "code": &command.Code, "description": &command.Description,
		"status": &command.Status, "transfer_reason": &command.TransferReason,
	} {
		if raw, exists := fields[name]; exists && json.Unmarshal(raw, target) != nil {
			return project.ErrMutationInvalid
		}
	}
	_, command.NameSet = fields["name"]
	_, command.DescriptionSet = fields["description"]
	_, command.StatusSet = fields["status"]
	return nil
}

func decodeNullableMutationTime(raw json.RawMessage) (*time.Time, error) {
	if string(raw) == "null" {
		return nil, nil
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return nil, project.ErrMutationInvalid
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, project.ErrMutationInvalid
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

var _ toolruntime.IdempotentMutationHandler = projectSetWriteHandler{}

type projectSetDeleteHandler struct{ service ProjectSetDeletionService }

func (handler projectSetDeleteHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}

func (handler projectSetDeleteHandler) Apply(ctx context.Context, call toolruntime.MutationCall) (json.RawMessage, error) {
	if handler.service == nil || call.Authorization.WorkspaceID == 0 || call.Authorization.PrincipalID == 0 ||
		call.ChangeSetID == "" || call.ApplyKey == "" {
		return nil, project.ErrMutationInvalid
	}
	command, err := decodeProjectSetMutation(call.Input, project.SetDelete)
	if err != nil || command.ID == 0 {
		return nil, project.ErrMutationInvalid
	}
	digest := sha256.Sum256(call.Input)
	command.WorkspaceID, command.ActorUserID = call.Authorization.WorkspaceID, call.Authorization.PrincipalID
	command.ApplyKey, command.InputDigest = call.ApplyKey, hex.EncodeToString(digest[:])
	result, err := handler.service.ApplyProjectSetDeletion(ctx, command)
	if errors.Is(err, project.ErrMutationRetryable) {
		return nil, toolruntime.RetryableApplyError(err)
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		ID      uint `json:"id"`
		Deleted bool `json:"deleted"`
	}{ID: result.ID, Deleted: result.Deleted})
}

var _ toolruntime.IdempotentMutationHandler = projectSetDeleteHandler{}

type projectPlanWriteHandler struct {
	service   ProjectPlanMutationService
	operation project.PlanMutationOperation
}

func (handler projectPlanWriteHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}
func (handler projectPlanWriteHandler) Apply(ctx context.Context, call toolruntime.MutationCall) (json.RawMessage, error) {
	if handler.service == nil || call.Authorization.WorkspaceID == 0 || call.Authorization.PrincipalID == 0 || call.ChangeSetID == "" || call.ApplyKey == "" {
		return nil, project.ErrMutationInvalid
	}
	command, err := decodeProjectPlanMutation(call.Input, handler.operation)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(call.Input)
	command.WorkspaceID, command.ActorUserID, command.ApplyKey, command.InputDigest = call.Authorization.WorkspaceID, call.Authorization.PrincipalID, call.ApplyKey, hex.EncodeToString(digest[:])
	result, err := handler.service.ApplyPlanUnit(ctx, command)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}
func decodeProjectPlanMutation(input json.RawMessage, operation project.PlanMutationOperation) (project.PlanMutation, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(input, &fields) != nil {
		return project.PlanMutation{}, project.ErrMutationInvalid
	}
	command := project.PlanMutation{Operation: operation}
	for name, target := range map[string]*uint{"id": &command.ID, "project_new_id": &command.ProjectSetID, "workflow_template_id": &command.WorkflowTemplateID} {
		if raw, ok := fields[name]; ok && json.Unmarshal(raw, target) != nil {
			return project.PlanMutation{}, project.ErrMutationInvalid
		}
	}
	for name, target := range map[string]*string{"title": &command.Title, "type": &command.Type, "goal": &command.Goal, "risk_level": &command.RiskLevel} {
		if raw, ok := fields[name]; ok && json.Unmarshal(raw, target) != nil {
			return project.PlanMutation{}, project.ErrMutationInvalid
		}
	}
	_, command.TitleSet = fields["title"]
	_, command.TypeSet = fields["type"]
	_, command.GoalSet = fields["goal"]
	_, command.RiskLevelSet = fields["risk_level"]
	if raw, ok := fields["owner_user_id"]; ok {
		command.OwnerSet = true
		if string(raw) != "null" {
			var value uint
			if json.Unmarshal(raw, &value) != nil || value == 0 {
				return project.PlanMutation{}, project.ErrMutationInvalid
			}
			command.OwnerUserID = &value
		}
	}
	dateFields := map[string]struct {
		target **time.Time
		set    *bool
	}{
		"planned_start_at": {&command.PlannedStartAt, &command.PlannedStartSet}, "planned_end_at": {&command.PlannedEndAt, &command.PlannedEndSet},
	}
	for name, field := range dateFields {
		if raw, ok := fields[name]; ok {
			*field.set = true
			value, err := decodeNullableMutationTime(raw)
			if err != nil {
				return project.PlanMutation{}, err
			}
			*field.target = value
		}
	}
	return command, nil
}

var _ toolruntime.IdempotentMutationHandler = projectPlanWriteHandler{}

type projectPlanCreateHandler struct{ service ProjectPlanCreationService }

func (handler projectPlanCreateHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}

func (handler projectPlanCreateHandler) Apply(ctx context.Context, call toolruntime.MutationCall) (json.RawMessage, error) {
	if handler.service == nil || call.Authorization.WorkspaceID == 0 || call.Authorization.PrincipalID == 0 || call.ChangeSetID == "" || call.ApplyKey == "" {
		return nil, project.ErrMutationInvalid
	}
	command, err := decodeProjectPlanMutation(call.Input, project.PlanCreate)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(call.Input)
	command.WorkspaceID, command.ActorUserID, command.ApplyKey, command.InputDigest = call.Authorization.WorkspaceID, call.Authorization.PrincipalID, call.ApplyKey, hex.EncodeToString(digest[:])
	result, err := handler.service.ApplyPlanUnitCreation(ctx, command)
	if errors.Is(err, project.ErrMutationRetryable) {
		return nil, toolruntime.RetryableApplyError(err)
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

var _ toolruntime.IdempotentMutationHandler = projectPlanCreateHandler{}

type projectPlanDeleteHandler struct{ service ProjectPlanDeletionService }

func (handler projectPlanDeleteHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}
func (handler projectPlanDeleteHandler) Apply(ctx context.Context, call toolruntime.MutationCall) (json.RawMessage, error) {
	if handler.service == nil || call.Authorization.WorkspaceID == 0 || call.Authorization.PrincipalID == 0 || call.ApplyKey == "" || call.ChangeSetID == "" {
		return nil, project.ErrMutationInvalid
	}
	command, err := decodeProjectPlanMutation(call.Input, project.PlanDelete)
	if err != nil || command.ID == 0 {
		return nil, project.ErrMutationInvalid
	}
	digest := sha256.Sum256(call.Input)
	command.WorkspaceID, command.ActorUserID, command.ApplyKey, command.InputDigest = call.Authorization.WorkspaceID, call.Authorization.PrincipalID, call.ApplyKey, hex.EncodeToString(digest[:])
	result, err := handler.service.ApplyPlanUnitDeletion(ctx, command)
	if errors.Is(err, project.ErrMutationRetryable) {
		return nil, toolruntime.RetryableApplyError(err)
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		ID      uint `json:"id"`
		Deleted bool `json:"deleted"`
	}{result.ID, result.Deleted})
}

var _ toolruntime.IdempotentMutationHandler = projectPlanDeleteHandler{}
