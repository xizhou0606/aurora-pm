package httpapi

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

type taskBody struct {
	RequirementID    uint                  `json:"requirement_id"`
	TaskClientID     optionalValue[uint]   `json:"task_client_id"`
	Kind             *string               `json:"kind"`
	Title            *string               `json:"title"`
	Description      *string               `json:"description"`
	Status           *string               `json:"status"`
	Priority         *string               `json:"priority"`
	AssigneeID       optionalValue[uint]   `json:"assignee_id"`
	ReporterUserID   optionalValue[uint]   `json:"reporter_user_id"`
	ParentID         optionalValue[uint]   `json:"parent_id"`
	EstimatedMinutes optionalValue[int]    `json:"estimated_minutes"`
	StartsAt         optionalValue[string] `json:"starts_at"`
	DueAt            optionalValue[string] `json:"due_at"`
}

func taskBodyOnlyStatus(body taskBody) bool {
	return body.Status != nil && body.RequirementID == 0 && body.Kind == nil && body.Title == nil &&
		body.Description == nil && body.Priority == nil && !body.TaskClientID.Set && !body.AssigneeID.Set &&
		!body.ReporterUserID.Set && !body.ParentID.Set && !body.EstimatedMinutes.Set &&
		!body.StartsAt.Set && !body.DueAt.Set
}

