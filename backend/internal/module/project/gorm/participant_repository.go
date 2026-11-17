package projectgorm

import (
	"context"
	"strings"

	"gorm.io/gorm"

	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
)

// MemberRepository is the workspace user directory used by member pickers and
// member validation. Plan-unit membership state lives in
// PlanUnitMemberRepository; project-level owner/participant facts are retired.
type MemberRepository struct{ db *gorm.DB }

func NewMemberRepository(db *gorm.DB) *MemberRepository { return &MemberRepository{db: db} }

type identityUserRefRow struct {
	ID          uint
	Username    string
	DisplayName string
	Email       string
	AvatarURL   string
}

// VerifyMemberUsers returns the canonical fact IDs of the given reference IDs
// when every one resolves to an active workspace PM user.
func (repository *MemberRepository) VerifyMemberUsers(ctx context.Context, workspaceID uint, ids []uint) error {
	if repository == nil || repository.db == nil || workspaceID == 0 {
		return project.ErrParticipantInvalid
	}
	if len(ids) == 0 {
		return nil
	}
	users, err := repository.findActiveUsersByReferenceIDs(ctx, workspaceID, ids)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if _, ok := users[id]; !ok {
			return project.ErrParticipantInvalid
		}
	}
	return nil
}

// SearchWorkspaceActiveUsers searches active PM users inside one workspace.
func (repository *MemberRepository) SearchWorkspaceActiveUsers(
	ctx context.Context,
	workspaceID uint,
	query string,
	page, pageSize int,
) (project.MemberPage, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 {
		return project.MemberPage{}, project.ErrParticipantInvalid
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	base := repository.db.WithContext(ctx).Table("users AS u").
		Joins("JOIN workspace_members AS wm ON wm.user_id = u.id AND wm.workspace_id = ? AND wm.enabled = ?", workspaceID, true).
		Joins("JOIN workspaces AS w ON w.id = wm.workspace_id AND w.enabled = ?", true).
		Where("u.active = ? AND u.status = ?", true, "active")
	if value := strings.ToLower(strings.TrimSpace(query)); value != "" {
		pattern := "%" + memberLike(value) + "%"
		base = base.Where(
			"LOWER(u.username) LIKE ? ESCAPE '\\' OR LOWER(u.display_name) LIKE ? ESCAPE '\\' OR LOWER(u.email) LIKE ? ESCAPE '\\'",
			pattern, pattern, pattern,
		)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return project.MemberPage{}, err
	}
	var rows []identityUserRefRow
	if err := base.Select("u.id, u.username, u.display_name, u.email, u.avatar_url").
		Order("u.display_name ASC, u.username ASC, u.id ASC").
		Offset((page - 1) * pageSize).Limit(pageSize).Scan(&rows).Error; err != nil {
		return project.MemberPage{}, err
	}
	items := make([]project.UserRef, 0, len(rows))
	for _, row := range rows {
		items = append(items, project.UserRef{
			ID: row.ID, Username: row.Username,
			DisplayName: row.DisplayName, Email: row.Email, AvatarURL: row.AvatarURL,
		})
	}
	return project.MemberPage{Items: items, Page: page, PageSize: pageSize, Total: total, HasMore: int64(page*pageSize) < total}, nil
}

func (repository *MemberRepository) findActiveUsersByReferenceIDs(ctx context.Context, workspaceID uint, ids []uint) (map[uint]identityUserRefRow, error) {
	rows := make([]identityUserRefRow, 0)
	err := repository.db.WithContext(ctx).Table("users AS u").
		Joins("JOIN workspace_members AS wm ON wm.user_id = u.id AND wm.workspace_id = ? AND wm.enabled = ?", workspaceID, true).
		Where("u.active = ? AND u.status = ?", true, "active").
		Where("u.id IN ?", ids).
		Select("u.id, u.username, u.display_name, u.email").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint]identityUserRefRow, len(rows)*2)
	for _, row := range rows {
		result[row.ID] = row
	}
	return result, nil
}

func memberLike(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return replacer.Replace(value)
}
