package identitygorm

import (
	"context"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

type ToolDirectoryRepository struct{ db *gorm.DB }

func NewToolDirectoryRepository(db *gorm.DB) *ToolDirectoryRepository {
	return &ToolDirectoryRepository{db: db}
}

func (repository *ToolDirectoryRepository) FindActiveUser(
	ctx context.Context,
	userID uint,
) (identity.ToolUser, error) {
	var user identity.ToolUser
	err := repository.db.WithContext(ctx).Table("users AS u").
		Select("u.id, u.username, u.display_name").
		Where("u.id = ? AND u.active = ? AND u.status = ?", userID, true, string(identity.AccountActive)).
		Take(&user).Error
	return user, err
}

func (repository *ToolDirectoryRepository) ListWorkspaceMembers(
	ctx context.Context,
	workspaceID uint,
) (identity.ToolUserPage, error) {
	query := repository.workspaceUsersQuery(ctx, workspaceID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return identity.ToolUserPage{}, err
	}
	items, err := scanToolUsers(query)
	if err != nil {
		return identity.ToolUserPage{}, err
	}
	return identity.ToolUserPage{Items: items, Total: total}, nil
}

func (repository *ToolDirectoryRepository) SearchWorkspaceUsers(
	ctx context.Context,
	workspaceID uint,
	filter identity.ToolUserFilter,
) (identity.ToolUserPage, error) {
	query := repository.workspaceUsersQuery(ctx, workspaceID)
	if filter.ID > 0 {
		query = query.Where("u.id = ?", filter.ID)
	}
	if username := strings.ToLower(strings.TrimSpace(filter.Username)); username != "" {
		query = query.Where("LOWER(u.username) = ?", username)
	}
	if value := strings.ToLower(strings.TrimSpace(filter.Query)); value != "" {
		if filter.Exact {
			query = query.Where("LOWER(u.username) = ? OR LOWER(u.display_name) = ?", value, value)
		} else {
			pattern := "%" + escapeToolDirectoryLike(value) + "%"
			query = query.Where(
				"LOWER(u.username) LIKE ? ESCAPE '\\' OR LOWER(u.display_name) LIKE ? ESCAPE '\\'",
				pattern, pattern,
			)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return identity.ToolUserPage{}, err
	}
	items, err := scanToolUsers(query.Limit(filter.Limit).Offset(filter.Offset))
	if err != nil {
		return identity.ToolUserPage{}, err
	}
	return identity.ToolUserPage{Items: items, Total: total}, nil
}

func (repository *ToolDirectoryRepository) workspaceUsersQuery(ctx context.Context, workspaceID uint) *gorm.DB {
	return repository.db.WithContext(ctx).Table("users AS u").
		Joins("JOIN workspace_members AS wm ON wm.user_id = u.id AND wm.workspace_id = ? AND wm.enabled = ?", workspaceID, true).
		Joins("JOIN workspaces AS w ON w.id = wm.workspace_id AND w.enabled = ?", true).
		Where("u.active = ? AND u.status = ?", true, string(identity.AccountActive))
}

func scanToolUsers(query *gorm.DB) ([]identity.ToolUser, error) {
	var rows []struct {
		ID          uint
		Username    string
		DisplayName string
	}
	if err := query.Select("u.id, u.username, u.display_name").
		Order("u.display_name ASC, u.username ASC, u.id ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]identity.ToolUser, 0, len(rows))
	for _, row := range rows {
		items = append(items, identity.ToolUser{ID: row.ID, Username: row.Username, DisplayName: row.DisplayName})
	}
	return items, nil
}

func (repository *ToolDirectoryRepository) ListTeamMembers(
	ctx context.Context,
	workspaceID uint,
	teamID uint,
) (identity.ToolTeamMemberPage, error) {
	query := repository.db.WithContext(ctx).Table("team_members AS tm").
		Joins("JOIN teams AS t ON t.id = tm.team_id AND t.workspace_id = tm.workspace_id").
		Joins("JOIN users AS u ON u.id = tm.user_id").
		Joins("JOIN workspace_members AS wm ON wm.user_id = u.id AND wm.workspace_id = tm.workspace_id AND wm.enabled = ?", true).
		Where("tm.workspace_id = ? AND tm.team_id = ? AND u.active = ? AND u.status = ?", workspaceID, teamID, true, string(identity.AccountActive))
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return identity.ToolTeamMemberPage{}, err
	}
	var rows []struct {
		ID          uint
		Username    string
		DisplayName string
		Role        string
	}
	if err := query.Select("u.id, u.username, u.display_name, tm.role").
		Order("u.display_name ASC, u.username ASC, u.id ASC").Scan(&rows).Error; err != nil {
		return identity.ToolTeamMemberPage{}, err
	}
	items := make([]identity.ToolTeamMember, 0, len(rows))
	for _, row := range rows {
		items = append(items, identity.ToolTeamMember{
			ToolUser: identity.ToolUser{ID: row.ID, Username: row.Username, DisplayName: row.DisplayName}, Role: row.Role,
		})
	}
	return identity.ToolTeamMemberPage{Items: items, Total: total}, nil
}

func escapeToolDirectoryLike(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	return strings.ReplaceAll(value, "_", "\\_")
}

var _ identity.ToolDirectoryStore = (*ToolDirectoryRepository)(nil)
