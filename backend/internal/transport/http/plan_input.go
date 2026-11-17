package httpapi

import (
	"fmt"
	"strings"
	"time"

	projectgorm "github.com/xizhou0606/aurora-pm/backend/internal/module/project/gorm"
	"gorm.io/gorm"
)

func (s *Server) planUnitFromBody(project projectgorm.ProjectSet, body planUnitBody) (projectgorm.PlanUnit, error) {
	if body.Status.Set {
		return projectgorm.PlanUnit{}, fmt.Errorf(workflowManagedStatusMessage)
	}
	updates, err := s.planUnitUpdates(project, body)
	if err != nil {
		return projectgorm.PlanUnit{}, err
	}
	unit := projectgorm.PlanUnit{
		WorkspaceID: project.WorkspaceID, TeamID: project.TeamID, ProjectSetID: project.ID,
		Title: strings.TrimSpace(*body.Title), Type: "version", Status: "planning",
	}
	applyPlanUnitUpdates(&unit, updates)
	return unit, nil
}

func applyPlanUnitUpdates(unit *projectgorm.PlanUnit, updates map[string]any) {
	if value, ok := updates["title"].(string); ok {
		unit.Title = value
	}
	if value, ok := updates["goal"].(string); ok {
		unit.Goal = value
	}
	if value, ok := updates["type"].(string); ok && value != "" {
		unit.Type = value
	}
	if value, ok := updates["risk_level"].(string); ok {
		unit.RiskLevel = value
	}
	if value, exists := updates["owner_user_id"]; exists {
		unit.OwnerUserID, _ = value.(*uint)
	}
	assignPlanTime := func(key string, target **time.Time) {
		if value, exists := updates[key]; exists {
			*target, _ = value.(*time.Time)
		}
	}
	assignPlanTime("planned_start_at", &unit.PlannedStartAt)
	assignPlanTime("planned_end_at", &unit.PlannedEndAt)
	assignPlanTime("design_freeze_due_at", &unit.DesignFreezeDueAt)
	assignPlanTime("development_complete_due_at", &unit.DevelopmentCompleteAt)
	assignPlanTime("test_acceptance_due_at", &unit.TestAcceptanceDueAt)
	assignPlanTime("actual_start_at", &unit.ActualStartAt)
	assignPlanTime("actual_completed_at", &unit.ActualCompletedAt)
}

func planUnitScheduleDateCASConflicts(
	expectations []scheduleDateExpectation,
	unit projectgorm.PlanUnit,
) []workflowScheduleConflictItem {
	return scheduleDateStaleConflicts(expectations, map[string]*time.Time{
		"planned_start_at":            unit.PlannedStartAt,
		"planned_end_at":              unit.PlannedEndAt,
		"design_freeze_due_at":        unit.DesignFreezeDueAt,
		"development_complete_due_at": unit.DevelopmentCompleteAt,
		"test_acceptance_due_at":      unit.TestAcceptanceDueAt,
		"actual_start_at":             unit.ActualStartAt,
		"actual_completed_at":         unit.ActualCompletedAt,
	}, "plan_unit", unit.ID, "")
}

