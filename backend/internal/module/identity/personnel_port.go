package identity

import "context"

type PersonnelRepository interface {
	ListPersonnelUsers(context.Context, PersonnelUserFilter) (PersonnelUserPage, error)
	FindPersonnelUser(context.Context, uint) (PersonnelUser, error)
	ListUserGroups(context.Context, bool) ([]UserGroup, error)
	FindUserGroup(context.Context, uint) (UserGroup, error)
	CreateUserGroup(context.Context, uint, UserGroupInput) (UserGroup, error)
	UpdateUserGroup(context.Context, uint, uint, UserGroupUpdate) (UserGroup, error)
	DeleteUserGroup(context.Context, uint, uint) error
	ReplacePersonnelUserGroup(context.Context, uint, uint, uint) error
	ReplacePersonnelUserRoles(context.Context, uint, uint, []uint) error
	ReplacePersonnelUserAccess(context.Context, uint, uint, uint, []uint) error
	TransitionPersonnelUserStatus(context.Context, uint, uint, AccountStatus, string) (PersonnelUser, error)
	ListPersonnelAuditEvents(context.Context, PersonnelAuditFilter) ([]PersonnelAuditEvent, error)
}

// PermissionGrantStore is intentionally implemented by the future canonical
// permission-catalog adapter, not by personnel persistence. It gives roles and
// groups the same typed binding surface without duplicating permission codes.
type PermissionGrantStore interface {
	PermissionNodeIDs(context.Context, PermissionSubjectRef) ([]uint, error)
	ReplacePermissionNodeIDs(context.Context, PermissionSubjectRef, []uint) error
}