func (s *Server) listTasks(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	projectID := queryUint(c, "project_new_id")
	planUnitID := queryUint(c, "version_task_new_id")
	requirementID := queryUint(c, "requirement_new_id", "requirement_id")
	if projectID == 0 && requirementID == 0 {
		bad(c, "project_new_id or requirement_new_id required")
		return
	}
	query := pmTaskQuery(s.db).Where("workspace_id = ?", workspace.ID)
	var requirement requirementgorm.Requirement
	if requirementID > 0 {
		if s.db.Where("id = ? AND workspace_id = ?", requirementID, workspace.ID).
			First(&requirement).Error != nil {
			fail(c, http.StatusNotFound, "requirement not found")
			return
		}
		if projectID > 0 && projectID != requirement.ProjectSetID {
			bad(c, "requirement does not belong to project")
			return
		}
		projectID = requirement.ProjectSetID
		query = query.Where("requirement_id = ?", requirementID)
	}
	if planUnitID > 0 {
		var planUnit projectgorm.PlanUnit
		if s.db.Where("id = ? AND workspace_id = ?", planUnitID, workspace.ID).
			First(&planUnit).Error != nil {
			fail(c, http.StatusNotFound, "plan unit not found")
			return
		}
		if projectID > 0 && projectID != planUnit.ProjectSetID {
			bad(c, "plan unit does not belong to project")
			return
		}
		if requirementID > 0 && requirement.PlanUnitID != planUnitID {
			bad(c, "requirement does not belong to plan unit")
			return
		}
		projectID = planUnit.ProjectSetID
		query = query.Where("project_id = ?", planUnitID)
	}
	query = query.Where("project_set_id = ?", projectID)
	var project projectgorm.ProjectSet
	if s.db.Where("id = ? AND workspace_id = ?", projectID, workspace.ID).First(&project).Error != nil {
		fail(c, http.StatusNotFound, "project set not found")
		return
	}
	if !s.requireProjectAccess(c, workspace.ID, project.ID) {
		return
	}
	limit, offset := pagination(c, 50)
	var total int64
	query.Model(&taskgorm.WorkTask{}).Count(&total)
	var tasks []taskgorm.WorkTask
	if query.Order("id DESC").Limit(limit).Offset(offset).Find(&tasks).Error != nil {
		dbFail(c)
		return
	}
	rows := make([]gin.H, 0, len(tasks))
	for _, task := range tasks {
		rows = append(rows, taskDTO(task))
	}
	users, err := s.listUserProfiles(taskListUserIDs(tasks))
	if err != nil {
		dbFail(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": rows, "total": total, "users": users})
}

func (s *Server) createTask(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	var body taskBody
	if c.ShouldBindJSON(&body) != nil || body.Title == nil || strings.TrimSpace(*body.Title) == "" {
		bad(c, "title required")
		return
	}
	if body.ParentID.Set && body.ParentID.Value != nil {
		var parent taskgorm.WorkTask
		query := applyWorkspaceHierarchyScope(
			pmTaskQuery(s.db).Where("id = ? AND workspace_id = ?", *body.ParentID.Value, workspace.ID),
			"work_tasks", workspace.ID,
		)
		if query.First(&parent).Error != nil ||
			taskgorm.NormalizePMWorkTaskKind(parent.Kind) != taskgorm.WorkTaskKindTask {
			fail(c, http.StatusNotFound, "parent task not found")
			return
		}
		if body.RequirementID > 0 && body.RequirementID != parent.RequirementID {
			bad(c, "parent task does not belong to requirement")
			return
		}
		body.RequirementID = parent.RequirementID
		if body.Kind == nil {
			kind := taskgorm.WorkTaskKindSubtask
			body.Kind = &kind
		}
	}
	if body.RequirementID == 0 {
		bad(c, "requirement_id required")
		return
	}
	var requirement requirementgorm.Requirement
	if s.db.Where("id = ? AND workspace_id = ?", body.RequirementID, workspace.ID).
		First(&requirement).Error != nil {
		fail(c, http.StatusNotFound, "requirement not found")
		return
	}
	var project projectgorm.ProjectSet
	if s.db.Where("id = ? AND workspace_id = ?", requirement.ProjectSetID, workspace.ID).
		First(&project).Error != nil {
		fail(c, http.StatusNotFound, "project set not found")
		return
	}
	if err := s.validateWorkspaceHierarchy(workspace.ID, project, requirement); err != nil {
		bad(c, err.Error())
		return
	}
	if !s.requireProjectAccess(c, workspace.ID, project.ID) {
		return
	}
	a, _ := currentActor(c)
	if !body.ReporterUserID.Set {
		reporterID := factUserID(a.User)
		body.ReporterUserID = optionalValue[uint]{Set: true, Value: &reporterID}
	}
	task, err := s.taskFromBody(project, requirement, body)
	if err != nil {
		bad(c, err.Error())
		return
	}
	if err := s.validateActivePMUserReferences(c, taskBodyUserFactIDs(body)); err != nil {
		respondPMUserRegistryValidationError(c, err)
		return
	}
	applyTaskLifecycle(&task, map[string]any{"status": task.Status})
	taskNotificationCtx := workspaceContext{}
	if s.cfg.Notification.Enabled && task.AssigneeID != nil {
		taskNotificationCtx = s.loadWorkspaceContext(
			workspace.ID,
			workspaceContextReferences{}.withTask(task),
		)
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		lockedProject, err := lockProjectSetTx(tx, workspace.ID, project.ID)
		if err != nil {
			return err
		}
		project = lockedProject
		lockedRequirement, err := lockProjectRequirementTx(
			tx, workspace.ID, requirement.ID, project.ID,
		)
		if err != nil {
			return err
		}
		requirement = lockedRequirement
		if !requirementMatchesProject(requirement, project) {
			return errProjectChildScopeChanged
		}
		applyTaskRequirementScope(&task, requirement)
		if task.ParentID != nil {
			parent, err := lockProjectWorkTaskTx(tx, workspace.ID, *task.ParentID, project.ID)
			if err != nil {
				return err
			}
			if parent.TeamID != requirement.TeamID || parent.ProjectID != requirement.PlanUnitID ||
				parent.RequirementID != requirement.ID ||
				taskgorm.NormalizePMWorkTaskKind(parent.Kind) != taskgorm.WorkTaskKindTask {
				return errProjectChildScopeChanged
			}
		}
		key, err := nextTaskKey(tx, project)
		if err != nil {
			return err
		}
		task.TaskKey = key
		if err := s.inheritTaskDueOnCreate(tx, &task); err != nil {
			return err
		}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		if err := tx.Create(&taskgorm.WorkTaskActivity{
			WorkspaceID: workspace.ID, WorkTaskID: task.ID, UserID: factUserID(a.User),
			Action: "created", Summary: "创建了任务",
		}).Error; err != nil {
			return err
		}
		if err := s.recordTaskAssignmentNotification(tx, c, task, taskNotificationCtx); err != nil {
			return err
		}
		return s.recordMutation(tx, c, project.ID, "work_task", task.ID, "created", task.Title, task)
	})
	if err != nil {
		if respondProjectChildScopeConflict(c, err) {
			return
		}
		dbFail(c)
		return
	}
	context := s.loadWorkspaceContext(
		workspace.ID,
		workspaceContextReferences{}.withTask(task),
	)
	hierarchy := s.loadWorkspaceTaskHierarchyContext(workspace.ID, []taskgorm.WorkTask{task})
	row := s.enrichWorkspaceTaskContract(taskDTO(task), task, context, hierarchy)
	row["watched"] = false
	created(c, row)
}

func (s *Server) patchTask(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	task, project, allowed := s.taskByID(c, id)
	if !allowed {
		return
	}
	var body taskBody
	statusRoute := CanonicalRouteTemplate(c.FullPath()) == "/api/v1/platform/tasks/:id/status"
	var decodeErr error
	if statusRoute {
		decodeErr = decodeStrictTaskNativeJSON(c, &body)
	} else {
		decodeErr = c.ShouldBindJSON(&body)
	}
	if decodeErr != nil {
		bad(c, "invalid body")
		return
	}
	actionCode := accesscatalog.ActionWorkItemsTasksUpdate
	statusOnly := taskBodyOnlyStatus(body)
	if statusOnly {
		actionCode = accesscatalog.ActionWorkItemsTasksTransitionStatus
	}
	if statusRoute && !statusOnly {
		bad(c, "status is required and must be the only field")
		return
	}
	if !s.requireTaskCreatorAction(c, actionCode, task) {
		return
	}
	targetProject := project
	movingScope := body.RequirementID > 0 && body.RequirementID != task.RequirementID
	var targetRequirement requirementgorm.Requirement
	if movingScope {
		if taskgorm.NormalizePMWorkTaskKind(task.Kind) == taskgorm.WorkTaskKindSubtask {
			taskMoveNotAllowed(c)
			return
		}
		workspace, _ := currentWorkspace(c)
		if s.db.Where("id = ? AND workspace_id = ?", body.RequirementID, workspace.ID).
			First(&targetRequirement).Error != nil {
			fail(c, http.StatusNotFound, "requirement not found")
			return
		}
		targetProject = projectgorm.ProjectSet{}
		if s.db.Where("id = ? AND workspace_id = ?", targetRequirement.ProjectSetID, workspace.ID).
			First(&targetProject).Error != nil {
			fail(c, http.StatusNotFound, "project set not found")
			return
		}
		if err := s.validateWorkspaceHierarchy(workspace.ID, targetProject, targetRequirement); err != nil {
			bad(c, err.Error())
			return
		}
	}
	updates, err := s.taskUpdates(project, task.RequirementID, task.ID, body)
	if err != nil {
		bad(c, err.Error())
		return
	}
	if err := s.validateActivePMUserReferences(c, taskBodyUserFactIDs(body)); err != nil {
		respondPMUserRegistryValidationError(c, err)
		return
	}
	if movingScope {
		updates["team_id"] = targetRequirement.TeamID
		updates["project_set_id"] = targetRequirement.ProjectSetID
		updates["project_id"] = targetRequirement.PlanUnitID
		updates["requirement_id"] = targetRequirement.ID
	}
	before := task
	after := task
	applyTaskUpdates(&after, updates)
	if movingScope {
		after.TeamID = targetRequirement.TeamID
		after.ProjectSetID = targetRequirement.ProjectSetID
		after.ProjectID = targetRequirement.PlanUnitID
		after.RequirementID = targetRequirement.ID
	}
	applyTaskLifecycle(&after, updates)
	a, _ := currentActor(c)
	taskNotificationCtx := workspaceContext{}
	if s.cfg.Notification.Enabled &&
		taskAssigneeChanged(before.AssigneeID, after.AssigneeID) && after.AssigneeID != nil {
		taskNotificationCtx = s.loadWorkspaceContext(
			after.WorkspaceID,
			workspaceContextReferences{}.withTask(after),
		)
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		lockedProjects, err := lockProjectSetsTx(
			tx, before.WorkspaceID, []uint{project.ID, targetProject.ID},
		)
		if err != nil {
			return err
		}
		lockedProject, exists := lockedProjects[project.ID]
		if !exists {
			return gorm.ErrRecordNotFound
		}
		project = lockedProject
		lockedTargetProject, exists := lockedProjects[targetProject.ID]
		if !exists {
			return gorm.ErrRecordNotFound
		}
		targetProject = lockedTargetProject
		if movingScope {
			lockedRequirement, err := lockProjectRequirementTx(
				tx, before.WorkspaceID, targetRequirement.ID, targetProject.ID,
			)
			if err != nil {
				return err
			}
			targetRequirement = lockedRequirement
			if !requirementMatchesProject(targetRequirement, targetProject) {
				return errProjectChildScopeChanged
			}
			updates["team_id"] = targetRequirement.TeamID
			updates["project_set_id"] = targetRequirement.ProjectSetID
			updates["project_id"] = targetRequirement.PlanUnitID
			updates["requirement_id"] = targetRequirement.ID
		}
		expectedTaskProjects := map[uint]uint{before.ID: project.ID}
		if body.ParentID.Set && body.ParentID.Value != nil {
			expectedParentProjectID := project.ID
			if movingScope {
				expectedParentProjectID = targetProject.ID
			}
			expectedTaskProjects[*body.ParentID.Value] = expectedParentProjectID
		}
		lockedTasks, err := lockProjectWorkTasksTx(tx, before.WorkspaceID, expectedTaskProjects)
		if err != nil {
			return err
		}
		lockedTask := lockedTasks[before.ID]
		if !sameTaskScope(lockedTask, before) {
			return errProjectChildScopeChanged
		}
		if body.ParentID.Set && body.ParentID.Value != nil {
			parent := lockedTasks[*body.ParentID.Value]
			expectedRequirementID := before.RequirementID
			expectedPlanUnitID := before.ProjectID
			expectedTeamID := before.TeamID
			if movingScope {
				expectedRequirementID = targetRequirement.ID
				expectedPlanUnitID = targetRequirement.PlanUnitID
				expectedTeamID = targetRequirement.TeamID
			}
			if parent.RequirementID != expectedRequirementID || parent.ProjectID != expectedPlanUnitID ||
				parent.TeamID != expectedTeamID ||
				taskgorm.NormalizePMWorkTaskKind(parent.Kind) != taskgorm.WorkTaskKindTask {
				return errProjectChildScopeChanged
			}
		}
		if len(updates) > 0 {
			result := tx.Model(&taskgorm.WorkTask{}).
				Where("id = ? AND workspace_id = ?", before.ID, before.WorkspaceID).
				Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
		}
		if movingScope {
			descendants, err := collectTaskDescendants(tx, before.WorkspaceID, before.ID)
			if err != nil {
				return err
			}
			ids := make([]uint, 0, len(descendants))
			for _, descendant := range descendants {
				ids = append(ids, descendant.ID)
			}
			if len(ids) > 0 {
				if err := tx.Model(&taskgorm.WorkTask{}).Where(
					"workspace_id = ? AND id IN ?", before.WorkspaceID, ids,
				).Updates(map[string]any{
					"team_id": targetRequirement.TeamID, "project_set_id": targetRequirement.ProjectSetID,
					"project_id": targetRequirement.PlanUnitID, "requirement_id": targetRequirement.ID,
				}).Error; err != nil {
					return err
				}
			}
		}
		if err := tx.Where("id = ? AND workspace_id = ?", before.ID, before.WorkspaceID).
			First(&after).Error; err != nil {
			return err
		}
		parentChanged := body.ParentID.Set
		if parentChanged && after.DueAt == nil {
			due, err := resolveInheritedTaskDueAt(tx, after.WorkspaceID, after.ParentID)
			if err != nil {
				return err
			}
			if due != nil {
				after.DueAt = due
				updates["due_at"] = due
				if err := tx.Model(&taskgorm.WorkTask{}).Where(
					"id = ? AND workspace_id = ?", after.ID, after.WorkspaceID,
				).Update("due_at", due).Error; err != nil {
					return err
				}
			}
		}
		if (body.DueAt.Set && after.DueAt != nil) || parentChanged {
			if err := s.propagateTaskDueToUnsetDescendants(tx, after, factUserID(a.User)); err != nil {
				return err
			}
		}
		activityRows := taskUpdateActivityRows(before, updates, factUserID(a.User))
		if len(activityRows) > 0 {
			if err := tx.Create(&activityRows).Error; err != nil {
				return err
			}
		}
		if isTerminalTaskStatus(after.Status) {
			if err := s.autoCompleteTaskAncestors(
				tx, after.WorkspaceID, after.ParentID, factUserID(a.User),
			); err != nil {
				return err
			}
		}
		if err := s.recordTaskStatusNotification(tx, after, lockedTask.Status, factUserID(a.User)); err != nil {
			return err
		}
		if taskAssigneeChanged(before.AssigneeID, after.AssigneeID) && after.AssigneeID != nil {
			if err := s.recordTaskAssignmentNotification(tx, c, after, taskNotificationCtx); err != nil {
				return err
			}
		}
		return s.recordMutation(
			tx, c, targetProject.ID, "work_task", after.ID, "updated", after.Title, updates,
		)
	})
	if err != nil {
		if respondProjectChildScopeConflict(c, err) {
			return
		}
		dbFail(c)
		return
	}
	_ = s.db.Where("id = ? AND workspace_id = ?", after.ID, after.WorkspaceID).First(&after).Error
	context := s.loadWorkspaceContext(
		after.WorkspaceID,
		workspaceContextReferences{}.withTask(after),
	)
	hierarchy := s.loadWorkspaceTaskHierarchyContext(after.WorkspaceID, []taskgorm.WorkTask{after})
	row := s.enrichWorkspaceTaskContract(taskDTO(after), after, context, hierarchy)
	row["watched"] = s.userWatchesTask(after.WorkspaceID, after.ID, currentFactUserID(c))
	ok(c, row)
}

