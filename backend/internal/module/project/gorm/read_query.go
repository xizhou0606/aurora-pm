package projectgorm

import (
	"context"
	"errors"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

type ReadRepository struct{ db *gorm.DB }

func NewReadRepository(db *gorm.DB) *ReadRepository { return &ReadRepository{db: db} }

func (repository *ReadRepository) GetProjectSet(ctx context.Context, workspaceID, id uint) (project.ProjectSetSnapshot, error) {
	var row ProjectSet
	err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return project.ProjectSetSnapshot{}, project.ErrProjectSetNotFound
	}
	if err != nil {
		return project.ProjectSetSnapshot{}, err
	}
	return project.ProjectSetSnapshot{
		ID: row.ID, WorkspaceID: row.WorkspaceID, TeamID: row.TeamID, Name: row.Name, Code: row.Code,
		Description: row.Description, OwnerUserID: row.OwnerUserID, PlannedStartAt: row.PlannedStartAt,
		PlannedEndAt: row.PlannedEndAt, Status: row.Status, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

func (repository *ReadRepository) GetPlanUnit(ctx context.Context, workspaceID, id uint) (project.PlanUnitSnapshot, error) {
	var row PlanUnit
	err := repository.db.WithContext(ctx).Where("workspace_id = ? AND id = ?", workspaceID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return project.PlanUnitSnapshot{}, project.ErrPlanUnitNotFound
	}
	if err != nil {
		return project.PlanUnitSnapshot{}, err
	}
	return project.PlanUnitSnapshot{
		ID: row.ID, WorkspaceID: row.WorkspaceID, TeamID: row.TeamID, ProjectSetID: row.ProjectSetID,
		Title: row.Title, Type: row.Type, Goal: row.Goal, OwnerUserID: row.OwnerUserID,
		PlannedStartAt: row.PlannedStartAt, PlannedEndAt: row.PlannedEndAt,
		DesignFreezeDueAt: row.DesignFreezeDueAt, DevelopmentCompleteAt: row.DevelopmentCompleteAt,
		TestAcceptanceDueAt: row.TestAcceptanceDueAt, ActualStartAt: row.ActualStartAt,
		ActualCompletedAt: row.ActualCompletedAt, RiskLevel: row.RiskLevel, Status: row.Status,
		SortOrder: row.SortOrder, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}
