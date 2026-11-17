package httpapi

import (
	"fmt"
	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	requirementgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/requirement/gorm"
	"strings"
)

func (s *Server) requirementFromBody(
	project projectgorm.ProjectSet,
	unit projectgorm.PlanUnit,
	body requirementBody,
) (requirementgorm.Requirement, error) {
	if body.Status.Set {
		return requirementgorm.Requirement{}, fmt.Errorf(workflowManagedStatusMessage)
	}
	if err := validateRequirementUserIDs(body); err != nil {
		return requirementgorm.Requirement{}, err
	}
	// 需求优先级为创建必填字段（P0–P3），缺省不再静默回填。
	if body.Priority == nil || strings.TrimSpace(*body.Priority) == "" {
		return requirementgorm.Requirement{}, fmt.Errorf("priority is required")
	}
	if !validPriority(*body.Priority) {
		return requirementgorm.Requirement{}, fmt.Errorf("priority is invalid")
	}
	priority := normalizePriority(*body.Priority)
	requirement := requirementgorm.Requirement{
		WorkspaceID: project.WorkspaceID, TeamID: project.TeamID,
		ProjectSetID: project.ID, PlanUnitID: unit.ID, Title: strings.TrimSpace(*body.Title),
		Description: stringValue(body.Description), ProductDocURL: stringValue(body.ProductDocURL),
		DesignURL:          stringValue(body.DesignURL),
		DeveloperUserIDs:   uintListValue(body.DeveloperUserIDs),
		AcceptanceCriteria: stringValue(body.AcceptanceCriteria), Priority: priority, Status: "backlog",
	}
	applyRequirementOwnerFields(&requirement, body)
	return requirement, nil
}

func (s *Server) requirementUpdates(project projectgorm.ProjectSet, body requirementBody) (map[string]any, error) {
	if body.Status.Set {
		return nil, fmt.Errorf(workflowManagedStatusMessage)
	}
	if err := validateRequirementUserIDs(body); err != nil {
		return nil, err
	}
	updates := map[string]any{}
	for key, value := range map[string]*string{
		"title": body.Title, "description": body.Description, "product_doc_url": body.ProductDocURL,
		"design_url": body.DesignURL, "acceptance_criteria": body.AcceptanceCriteria,
	} {
		if value != nil {
			updates[key] = strings.TrimSpace(*value)
		}
	}
	applyRequirementOwnerUpdates(updates, body)
	if body.DeveloperUserIDs != nil {
		updates["developer_user_ids"] = requirementgorm.UniqueOwnerIDs(requirementgorm.UintList(*body.DeveloperUserIDs))
	}
	if body.Priority != nil {
		if !validPriority(*body.Priority) {
			return nil, fmt.Errorf("priority is invalid")
		}
		updates["priority"] = normalizePriority(*body.Priority)
	}
	return updates, nil
}

func validateRequirementUserIDs(body requirementBody) error {
	var userIDs []uint
	userIDs = appendOwnerIDs(userIDs, body.ProductOwnerUserIDs, body.ProductOwnerUserID)
	userIDs = appendOwnerIDs(userIDs, body.DesignOwnerUserIDs, body.DesignOwnerUserID)
	userIDs = appendOwnerIDs(userIDs, body.FrontendOwnerUserIDs, body.FrontendOwnerUserID)
	userIDs = appendOwnerIDs(userIDs, body.BackendOwnerUserIDs, body.BackendOwnerUserID)
	if body.TesterUserIDs != nil {
		userIDs = append(userIDs, (*body.TesterUserIDs)...)
	}
	if body.DeveloperUserIDs != nil {
		userIDs = append(userIDs, (*body.DeveloperUserIDs)...)
	}
	for _, userID := range userIDs {
		if userID == 0 {
			return fmt.Errorf("assigned user IDs must be positive")
		}
	}
	return nil
}

func uintListValue(value *[]uint) requirementgorm.UintList {
	if value == nil {
		return requirementgorm.UintList{}
	}
	return requirementgorm.UintList(*value)
}
