package identitygorm

import (
	"context"
	"strings"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

type BaselineDirectoryRepository struct{ db *gorm.DB }

func NewBaselineDirectoryRepository(db *gorm.DB) *BaselineDirectoryRepository {
	return &BaselineDirectoryRepository{db: db}
}

func (repository *BaselineDirectoryRepository) ListAvailableWorkspaces(
	ctx context.Context,
	principalID uint,
) ([]identity.AvailableWorkspace, error) {
	administrator, err := isAccessAdministrator(ctx, repository.db, principalID)
	if err != nil {
		return nil, err
	}

	query := repository.db.WithContext(ctx).Table("workspaces AS w").
		Select("w.id, w.name, w.slug, w.is_default").
		Where("w.enabled = ?", true)
	if !administrator {
		query = query.Joins(
			"JOIN workspace_members AS wm ON wm.workspace_id = w.id AND wm.user_id = ? AND wm.enabled = ?",
			principalID,
			true,
		)
	}
	var items []identity.AvailableWorkspace
	err = query.
		Order("w.is_default DESC, w.id ASC").
		Scan(&items).Error
	return items, err
}

func (repository *BaselineDirectoryRepository) SearchUsers(
	ctx context.Context,
	workspaceID uint,
	filter identity.DirectoryUserFilter,
) (identity.DirectoryUserPage, error) {
	query := repository.db.WithContext(ctx).Table("users AS u").
		Joins(
			"JOIN workspace_members AS wm ON wm.user_id = u.id AND wm.workspace_id = ? AND wm.enabled = ?",
			workspaceID,
			true,
		).
		Where("u.active = ? AND u.status = ?", true, string(identity.AccountActive))
	if len(filter.IDs) > 0 {
		query = query.Where("u.id IN ?", filter.IDs)
	}
	if username := strings.ToLower(filter.Username); username != "" {
		query = query.Where("LOWER(u.username) = ?", username)
	}
	if value := strings.ToLower(filter.Query); value != "" {
		if filter.Exact {
			query = query.Where("LOWER(u.username) = ? OR LOWER(u.display_name) = ?", value, value)
		} else {
			pattern := "%" + escapeBaselineDirectoryLike(value) + "%"
			query = query.Where(
				"LOWER(u.username) LIKE ? ESCAPE '\\' OR LOWER(u.display_name) LIKE ? ESCAPE '\\'",
				pattern, pattern,
			)
		}
	}
	var total int64
	if err := query.Distinct("u.id").Count(&total).Error; err != nil {
		return identity.DirectoryUserPage{}, err
	}
	var rows []struct {
		ID          uint
		Username    string
		DisplayName string
		AvatarURL   string
		Email       string
	}
	if err := query.Distinct().Select("u.id, u.username, u.display_name, u.avatar_url, u.email").
		Order("u.display_name ASC, u.username ASC, u.id ASC").
		Limit(filter.Limit).Offset(filter.Offset).Scan(&rows).Error; err != nil {
		return identity.DirectoryUserPage{}, err
	}
	items := make([]identity.DirectoryUser, 0, len(rows))
	for _, row := range rows {
		id := row.ID
		items = append(items, identity.DirectoryUser{
			ID: id, Username: row.Username, DisplayName: row.DisplayName, AvatarURL: row.AvatarURL, Email: "",
		})
		if len(filter.IDs) > 0 || filter.Exact {
			items[len(items)-1].Email = row.Email
		}
	}
	return identity.DirectoryUserPage{Items: items, Total: total}, nil
}

func (repository *BaselineDirectoryRepository) ListTeams(
	ctx context.Context,
	workspaceID uint,
) ([]identity.DirectoryTeam, error) {
	var items []identity.DirectoryTeam
	err := repository.db.WithContext(ctx).Table("teams AS t").
		Select("t.id, t.name, t.description").
		Where("t.workspace_id = ?", workspaceID).
		Order("t.name ASC, t.id ASC").Scan(&items).Error
	return items, err
}

func (repository *BaselineDirectoryRepository) ListTeamMembers(
	ctx context.Context,
	workspaceID uint,
	teamID uint,
) ([]identity.DirectoryTeamMember, error) {
	var teamCount int64
	if err := repository.db.WithContext(ctx).Table("teams AS t").
		Where("t.workspace_id = ? AND t.id = ?", workspaceID, teamID).
		Count(&teamCount).Error; err != nil {
		return nil, err
	}
	if teamCount != 1 {
		return nil, identity.ErrBaselineDirectoryTeamMissing
	}
	var rows []struct {
		ID          uint
		Username    string
		DisplayName string
		AvatarURL   string
		Email       string
		Role        string
	}
	err := repository.db.WithContext(ctx).Table("team_members AS tm").
		Joins("JOIN users AS u ON u.id = tm.user_id").
		Where(
			"tm.workspace_id = ? AND tm.team_id = ? AND u.active = ? AND u.status = ?",
			workspaceID, teamID, true, string(identity.AccountActive),
		).
		Select("u.id, u.username, u.display_name, u.avatar_url, u.email, tm.role").
		Order("u.display_name ASC, u.username ASC, u.id ASC").Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]identity.DirectoryTeamMember, 0, len(rows))
	for _, row := range rows {
		id := row.ID
		items = append(items, identity.DirectoryTeamMember{
			DirectoryUser: identity.DirectoryUser{
				ID: id, Username: row.Username, DisplayName: row.DisplayName, AvatarURL: row.AvatarURL,
			},
			Role: row.Role,
		})
	}
	return items, nil
}

func escapeBaselineDirectoryLike(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "%", "\\%")
	return strings.ReplaceAll(value, "_", "\\_")
}

var _ identity.BaselineDirectoryStore = (*BaselineDirectoryRepository)(nil)
