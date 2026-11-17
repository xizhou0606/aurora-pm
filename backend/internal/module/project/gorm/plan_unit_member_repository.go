package projectgorm

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
	"github.com/xizhou0606/aurora-pm/backend/internal/module/task"
)

// PlanUnitMemberRepository persists version-level owner + participant
// membership: plan_units.owner_user_id plus plan_unit_participants rows. It
// embeds the workspace user directory for member validation and option
// search.
type PlanUnitMemberRepository struct {
	*MemberRepository
	taskHandover func(*gorm.DB) task.MemberHandover
}

func NewPlanUnitMemberRepository(db *gorm.DB, factories ...func(*gorm.DB) task.MemberHandover) *PlanUnitMemberRepository {
	r := &PlanUnitMemberRepository{MemberRepository: NewMemberRepository(db)}
	if len(factories) > 0 {
		r.taskHandover = factories[0]
	}
	return r
}

// loadPlanUnitMemberState reads the unit fact plus its participant rows.
func (repository *PlanUnitMemberRepository) loadPlanUnitMemberState(
	ctx context.Context, db *gorm.DB, workspaceID, planUnitID uint,
) (project.MemberState, uint, error) {
	var unit PlanUnit
	err := db.WithContext(ctx).
		Where("id = ? AND workspace_id = ?", planUnitID, workspaceID).First(&unit).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return project.MemberState{}, 0, project.ErrParticipantPlanUnitNotFound
	}
	if err != nil {
		return project.MemberState{}, 0, err
	}
	state := project.MemberState{
		OwnerUserID: unit.OwnerUserID, UpdatedAt: unit.UpdatedAt, ParticipantUserIDs: []uint{},
	}
	var rows []PlanUnitParticipant
	if err := db.WithContext(ctx).
		Where("workspace_id = ? AND plan_unit_id = ?", workspaceID, planUnitID).
		Order("user_id ASC").Find(&rows).Error; err != nil {
		return project.MemberState{}, 0, err
	}
	ids := make([]uint, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.UserID)
	}
	state.ParticipantUserIDs = ids
	return state, unit.ProjectSetID, nil
}

// LoadPlanUnitMemberState returns the owner + participant state of one unit.
func (repository *PlanUnitMemberRepository) LoadPlanUnitMemberState(
	ctx context.Context, workspaceID, planUnitID uint,
) (project.MemberState, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || planUnitID == 0 {
		return project.MemberState{}, project.ErrParticipantInvalid
	}
	state, _, err := repository.loadPlanUnitMemberState(ctx, repository.db, workspaceID, planUnitID)
	return state, err
}

