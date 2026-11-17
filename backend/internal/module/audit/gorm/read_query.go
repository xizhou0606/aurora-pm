package auditgorm

import (
	"context"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/audit"
	"gorm.io/gorm"
)

type ReadRepository struct{ db *gorm.DB }

func NewReadRepository(db *gorm.DB) *ReadRepository { return &ReadRepository{db: db} }

func (repository *ReadRepository) ListProjectEvents(
	ctx context.Context,
	workspaceID uint,
	projectSetID uint,
	limit int,
	offset int,
) (audit.ProjectEventPage, error) {
	query := repository.db.WithContext(ctx).Model(&AuditLog{}).
		Where("workspace_id = ? AND project_id = ?", workspaceID, projectSetID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return audit.ProjectEventPage{}, err
	}
	var rows []AuditLog
	if err := query.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return audit.ProjectEventPage{}, err
	}
	items := make([]audit.EventSnapshot, 0, len(rows))
	for _, row := range rows {
		items = append(items, audit.EventSnapshot{
			ID: row.ID, WorkspaceID: row.WorkspaceID, UserID: row.UserID, Username: row.Username,
			Role: row.Role, Method: row.Method, StatusCode: row.StatusCode, Resource: row.Resource,
			ResourceID: row.ResourceID, ResourceRef: row.ResourceRef, ProjectID: row.ProjectID, Action: row.Action, Target: row.Target,
			Detail: row.Detail, CreatedAt: row.CreatedAt,
		})
	}
	return audit.ProjectEventPage{Items: items, Total: total}, nil
}

var _ audit.ReadStore = (*ReadRepository)(nil)
