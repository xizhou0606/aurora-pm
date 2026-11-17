package httpapi

import (
	"strconv"
	"strings"
	"time"

	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

type taskBatchValidationField struct {
	ClientID string `json:"client_id,omitempty"`
	Field    string `json:"field"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

type taskBatchValidationError struct {
	Fields []taskBatchValidationField
}

func (e *taskBatchValidationError) Error() string {
	if len(e.Fields) == 0 {
		return "batch task validation failed"
	}
	return e.Fields[0].Message
}

func batchFieldError(clientID, field, code, message string) error {
	return &taskBatchValidationError{Fields: []taskBatchValidationField{{
		ClientID: clientID, Field: field, Code: code, Message: message,
	}}}
}

func normalizeTaskBatchParent(
	clientID string,
	body taskBatchParentBody,
	defaultAssigneeID uint,
) (taskBatchParentBody, error) {
	if body.Priority == nil {
		priority := "normal"
		body.Priority = &priority
	}
	if !body.AssigneeID.Set {
		body.AssigneeID = optionalValue[uint]{Set: true, Value: &defaultAssigneeID}
	}
	if err := validateTaskBatchInput(
		clientID, body.Title, body.Status, body.Priority, body.AssigneeID, body.ReporterUserID,
		body.EstimatedMinutes, body.StartsAt, body.DueAt,
	); err != nil {
		return body, err
	}
	return body, nil
}

func normalizeTaskBatchChild(
	clientID string,
	body taskBatchChildBody,
	parent taskBatchParentBody,
) (taskBatchChildBody, error) {
	if body.Priority == nil {
		body.Priority = parent.Priority
	}
	if !body.TaskClientID.Set {
		body.TaskClientID = parent.TaskClientID
	}
	if !body.AssigneeID.Set {
		body.AssigneeID = parent.AssigneeID
	}
	if !body.StartsAt.Set {
		body.StartsAt = parent.StartsAt
	}
	if !body.DueAt.Set {
		body.DueAt = parent.DueAt
	}
	if err := validateTaskBatchInput(
		clientID, body.Title, body.Status, body.Priority, body.AssigneeID, body.ReporterUserID,
		body.EstimatedMinutes, body.StartsAt, body.DueAt,
	); err != nil {
		return body, err
	}
	return body, nil
}

func validateTaskBatchInput(
	clientID, title string,
	status *string,
	priority *string,
	assignee, reporter optionalValue[uint],
	estimated optionalValue[int],
	startsAt, dueAt optionalValue[string],
) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return batchFieldError(clientID, "title", validationRequired, "title required")
	}
	if len([]rune(title)) > 220 {
		return batchFieldError(clientID, "title", validationMaximumLength, "title cannot exceed 220 characters")
	}
	if priority == nil || !validTaskPriority(*priority) {
		return batchFieldError(clientID, "priority", validationInvalidPriority, "priority is invalid")
	}
	if status != nil && !validTaskStatus(*status) {
		return batchFieldError(clientID, "status", validationInvalidStatus, "status is invalid")
	}
	if assignee.Value == nil || *assignee.Value == 0 {
		return batchFieldError(clientID, "assignee_id", validationRequired, "assignee_id required")
	}
	if reporter.Set && (reporter.Value == nil || *reporter.Value == 0) {
		return batchFieldError(clientID, "reporter_user_id", validationInvalidReporter, "reporter_user_id must be positive")
	}
	if estimated.Set && estimated.Value != nil && *estimated.Value < 0 {
		return batchFieldError(clientID, "estimated_minutes", validationMinimumValue,
			"estimated_minutes cannot be negative")
	}
	start, err := taskBatchTime(clientID, "starts_at", startsAt)
	if err != nil {
		return err
	}
	due, err := taskBatchTime(clientID, "due_at", dueAt)
	if err != nil {
		return err
	}
	if start != nil && due != nil && start.After(*due) {
		return batchFieldError(clientID, "due_at", validationInvalidRange, "starts_at cannot be after due_at")
	}
	return nil
}

func taskBatchTime(clientID, field string, value optionalValue[string]) (*time.Time, error) {
	if !value.Set {
		return nil, nil
	}
	parsed, err := parseOptionalTime(value.Value)
	if err != nil {
		return nil, batchFieldError(clientID, field, validationInvalidDate, err.Error())
	}
	return parsed, nil
}

func taskBatchInputError(clientID string, err error) error {
	message := err.Error()
	field := "tasks"
	for _, candidate := range []string{
		"title", "description", "status", "priority", "task_client_id", "assignee_id", "reporter_user_id",
		"starts_at", "due_at", "estimated_minutes",
	} {
		if strings.Contains(message, candidate) {
			field = candidate
			break
		}
	}
	return batchFieldError(clientID, field, validationInvalid, message)
}

type taskBatchUserField struct {
	ClientID string
	Field    string
}

func taskBatchUserError(err error, clients map[uint][]taskBatchUserField) error {
	message := err.Error()
	fields := make([]taskBatchValidationField, 0)
	const marker = "platform user id "
	if index := strings.Index(message, marker); index >= 0 {
		remainder := message[index+len(marker):]
		idText := strings.Fields(remainder)
		if len(idText) > 0 {
			if id, parseErr := strconv.ParseUint(idText[0], 10, 64); parseErr == nil {
				for _, client := range clients[uint(id)] {
					code := validationInvalidAssignee
					if client.Field == "reporter_user_id" {
						code = validationInvalidReporter
					}
					fields = append(fields, taskBatchValidationField{
						ClientID: client.ClientID, Field: client.Field,
						Code: code, Message: message,
					})
				}
			}
		}
	}
	if len(fields) == 0 {
		for _, clientFields := range clients {
			for _, client := range clientFields {
				code := validationAssigneeValidationFailed
				if client.Field == "reporter_user_id" {
					code = validationReporterValidationFailed
				}
				fields = append(fields, taskBatchValidationField{
					ClientID: client.ClientID, Field: client.Field,
					Code: code, Message: message,
				})
			}
		}
	}
	return &taskBatchValidationError{Fields: fields}
}

func taskBatchParentTaskBody(body taskBatchParentBody, reporterID uint) taskBody {
	title := strings.TrimSpace(body.Title)
	kind := taskgorm.WorkTaskKindTask
	reporter := body.ReporterUserID
	if !reporter.Set {
		reporter = optionalValue[uint]{Set: true, Value: &reporterID}
	}
	return taskBody{
		RequirementID: body.RequirementID, Kind: &kind, Title: &title,
		TaskClientID: body.TaskClientID,
		Description:  body.Description, Status: body.Status,
		Priority: body.Priority, AssigneeID: body.AssigneeID, ReporterUserID: reporter,
		EstimatedMinutes: body.EstimatedMinutes, StartsAt: body.StartsAt, DueAt: body.DueAt,
	}
}

func taskBatchChildTaskBody(body taskBatchChildBody, reporterID uint) taskBody {
	title := strings.TrimSpace(body.Title)
	// Validate common task fields as a root task first. The transaction assigns
	// the generated parent id and converts it to a subtask before persistence.
	kind := taskgorm.WorkTaskKindTask
	reporter := body.ReporterUserID
	if !reporter.Set {
		reporter = optionalValue[uint]{Set: true, Value: &reporterID}
	}
	return taskBody{
		Kind: &kind, Title: &title, Description: body.Description, Status: body.Status,
		TaskClientID: body.TaskClientID,
		Priority:     body.Priority, AssigneeID: body.AssigneeID, ReporterUserID: reporter,
		EstimatedMinutes: body.EstimatedMinutes, StartsAt: body.StartsAt, DueAt: body.DueAt,
	}
}

func appendTaskBatchUserFacts(
	ids []uint,
	fields map[uint][]taskBatchUserField,
	clientID string,
	task taskgorm.WorkTask,
) []uint {
	for field, userID := range map[string]*uint{
		"assignee_id": task.AssigneeID, "reporter_user_id": task.ReporterUserID,
	} {
		if userID == nil || *userID == 0 {
			continue
		}
		ids = append(ids, *userID)
		fields[*userID] = append(fields[*userID], taskBatchUserField{ClientID: clientID, Field: field})
	}
	return ids
}
