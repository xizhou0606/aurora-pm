package httpapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	identitygorm "github.com/xizhou0606/aurora-pm/backend/internal/module/identity/gorm"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

const (
	maxBatchParentTasks = 50
	maxBatchTotalTasks  = 200
)

type taskBatchCreateBody struct {
	Tasks []taskBatchParentBody `json:"tasks"`
}

type taskBatchParentBody struct {
	ClientID         string                `json:"client_id"`
	RequirementID    uint                  `json:"requirement_id"`
	TaskClientID     optionalValue[uint]   `json:"task_client_id"`
	Title            string                `json:"title"`
	Description      *string               `json:"description"`
	Status           *string               `json:"status"`
	Priority         *string               `json:"priority"`
	AssigneeID       optionalValue[uint]   `json:"assignee_id"`
	ReporterUserID   optionalValue[uint]   `json:"reporter_user_id"`
	EstimatedMinutes optionalValue[int]    `json:"estimated_minutes"`
	StartsAt         optionalValue[string] `json:"starts_at"`
	DueAt            optionalValue[string] `json:"due_at"`
	Children         []taskBatchChildBody  `json:"children"`
}

type taskBatchChildBody struct {
	ClientID         string                `json:"client_id"`
	Title            string                `json:"title"`
	TaskClientID     optionalValue[uint]   `json:"task_client_id"`
	Description      *string               `json:"description"`
	Status           *string               `json:"status"`
	Priority         *string               `json:"priority"`
	AssigneeID       optionalValue[uint]   `json:"assignee_id"`
	ReporterUserID   optionalValue[uint]   `json:"reporter_user_id"`
	EstimatedMinutes optionalValue[int]    `json:"estimated_minutes"`
	StartsAt         optionalValue[string] `json:"starts_at"`
	DueAt            optionalValue[string] `json:"due_at"`
}

type preparedTaskBatch struct {
	ClientID string
	Project  projectgorm.ProjectSet
	Parent   taskgorm.WorkTask
	Children []preparedTaskBatchChild
}

type preparedTaskBatchChild struct {
	ClientID string
	Task     taskgorm.WorkTask
}

