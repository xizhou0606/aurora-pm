package pmtools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/quality"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
)

const (
	QualityBugCreate       = "quality.bug.create"
	QualityBugUpdate       = "quality.bug.update"
	QualityBugAssign       = "quality.bug.assign"
	QualityBugStatusUpdate = "quality.bug.status.update"
	QualityBugDelete       = "quality.bug.delete"
	QualityBugBatchCreate  = "quality.bug.batch_create"
	QualityBugBatchAssign  = "quality.bug.batch_assign"
	QualityBugBatchDelete  = "quality.bug.batch_delete"
)

type QualityMutationService interface {
	ApplyBugMutation(context.Context, quality.BugMutation) (quality.BugMutationResult, error)
}

type qualityWriteHandler struct {
	service        QualityMutationService
	operation      quality.BugMutationOperation
	transitionOnly bool
	assignmentOnly bool
}

func QualityWriteHandlerNames() []string {
	return []string{QualityBugAssign, QualityBugBatchAssign, QualityBugBatchCreate, QualityBugBatchDelete, QualityBugCreate, QualityBugDelete, QualityBugStatusUpdate, QualityBugUpdate}
}

func NewQualityWriteHandlers(service QualityMutationService) (map[string]toolruntime.ToolHandler, error) {
	if service == nil {
		return nil, fmt.Errorf("quality mutation service is required")
	}
	return map[string]toolruntime.ToolHandler{
		QualityBugCreate:       qualityWriteHandler{service: service, operation: quality.BugCreate},
		QualityBugUpdate:       qualityWriteHandler{service: service, operation: quality.BugUpdate},
		QualityBugAssign:       qualityWriteHandler{service: service, operation: quality.BugUpdate, assignmentOnly: true},
		QualityBugStatusUpdate: qualityWriteHandler{service: service, operation: quality.BugUpdate, transitionOnly: true},
		QualityBugDelete:       qualityWriteHandler{service: service, operation: quality.BugDelete},
		QualityBugBatchCreate:  qualityWriteHandler{service: service, operation: quality.BugBatchCreate},
		QualityBugBatchAssign:  qualityWriteHandler{service: service, operation: quality.BugBatchAssign},
		QualityBugBatchDelete:  qualityWriteHandler{service: service, operation: quality.BugBatchDelete},
	}, nil
}

func (handler qualityWriteHandler) Invoke(context.Context, toolruntime.ToolCall) (json.RawMessage, error) {
	return nil, toolruntime.ErrChangeSetRequired
}

type qualityBugInput struct {
	AssignmentNote        string             `json:"assignment_note"`
	AssignmentMaterials   []quality.Material `json:"assignment_materials"`
	RegressionNote        string             `json:"regression_note"`
	ActivityNote          string             `json:"activity_note"`
	RegressionMaterials   []quality.Material `json:"regression_materials"`
	ReproductionMaterials []quality.Material `json:"reproduction_materials"`
	ResolutionMaterials   []quality.Material `json:"resolution_materials"`
	ID                    uint               `json:"id"`
	RequirementID         uint               `json:"requirement_id"`
	Title                 string             `json:"title"`
	Status                string             `json:"status"`
	Priority              string             `json:"priority"`
	BugType               string             `json:"bug_type"`
	DefectDomain          string             `json:"defect_domain"`
	SeverityKey           string             `json:"severity_key"`
	DueAt                 *time.Time         `json:"due_at"`
	FoundEnvironment      string             `json:"found_environment"`
	OperatingSystem       string             `json:"operating_system"`
	Browser               string             `json:"browser"`
	FoundVersion          string             `json:"found_version"`
	ReproductionSteps     string             `json:"reproduction_steps"`
	ExpectedResult        string             `json:"expected_result"`
	ActualResult          string             `json:"actual_result"`
	ResolutionReason      string             `json:"resolution_reason"`
	ResolutionNote        string             `json:"resolution_note"`
	AssigneeUserID        *uint              `json:"assignee_user_id"`
	ReviewerUserID        *uint              `json:"reviewer_user_id"`
	SourceExecutionID     *uint              `json:"source_execution_id"`
	SourceTestID          *uint              `json:"source_test_id"`
	IdempotencyKey        string             `json:"idempotency_key"`
}

type qualityBatchItemInput struct {
	ClientID string `json:"client_id"`
	qualityBugInput
}

