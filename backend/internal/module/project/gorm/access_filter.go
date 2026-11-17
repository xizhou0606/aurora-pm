package projectgorm

import (
	"context"
	"strings"

	"gorm.io/gorm"

	project "github.com/xizhou0606/aurora-pm/backend/internal/module/project"
)

// PlanUnitAccessRepository resolves the version-level membership facts behind
// the access formula and builds the SQL row filters. It implements
// project.SetCreatorReader, project.PlanUnitAccessReader and
// project.AccessFilterBuilder.
type PlanUnitAccessRepository struct{ db *gorm.DB }

func NewPlanUnitAccessRepository(db *gorm.DB) *PlanUnitAccessRepository {
	return &PlanUnitAccessRepository{db: db}
}

// unitRolePredicate is the owner-or-participant predicate over a plan_units
// alias. Args: userID (owner), userID (participant).
const unitRolePredicate = "(pu.owner_user_id = ? OR ? IN (" +
	"SELECT pup.user_id FROM plan_unit_participants pup WHERE pup.plan_unit_id = pu.id AND pup.workspace_id = pu.workspace_id))"

// ProjectCreatorID returns project_sets.created_by_user_id, 0 when unset.
func (repository *PlanUnitAccessRepository) ProjectCreatorID(
	ctx context.Context, workspaceID, projectSetID uint,
) (uint, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || projectSetID == 0 {
		return 0, project.ErrParticipantInvalid
	}
	var creator *uint
	err := repository.db.WithContext(ctx).
		Table("project_sets").
		Select("created_by_user_id").
		Where("id = ? AND workspace_id = ?", projectSetID, workspaceID).
		Scan(&creator).Error
	if err != nil {
		return 0, err
	}
	if creator == nil {
		return 0, nil
	}
	return *creator, nil
}

// HasAnyPlanUnitAccess reports whether the user owns or participates in any
// plan unit under the project set.
func (repository *PlanUnitAccessRepository) HasAnyPlanUnitAccess(
	ctx context.Context, workspaceID, projectSetID, userID uint,
) (bool, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || projectSetID == 0 || userID == 0 {
		return false, project.ErrParticipantInvalid
	}
	var count int64
	err := repository.db.WithContext(ctx).
		Table("plan_units AS pu").
		Where("pu.workspace_id = ? AND pu.project_new_id = ?", workspaceID, projectSetID).
		Where(unitRolePredicate, userID, userID).
		Limit(1).Count(&count).Error
	return count > 0, err
}

// HasPlanUnitAccess reports the version-level data-access fact: set creator,
// unit owner, or unit participant.
func (repository *PlanUnitAccessRepository) HasPlanUnitAccess(
	ctx context.Context, workspaceID, planUnitID, userID uint,
) (bool, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || planUnitID == 0 || userID == 0 {
		return false, project.ErrParticipantInvalid
	}
	var count int64
	err := repository.db.WithContext(ctx).
		Table("plan_units AS pu").
		Joins("JOIN project_sets src ON src.id = pu.project_new_id AND src.workspace_id = pu.workspace_id").
		Where("pu.id = ? AND pu.workspace_id = ?", planUnitID, workspaceID).
		Where("(src.created_by_user_id = ? OR "+unitRolePredicate+")", userID, userID, userID).
		Limit(1).Count(&count).Error
	return count > 0, err
}

// CanManagePlanUnitMembers reports the version-level management fact: set
// creator or unit owner.
func (repository *PlanUnitAccessRepository) CanManagePlanUnitMembers(
	ctx context.Context, workspaceID, planUnitID, userID uint,
) (bool, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || planUnitID == 0 || userID == 0 {
		return false, project.ErrParticipantInvalid
	}
	var count int64
	err := repository.db.WithContext(ctx).
		Table("plan_units AS pu").
		Joins("JOIN project_sets src ON src.id = pu.project_new_id AND src.workspace_id = pu.workspace_id").
		Where("pu.id = ? AND pu.workspace_id = ?", planUnitID, workspaceID).
		Where("(src.created_by_user_id = ? OR pu.owner_user_id = ?)", userID, userID).
		Limit(1).Count(&count).Error
	return count > 0, err
}

// CanManagePlanUnitMembersBatch evaluates the management fact for many plan
// units in one query. Absent IDs resolve to false — the caller receives a
// complete map so list endpoints cannot loop per unit.
func (repository *PlanUnitAccessRepository) CanManagePlanUnitMembersBatch(
	ctx context.Context, workspaceID uint, planUnitIDs []uint, userID uint,
) (map[uint]bool, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || userID == 0 || len(planUnitIDs) == 0 {
		return map[uint]bool{}, project.ErrParticipantInvalid
	}
	var manageable []uint
	err := repository.db.WithContext(ctx).
		Table("plan_units AS pu").
		Joins("JOIN project_sets src ON src.id = pu.project_new_id AND src.workspace_id = pu.workspace_id").
		Where("pu.id IN ? AND pu.workspace_id = ?", planUnitIDs, workspaceID).
		Where("(src.created_by_user_id = ? OR pu.owner_user_id = ?)", userID, userID).
		Pluck("pu.id", &manageable).Error
	if err != nil {
		return nil, err
	}
	facts := make(map[uint]bool, len(planUnitIDs))
	for _, planUnitID := range planUnitIDs {
		facts[planUnitID] = false
	}
	for _, planUnitID := range manageable {
		facts[planUnitID] = true
	}
	return facts, nil
}

