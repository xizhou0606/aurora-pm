package httpapi

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

type preparedNativeTaskUpdate struct {
	Before  taskgorm.WorkTask
	After   taskgorm.WorkTask
	Project projectgorm.ProjectSet
	Updates map[string]any
}

type preparedNativeTaskBatchUpdate struct {
	Workspace identitygorm.Workspace
	IDs       []uint
	TaskBody  taskBody
	Prepared  []preparedNativeTaskUpdate
}

func (s *Server) prepareNativeTaskBatchUpdateRequest(c *gin.Context) (preparedNativeTaskBatchUpdate, bool) {
	workspace, _ := currentWorkspace(c)
	var body nativeTaskBatchUpdateBody
	if err := decodeStrictTaskNativeJSON(c, &body); err != nil {
		bad(c, "invalid body")
		return preparedNativeTaskBatchUpdate{}, false
	}
	if body.Atomic == nil || !*body.Atomic {
		bad(c, "atomic=true required")
		return preparedNativeTaskBatchUpdate{}, false
	}
	ids := uniqueTaskIDs(body.IDs)
	if len(ids) == 0 || len(ids) > maxNativeTaskBatchUpdate || len(ids) != len(body.IDs) {
		bad(c, "ids must contain 1-500 unique positive task IDs")
		return preparedNativeTaskBatchUpdate{}, false
	}
	if body.Updates.empty() {
		bad(c, "updates required")
		return preparedNativeTaskBatchUpdate{}, false
	}
	taskUpdateBody := body.Updates.taskBody()
	if err := s.validateActivePMUserReferences(c, taskBodyUserFactIDs(taskUpdateBody)); err != nil {
		respondPMUserRegistryValidationError(c, err)
		return preparedNativeTaskBatchUpdate{}, false
	}
	prepared, err := s.prepareNativeTaskBatchUpdates(workspace.ID, ids, taskUpdateBody)
	if err != nil {
		var validation *nativeTaskUpdateValidationError
		if errors.As(err, &validation) {
			fail(c, validation.Status, validation.Message)
			return preparedNativeTaskBatchUpdate{}, false
		}
		bad(c, err.Error())
		return preparedNativeTaskBatchUpdate{}, false
	}
	return preparedNativeTaskBatchUpdate{
		Workspace: workspace, IDs: ids, TaskBody: taskUpdateBody, Prepared: prepared,
	}, true
}

func (s *Server) batchUpdateNativeTasks(c *gin.Context) {
	s.executeNativeTaskBatchUpdate(c, accesscatalog.ActionWorkItemsTasksBatchUpdate, false)
}

func (s *Server) batchUpdateNativeTaskStatuses(c *gin.Context) {
	s.executeNativeTaskBatchUpdate(c, accesscatalog.ActionWorkItemsTasksTransitionStatus, true)
}

