package projectgorm

import (
	"context"
	"errors"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

type ListRepository struct{ db *gorm.DB }

func NewListRepository(db *gorm.DB) *ListRepository { return &ListRepository{db: db} }

func (repository *ListRepository) ListProjectSets(
	ctx context.Context,
	workspaceID uint,
	filter project.ProjectSetListFilter,
) (project.ProjectSetListPage, error) {
	query := repository.db.WithContext(ctx).Model(&ProjectSet{}).Where("workspace_id = ?", workspaceID)
	if filter.TeamID > 0 {
		query = query.Where("team_id = ?", filter.TeamID)
	}
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR code LIKE ?", like, like)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return project.ProjectSetListPage{}, err
	}
	var rows []ProjectSet
	if err := query.Order("updated_at DESC, id DESC").
		Limit(filter.Limit).Offset(filter.Offset).Find(&rows).Error; err != nil {
		return project.ProjectSetListPage{}, err
	}
	items := make([]project.ProjectSetSnapshot, 0, len(rows))
	for _, row := range rows {
		items = append(items, projectSetListSnapshot(row))
	}
	return project.ProjectSetListPage{Items: items, Total: total}, nil
}

func (repository *ListRepository) ListPlanUnits(
	ctx context.Context,
	workspaceID uint,
	projectSetID uint,
) ([]project.PlanUnitSnapshot, error) {
	var parent ProjectSet
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND id = ?", workspaceID, projectSetID).First(&parent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, project.ErrProjectSetNotFound
		}
		return nil, err
	}
	var rows []PlanUnit
	if err := repository.db.WithContext(ctx).
		Where("workspace_id = ? AND project_new_id = ?", workspaceID, projectSetID).
		Order("id DESC").Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]project.PlanUnitSnapshot, 0, len(rows))
	for _, row := range rows {
		items = append(items, planUnitListSnapshot(row))
	}
	return items, nil
}

// ListAccessibleProjectSets lists project sets restricted by the caller's
// resolved access filter. An empty filter SQL is the admin fast path and
// applies no restriction — the domain service resolves that before calling.
func (repository *ListRepository) ListAccessibleProjectSets(
	ctx context.Context,
	workspaceID uint,
	filter project.ProjectSetAccessListFilter,
) (project.ProjectSetListPage, error) {
	query := repository.db.WithContext(ctx).Model(&ProjectSet{}).Where("workspace_id = ?", workspaceID)
	if filter.Access.SQL != "" {
		query = query.Where(filter.Access.SQL, filter.Access.Args...)
	}
	if filter.TeamID > 0 {
		query = query.Where("team_id = ?", filter.TeamID)
	}
	if keyword := strings.TrimSpace(filter.Query); keyword != "" {
		like := "%" + keyword + "%"
		query = query.Where("name LIKE ? OR code LIKE ?", like, like)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return project.ProjectSetListPage{}, err
	}
	var rows []ProjectSet
	if err := query.Order("updated_at DESC, id DESC").
		Limit(filter.Limit).Offset(filter.Offset).Find(&rows).Error; err != nil {
		return project.ProjectSetListPage{}, err
	}
	items := make([]project.ProjectSetSnapshot, 0, len(rows))
	for _, row := range rows {
		items = append(items, projectSetListSnapshot(row))
	}
	return project.ProjectSetListPage{Items: items, Total: total}, nil
}

var _ project.ProjectSetAccessListStore = (*ListRepository)(nil)

func projectSetListSnapshot(row ProjectSet) project.ProjectSetSnapshot {
	return project.ProjectSetSnapshot{
		ID: row.ID, WorkspaceID: row.WorkspaceID, TeamID: row.TeamID, Name: row.Name, Code: row.Code,
		Description: row.Description, OwnerUserID: row.OwnerUserID, PlannedStartAt: row.PlannedStartAt,
		PlannedEndAt: row.PlannedEndAt, Status: row.Status, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func planUnitListSnapshot(row PlanUnit) project.PlanUnitSnapshot {
	return project.PlanUnitSnapshot{
		ID: row.ID, WorkspaceID: row.WorkspaceID, TeamID: row.TeamID, ProjectSetID: row.ProjectSetID,
		Title: row.Title, Type: row.Type, Goal: row.Goal, OwnerUserID: row.OwnerUserID,
		PlannedStartAt: row.PlannedStartAt, PlannedEndAt: row.PlannedEndAt,
		DesignFreezeDueAt: row.DesignFreezeDueAt, DevelopmentCompleteAt: row.DevelopmentCompleteAt,
		TestAcceptanceDueAt: row.TestAcceptanceDueAt, ActualStartAt: row.ActualStartAt,
		ActualCompletedAt: row.ActualCompletedAt, RiskLevel: row.RiskLevel, Status: row.Status,
		SortOrder: row.SortOrder, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}
