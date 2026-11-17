package httpapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

func (s *Server) cloneNativeTask(c *gin.Context) {
	id, valid := paramID(c, "id")
	if !valid {
		return
	}
	source, sourceProject, allowed := s.taskByID(c, id)
	if !allowed {
		return
	}
	var body nativeTaskCloneBody
	if err := decodeStrictTaskNativeJSON(c, &body); err != nil && !errors.Is(err, io.EOF) {
		bad(c, "invalid body")
		return
	}
	workspace, _ := currentWorkspace(c)
	targetRequirementID := source.RequirementID
	if body.TargetRequirementID != nil {
		if *body.TargetRequirementID == 0 {
			bad(c, "target_requirement_id must be positive")
			return
		}
		targetRequirementID = *body.TargetRequirementID
	}
	if targetRequirementID != source.RequirementID &&
		taskgorm.NormalizePMWorkTaskKind(source.Kind) == taskgorm.WorkTaskKindSubtask {
		bad(c, "subtask clone cannot move to another requirement")
		return
	}
	var requirement requirementgorm.Requirement
	if err := s.db.Where(
		"id = ? AND workspace_id = ?", targetRequirementID, workspace.ID,
	).First(&requirement).Error; err != nil {
		fail(c, http.StatusNotFound, "target requirement not found")
		return
	}
	targetProject := sourceProject
	if targetProject.ID != requirement.ProjectSetID {
		if err := s.db.Where(
			"id = ? AND workspace_id = ?", requirement.ProjectSetID, workspace.ID,
		).First(&targetProject).Error; err != nil {
			fail(c, http.StatusNotFound, "target project set not found")
			return
		}
	}
	if err := s.validateWorkspaceHierarchy(workspace.ID, targetProject, requirement); err != nil {
		bad(c, err.Error())
		return
	}
	// Gate target project access when cloning across projects.
	if targetProject.ID != sourceProject.ID {
		if !s.requireProjectAccess(c, workspace.ID, targetProject.ID) {
			return
		}
	}
	title := strings.TrimSpace(source.Title) + nativeTaskCloneSuffix(body)
	if strings.TrimSpace(title) == "" || utf8.RuneCountInString(title) > 220 {
		bad(c, "cloned task title is invalid")
		return
	}
	clone := taskgorm.WorkTask{
		WorkspaceID: workspace.ID, TeamID: targetProject.TeamID, ProjectSetID: targetProject.ID,
		ProjectID: requirement.PlanUnitID, RequirementID: requirement.ID,
		Kind: source.Kind, Title: title, Description: source.Description,
		Status: "draft", Priority: source.Priority,
		AssigneeID: source.AssigneeID, ReporterUserID: source.ReporterUserID,
		EstimatedMinutes: source.EstimatedMinutes,
		StartsAt:         source.StartsAt, DueAt: source.DueAt,
	}
	if targetRequirementID == source.RequirementID {
		clone.ParentID = source.ParentID
	}
	if err := s.validateActivePMUserReferences(c, taskCloneUserFactIDs(clone)); err != nil {
		respondPMUserRegistryValidationError(c, err)
		return
	}
	clone.CompletedAt = nil
	actor, _ := currentActor(c)
	notificationContext := workspaceContext{}
	if s.cfg.Notification.Enabled && clone.AssigneeID != nil {
		notificationContext = s.loadWorkspaceContext(
			workspace.ID,
			workspaceContextReferences{}.withTask(clone),
		)
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		key, err := nextTaskKey(tx, targetProject)
		if err != nil {
			return err
		}
		clone.TaskKey = key
		if err := s.inheritTaskDueOnCreate(tx, &clone); err != nil {
			return err
		}
		if err := tx.Create(&clone).Error; err != nil {
			return err
		}
		if err := tx.Create(&taskgorm.WorkTaskActivity{
			WorkspaceID: clone.WorkspaceID, WorkTaskID: clone.ID, UserID: factUserID(actor.User),
			Action: "cloned", Detail: fmt.Sprintf("source_task_id=%d", source.ID),
			Summary: fmt.Sprintf("克隆自任务 %s", source.TaskKey),
		}).Error; err != nil {
			return err
		}
		if err := s.recordTaskAssignmentNotification(tx, c, clone, notificationContext); err != nil {
			return err
		}
		return s.recordMutation(
			tx, c, targetProject.ID, "work_task", clone.ID, "created", clone.Title,
			gin.H{"task": taskDTO(clone), "cloned_from_task_id": source.ID},
		)
	})
	if err != nil {
		dbFail(c)
		return
	}
	context := s.loadWorkspaceContext(
		workspace.ID,
		workspaceContextReferences{}.withTask(clone),
	)
	hierarchy := s.loadWorkspaceTaskHierarchyContext(workspace.ID, []taskgorm.WorkTask{clone})
	row := s.enrichWorkspaceTaskContract(taskDTO(clone), clone, context, hierarchy)
	row["watched"] = false
	row["cloned_from"] = gin.H{"task_key": source.TaskKey, "title": source.Title}
	created(c, row)
}

func taskCloneUserFactIDs(task taskgorm.WorkTask) []uint {
	ids := make([]uint, 0, 2)
	for _, id := range []*uint{task.AssigneeID, task.ReporterUserID} {
		if id != nil && *id > 0 {
			ids = append(ids, *id)
		}
	}
	return ids
}
