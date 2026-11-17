// Package project owns the access formula over project-set and plan-unit
// membership facts. This file keeps the point gates; the SQL-level filters
// live behind AccessFilterBuilder.
package project

import (
	"context"
	"errors"
)

// AdminRoleChecker is a narrow identity port scoped by workspace: is the user
// an active member of this workspace AND holds the access-administrator role?
// The userID may be either the PM-internal users.id or the imported external
// fact ID; the adapter reconciles both.
type AdminRoleChecker interface {
	// IsAccessAdministrator returns true when the user is active in the
	// workspace AND has the access-administrator role.
	IsAccessAdministrator(ctx context.Context, workspaceID, userID uint) (bool, error)
}

// UserActiveChecker reports whether a PM user account is active and belongs
// to the workspace. Every access gate must verify workspace membership. The
// userID may be either the PM-internal users.id or the external fact ID.
type UserActiveChecker interface {
	IsActive(ctx context.Context, workspaceID, userID uint) (bool, error)
}

// AccessService implements AccessChecker, PlanUnitAccessChecker and
// AccessFilterBuilder using the contract's access formulas:
//
//	project set: Admin OR (active AND (creator OR any-unit owner/participant))
//	plan unit:   Admin OR (active AND (set creator OR unit owner OR participant))
type AccessService struct {
	adminRole     AdminRoleChecker
	activeCheck   UserActiveChecker
	creatorReader SetCreatorReader
	unitAccess    PlanUnitAccessReader
	filterBuilder AccessFilterBuilder
}

func NewAccessService(
	adminRole AdminRoleChecker,
	activeCheck UserActiveChecker,
	creatorReader SetCreatorReader,
	unitAccess PlanUnitAccessReader,
	filterBuilder AccessFilterBuilder,
) (*AccessService, error) {
	if adminRole == nil || activeCheck == nil || creatorReader == nil || unitAccess == nil || filterBuilder == nil {
		return nil, errors.New("project access service requires all dependencies")
	}
	return &AccessService{
		adminRole:     adminRole,
		activeCheck:   activeCheck,
		creatorReader: creatorReader,
		unitAccess:    unitAccess,
		filterBuilder: filterBuilder,
	}, nil
}

// CanAccessProject applies the project set visibility formula:
//
//	Admin OR (active AND (creator OR owner/participant of any plan unit))
//
// The admin branch is evaluated first so that a platform super_admin (whose
// account is active) is granted access without requiring workspace
// membership; the admin branch itself validates account activeness.
func (svc *AccessService) CanAccessProject(ctx context.Context, workspaceID, projectSetID, userID uint) (bool, error) {
	if workspaceID == 0 || projectSetID == 0 || userID == 0 {
		return false, ErrParticipantInvalid
	}
	isAdmin, err := svc.adminRole.IsAccessAdministrator(ctx, workspaceID, userID)
	if err != nil {
		return false, err
	}
	if isAdmin {
		return true, nil
	}
	active, err := svc.activeCheck.IsActive(ctx, workspaceID, userID)
	if err != nil || !active {
		return false, err
	}
	creatorID, err := svc.creatorReader.ProjectCreatorID(ctx, workspaceID, projectSetID)
	if err != nil {
		return false, err
	}
	if creatorID == userID {
		return true, nil
	}
	return svc.unitAccess.HasAnyPlanUnitAccess(ctx, workspaceID, projectSetID, userID)
}

// CanAccessPlanUnit applies the plan unit data formula:
//
//	Admin OR (active AND (set creator OR unit owner OR unit participant))
func (svc *AccessService) CanAccessPlanUnit(ctx context.Context, workspaceID, planUnitID, userID uint) (bool, error) {
	if workspaceID == 0 || planUnitID == 0 || userID == 0 {
		return false, ErrParticipantInvalid
	}
	isAdmin, err := svc.adminRole.IsAccessAdministrator(ctx, workspaceID, userID)
	if err != nil {
		return false, err
	}
	if isAdmin {
		return true, nil
	}
	active, err := svc.activeCheck.IsActive(ctx, workspaceID, userID)
	if err != nil || !active {
		return false, err
	}
	return svc.unitAccess.HasPlanUnitAccess(ctx, workspaceID, planUnitID, userID)
}

// CanManagePlanUnitMembers applies the member-management formula:
//
//	Admin OR (active AND (set creator OR unit owner))
func (svc *AccessService) CanManagePlanUnitMembers(ctx context.Context, workspaceID, planUnitID, userID uint) (bool, error) {
	if workspaceID == 0 || planUnitID == 0 || userID == 0 {
		return false, ErrParticipantInvalid
	}
	isAdmin, err := svc.adminRole.IsAccessAdministrator(ctx, workspaceID, userID)
	if err != nil {
		return false, err
	}
	if isAdmin {
		return true, nil
	}
	active, err := svc.activeCheck.IsActive(ctx, workspaceID, userID)
	if err != nil || !active {
		return false, err
	}
	return svc.unitAccess.CanManagePlanUnitMembers(ctx, workspaceID, planUnitID, userID)
}