// IsProjectSetMemberFact reports whether the user is the set creator or holds
// any unit role under the set.
func (repository *PlanUnitAccessRepository) IsProjectSetMemberFact(
	ctx context.Context, workspaceID, projectSetID, userID uint,
) (bool, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || projectSetID == 0 || userID == 0 {
		return false, project.ErrParticipantInvalid
	}
	creator, err := repository.ProjectCreatorID(ctx, workspaceID, projectSetID)
	if err != nil {
		return false, err
	}
	if creator == userID {
		return true, nil
	}
	return repository.HasAnyPlanUnitAccess(ctx, workspaceID, projectSetID, userID)
}

// BuildMemberFilter returns the table-specific SQL EXISTS filter derived from
// the version-level membership facts:
//
//	project_sets            -> creator OR any-unit role under the set
//	requirements            -> set creator OR unit role via version_task_new_id
//	work_tasks/quality_bugs -> set creator OR unit role via requirement link
//	default                 -> set formula on the given project column
//
// The outer column MUST be qualified with the outer table name; inner columns
// are qualified with the same table so the comparison can never degenerate
// into a tautology against the subquery's own columns.
func (repository *PlanUnitAccessRepository) BuildMemberFilter(
	ctx context.Context,
	table, projectColumn string,
	workspaceID, userID uint,
) (project.AccessFilter, error) {
	if repository == nil || repository.db == nil || workspaceID == 0 || userID == 0 ||
		strings.TrimSpace(projectColumn) == "" {
		return project.AccessFilter{}, project.ErrParticipantInvalid
	}
	column := strings.TrimSpace(projectColumn)
	if t := strings.TrimSpace(table); t != "" && !strings.Contains(column, ".") {
		column = t + "." + column
	}
	qualify := func(name string) string {
		if t := strings.TrimSpace(table); t != "" && !strings.Contains(name, ".") {
			return t + "." + name
		}
		return name
	}
	setCreatorSubquery := "EXISTS (SELECT 1 FROM project_sets src WHERE src.id = " + column +
		" AND src.created_by_user_id = ?)"
	unitRoleSubquery := "EXISTS (SELECT 1 FROM plan_units pu WHERE pu.project_new_id = " + column +
		" AND " + unitRolePredicate + ")"
	setFilter := project.AccessFilter{
		SQL:  setCreatorSubquery + " OR " + unitRoleSubquery,
		Args: []any{userID, userID, userID},
	}

	switch strings.TrimSpace(table) {
	case "project_sets":
		return setFilter, nil
	case "plan_units":
		// Within an accessible set the creator sees every unit; anyone else
		// sees only the units they own or participate in. The creator check
		// must compare the unit's owning SET (project_new_id) — never the
		// unit's own id, which would collide across entities. The qualifier
		// follows the caller's alias when the project column carries one.
		qualifier := strings.TrimSpace(table)
		if i := strings.LastIndex(column, "."); i > 0 {
			qualifier = column[:i]
		}
		return project.AccessFilter{
			SQL: "EXISTS (SELECT 1 FROM project_sets src WHERE src.id = " + qualifier + ".project_new_id" +
				" AND src.created_by_user_id = ?) OR (EXISTS (SELECT 1 FROM plan_units pu WHERE pu.id = " + qualifier +
				".id AND " + unitRolePredicate + "))",
			Args: []any{userID, userID, userID},
		}, nil
	case "requirements":
		unitColumn := qualify("version_task_new_id")
		return project.AccessFilter{
			SQL: setCreatorSubquery + " OR EXISTS (SELECT 1 FROM plan_units pu WHERE pu.id = " + unitColumn +
				" AND " + unitRolePredicate + ")",
			Args: []any{userID, userID, userID},
		}, nil
	case "work_tasks":
		// Tasks carry the unit directly in project_id; older rows may only
		// reference a requirement, so keep the requirement fallback.
		unitColumn := qualify("project_id")
		requirementColumn := qualify("requirement_id")
		return project.AccessFilter{
			SQL: setCreatorSubquery +
				" OR (" + unitColumn + " IS NOT NULL AND EXISTS (SELECT 1 FROM plan_units pu WHERE pu.id = " + unitColumn +
				" AND " + unitRolePredicate + "))" +
				" OR EXISTS (SELECT 1 FROM requirements r WHERE r.id = " + requirementColumn +
				" AND EXISTS (SELECT 1 FROM plan_units pu WHERE pu.id = r.version_task_new_id AND " + unitRolePredicate + "))",
			Args: []any{userID, userID, userID, userID, userID},
		}, nil
	case "quality_bugs":
		requirementColumn := qualify("requirement_id")
		return project.AccessFilter{
			SQL: setCreatorSubquery + " OR EXISTS (SELECT 1 FROM requirements r WHERE r.id = " + requirementColumn +
				" AND EXISTS (SELECT 1 FROM plan_units pu WHERE pu.id = r.version_task_new_id AND " + unitRolePredicate + "))",
			Args: []any{userID, userID, userID},
		}, nil
	default:
		return setFilter, nil
	}
}

// Compile-time interface checks.
var (
	_ project.SetCreatorReader     = (*PlanUnitAccessRepository)(nil)
	_ project.PlanUnitAccessReader = (*PlanUnitAccessRepository)(nil)
	_ project.AccessFilterBuilder  = (*PlanUnitAccessRepository)(nil)
)
