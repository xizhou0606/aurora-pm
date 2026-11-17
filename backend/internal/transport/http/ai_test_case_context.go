package httpapi

import (
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"gorm.io/gorm"

	// The runtime generates at most twelve independently validated batches. These
	// admission limits are part of the supported complex-generation contract;
	// larger scopes must be split into separate version runs instead of being
	// silently truncated or sent as one oversized model request.
	taskgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/task/gorm"
)

const (
	aiTestCaseRequirementLimit = 20
	aiTestCaseTaskLimit        = 100
	aiTestCaseContextByteLimit = 1 << 20
	aiTestCaseContextItemLimit = 128 << 10
)

func (s *Server) aiTestCaseContext(tx *gorm.DB, workspaceID uint, subjectType string, subjectID uint) (gin.H, error) {
	requirements := make([]requirementgorm.Requirement, 0)
	singleRequirementDocument := false
	switch subjectType {
	case "requirement":
		var row requirementgorm.Requirement
		if tx.Where("id = ? AND workspace_id = ?", subjectID, workspaceID).First(&row).Error != nil {
			return nil, fmt.Errorf("requirement not found")
		}
		requirements = append(requirements, row)
		singleRequirementDocument = true
	case "plan_unit":
		if err := tx.Where("workspace_id = ? AND version_task_new_id = ?", workspaceID, subjectID).
			Order("id").Limit(aiTestCaseRequirementLimit + 1).Find(&requirements).Error; err != nil {
			return nil, err
		}
		if len(requirements) > aiTestCaseRequirementLimit {
			return nil, fmt.Errorf("test case context exceeds the requirement limit of %d", aiTestCaseRequirementLimit)
		}
	default:
		return nil, fmt.Errorf("test cases do not support subject")
	}
	requirementIDs := make([]uint, 0, len(requirements))
	items := make([]gin.H, 0, len(requirements))
	for _, requirement := range requirements {
		requirementIDs = append(requirementIDs, requirement.ID)
		snapshot := requirementAISnapshot(requirement)
		if err := validateAITestCaseContextItem("requirement", snapshot); err != nil {
			return nil, err
		}
		items = append(items, snapshot)
	}
	tasks := make([]taskgorm.WorkTask, 0)
	if !singleRequirementDocument && len(requirementIDs) > 0 {
		if err := tx.Select("id", "requirement_id", "task_key", "title", "description", "status", "priority", "updated_at").
			Where("workspace_id = ? AND requirement_id IN ?", workspaceID, requirementIDs).
			Order("id").Limit(aiTestCaseTaskLimit + 1).Find(&tasks).Error; err != nil {
			return nil, err
		}
		if len(tasks) > aiTestCaseTaskLimit {
			return nil, fmt.Errorf("test case context exceeds the task limit of %d", aiTestCaseTaskLimit)
		}
	}
	taskItems := make([]gin.H, 0, len(tasks))
	for _, task := range tasks {
		snapshot := gin.H{
			"id": task.ID, "requirement_id": task.RequirementID, "key": task.TaskKey,
			"title": task.Title, "description": task.Description, "status": task.Status, "priority": task.Priority,
			"updated_at": task.UpdatedAt.UTC().Format("2006-01-02T15:04:05.000000000Z"),
		}
		if err := validateAITestCaseContextItem("task", snapshot); err != nil {
			return nil, err
		}
		taskItems = append(taskItems, snapshot)
	}
	context := gin.H{
		"requirements": items,
		"tasks":        taskItems,
		"counts": gin.H{
			"requirements": len(items),
			"tasks":        len(taskItems),
		},
		"coverageRequired": true,
	}
	if singleRequirementDocument {
		context["mode"] = "single_requirement_document"
	}
	raw, err := json.Marshal(context)
	if err != nil {
		return nil, fmt.Errorf("encode test case context: %w", err)
	}
	if len(raw) > aiTestCaseContextByteLimit {
		return nil, fmt.Errorf("test case context exceeds the supported 1 MiB input limit; split the plan unit into smaller runs")
	}
	return context, nil
}

func validateAITestCaseContextItem(kind string, item gin.H) error {
	raw, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("encode test case %s: %w", kind, err)
	}
	if len(raw) > aiTestCaseContextItemLimit {
		return fmt.Errorf("test case %s exceeds the supported 128 KiB item limit; split the source material", kind)
	}
	return nil
}