// SavePlanUnitMemberState atomically updates the unit owner fact and replaces
// the participant set inside one transaction. A stale expected_updated_at
// returns ErrParticipantStale without any write. Every userID is validated as
// active and workspace-scoped before any write.
func (repository *PlanUnitMemberRepository) SavePlanUnitMemberState(
	ctx context.Context,
	workspaceID, planUnitID uint,
	state project.MemberState,
	meta project.MemberMutationMeta,
) (project.MemberState, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || planUnitID == 0 ||
		state.OwnerUserID == nil || *state.OwnerUserID == 0 || meta.ActorUserID == 0 {
		return project.MemberState{}, project.ErrParticipantInvalid
	}
	var verifier *MemberRepository
	if repository.db != nil {
		verifier = NewMemberRepository(repository.db)
	}
	if verifier == nil {
		return project.MemberState{}, project.ErrParticipantInvalid
	}
	if err := verifier.VerifyMemberUsers(ctx, workspaceID, []uint{*state.OwnerUserID}); err != nil {
		return project.MemberState{}, err
	}
	validIDs := dedupParticipantIDs(state.ParticipantUserIDs)
	for _, userID := range validIDs {
		if userID == 0 {
			return project.MemberState{}, project.ErrParticipantInvalid
		}
		if userID == *state.OwnerUserID {
			return project.MemberState{}, project.ErrParticipantIsOwner
		}
	}
	if err := verifier.VerifyMemberUsers(ctx, workspaceID, validIDs); err != nil {
		return project.MemberState{}, err
	}
	state.ParticipantUserIDs = validIDs

	var result project.MemberState
	err := repository.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var unit PlanUnit
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("id = ? AND workspace_id = ?", planUnitID, workspaceID).
			First(&unit).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return project.ErrParticipantPlanUnitNotFound
			}
			return err
		}
		if meta.ExpectedAt != nil && !unit.UpdatedAt.Equal(*meta.ExpectedAt) {
			return project.ErrParticipantStale
		}
		current, _, err := repository.loadPlanUnitMemberState(ctx, tx, workspaceID, planUnitID)
		if err != nil {
			return err
		}
		if err := repository.resolveMemberRemoval(ctx, tx, workspaceID, planUnitID, current, state, meta); err != nil {
			return err
		}
		now := time.Now().UTC()
		if err := tx.Model(&unit).Updates(map[string]any{
			"owner_user_id": *state.OwnerUserID, "updated_at": now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("workspace_id = ? AND plan_unit_id = ?", workspaceID, planUnitID).
			Delete(&PlanUnitParticipant{}).Error; err != nil {
			return err
		}
		for _, userID := range state.ParticipantUserIDs {
			row := PlanUnitParticipant{
				WorkspaceID: workspaceID, PlanUnitID: planUnitID, UserID: userID,
				CreatedByUserID: meta.ActorUserID, CreatedAt: now,
			}
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
				return err
			}
		}
		// Audit row in the same transaction: member mutation fails
		// atomically if the audit write fails, and vice versa.
		audit := PlanUnitMemberMutationAudit{
			WorkspaceID:    workspaceID,
			PlanUnitID:     planUnitID,
			ActorUserID:    meta.ActorUserID,
			Operation:      "replace_members",
			OwnerUserID:    state.OwnerUserID,
			ParticipantIDs: participantIDsCompact(state.ParticipantUserIDs),
			Reason:         meta.Reason,
			CreatedAt:      now,
		}
		if err := tx.Create(&audit).Error; err != nil {
			return err
		}
		loaded, _, err := repository.loadPlanUnitMemberState(ctx, tx, workspaceID, planUnitID)
		if err != nil {
			return err
		}
		result = loaded
		return nil
	})
	if err != nil {
		return project.MemberState{}, err
	}
	return result, nil
}

// ListPlanUnitMemberUsers returns the deduplicated owner + participant union
// paged and searchable. Query matching is case-insensitive on
// username/display_name/email.
func (repository *PlanUnitMemberRepository) ListPlanUnitMemberUsers(
	ctx context.Context,
	workspaceID, planUnitID uint,
	query string,
	page, pageSize int,
) (project.MemberPage, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || planUnitID == 0 {
		return project.MemberPage{}, project.ErrParticipantInvalid
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 20
	}
	memberIDs, err := repository.unitMemberFactIDs(ctx, workspaceID, planUnitID)
	if err != nil {
		return project.MemberPage{}, err
	}
	base := repository.db.WithContext(ctx).Table("users AS u").
		Where("u.active = ? AND u.status = ?", true, "active")
	if len(memberIDs) > 0 {
		// Membership rows contain canonical internal users.id after cutover.
		base = base.Where("u.id IN ?", memberIDs)
	} else {
		base = base.Where("1 = 0")
	}
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

func (repository *PlanUnitMemberRepository) unitMemberFactIDs(ctx context.Context, workspaceID, planUnitID uint) ([]uint, error) {
	state, _, err := repository.loadPlanUnitMemberState(ctx, repository.db, workspaceID, planUnitID)
	if err != nil {
		return nil, err
	}
	seen := make(map[uint]struct{}, len(state.ParticipantUserIDs)+1)
	if state.OwnerUserID != nil {
		seen[*state.OwnerUserID] = struct{}{}
	}
	for _, id := range state.ParticipantUserIDs {
		seen[id] = struct{}{}
	}
	ids := make([]uint, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids, nil
}

// participantIDsCompact renders deduplicated participant IDs as a
// comma-separated audit string.
func participantIDsCompact(ids []uint) string {
	ids = dedupParticipantIDs(ids)
	if len(ids) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		parts = append(parts, strconv.FormatUint(uint64(id), 10))
	}
	return strings.Join(parts, ",")
}

// dedupParticipantIDs returns a copy of ids with duplicates removed. It does
// NOT filter out zero or owner — the caller must validate those separately.
func dedupParticipantIDs(ids []uint) []uint {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[uint]struct{}, len(ids))
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}
