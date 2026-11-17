package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
	"gorm.io/gorm"
)

func (s *Server) getTaskTestDefinition(c *gin.Context) {
	taskID, valid := paramID(c, "id")
	if !valid {
		return
	}
	task, _, allowed := s.taskByID(c, taskID)
	if !allowed {
		return
	}
	var row taskgorm.TaskTestDefinition
	if err := s.db.Where(
		"workspace_id = ? AND work_task_id = ?", task.WorkspaceID, task.ID,
	).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fail(c, http.StatusNotFound, "task test definition not found")
			return
		}
		dbFail(c)
		return
	}
	result := taskTestDefinitionDTO(row)
	result["task"] = s.taskTestingTaskContract(task)
	ok(c, result)
}

func (s *Server) putTaskTestDefinition(c *gin.Context) {
	taskID, valid := paramID(c, "id")
	if !valid {
		return
	}
	task, project, allowed := s.taskByID(c, taskID)
	if !allowed {
		return
	}
	var body taskTestDefinitionBody
	if c.ShouldBindJSON(&body) != nil {
		bad(c, "invalid body")
		return
	}
	body, err := normalizeTaskTestDefinitionBody(body)
	if err != nil {
		bad(c, err.Error())
		return
	}
	casesJSON, err := json.Marshal(body.Cases)
	if err != nil {
		bad(c, "cases are invalid")
		return
	}
	actorID := currentFactUserID(c)
	var result taskgorm.TaskTestDefinition
	createdRow := false
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var current taskgorm.TaskTestDefinition
		loadErr := tx.Where(
			"workspace_id = ? AND work_task_id = ?", task.WorkspaceID, task.ID,
		).First(&current).Error
		switch {
		case errors.Is(loadErr, gorm.ErrRecordNotFound):
			if body.Version != nil && *body.Version != 0 {
				return errTaskTestVersionConflict
			}
			result = taskgorm.TaskTestDefinition{
				WorkspaceID: task.WorkspaceID, WorkTaskID: task.ID, Version: 1,
				Purpose: body.Purpose, Preconditions: body.Preconditions,
				EnvironmentGuidance: body.EnvironmentGuidance, CasesJSON: string(casesJSON),
				CreatedByUserID: actorID, UpdatedByUserID: actorID,
			}
			if err := tx.Create(&result).Error; err != nil {
				return err
			}
			createdRow = true
			return s.recordMutation(
				tx, c, project.ID, "task_test_definition", result.ID, "created", task.Title,
				taskTestDefinitionDTO(result),
			)
		case loadErr != nil:
			return loadErr
		}
		if body.Version == nil || *body.Version != current.Version {
			return errTaskTestVersionConflict
		}
		now := time.Now().UTC()
		nextVersion := current.Version + 1
		updates := map[string]any{
			"version": nextVersion, "purpose": body.Purpose,
			"preconditions":        body.Preconditions,
			"environment_guidance": body.EnvironmentGuidance,
			"cases_json":           string(casesJSON), "updated_by_user_id": actorID,
			"updated_at": now,
		}
		updated := tx.Model(&taskgorm.TaskTestDefinition{}).
			Where("id = ? AND workspace_id = ? AND work_task_id = ? AND version = ?",
				current.ID, task.WorkspaceID, task.ID, current.Version).
			Updates(updates)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errTaskTestVersionConflict
		}
		current.Version = nextVersion
		current.Purpose = body.Purpose
		current.Preconditions = body.Preconditions
		current.EnvironmentGuidance = body.EnvironmentGuidance
		current.CasesJSON = string(casesJSON)
		current.UpdatedByUserID = actorID
		current.UpdatedAt = now
		result = current
		return s.recordMutation(
			tx, c, project.ID, "task_test_definition", result.ID, "updated", task.Title,
			taskTestDefinitionDTO(result),
		)
	})
	if errors.Is(err, errTaskTestVersionConflict) {
		fail(c, http.StatusConflict, "task test definition version conflict")
		return
	}
	if err != nil {
		dbFail(c)
		return
	}
	if createdRow {
		response := taskTestDefinitionDTO(result)
		response["task"] = s.taskTestingTaskContract(task)
		created(c, response)
		return
	}
	response := taskTestDefinitionDTO(result)
	response["task"] = s.taskTestingTaskContract(task)
	ok(c, response)
}