func (s *Server) batchCreateTasks(c *gin.Context) {
	workspace, _ := currentWorkspace(c)
	actor, _ := currentActor(c)
	var body taskBatchCreateBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	prepared, total, err := s.prepareTaskBatch(c, workspace, actor, body)
	if err != nil {
		var validation *taskBatchValidationError
		if errors.As(err, &validation) {
			c.AbortWithStatusJSON(http.StatusUnprocessableEntity, gin.H{
				"code": apiErrorTaskBatchValidationFailed, "error": "batch task validation failed",
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
	// Gate every unique project resolved from the batch.
	checkedProjects := map[uint]bool{}
	for _, batch := range prepared {
		if !checkedProjects[batch.Project.ID] {
			if !s.requireProjectAccess(c, workspace.ID, batch.Project.ID) {
				return
			}
			checkedProjects[batch.Project.ID] = true
		}
	}
	notificationContext := workspaceContext{}
	if s.cfg.Notification.Enabled {
		notificationTasks := make([]taskgorm.WorkTask, 0, total)
		for _, batch := range prepared {
			notificationTasks = append(notificationTasks, batch.Parent)
			for _, child := range batch.Children {
				notificationTasks = append(notificationTasks, child.Task)
			}
		}
		notificationContext = s.loadWorkspaceContext(
			workspace.ID,
			workspaceContextReferences{}.withTasks(notificationTasks),
		)
	}
	createdRows := make([]gin.H, 0, len(prepared))
	err = s.db.Transaction(func(tx *gorm.DB) error {
		projectIDs := make([]uint, 0, len(prepared))
		expectedRequirementProjects := make(map[uint]uint, len(prepared))
		for index := range prepared {
			projectIDs = append(projectIDs, prepared[index].Project.ID)
			requirementID := prepared[index].Parent.RequirementID
			if expectedProjectID, exists := expectedRequirementProjects[requirementID]; exists && expectedProjectID != prepared[index].Project.ID {
				return errProjectChildScopeChanged
			}
			expectedRequirementProjects[requirementID] = prepared[index].Project.ID
		}
		lockedProjects, lockErr := lockProjectSetsTx(tx, workspace.ID, projectIDs)
		if lockErr != nil {
			return lockErr
		}
		lockedRequirements, lockErr := lockProjectRequirementsTx(
			tx, workspace.ID, expectedRequirementProjects,
		)
		if lockErr != nil {
			return lockErr
		}
		for index := range prepared {
			lockedProject, exists := lockedProjects[prepared[index].Project.ID]
			if !exists {
				return gorm.ErrRecordNotFound
			}
			prepared[index].Project = lockedProject
			lockedRequirement, exists := lockedRequirements[prepared[index].Parent.RequirementID]
			if !exists {
				return errProjectChildScopeChanged
			}
			if !requirementMatchesProject(lockedRequirement, lockedProject) {
				return errProjectChildScopeChanged
			}
			applyTaskRequirementScope(&prepared[index].Parent, lockedRequirement)
			for childIndex := range prepared[index].Children {
				applyTaskRequirementScope(&prepared[index].Children[childIndex].Task, lockedRequirement)
			}
			row, createErr := s.createPreparedTaskBatch(
				tx, c, actor, notificationContext, &prepared[index],
			)
			if createErr != nil {
				return createErr
			}
			createdRows = append(createdRows, row)
		}
		return nil
	})
	if err != nil {
		if respondProjectChildScopeConflict(c, err) {
			return
		}
		dbFail(c)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": gin.H{"created": createdRows, "total": total}})
}

func (s *Server) prepareTaskBatch(
	c *gin.Context,
	workspace identitygorm.Workspace,
	authActor actor,
	body taskBatchCreateBody,
) ([]preparedTaskBatch, int, error) {
	if len(body.Tasks) == 0 {
		return nil, 0, batchFieldError("", "tasks", validationRequired, "tasks required")
	}
	if len(body.Tasks) > maxBatchParentTasks {
		return nil, 0, batchFieldError("", "tasks", validationMaximumParentTasks,
			fmt.Sprintf("parent task count cannot exceed %d", maxBatchParentTasks))
	}
	total := len(body.Tasks)
	for _, parent := range body.Tasks {
		total += len(parent.Children)
	}
	if total > maxBatchTotalTasks {
		return nil, 0, batchFieldError("", "tasks", validationMaximumTotalTasks,
			fmt.Sprintf("total task count cannot exceed %d", maxBatchTotalTasks))
	}
	seenClientIDs := make(map[string]struct{}, total)
	userIDs := make([]uint, 0, total*2)
	userFields := make(map[uint][]taskBatchUserField, total*2)
	prepared := make([]preparedTaskBatch, 0, len(body.Tasks))
	projects := make(map[uint]projectgorm.ProjectSet)
	requirements := make(map[uint]requirementgorm.Requirement)
	reporterID := factUserID(authActor.User)
	for _, parentBody := range body.Tasks {
		clientID, err := reserveTaskBatchClientID(seenClientIDs, parentBody.ClientID)
		if err != nil {
			return nil, 0, err
		}
		project, requirement, err := s.taskBatchScope(
			workspace.ID, parentBody.RequirementID, projects, requirements,
		)
		if err != nil {
			return nil, 0, batchFieldError(clientID, "requirement_id", validationInvalidScope, err.Error())
		}
		parentBody, err = normalizeTaskBatchParent(clientID, parentBody, reporterID)
		if err != nil {
			return nil, 0, err
		}
		parent, err := s.taskFromBody(project, requirement, taskBatchParentTaskBody(parentBody, reporterID))
		if err != nil {
			return nil, 0, taskBatchInputError(clientID, err)
		}
		applyTaskLifecycle(&parent, map[string]any{"status": parent.Status})
		userIDs = appendTaskBatchUserFacts(userIDs, userFields, clientID, parent)
		children := make([]preparedTaskBatchChild, 0, len(parentBody.Children))
		for _, childBody := range parentBody.Children {
			childClientID, childErr := reserveTaskBatchClientID(seenClientIDs, childBody.ClientID)
			if childErr != nil {
				return nil, 0, childErr
			}
			childBody, childErr = normalizeTaskBatchChild(childClientID, childBody, parentBody)
			if childErr != nil {
				return nil, 0, childErr
			}
			child, childErr := s.taskFromBody(
				project, requirement, taskBatchChildTaskBody(childBody, reporterID),
			)
			if childErr != nil {
				return nil, 0, taskBatchInputError(childClientID, childErr)
			}
			child.Kind = taskgorm.WorkTaskKindSubtask
			applyTaskLifecycle(&child, map[string]any{"status": child.Status})
			userIDs = appendTaskBatchUserFacts(userIDs, userFields, childClientID, child)
			children = append(children, preparedTaskBatchChild{ClientID: childClientID, Task: child})
		}
		prepared = append(prepared, preparedTaskBatch{
			ClientID: clientID, Project: project, Parent: parent, Children: children,
		})
	}
	if err := s.validateActivePMUserReferences(c, userIDs); err != nil {
		if errors.Is(err, errPMUserRegistryUnavailable) {
			return nil, 0, err
		}
		return nil, 0, taskBatchUserError(err, userFields)
	}
	return prepared, total, nil
}

func (s *Server) taskBatchScope(
	workspaceID, requirementID uint,
	projects map[uint]projectgorm.ProjectSet,
	requirements map[uint]requirementgorm.Requirement,
) (projectgorm.ProjectSet, requirementgorm.Requirement, error) {
	if requirementID == 0 {
		return projectgorm.ProjectSet{}, requirementgorm.Requirement{}, fmt.Errorf("requirement_id required")
	}
	requirement, exists := requirements[requirementID]
	if !exists {
		if err := s.db.Where("id = ? AND workspace_id = ?", requirementID, workspaceID).
			First(&requirement).Error; err != nil {
			return projectgorm.ProjectSet{}, requirementgorm.Requirement{}, fmt.Errorf("requirement not found")
		}
		requirements[requirementID] = requirement
	}
	project, exists := projects[requirement.ProjectSetID]
	if !exists {
		if err := s.db.Where("id = ? AND workspace_id = ?", requirement.ProjectSetID, workspaceID).
			First(&project).Error; err != nil {
			return projectgorm.ProjectSet{}, requirementgorm.Requirement{}, fmt.Errorf("project set not found")
		}
		projects[project.ID] = project
	}
	if err := s.validateWorkspaceHierarchy(workspaceID, project, requirement); err != nil {
		return projectgorm.ProjectSet{}, requirementgorm.Requirement{}, err
	}
	return project, requirement, nil
}

func (s *Server) createPreparedTaskBatch(
	tx *gorm.DB,
	c *gin.Context,
	authActor actor,
	notificationContext workspaceContext,
	batch *preparedTaskBatch,
) (gin.H, error) {
	if err := s.createPreparedTask(tx, c, authActor, notificationContext, batch.Project, &batch.Parent); err != nil {
		return nil, err
	}
	children := make([]gin.H, 0, len(batch.Children))
	for index := range batch.Children {
		child := &batch.Children[index]
		parentID := batch.Parent.ID
		child.Task.ParentID = &parentID
		if err := s.createPreparedTask(tx, c, authActor, notificationContext, batch.Project, &child.Task); err != nil {
			return nil, err
		}
		children = append(children, taskBatchCreatedRow(child.ClientID, child.Task, nil))
	}
	return taskBatchCreatedRow(batch.ClientID, batch.Parent, children), nil
}

func (s *Server) createPreparedTask(
	tx *gorm.DB,
	c *gin.Context,
	authActor actor,
	notificationContext workspaceContext,
	project projectgorm.ProjectSet,
	task *taskgorm.WorkTask,
) error {
	key, err := nextTaskKey(tx, project)
	if err != nil {
		return err
	}
	task.TaskKey = key
	if err := s.inheritTaskDueOnCreate(tx, task); err != nil {
		return err
	}
	if err := tx.Create(task).Error; err != nil {
		return err
	}
	if err := tx.Create(&taskgorm.WorkTaskActivity{
		WorkspaceID: task.WorkspaceID, WorkTaskID: task.ID, UserID: factUserID(authActor.User),
		Action: "created", Summary: "创建了任务",
	}).Error; err != nil {
		return err
	}
	if err := s.recordTaskAssignmentNotification(tx, c, *task, notificationContext); err != nil {
		return err
	}
	return s.recordMutation(tx, c, project.ID, "work_task", task.ID, "created", task.Title, task)
}

func taskBatchCreatedRow(clientID string, task taskgorm.WorkTask, children []gin.H) gin.H {
	row := gin.H{"client_id": clientID, "id": task.ID, "task_key": task.TaskKey}
	if children != nil {
		row["children"] = children
	}
	return row
}

func reserveTaskBatchClientID(seen map[string]struct{}, raw string) (string, error) {
	clientID := strings.TrimSpace(raw)
	if clientID == "" {
		return "", batchFieldError("", "client_id", validationRequired, "client_id required")
	}
	if len(clientID) > 128 {
		return "", batchFieldError(clientID, "client_id", validationMaximumLength, "client_id is too long")
	}
	if _, exists := seen[clientID]; exists {
		return "", batchFieldError(clientID, "client_id", validationDuplicate, "client_id must be unique")
	}
	seen[clientID] = struct{}{}
	return clientID, nil
}
