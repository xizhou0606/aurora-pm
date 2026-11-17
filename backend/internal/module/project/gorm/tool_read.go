package projectgorm

import (
	"context"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

func (repository *ReadRepository) ListDeliveryLabels(
	ctx context.Context,
	workspaceID uint,
	planUnitID uint,
) ([]project.DeliveryLabelSnapshot, error) {
	var unit PlanUnit
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, planUnitID).First(&unit).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, project.ErrPlanUnitNotFound
		}
		return nil, err
	}
	var rows []DeliveryNodeLabel
	if err := repository.db.WithContext(ctx).Where(
		"workspace_id = ? AND project_set_id = ? AND plan_unit_id = ? AND deleted = ?",
		workspaceID, unit.ProjectSetID, planUnitID, false,
	).Order("sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]project.DeliveryLabelSnapshot, 0, len(rows))
	for _, row := range rows {
		items = append(items, project.DeliveryLabelSnapshot{
			ID: row.ID, WorkspaceID: row.WorkspaceID, ProjectSetID: row.ProjectSetID, PlanUnitID: row.PlanUnitID,
			ClientKey: row.ClientKey, Name: row.Name, SystemKey: row.SystemKey, SortOrder: row.SortOrder,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return items, nil
}

// ListScopeOptions enumerates project sets and plan units for the caller's
// accessible projects only. The filter comes from the domain access formula
// (AccessService.BuildMemberFilter); an empty filter is the admin fast path.
func (repository *ReadRepository) ListScopeOptions(ctx context.Context, workspaceID uint, filter project.AccessFilter) (project.ScopeOptions, error) {
	query := repository.db.WithContext(ctx).Where("workspace_id = ?", workspaceID)
	if filter.SQL != "" {
		query = query.Where(filter.SQL, filter.Args...)
	}
	var projectRows []ProjectSet
	if err := query.Order("updated_at DESC, id DESC").Find(&projectRows).Error; err != nil {
		return project.ScopeOptions{}, err
	}
	result := project.ScopeOptions{
		ProjectSets: make([]project.ScopeProjectSet, 0, len(projectRows)),
		PlanUnits:   []project.PlanUnitSnapshot{},
	}
	projectIDs := make([]uint, 0, len(projectRows))
	for _, row := range projectRows {
		projectIDs = append(projectIDs, row.ID)
		result.ProjectSets = append(result.ProjectSets, project.ScopeProjectSet{
			ID: row.ID, TeamID: row.TeamID, Name: row.Name, Code: row.Code,
		})
	}
	if len(projectIDs) == 0 {
		return result, nil
	}
	var unitRows []PlanUnit
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND project_new_id IN ?", workspaceID, projectIDs).
		Order("project_new_id ASC, sort_order ASC, id DESC").Find(&unitRows).Error; err != nil {
		return project.ScopeOptions{}, err
	}
	result.PlanUnits = make([]project.PlanUnitSnapshot, 0, len(unitRows))
	for _, row := range unitRows {
		result.PlanUnits = append(result.PlanUnits, planUnitListSnapshot(row))
	}
	return result, nil
}

var _ project.ToolReadStore = (*ReadRepository)(nil)
