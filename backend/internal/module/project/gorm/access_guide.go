package projectgorm

import (
	"context"
	"errors"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"gorm.io/gorm"
)

type AccessGuideRepository struct{ db *gorm.DB }

func NewAccessGuideRepository(db *gorm.DB) *AccessGuideRepository {
	return &AccessGuideRepository{db: db}
}
func (r *AccessGuideRepository) LocateProject(ctx context.Context, id uint) (uint, uint, error) {
	var row PlanUnit
	err := r.db.WithContext(ctx).Select("id", "workspace_id").First(&row, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, 0, nil
	}
	return row.WorkspaceID, row.ID, err
}
func (r *AccessGuideRepository) ReadGuideProject(ctx context.Context, w, id uint) (project.AccessGuide, error) {
	var row struct {
		ID, TeamID, ProjectSetID uint
		Title, SetName           string
		OwnerUserID              *uint
	}
	err := r.db.WithContext(ctx).Table("plan_units AS pu").Select("pu.id, pu.team_id, pu.project_new_id AS project_set_id, pu.title, pu.owner_user_id, ps.name AS set_name").Joins("JOIN project_sets ps ON ps.id=pu.project_new_id AND ps.workspace_id=pu.workspace_id").Where("pu.workspace_id = ? AND pu.id = ?", w, id).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return project.AccessGuide{}, nil
	}
	owner := uint(0)
	if row.OwnerUserID != nil {
		owner = *row.OwnerUserID
	}
	return project.AccessGuide{ProjectSetID: row.ProjectSetID, PlanUnitID: row.ID, TeamID: row.TeamID, ProjectName: row.Title, ProjectSetName: row.SetName, OwnerUserID: owner}, err
}
