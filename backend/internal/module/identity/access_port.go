package identity

import "context"

// AccessRepository is the identity module's persistence boundary for global
// roles and registered service interfaces. User bindings use PM users.id.
type AccessRepository interface {
	ListActiveUsers(context.Context) ([]UserSummary, error)
	ListRoles(context.Context, bool) ([]Role, error)
	FindRole(context.Context, uint) (Role, error)
	CreateRole(context.Context, RoleInput) (Role, error)
	CreateRoleAudited(context.Context, uint, RoleInput) (Role, error)
	UpdateRole(context.Context, uint, RoleUpdate) (Role, error)
	UpdateRoleAudited(context.Context, uint, uint, RoleUpdate) (Role, error)
	DeleteRole(context.Context, uint) error
	DeleteRoleAudited(context.Context, uint, uint) error

	ListServiceInterfaces(context.Context, bool) ([]ServiceInterface, error)
	FindServiceInterface(context.Context, uint) (ServiceInterface, error)
	CreateServiceInterface(context.Context, ServiceInterfaceInput) (ServiceInterface, error)
	CreateServiceInterfaces(context.Context, []ServiceInterfaceInput) ([]ServiceInterface, error)
	UpdateServiceInterface(context.Context, uint, ServiceInterfaceUpdate) (ServiceInterface, error)
	DeleteServiceInterface(context.Context, uint) error

	UserRoleIDs(context.Context, uint) ([]uint, error)
	ReplaceUserRoles(context.Context, uint, []uint) error
	RoleServiceInterfaceIDs(context.Context, uint) ([]uint, error)
	ReplaceRoleServiceInterfaces(context.Context, uint, []uint) error
	RoleActionCodes(context.Context, uint) ([]string, error)
	ReplaceRoleActionCodes(context.Context, uint, uint, []string) error
	GroupServiceInterfaceIDs(context.Context, uint) ([]uint, error)
	ReplaceGroupServiceInterfaces(context.Context, uint, uint, []uint) error
	GroupActionCodes(context.Context, uint) ([]string, error)
	ReplaceGroupActionCodes(context.Context, uint, uint, []string) error
	RoleMenuIDs(context.Context, uint) ([]uint, error)
	ReplaceRoleMenus(context.Context, uint, []uint) error
	EnsureAdministratorMenus(context.Context, []uint) error
	GroupMenuIDs(context.Context, uint) ([]uint, error)
	ReplaceGroupMenus(context.Context, uint, uint, []uint) error
	DirectMenuIDsForUser(context.Context, uint) ([]uint, error)
	EffectiveActionCodes(context.Context, uint) ([]string, error)
	CountMenuGrants(context.Context, uint) (int64, error)
	Authorize(context.Context, uint, string, string) (AuthorizationResult, error)
	Bootstrap(context.Context, BootstrapSpec) error
}

// Authorizer is consumed at the HTTP entry boundary. It receives only the PM
// user ID and canonical matched route identity.
type Authorizer interface {
	Authorize(context.Context, uint, string, string) (AuthorizationResult, error)
}

// MenuGrantValidator is implemented by the workbench menu module. Identity
// validates role-menu input through this port without reading menu storage.
type MenuGrantValidator interface {
	ValidateEnabledMenuIDs(context.Context, []uint) error
}

// ServiceInterfaceValidator is implemented by the HTTP route inventory. It
// rejects browser-supplied method/template pairs that are not live protected
// Gin routes before a permission resource is persisted.
type ServiceInterfaceValidator interface {
	ValidateServiceInterface(context.Context, string, string) error
}

// ServiceInterfaceCandidate is a live canonical HTTP route that may be
// registered as an access resource. The HTTP transport owns the inventory;
// identity only consumes this narrow contract.
type ServiceInterfaceCandidate struct {
	HTTPMethod    string
	RouteTemplate string
	ActionCode    string
}

// ServiceInterfaceInventory is the read-only route catalog used by access
// administration. It intentionally exposes no Gin types to the identity
// module.
type ServiceInterfaceInventory interface {
	ServiceInterfaceValidator
	ServiceInterfaceCandidates(context.Context) []ServiceInterfaceCandidate
}