func (handler qualityWriteHandler) Apply(ctx context.Context, call toolruntime.MutationCall) (json.RawMessage, error) {
	if handler.service == nil || call.Authorization.WorkspaceID == 0 || call.Authorization.PrincipalID == 0 ||
		call.ChangeSetID == "" || call.ApplyKey == "" {
		return nil, quality.ErrBugMutationInvalid
	}
	command := quality.BugMutation{
		StatusTransition: handler.transitionOnly,
		Operation:        handler.operation, WorkspaceID: call.Authorization.WorkspaceID,
		PrincipalID: call.Authorization.PrincipalID, ApplyKey: call.ApplyKey, ChangeSetID: call.ChangeSetID,
	}
	digest := sha256.Sum256(call.Input)
	command.InputDigest = hex.EncodeToString(digest[:])
	switch handler.operation {
	case quality.BugCreate, quality.BugUpdate:
		var input qualityBugInput
		fields, err := decodeQualityObject(call.Input, &input)
		if err != nil {
			return nil, quality.ErrBugMutationInvalid
		}
		if handler.transitionOnly && !qualityTransitionFieldsOnly(input, fields) {
			return nil, quality.ErrBugMutationInvalid
		}
		if handler.assignmentOnly && !qualityAssignmentFieldsOnly(input, fields) {
			return nil, quality.ErrBugMutationInvalid
		}
		command.ID, command.Values = input.ID, qualityValuesFromInput(input, fields)
	case quality.BugDelete:
		var input struct {
			ID             uint   `json:"id"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if _, err := decodeQualityObject(call.Input, &input); err != nil {
			return nil, quality.ErrBugMutationInvalid
		}
		command.ID = input.ID
	case quality.BugBatchCreate:
		var input struct {
			Bugs           []json.RawMessage `json:"bugs"`
			IdempotencyKey string            `json:"idempotency_key"`
		}
		if _, err := decodeQualityObject(call.Input, &input); err != nil {
			return nil, quality.ErrBugMutationInvalid
		}
		command.Items = make([]quality.BatchBugMutationItem, 0, len(input.Bugs))
		for _, raw := range input.Bugs {
			var item qualityBatchItemInput
			fields, err := decodeQualityObject(raw, &item)
			if err != nil {
				return nil, quality.ErrBugMutationInvalid
			}
			command.Items = append(command.Items, quality.BatchBugMutationItem{ClientID: item.ClientID, Values: qualityValuesFromInput(item.qualityBugInput, fields)})
		}
	case quality.BugBatchAssign:
		var input struct {
			IDs            []uint `json:"ids"`
			AssigneeUserID *uint  `json:"assignee_user_id"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		fields, err := decodeQualityObject(call.Input, &input)
		if err != nil || input.AssigneeUserID == nil || *input.AssigneeUserID == 0 {
			return nil, quality.ErrBugMutationInvalid
		}
		command.IDs = input.IDs
		command.Values = quality.BugMutationValues{
			AssigneeUserID: input.AssigneeUserID,
			Fields:         map[string]bool{"assignee_user_id": fields["assignee_user_id"] != nil},
		}
	case quality.BugBatchDelete:
		var input struct {
			IDs            []uint `json:"ids"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if _, err := decodeQualityObject(call.Input, &input); err != nil {
			return nil, quality.ErrBugMutationInvalid
		}
		command.IDs = input.IDs
	default:
		return nil, quality.ErrBugMutationInvalid
	}
	result, err := handler.service.ApplyBugMutation(ctx, command)
	if err != nil {
		if errors.Is(err, quality.ErrBugMutationInvalid) {
			return nil, toolruntime.LookupBusinessError("QUALITY_INPUT_INVALID")
		}
		if errors.Is(err, quality.ErrBugMutationNotFound) {
			return nil, toolruntime.LookupBusinessError("QUALITY_NOT_FOUND")
		}
		if errors.Is(err, quality.ErrBugMutationConflict) {
			return nil, toolruntime.LookupBusinessError("QUALITY_CONFLICT")
		}
		return nil, err
	}
	switch handler.operation {
	case quality.BugCreate, quality.BugUpdate:
		if result.Bug == nil {
			return nil, quality.ErrBugMutationConflict
		}
		return json.Marshal(result.Bug)
	case quality.BugDelete:
		return json.Marshal(struct {
			ID      uint `json:"id"`
			Deleted bool `json:"deleted"`
		}{result.DeletedID, result.Deleted})
	case quality.BugBatchCreate:
		return json.Marshal(struct {
			Created      []quality.BatchBugMutationResult `json:"created"`
			CreatedCount int                              `json:"created_count"`
			Atomic       bool                             `json:"atomic"`
		}{result.Created, result.CreatedCount, result.Atomic})
	case quality.BugBatchAssign:
		return json.Marshal(struct {
			Updated      []quality.BugSnapshot `json:"updated"`
			UpdatedCount int                   `json:"updated_count"`
			Atomic       bool                  `json:"atomic"`
		}{result.Updated, result.UpdatedCount, result.Atomic})
	case quality.BugBatchDelete:
		return json.Marshal(struct {
			DeletedIDs   []uint `json:"deleted_ids"`
			DeletedCount int    `json:"deleted_count"`
			Atomic       bool   `json:"atomic"`
		}{result.DeletedIDs, result.DeletedCount, result.Atomic})
	}
	return nil, quality.ErrBugMutationInvalid
}

func qualityTransitionFieldsOnly(input qualityBugInput, fields map[string]json.RawMessage) bool {
	if input.ID == 0 || input.Status == "" {
		return false
	}
	allowed := map[string]struct{}{
		"id": {}, "status": {}, "resolution_reason": {}, "resolution_note": {},
		"reviewer_user_id": {}, "idempotency_key": {},
		"regression_note": {}, "activity_note": {}, "regression_materials": {}, "resolution_materials": {},
	}
	for field := range fields {
		if _, ok := allowed[field]; !ok {
			return false
		}
	}
	return true
}

func qualityAssignmentFieldsOnly(input qualityBugInput, fields map[string]json.RawMessage) bool {
	if input.ID == 0 || input.AssigneeUserID == nil || *input.AssigneeUserID == 0 {
		return false
	}
	allowed := map[string]struct{}{
		"id": {}, "assignee_user_id": {}, "idempotency_key": {}, "assignment_note": {}, "assignment_materials": {},
	}
	for field := range fields {
		if _, ok := allowed[field]; !ok {
			return false
		}
	}
	return true
}

func decodeQualityObject(raw json.RawMessage, destination any) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return nil, err
	}
	if err := decoder.Decode(&struct{}{}); !errorsIsEOF(err) {
		return nil, fmt.Errorf("trailing JSON")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, fmt.Errorf("object required")
	}
	return fields, nil
}

func errorsIsEOF(err error) bool { return err == io.EOF }

func qualityValuesFromInput(input qualityBugInput, fields map[string]json.RawMessage) quality.BugMutationValues {
	set := make(map[string]bool, len(fields))
	for name := range fields {
		if name != "id" && name != "idempotency_key" {
			set[name] = true
		}
	}
	return quality.BugMutationValues{
		ReproductionMaterials: input.ReproductionMaterials, ResolutionMaterials: input.ResolutionMaterials,
		AssignmentNote: input.AssignmentNote, AssignmentMaterials: input.AssignmentMaterials,
		RegressionNote: input.RegressionNote, ActivityNote: input.ActivityNote, RegressionMaterials: input.RegressionMaterials,
		RequirementID: input.RequirementID, Title: input.Title, Status: input.Status, Priority: input.Priority,
		BugType: input.BugType, DefectDomain: input.DefectDomain, SeverityKey: input.SeverityKey, DueAt: input.DueAt,
		FoundEnvironment: input.FoundEnvironment, OperatingSystem: input.OperatingSystem, Browser: input.Browser,
		FoundVersion: input.FoundVersion, ReproductionSteps: input.ReproductionSteps,
		ExpectedResult: input.ExpectedResult, ActualResult: input.ActualResult,
		ResolutionReason: quality.NormalizeBugResolutionReason(input.ResolutionReason), ResolutionNote: input.ResolutionNote,
		AssigneeUserID: input.AssigneeUserID, ReviewerUserID: input.ReviewerUserID,
		SourceExecutionID: input.SourceExecutionID, SourceTestID: input.SourceTestID, Fields: set,
	}
}

var _ toolruntime.IdempotentMutationHandler = qualityWriteHandler{}
var _ toolruntime.ToolHandler = qualityWriteHandler{}
