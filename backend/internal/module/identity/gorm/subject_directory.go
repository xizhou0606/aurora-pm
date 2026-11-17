package identitygorm

import (
	"context"
	"strings"

	identity "github.com/xizhou0606/aurora-pm/backend/internal/module/identity"
	"gorm.io/gorm"
)

type SubjectRepository struct{ db *gorm.DB }

func NewSubjectRepository(db *gorm.DB) *SubjectRepository { return &SubjectRepository{db: db} }

func (repository *SubjectRepository) HasActiveWorkspaceUser(
	ctx context.Context, workspaceID, userID uint,
) (bool, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || userID == 0 {
		return false, identity.ErrSubjectInvalid
	}
	// Admission must match HTTP/Tool Runtime: system access administrators may
	// enter enabled workspaces without a materialized membership row. This is
	// not a resource grant; private ownership and explicit subjects still apply.
	return NewWorkspaceMembershipRepository(repository.db).HasActiveWorkspaceMembership(ctx, userID, workspaceID)
}

func (repository *SubjectRepository) SearchNativeSubjects(
	ctx context.Context, workspaceID uint, kind identity.SubjectKind, search string, page, pageSize int,
) (identity.NativeSubjectPage, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 {
		return identity.NativeSubjectPage{}, identity.ErrSubjectInvalid
	}
	if kind != identity.SubjectUser {
		if err := repository.requireEnabledWorkspace(ctx, workspaceID); err != nil {
			return identity.NativeSubjectPage{}, err
		}
	}
	query, err := repository.subjectQuery(ctx, workspaceID, kind)
	if err != nil {
		return identity.NativeSubjectPage{}, err
	}
	if value := strings.ToLower(strings.TrimSpace(search)); value != "" {
		pattern := "%" + subjectLike(value) + "%"
		switch kind {
		case identity.SubjectUser:
			query = query.Where(
				"LOWER(u.username) LIKE ? ESCAPE '\\' OR LOWER(u.display_name) LIKE ? ESCAPE '\\'",
				pattern, pattern,
			)
		case identity.SubjectRole:
			query = query.Where("LOWER(ar.name) LIKE ? ESCAPE '\\' OR LOWER(ar.code) LIKE ? ESCAPE '\\'", pattern, pattern)
		case identity.SubjectUserGroup:
			query = query.Where("LOWER(aug.name) LIKE ? ESCAPE '\\' OR LOWER(aug.code) LIKE ? ESCAPE '\\'", pattern, pattern)
		}
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return identity.NativeSubjectPage{}, err
	}
	var items []identity.NativeSubjectRecord
	if err := selectNativeSubjectColumns(query, kind).Order(subjectOrder(kind)).
		Offset((page - 1) * pageSize).Limit(pageSize).Scan(&items).Error; err != nil {
		return identity.NativeSubjectPage{}, err
	}
	return identity.NativeSubjectPage{Items: items, Page: page, PageSize: pageSize, Total: total}, nil
}