func (s *Server) deleteTask(c *gin.Context) {
	id, okID := paramID(c, "id")
	if !okID {
		return
	}
	task, project, allowed := s.taskByID(c, id)
	if !allowed {
		return
	}
	if !s.requireTaskCreatorAction(c, accesscatalog.ActionWorkItemsTasksDelete, task) {
		return
	}
	descendants, err := collectTaskDescendants(s.db, task.WorkspaceID, task.ID)
	if err != nil {
		dbFail(c)
		return
	}
	if len(descendants) > 0 && c.Query("cascade") != "true" {
		fail(c, http.StatusConflict, "task has child tasks")
		return
	}
	deletedTasks := append(descendants, task)
	taskIDs := make([]uint, 0, len(deletedTasks))
	for _, deletedTask := range deletedTasks {
		taskIDs = append(taskIDs, deletedTask.ID)
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := deleteWorkTaskDependents(tx, task.WorkspaceID, taskIDs); err != nil {
			return err
		}
		for _, deletedTask := range deletedTasks {
			if err := s.recordMutation(
				tx, c, project.ID, "work_task", deletedTask.ID, "deleted", deletedTask.Title, deletedTask,
			); err != nil {
				return err
			}
		}
		result := tx.Where("id IN ? AND workspace_id = ?", taskIDs, task.WorkspaceID).
			Delete(&taskgorm.WorkTask{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != int64(len(taskIDs)) {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		dbFail(c)
		return
	}
	ok(c, gin.H{"deleted": true, "deleted_count": len(taskIDs)})
}