func (s *Server) executeNativeTaskBatchUpdate(
	c *gin.Context,
	actionCode accesscatalog.ActionCode,
	statusOnly bool,
) {
	request, valid := s.prepareNativeTaskBatchUpdateRequest(c)
	if !valid {
		return
	}
	if statusOnly && !taskBodyOnlyStatus(request.TaskBody) {
		bad(c, "status is required and must be the only update field")
		return
	}
	workspace, ids := request.Workspace, request.IDs
	taskUpdateBody, prepared := request.TaskBody, request.Prepared
	// Gate every unique project referenced by the batch.
	checkedProjects := map[uint]bool{}
	for _, item := range prepared {
		if !checkedProjects[item.Project.ID] {
			if !s.requireProjectAccess(c, workspace.ID, item.Project.ID) {
				return
			}
			checkedProjects[item.Project.ID] = true
		}
	}
	tasks := make([]taskgorm.WorkTask, 0, len(prepared))
	for _, item := range prepared {
		tasks = append(tasks, item.Before)
	}
	if !s.requireTaskCreatorAction(c, actionCode, tasks...) {
		return
	}
	impact, err := s.calculateNativeTaskBatchUpdateImpact(workspace.ID, ids, prepared)
	if err != nil {
		dbFail(c)
		return
	}
	actor, _ := currentActor(c)
	notificationContext := workspaceContext{}
	if s.cfg.Notification.Enabled && taskUpdateBody.AssigneeID.Set && taskUpdateBody.AssigneeID.Value != nil {
		notificationTasks := make([]taskgorm.WorkTask, 0, len(prepared))
		for _, item := range prepared {
			notificationTasks = append(notificationTasks, item.After)
		}
		notificationContext = s.loadWorkspaceContext(
			workspace.ID,
			workspaceContextReferences{}.withTasks(notificationTasks),
		)
	}
	updatedTasks := make([]taskgorm.WorkTask, 0, len(prepared))
	err = s.db.Transaction(func(tx *gorm.DB) error {
		for index := range prepared {
			item := &prepared[index]
			lockedTasks, err := lockProjectWorkTasksTx(tx, workspace.ID, map[uint]uint{item.Before.ID: item.Project.ID})
			if err != nil {
				return err
			}
			lockedTask := lockedTasks[item.Before.ID]
			if !sameTaskScope(lockedTask, item.Before) {
				return errProjectChildScopeChanged
			}
			result := tx.Model(&taskgorm.WorkTask{}).
				Where("id = ? AND workspace_id = ?", item.Before.ID, workspace.ID).
				Updates(item.Updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
			if err := tx.Where(
				"id = ? AND workspace_id = ?", item.Before.ID, workspace.ID,
			).First(&item.After).Error; err != nil {
				return err
			}
			if taskUpdateBody.DueAt.Set && item.After.DueAt != nil {
				if err := s.propagateTaskDueToUnsetDescendants(
					tx, item.After, factUserID(actor.User),
				); err != nil {
					return err
				}
			}
			activities := taskUpdateActivityRows(item.Before, item.Updates, factUserID(actor.User))
			if len(activities) > 0 {
				if err := tx.Create(&activities).Error; err != nil {
					return err
				}
			}
			if isTerminalTaskStatus(item.After.Status) {
				if err := s.autoCompleteTaskAncestors(
					tx, workspace.ID, item.After.ParentID, factUserID(actor.User),
				); err != nil {
					return err
				}
			}
			if err := s.recordTaskStatusNotification(tx, item.After, lockedTask.Status, factUserID(actor.User)); err != nil {
				return err
			}
			if taskAssigneeChanged(item.Before.AssigneeID, item.After.AssigneeID) && item.After.AssigneeID != nil {
				if err := s.recordTaskAssignmentNotification(
					tx, c, item.After, notificationContext,
				); err != nil {
					return err
				}
			}
			if err := s.recordMutation(
				tx, c, item.Project.ID, "work_task", item.After.ID, "updated", item.After.Title,
				item.Updates,
			); err != nil {
				return err
			}
			updatedTasks = append(updatedTasks, item.After)
		}
		return nil
	})
	if err != nil {
		dbFail(c)
		return
	}
	context := s.loadWorkspaceContext(
		workspace.ID,
		workspaceContextReferences{}.withTasks(updatedTasks),
	)
	rows := s.workspaceTaskRows(c, workspace.ID, context, updatedTasks, nil)
	ok(c, gin.H{
		"items": rows, "updated_ids": ids, "updated_count": len(rows), "atomic": true,
		"affected_ids": impact.AffectedIDs, "affected_count": len(impact.AffectedIDs),
		"auto_completed_ancestor_ids":   impact.AutoCompletedAncestorIDs,
		"due_propagated_descendant_ids": impact.DuePropagatedDescendantIDs,
	})
}

func (s *Server) prepareNativeTaskBatchUpdates(
	workspaceID uint,
	ids []uint,
	body taskBody,
) ([]preparedNativeTaskUpdate, error) {
	var tasks []taskgorm.WorkTask
	query := pmTaskQuery(s.db).Where("workspace_id = ? AND id IN ?", workspaceID, ids)
	query = applyWorkspaceHierarchyScope(query, "work_tasks", workspaceID)
	if err := query.Find(&tasks).Error; err != nil {
		return nil, err
	}
	byID := make(map[uint]taskgorm.WorkTask, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}
	projects := make(map[uint]projectgorm.ProjectSet)
	prepared := make([]preparedNativeTaskUpdate, 0, len(ids))
	for _, id := range ids {
		task, exists := byID[id]
		if !exists {
			return nil, &nativeTaskUpdateValidationError{Status: http.StatusNotFound, Message: "task not found"}
		}
		project, exists := projects[task.ProjectSetID]
		if !exists {
			if err := s.db.Where(
				"id = ? AND workspace_id = ?", task.ProjectSetID, workspaceID,
			).First(&project).Error; err != nil {
				return nil, &nativeTaskUpdateValidationError{Status: http.StatusNotFound, Message: "project set not found"}
			}
			projects[project.ID] = project
		}
		updates, err := s.taskUpdates(project, task.RequirementID, task.ID, body)
		if err != nil {
			return nil, &nativeTaskUpdateValidationError{Status: http.StatusBadRequest, Message: err.Error()}
		}
		after := task
		applyTaskUpdates(&after, updates)
		applyTaskLifecycle(&after, updates)
		prepared = append(prepared, preparedNativeTaskUpdate{
			Before: task, After: after, Project: project, Updates: updates,
		})
	}
	return prepared, nil
}

type nativeTaskUpdateValidationError struct {
	Status  int
	Message string
}

func (err *nativeTaskUpdateValidationError) Error() string { return err.Message }
