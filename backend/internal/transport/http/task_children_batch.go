package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

type taskChildrenBatchBody struct {
	Children []taskBatchChildBody `json:"children"`
}

func (s *Server) batchCreateTaskChildren(c *gin.Context) {
	parentID, valid := paramID(c, "id")
	if !valid {
		return
	}
	parent, project, allowed := s.taskByID(c, parentID)
	if !allowed {
		return
	}
	if taskgorm.NormalizePMWorkTaskKind(parent.Kind) != taskgorm.WorkTaskKindTask {
		bad(c, "parent task must be a task")
		return
	}
	var body taskChildrenBatchBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	prepared, err := s.prepareTaskChildrenBatch(c, parent, project, body)
	if err != nil {
		var validation *taskBatchValidationError
		if errors.As(err, &validation) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
				"code":   apiErrorTaskChildrenBatchValidationFailed,
				"error":  "batch child task validation failed",
				"fields": validation.Fields,
			})
			return
		}
		if errors.Is(err, errPMUserRegistryUnavailable) {
			respondPMUserRegistryValidationError(c, err)
			return
		}
		bad(c, err.Error())
		return
	}
	workspace, _ := currentWorkspace(c)
	authActor, _ := currentActor(c)
	notificationContext := workspaceContext{}
	if s.cfg.Notification.Enabled {
		notificationTasks := make([]taskgorm.WorkTask, 0, len(prepared))
		for _, child := range prepared {
			notificationTasks = append(notificationTasks, child.Task)
		}
		notificationContext = s.loadWorkspaceContext(
			workspace.ID,
			workspaceContextReferences{}.withTasks(notificationTasks),
		)
	}
	createdRows := make([]gin.H, 0, len(prepared))
	err = s.db.Transaction(func(tx *gorm.DB) error {
		for index := range prepared {
			child := &prepared[index]
			if err := s.createPreparedTask(
				tx, c, authActor, notificationContext, project, &child.Task,
			); err != nil {
				return err
			}
			createdRows = append(createdRows, taskBatchCreatedRow(child.ClientID, child.Task, nil))
		}
		return nil
	})
	if err != nil {
		dbFail(c)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{
		"parent_id": parent.ID, "created": createdRows,
		"created_count": len(createdRows), "atomic": true,
	}})
}

func (s *Server) prepareTaskChildrenBatch(
	c *gin.Context,
	parent taskgorm.WorkTask,
	project projectgorm.ProjectSet,
	body taskChildrenBatchBody,
) ([]preparedTaskBatchChild, error) {
	if len(body.Children) == 0 {
		return nil, batchFieldError("", "children", validationRequired, "children required")
	}
	if len(body.Children) > maxBatchTotalTasks {
		return nil, batchFieldError("", "children", validationMaximumTotalTasks,
			fmt.Sprintf("child task count cannot exceed %d", maxBatchTotalTasks))
	}
	var requirement requirementgorm.Requirement
	if err := s.db.Where(
		"id = ? AND workspace_id = ?", parent.RequirementID, parent.WorkspaceID,
	).First(&requirement).Error; err != nil {
		return nil, batchFieldError("", "parent_id", validationInvalidScope, "requirement not found")
	}
	if err := s.validateWorkspaceHierarchy(parent.WorkspaceID, project, requirement); err != nil {
		return nil, batchFieldError("", "parent_id", validationInvalidScope, err.Error())
	}
	authActor, _ := currentActor(c)
	defaults := taskBatchParentBody{
		Priority:      stringPointer(parent.Priority),
		AssigneeID:    optionalUintDefault(parent.AssigneeID, factUserID(authActor.User)),
		StartsAt:      optionalTimeDefault(parent.StartsAt),
		DueAt:         optionalTimeDefault(parent.DueAt),
		RequirementID: parent.RequirementID,
	}
	seenClientIDs := make(map[string]struct{}, len(body.Children))
	userIDs := make([]uint, 0, len(body.Children)*2)
	userFields := make(map[uint][]taskBatchUserField, len(body.Children)*2)
	prepared := make([]preparedTaskBatchChild, 0, len(body.Children))
	for _, raw := range body.Children {
		clientID, err := reserveTaskBatchClientID(seenClientIDs, raw.ClientID)
		if err != nil {
			return nil, err
		}
		childBody, err := normalizeTaskBatchChild(clientID, raw, defaults)
		if err != nil {
			return nil, err
		}
		child, err := s.taskFromBody(
			project, requirement,
			taskBatchChildTaskBody(childBody, factUserID(authActor.User)),
		)
		if err != nil {
			return nil, taskBatchInputError(clientID, err)
		}
		child.Kind = taskgorm.WorkTaskKindSubtask
		child.ParentID = &parent.ID
		applyTaskLifecycle(&child, map[string]any{"status": child.Status})
		userIDs = appendTaskBatchUserFacts(userIDs, userFields, clientID, child)
		prepared = append(prepared, preparedTaskBatchChild{ClientID: clientID, Task: child})
	}
	if err := s.validateActivePMUserReferences(c, userIDs); err != nil {
		if errors.Is(err, errPMUserRegistryUnavailable) {
			return nil, err
		}
		return nil, taskBatchUserError(err, userFields)
	}
	return prepared, nil
}

func stringPointer(value string) *string {
	copy := value
	return &copy
}

func optionalUintDefault(value *uint, fallback uint) optionalValue[uint] {
	if value != nil && *value > 0 {
		copy := *value
		return optionalValue[uint]{Set: true, Value: &copy}
	}
	return optionalValue[uint]{Set: true, Value: &fallback}
}

func optionalTimeDefault(value *time.Time) optionalValue[string] {
	if value == nil {
		return optionalValue[string]{}
	}
	formatted := value.Format(time.RFC3339)
	return optionalValue[string]{Set: true, Value: &formatted}
}