// PlanUnitManageFactBatcher is the optional repository capability that
// evaluates the management fact for many plan units in one query. Consumers
// holding the point-gate interface can type-assert it to avoid per-row
// queries on list endpoints.
type PlanUnitManageFactBatcher interface {
	CanManagePlanUnitMembersBatch(
		ctx context.Context, workspaceID uint, planUnitIDs []uint, userID uint,
	) (map[uint]bool, error)
}

// CanManagePlanUnitMembersBatch applies the member-management formula across
// many plan units. The admin and active checks run once for the whole batch;
// when the underlying reader cannot batch, it falls back to the per-unit
// point gate. Every requested unit gets an entry, so callers must not loop
// per unit to fill gaps.
func (svc *AccessService) CanManagePlanUnitMembersBatch(
	ctx context.Context, workspaceID uint, planUnitIDs []uint, userID uint,
) (map[uint]bool, error) {
	if workspaceID == 0 || userID == 0 || len(planUnitIDs) == 0 {
		return map[uint]bool{}, ErrParticipantInvalid
	}
	isAdmin, err := svc.adminRole.IsAccessAdministrator(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if isAdmin {
		return allPlanUnitFacts(planUnitIDs, true), nil
	}
	active, err := svc.activeCheck.IsActive(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	if !active {
		return allPlanUnitFacts(planUnitIDs, false), nil
	}
	if batcher, ok := svc.unitAccess.(PlanUnitManageFactBatcher); ok {
		return batcher.CanManagePlanUnitMembersBatch(ctx, workspaceID, planUnitIDs, userID)
	}
	facts := make(map[uint]bool, len(planUnitIDs))
	for _, planUnitID := range planUnitIDs {
		allowed, err := svc.unitAccess.CanManagePlanUnitMembers(ctx, workspaceID, planUnitID, userID)
		if err != nil {
			return nil, err
		}
		facts[planUnitID] = allowed
	}
	return facts, nil
}

func allPlanUnitFacts(planUnitIDs []uint, value bool) map[uint]bool {
	facts := make(map[uint]bool, len(planUnitIDs))
	for _, planUnitID := range planUnitIDs {
		facts[planUnitID] = value
	}
	return facts
}

func (svc *AccessService) RequireProjectAccess(ctx context.Context, workspaceID, projectSetID, userID uint) error {
	allowed, err := svc.CanAccessProject(ctx, workspaceID, projectSetID, userID)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrProjectAccessNotFound
	}
	return nil
}

// IsAccessAdministrator delegates to the admin-role port so the transport
// layer can authorize member management without reading role strings.
func (svc *AccessService) IsAccessAdministrator(ctx context.Context, workspaceID, userID uint) (bool, error) {
	if workspaceID == 0 || userID == 0 {
		return false, ErrParticipantInvalid
	}
	return svc.adminRole.IsAccessAdministrator(ctx, workspaceID, userID)
}

// BuildMemberFilter produces a SQL filter. For admins it returns an empty
// filter (no restriction). Otherwise it builds the table-specific membership
// filter. The `table` parameter is forwarded to the adapter for column
// qualification.
//
// A user who is not an active member of the workspace gets a match-nothing
// filter: returning an empty filter here would silently grant the admin fast
// path to a non-member.
func (svc *AccessService) BuildMemberFilter(ctx context.Context, table, projectColumn string, workspaceID, userID uint) (AccessFilter, error) {
	if workspaceID == 0 || userID == 0 {
		return AccessFilter{}, ErrParticipantInvalid
	}
	isAdmin, err := svc.adminRole.IsAccessAdministrator(ctx, workspaceID, userID)
	if err != nil {
		return AccessFilter{}, err
	}
	if isAdmin {
		return AccessFilter{}, nil
	}
	active, err := svc.activeCheck.IsActive(ctx, workspaceID, userID)
	if err != nil {
		return AccessFilter{}, err
	}
	if !active {
		// Fail closed: an inactive or non-member user matches no project.
		return AccessFilter{SQL: "1 = 0"}, nil
	}
	return svc.filterBuilder.BuildMemberFilter(ctx, table, projectColumn, workspaceID, userID)
}

// VerifyProjectMembers checks that every userID holds some membership fact in
// the project set: creator, plan-unit owner, or plan-unit participant. Admin
// is NOT automatically a member for assignment purposes (per contract §2.6).
// All userIDs must be active.
func (svc *AccessService) VerifyProjectMembers(ctx context.Context, workspaceID, projectSetID uint, userIDs []uint) error {
	if workspaceID == 0 || projectSetID == 0 {
		return ErrParticipantInvalid
	}
	for _, userID := range userIDs {
		if userID == 0 {
			continue
		}
		active, err := svc.activeCheck.IsActive(ctx, workspaceID, userID)
		if err != nil {
			return err
		}
		if !active {
			return ErrParticipantNotOwner
		}
		member, err := svc.unitAccess.IsProjectSetMemberFact(ctx, workspaceID, projectSetID, userID)
		if err != nil {
			return err
		}
		if !member {
			return ErrParticipantNotOwner
		}
	}
	return nil
}