func (repository *SubjectRepository) ResolveNativeSubjects(
	ctx context.Context, workspaceID uint, kind identity.SubjectKind, ids []uint,
) ([]identity.NativeSubjectRecord, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 {
		return nil, identity.ErrSubjectInvalid
	}
	if len(ids) == 0 {
		return []identity.NativeSubjectRecord{}, nil
	}
	if kind != identity.SubjectUser {
		if err := repository.requireEnabledWorkspace(ctx, workspaceID); err != nil {
			return nil, err
		}
	}
	query, err := repository.subjectQuery(ctx, workspaceID, kind)
	if err != nil {
		return nil, err
	}
	query = query.Where(subjectIDColumn(kind)+" IN ?", ids)
	var items []identity.NativeSubjectRecord
	if err := selectNativeSubjectColumns(query, kind).Order(subjectIDColumn(kind) + " ASC").Scan(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

func (repository *SubjectRepository) MatchNativeSubjects(
	ctx context.Context, workspaceID, userID uint, kind identity.SubjectKind, ids []uint,
) (map[uint]bool, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || userID == 0 {
		return nil, identity.ErrSubjectInvalid
	}
	result := make(map[uint]bool, len(ids))
	for _, id := range ids {
		result[id] = false
	}
	if len(ids) == 0 {
		return result, nil
	}
	var matched []uint
	var query *gorm.DB
	switch kind {
	case identity.SubjectRole:
		query = repository.db.WithContext(ctx).Table("access_roles ar").
			Select("ar.id").
			Joins("JOIN access_user_roles aur ON aur.role_id = ar.id AND aur.user_id = ?", userID).
			Joins("JOIN users u ON u.id = aur.user_id AND u.active = ? AND u.status = ?", true, string(identity.AccountActive)).
			Joins("JOIN workspace_members wm ON wm.user_id = u.id AND wm.workspace_id = ? AND wm.enabled = ?", workspaceID, true).
			Joins("JOIN workspaces w ON w.id = wm.workspace_id AND w.enabled = ?", true).
			Where("ar.enabled = ? AND ar.id IN ?", true, ids)
	case identity.SubjectUserGroup:
		query = repository.db.WithContext(ctx).Table("access_user_groups aug").
			Select("aug.id").
			Joins("JOIN access_user_group_bindings augb ON augb.group_id = aug.id AND augb.user_id = ?", userID).
			Joins("JOIN users u ON u.id = augb.user_id AND u.active = ? AND u.status = ?", true, string(identity.AccountActive)).
			Joins("JOIN workspace_members wm ON wm.user_id = u.id AND wm.workspace_id = ? AND wm.enabled = ?", workspaceID, true).
			Joins("JOIN workspaces w ON w.id = wm.workspace_id AND w.enabled = ?", true).
			Where("aug.enabled = ? AND aug.id IN ?", true, ids)
	default:
		return nil, identity.ErrSubjectInvalid
	}
	if err := query.Pluck(subjectIDColumn(kind), &matched).Error; err != nil {
		return nil, err
	}
	for _, id := range matched {
		result[id] = true
	}
	return result, nil
}

func (repository *SubjectRepository) subjectQuery(
	ctx context.Context, workspaceID uint, kind identity.SubjectKind,
) (*gorm.DB, error) {
	switch kind {
	case identity.SubjectUser:
		return repository.activeWorkspaceUsers(ctx, workspaceID), nil
	case identity.SubjectRole:
		return repository.db.WithContext(ctx).Table("access_roles ar").Where("ar.enabled = ?", true), nil
	case identity.SubjectUserGroup:
		return repository.db.WithContext(ctx).Table("access_user_groups aug").Where("aug.enabled = ?", true), nil
	default:
		return nil, identity.ErrSubjectInvalid
	}
}

func (repository *SubjectRepository) activeWorkspaceUsers(ctx context.Context, workspaceID uint) *gorm.DB {
	return repository.db.WithContext(ctx).Table("users u").
		Joins("JOIN workspace_members wm ON wm.user_id = u.id AND wm.workspace_id = ? AND wm.enabled = ?", workspaceID, true).
		Joins("JOIN workspaces w ON w.id = wm.workspace_id AND w.enabled = ?", true).
		Where("u.active = ? AND u.status = ?", true, string(identity.AccountActive))
}

func (repository *SubjectRepository) requireEnabledWorkspace(ctx context.Context, workspaceID uint) error {
	var count int64
	if err := repository.db.WithContext(ctx).Model(&Workspace{}).
		Where("id = ? AND enabled = ?", workspaceID, true).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return identity.ErrSubjectInvalid
	}
	return nil
}

func selectNativeSubjectColumns(query *gorm.DB, kind identity.SubjectKind) *gorm.DB {
	switch kind {
	case identity.SubjectUser:
		return query.Select("u.id, COALESCE(NULLIF(u.display_name, ''), u.username) AS display_name, u.username AS secondary_text")
	case identity.SubjectRole:
		return query.Select("ar.id, ar.name AS display_name, ar.code AS secondary_text")
	case identity.SubjectUserGroup:
		return query.Select("aug.id, aug.name AS display_name, aug.code AS secondary_text")
	default:
		return query
	}
}

func subjectIDColumn(kind identity.SubjectKind) string {
	switch kind {
	case identity.SubjectUser:
		return "u.id"
	case identity.SubjectRole:
		return "ar.id"
	case identity.SubjectUserGroup:
		return "aug.id"
	default:
		return "id"
	}
}

func subjectOrder(kind identity.SubjectKind) string {
	switch kind {
	case identity.SubjectUser:
		return "display_name ASC, u.username ASC, u.id ASC"
	case identity.SubjectRole:
		return "ar.name ASC, ar.code ASC, ar.id ASC"
	case identity.SubjectUserGroup:
		return "aug.name ASC, aug.code ASC, aug.id ASC"
	default:
		return "id ASC"
	}
}

func subjectLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

var _ identity.NativeSubjectStore = (*SubjectRepository)(nil)

// ResolveSubjectFactUserID keeps canonical resource principals separate from PM
// participant facts. Never interpret the same integer as either kind of identity.
func (repository *SubjectRepository) ResolveSubjectFactUserID(ctx context.Context, workspaceID, userID uint) (uint, error) {
	active, err := repository.HasActiveWorkspaceUser(ctx, workspaceID, userID)
	if err != nil || !active {
		return 0, identity.ErrSubjectInvalid
	}
	var user User
	if err := repository.db.WithContext(ctx).Select("id").Where("id = ?", userID).First(&user).Error; err != nil {
		return 0, err
	}
	return user.ID, nil
}
