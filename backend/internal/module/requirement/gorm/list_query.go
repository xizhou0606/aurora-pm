package requirementgorm

import (
	"context"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/requirement"
)

func (repository *ReadRepository) ListRequirements(
	ctx context.Context,
	workspaceID uint,
	filter requirement.ListFilter,
) (requirement.ListPage, error) {
	query := repository.db.WithContext(ctx).Model(&Requirement{}).Where("workspace_id = ?", workspaceID)
	if filter.ProjectSetID > 0 {
		query = query.Where("project_new_id = ?", filter.ProjectSetID)
	}
	if filter.PlanUnitID > 0 {
		query = query.Where("version_task_new_id = ?", filter.PlanUnitID)
	}
	if keyword := strings.ToLower(strings.TrimSpace(filter.Query)); keyword != "" {
		query = query.Where("LOWER(title) LIKE ? ESCAPE '\\'", "%"+escapeRequirementLike(keyword)+"%")
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return requirement.ListPage{}, err
	}
	var rows []Requirement
	if err := query.Order("id DESC").Limit(filter.Limit).Offset(filter.Offset).Find(&rows).Error; err != nil {
		return requirement.ListPage{}, err
	}
	return requirement.ListPage{Items: requirementSnapshots(rows), Total: total}, nil
}

func (repository *ReadRepository) ListScopeRequirements(
	ctx context.Context,
	workspaceID uint,
	projectSetIDs []uint,
) ([]requirement.Snapshot, error) {
	var rows []Requirement
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND project_new_id IN ?", workspaceID, projectSetIDs).
		Order("project_new_id ASC, version_task_new_id ASC, id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return requirementSnapshots(rows), nil
}

func requirementSnapshots(rows []Requirement) []requirement.Snapshot {
	items := make([]requirement.Snapshot, 0, len(rows))
	for _, row := range rows {
		items = append(items, requirementSnapshot(row))
	}
	return items
}

func requirementSnapshot(row Requirement) requirement.Snapshot {
	return requirement.Snapshot{
		ID: row.ID, WorkspaceID: row.WorkspaceID, TeamID: row.TeamID, ProjectSetID: row.ProjectSetID,
		PlanUnitID: row.ScheduledPlanUnitID, Version: row.Version, WorkflowInstanceID: row.WorkflowInstanceID,
		Title: row.Title, Description: row.Description, ProductDocURL: row.ProductDocURL,
		ProductOwnerUserIDs: uintSlice(row.ProductOwnerUserIDs), DesignURL: row.DesignURL,
		DesignOwnerUserIDs: uintSlice(row.DesignOwnerUserIDs), FrontendOwnerUserIDs: uintSlice(row.FrontendOwnerUserIDs),
		BackendOwnerUserIDs: uintSlice(row.BackendOwnerUserIDs), TesterUserIDs: uintSlice(row.TesterUserIDs),
		DeveloperUserIDs: uintSlice(row.DeveloperUserIDs), AcceptanceCriteria: row.AcceptanceCriteria,
		Priority: row.Priority, Status: row.Status, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func escapeRequirementLike(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	return strings.ReplaceAll(value, "_", "\\_")
}

var _ requirement.ListStore = (*ReadRepository)(nil)