func (s *Server) planUnitUpdates(project projectgorm.ProjectSet, body planUnitBody) (map[string]any, error) {
	if body.Status.Set {
		return nil, fmt.Errorf(workflowManagedStatusMessage)
	}
	updates := map[string]any{}
	if body.Title != nil && strings.TrimSpace(*body.Title) != "" {
		updates["title"] = strings.TrimSpace(*body.Title)
	}
	for key, value := range map[string]*string{
		"goal": body.Goal, "type": body.Type, "risk_level": body.RiskLevel,
	} {
		if value != nil {
			updates[key] = strings.TrimSpace(*value)
		}
	}
	if body.OwnerUserID.Set {
		if body.OwnerUserID.Value != nil && *body.OwnerUserID.Value == 0 {
			return nil, fmt.Errorf("owner_user_id must be positive")
		}
		updates["owner_user_id"] = body.OwnerUserID.Value
	}
	dates := map[string]optionalValue[string]{
		"planned_start_at": body.PlannedStartAt, "planned_end_at": body.PlannedEndAt,
		"design_freeze_due_at":        body.DesignFreezeDueAt,
		"development_complete_due_at": body.DevelopmentCompleteAt,
		"test_acceptance_due_at":      body.TestAcceptanceDueAt,
		"actual_start_at":             body.ActualStartAt, "actual_completed_at": body.ActualCompletedAt,
	}
	for key, value := range dates {
		if !value.Set {
			continue
		}
		parsed, err := parseOptionalTime(value.Value)
		if err != nil {
			return nil, err
		}
		updates[key] = parsed
	}
	return updates, nil
}

func (s *Server) replaceDeliveryNodes(
	tx *gorm.DB,
	project projectgorm.ProjectSet,
	unit projectgorm.PlanUnit,
	inputs []deliveryNodeBody,
) error {
	if err := tx.Where("workspace_id = ? AND plan_unit_id = ?", project.WorkspaceID, unit.ID).
		Delete(&projectgorm.DeliveryNode{}).Error; err != nil {
		return err
	}
	for _, input := range inputs {
		if input.OwnerUserID.Value != nil && *input.OwnerUserID.Value == 0 {
			return fmt.Errorf("delivery owner_user_id must be positive")
		}
		if input.LabelID != nil {
			var count int64
			tx.Model(&projectgorm.DeliveryNodeLabel{}).
				Where("id = ? AND workspace_id = ? AND project_set_id = ? AND plan_unit_id = ?",
					*input.LabelID, project.WorkspaceID, project.ID, unit.ID).Count(&count)
			if count != 1 {
				return fmt.Errorf("delivery label does not belong to plan unit")
			}
		} else if input.LabelClientKey != "" {
			var label projectgorm.DeliveryNodeLabel
			err := tx.Where("workspace_id = ? AND project_set_id = ? AND plan_unit_id = ? AND client_key = ?",
				project.WorkspaceID, project.ID, unit.ID, input.LabelClientKey).First(&label).Error
			if err != nil {
				return fmt.Errorf("delivery label client key was not created")
			}
			input.LabelID = &label.ID
			input.LabelName = label.Name
		}
		dueAt, err := parseOptionalTime(input.DueAt.Value)
		if err != nil {
			return err
		}
		completedAt, err := parseOptionalTime(input.ActualCompletedAt.Value)
		if err != nil {
			return err
		}
		node := projectgorm.DeliveryNode{
			WorkspaceID: project.WorkspaceID, PlanUnitID: unit.ID, LabelID: input.LabelID,
			LabelClientKey: input.LabelClientKey, LabelName: input.LabelName, Type: input.Type,
			DueAt: dueAt, ActualCompletedAt: completedAt, OwnerUserID: input.OwnerUserID.Value,
		}
		if err := tx.Create(&node).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) createInitialDeliveryLabels(
	tx *gorm.DB,
	project projectgorm.ProjectSet,
	unit projectgorm.PlanUnit,
	inputs []deliveryLabelInput,
) error {
	for _, input := range inputs {
		if strings.TrimSpace(input.ClientKey) == "" || strings.TrimSpace(input.Name) == "" {
			return fmt.Errorf("delivery label client_key and name are required")
		}
		label := projectgorm.DeliveryNodeLabel{
			WorkspaceID: project.WorkspaceID, ProjectSetID: project.ID, PlanUnitID: unit.ID,
			ClientKey: strings.TrimSpace(input.ClientKey), Name: strings.TrimSpace(input.Name),
			SystemKey: strings.TrimSpace(input.SystemKey), SortOrder: input.SortOrder,
		}
		if err := tx.Create(&label).Error; err != nil {
			return err
		}
	}
	return nil
}
