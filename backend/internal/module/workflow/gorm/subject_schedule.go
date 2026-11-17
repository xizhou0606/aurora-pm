package workflowgorm

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/workflow"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SubjectScheduleRepository struct{ db *gorm.DB }

func NewSubjectScheduleRepository(db *gorm.DB) *SubjectScheduleRepository {
	return &SubjectScheduleRepository{db: db}
}

func (repository *SubjectScheduleRepository) RequirementScheduleBounds(
	ctx context.Context, workspaceID, requirementID uint,
) (*time.Time, *time.Time, bool, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || requirementID == 0 {
		return nil, nil, false, workflow.ErrSubjectScheduleConflict
	}
	var rows []WorkflowInstance
	if err := repository.db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("workspace_id = ? AND subject_type = ? AND subject_id = ?", workspaceID, "requirement", requirementID).
		Order("id").Find(&rows).Error; err != nil {
		return nil, nil, false, err
	}
	var earliest, latest *time.Time
	for _, row := range rows {
		plans := map[string]workflow.NodePlan{}
		if strings.TrimSpace(row.NodePlansJSON) == "" || json.Unmarshal([]byte(row.NodePlansJSON), &plans) != nil || plans == nil {
			return nil, nil, false, workflow.ErrSubjectScheduleConflict
		}
		for nodeID, plan := range plans {
			if strings.TrimSpace(nodeID) == "" || plan.PlannedStartAt != nil && plan.PlannedEndAt != nil && plan.PlannedStartAt.After(*plan.PlannedEndAt) {
				return nil, nil, false, workflow.ErrSubjectScheduleConflict
			}
			if plan.PlannedStartAt != nil && (earliest == nil || plan.PlannedStartAt.Before(*earliest)) {
				value := *plan.PlannedStartAt
				earliest = &value
			}
			if plan.PlannedEndAt != nil && (latest == nil || plan.PlannedEndAt.After(*latest)) {
				value := *plan.PlannedEndAt
				latest = &value
			}
		}
	}
	return earliest, latest, earliest != nil || latest != nil, nil
}

func (repository *SubjectScheduleRepository) ValidateSubjectSchedules(
	ctx context.Context, workspaceID, planUnitID uint, requirementIDs []uint, start, end *time.Time,
) error {
	if repository == nil || repository.db == nil {
		return workflow.ErrSubjectScheduleConflict
	}
	query := repository.db.WithContext(ctx).Model(&WorkflowInstance{}).Where("workspace_id = ?", workspaceID)
	if len(requirementIDs) > 0 {
		query = query.Where("(subject_type = ? AND subject_id = ?) OR (subject_type = ? AND subject_id IN ?)",
			"plan_unit", planUnitID, "requirement", requirementIDs)
	} else {
		query = query.Where("subject_type = ? AND subject_id = ?", "plan_unit", planUnitID)
	}
	var rows []WorkflowInstance
	if err := query.Select("id", "node_plans_json").Order("id").Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		plans := map[string]workflow.NodePlan{}
		if strings.TrimSpace(row.NodePlansJSON) == "" || json.Unmarshal([]byte(row.NodePlansJSON), &plans) != nil || plans == nil {
			return workflow.ErrSubjectScheduleConflict
		}
		for nodeID, plan := range plans {
			if strings.TrimSpace(nodeID) == "" || plan.PlannedStartAt != nil && plan.PlannedEndAt != nil && plan.PlannedStartAt.After(*plan.PlannedEndAt) ||
				start != nil && plan.PlannedStartAt != nil && plan.PlannedStartAt.Before(*start) ||
				end != nil && plan.PlannedEndAt != nil && plan.PlannedEndAt.After(*end) {
				return workflow.ErrSubjectScheduleConflict
			}
		}
	}
	return nil
}

var _ workflow.SubjectScheduleStore = (*SubjectScheduleRepository)(nil)
