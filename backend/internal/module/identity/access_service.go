package identity

import (
	"context"
	"errors"
	"strings"

	"github.com/xizhou0606/aurora-pm/backend/internal/generated/accesscatalog"
)

// AccessService owns global role assignments and HTTP interface grants. It
// deliberately accepts only a PM user ID for authorization decisions.
type AccessService struct {
	repository     AccessRepository
	menuValidator  MenuGrantValidator
	routeValidator ServiceInterfaceValidator
}

func NewAccessService(repository AccessRepository, menuValidator MenuGrantValidator) *AccessService {
	return &AccessService{repository: repository, menuValidator: menuValidator}
}

// SetMenuGrantValidator is used only by the composition root to finish the
// identity/workbench dependency cycle before the HTTP server starts.
func (service *AccessService) SetMenuGrantValidator(menuValidator MenuGrantValidator) {
	service.menuValidator = menuValidator
}

// SetServiceInterfaceValidator is used by the HTTP composition root after Gin
// has registered all routes and can expose its immutable route inventory.
func (service *AccessService) SetServiceInterfaceValidator(validator ServiceInterfaceValidator) {
	service.routeValidator = validator
}

func (service *AccessService) ListActiveUsers(ctx context.Context) ([]UserSummary, error) {
	return service.repository.ListActiveUsers(ctx)
}

func (service *AccessService) ListRoles(ctx context.Context, includeDisabled bool) ([]Role, error) {
	return service.repository.ListRoles(ctx, includeDisabled)
}

func (service *AccessService) Role(ctx context.Context, id uint) (Role, error) {
	if id == 0 {
		return Role{}, ErrAccessRoleNotFound
	}
	return service.repository.FindRole(ctx, id)
}

func (service *AccessService) CreateRole(ctx context.Context, input RoleInput) (Role, error) {
	return service.createRole(ctx, 0, input)
}

func (service *AccessService) CreateRoleAudited(ctx context.Context, actorUserID uint, input RoleInput) (Role, error) {
	if actorUserID == 0 {
		return Role{}, ErrPersonnelActorRequired
	}
	return service.createRole(ctx, actorUserID, input)
}

func (service *AccessService) createRole(ctx context.Context, actorUserID uint, input RoleInput) (Role, error) {
	name, err := normalizeRoleName(input.Name)
	if err != nil {
		return Role{}, err
	}
	description, err := normalizeAccessDescription(input.Description)
	if err != nil {
		return Role{}, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		code, err := newRoleCode()
		if err != nil {
			return Role{}, err
		}
		normalized := RoleInput{
			Code: code, Name: name, Description: description, Enabled: input.Enabled,
		}
		var role Role
		if actorUserID > 0 {
			role, err = service.repository.CreateRoleAudited(ctx, actorUserID, normalized)
		} else {
			role, err = service.repository.CreateRole(ctx, normalized)
		}
		if !errors.Is(err, ErrAccessCodeConflict) {
			return role, err
		}
	}
	return Role{}, ErrAccessCodeConflict
}

func (service *AccessService) UpdateRole(ctx context.Context, id uint, input RoleUpdate) (Role, error) {
	return service.updateRole(ctx, 0, id, input)
}

func (service *AccessService) UpdateRoleAudited(ctx context.Context, actorUserID, id uint, input RoleUpdate) (Role, error) {
	if actorUserID == 0 {
		return Role{}, ErrPersonnelActorRequired
	}
	return service.updateRole(ctx, actorUserID, id, input)
}

func (service *AccessService) updateRole(ctx context.Context, actorUserID, id uint, input RoleUpdate) (Role, error) {
	if id == 0 {
		return Role{}, ErrAccessRoleNotFound
	}
	if input.Name != nil {
		value, err := normalizeRoleName(*input.Name)
		if err != nil {
			return Role{}, err
		}
		input.Name = &value
	}
	if input.Description != nil {
		value, err := normalizeAccessDescription(*input.Description)
		if err != nil {
			return Role{}, err
		}
		input.Description = &value
	}
	if actorUserID > 0 {
		return service.repository.UpdateRoleAudited(ctx, actorUserID, id, input)
	}
	return service.repository.UpdateRole(ctx, id, input)
}

func (service *AccessService) DeleteRole(ctx context.Context, id uint) error {
	return service.deleteRole(ctx, 0, id)
}

func (service *AccessService) DeleteRoleAudited(ctx context.Context, actorUserID, id uint) error {
	if actorUserID == 0 {
		return ErrPersonnelActorRequired
	}
	return service.deleteRole(ctx, actorUserID, id)
}

func (service *AccessService) deleteRole(ctx context.Context, actorUserID, id uint) error {
	if id == 0 {
		return ErrAccessRoleNotFound
	}
	if actorUserID > 0 {
		return service.repository.DeleteRoleAudited(ctx, actorUserID, id)
	}
	return service.repository.DeleteRole(ctx, id)
}

func (service *AccessService) ListServiceInterfaces(ctx context.Context, includeDisabled bool) ([]ServiceInterface, error) {
	return service.repository.ListServiceInterfaces(ctx, includeDisabled)
}

func (service *AccessService) ServiceInterface(ctx context.Context, id uint) (ServiceInterface, error) {
	if id == 0 {
		return ServiceInterface{}, ErrAccessServiceInterfaceNotFound
	}
	return service.repository.FindServiceInterface(ctx, id)
}

func (service *AccessService) CreateServiceInterface(ctx context.Context, input ServiceInterfaceInput) (ServiceInterface, error) {
	normalized, err := normalizeServiceInterface(input)
	if err != nil {
		return ServiceInterface{}, err
	}
	if service.routeValidator == nil {
		return ServiceInterface{}, ErrAccessRouteValidatorUnavailable
	}
	if err := service.routeValidator.ValidateServiceInterface(ctx, normalized.HTTPMethod, normalized.RouteTemplate); err != nil {
		return ServiceInterface{}, err
	}
	return service.repository.CreateServiceInterface(ctx, normalized)
}

// CreateServiceInterfaces validates the complete browser selection before the
// repository transaction starts, so bulk registration never leaves a partial
// route catalog behind.
func (service *AccessService) CreateServiceInterfaces(ctx context.Context, inputs []ServiceInterfaceInput) ([]ServiceInterface, error) {
	if len(inputs) == 0 {
		return nil, ErrAccessBulkEmpty
	}
	normalized := make([]ServiceInterfaceInput, 0, len(inputs))
	seenCodes := make(map[string]struct{}, len(inputs))
	seenRoutes := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		value, err := normalizeServiceInterface(input)
		if err != nil {
			return nil, err
		}
		if _, exists := seenCodes[value.Code]; exists {
			return nil, ErrAccessCodeConflict
		}
		key := value.HTTPMethod + "\x00" + value.RouteTemplate
		if _, exists := seenRoutes[key]; exists {
			return nil, ErrAccessRouteConflict
		}
		if service.routeValidator == nil {
			return nil, ErrAccessRouteValidatorUnavailable
		}
		if err := service.routeValidator.ValidateServiceInterface(ctx, value.HTTPMethod, value.RouteTemplate); err != nil {
			return nil, err
		}
		seenCodes[value.Code] = struct{}{}
		seenRoutes[key] = struct{}{}
		normalized = append(normalized, value)
	}
	return service.repository.CreateServiceInterfaces(ctx, normalized)
}

func (service *AccessService) UpdateServiceInterface(ctx context.Context, id uint, input ServiceInterfaceUpdate) (ServiceInterface, error) {
	if id == 0 {
		return ServiceInterface{}, ErrAccessServiceInterfaceNotFound
	}
	if input.Name != nil {
		value, err := normalizeAccessName(*input.Name)
		if err != nil {
			return ServiceInterface{}, err
		}
		input.Name = &value
	}
	if input.Description != nil {
		value, err := normalizeAccessDescription(*input.Description)
		if err != nil {
			return ServiceInterface{}, err
		}
		input.Description = &value
	}
	return service.repository.UpdateServiceInterface(ctx, id, input)
}

func (service *AccessService) DeleteServiceInterface(ctx context.Context, id uint) error {
	if id == 0 {
		return ErrAccessServiceInterfaceNotFound
	}
	return service.repository.DeleteServiceInterface(ctx, id)
}

func (service *AccessService) UserRoleIDs(ctx context.Context, userID uint) ([]uint, error) {
	if userID == 0 {
		return nil, ErrAccessUserNotFound
	}
	return service.repository.UserRoleIDs(ctx, userID)
}

func (service *AccessService) ReplaceUserRoles(ctx context.Context, userID uint, roleIDs []uint) error {
	if userID == 0 {
		return ErrAccessUserNotFound
	}
	ids, err := normalizedGrantIDs(roleIDs)
	if err != nil {
		return err
	}
	return service.repository.ReplaceUserRoles(ctx, userID, ids)
}

func (service *AccessService) RoleServiceInterfaceIDs(ctx context.Context, roleID uint) ([]uint, error) {
	if roleID == 0 {
		return nil, ErrAccessRoleNotFound
	}
	return service.repository.RoleServiceInterfaceIDs(ctx, roleID)
}

func (service *AccessService) ReplaceRoleServiceInterfaces(ctx context.Context, roleID uint, interfaceIDs []uint) error {
	if roleID == 0 {
		return ErrAccessRoleNotFound
	}
	ids, err := normalizedGrantIDs(interfaceIDs)
	if err != nil {
		return err
	}
	return service.repository.ReplaceRoleServiceInterfaces(ctx, roleID, ids)
}

func (service *AccessService) GroupServiceInterfaceIDs(ctx context.Context, groupID uint) ([]uint, error) {
	if groupID == 0 {
		return nil, ErrUserGroupNotFound
	}
	return service.repository.GroupServiceInterfaceIDs(ctx, groupID)
}

func (service *AccessService) ReplaceGroupServiceInterfaces(ctx context.Context, actorUserID, groupID uint, interfaceIDs []uint) error {
	if actorUserID == 0 {
		return ErrPersonnelActorRequired
	}
	if groupID == 0 {
		return ErrUserGroupNotFound
	}
	ids, err := normalizedGrantIDs(interfaceIDs)
	if err != nil {
		return err
	}
	return service.repository.ReplaceGroupServiceInterfaces(ctx, actorUserID, groupID, ids)
}

func (service *AccessService) RoleActionCodes(ctx context.Context, roleID uint) ([]string, error) {
	if roleID == 0 {
		return nil, ErrAccessRoleNotFound
	}
	return service.repository.RoleActionCodes(ctx, roleID)
}

func (service *AccessService) ReplaceRoleActionCodes(ctx context.Context, actorUserID, roleID uint, actionCodes []string) error {
	if actorUserID == 0 {
		return ErrPersonnelActorRequired
	}
	if roleID == 0 {
		return ErrAccessRoleNotFound
	}
	codes, err := normalizedActionCodes(actionCodes)
	if err != nil {
		return err
	}
	return service.repository.ReplaceRoleActionCodes(ctx, actorUserID, roleID, codes)
}

func (service *AccessService) GroupActionCodes(ctx context.Context, groupID uint) ([]string, error) {
	if groupID == 0 {
		return nil, ErrUserGroupNotFound
	}
	return service.repository.GroupActionCodes(ctx, groupID)
}

func (service *AccessService) ReplaceGroupActionCodes(ctx context.Context, actorUserID, groupID uint, actionCodes []string) error {
	if actorUserID == 0 {
		return ErrPersonnelActorRequired
	}
	if groupID == 0 {
		return ErrUserGroupNotFound
	}
	codes, err := normalizedActionCodes(actionCodes)
	if err != nil {
		return err
	}
	return service.repository.ReplaceGroupActionCodes(ctx, actorUserID, groupID, codes)
}

func (service *AccessService) RoleMenuIDs(ctx context.Context, roleID uint) ([]uint, error) {
	if roleID == 0 {
		return nil, ErrAccessRoleNotFound
	}
	return service.repository.RoleMenuIDs(ctx, roleID)
}

func (service *AccessService) ReplaceRoleMenus(ctx context.Context, roleID uint, menuIDs []uint) error {
	if roleID == 0 {
		return ErrAccessRoleNotFound
	}
	ids, err := normalizedGrantIDs(menuIDs)
	if err != nil {
		return err
	}
	if service.menuValidator == nil {
		return ErrAccessMenuValidatorUnavailable
	}
	if err := service.menuValidator.ValidateEnabledMenuIDs(ctx, ids); err != nil {
		return err
	}
	return service.repository.ReplaceRoleMenus(ctx, roleID, ids)
}

// EnsureAdministratorMenus is composition-root-only bootstrap behavior. The
// public role mutation path remains unable to alter system-managed roles.
func (service *AccessService) EnsureAdministratorMenus(ctx context.Context, menuIDs []uint) error {
	ids, err := normalizedGrantIDs(menuIDs)
	if err != nil {
		return err
	}
	if service.menuValidator == nil {
		return ErrAccessMenuValidatorUnavailable
	}
	if err := service.menuValidator.ValidateEnabledMenuIDs(ctx, ids); err != nil {
		return err
	}
	return service.repository.EnsureAdministratorMenus(ctx, ids)
}

func (service *AccessService) GroupMenuIDs(ctx context.Context, groupID uint) ([]uint, error) {
	if groupID == 0 {
		return nil, ErrUserGroupNotFound
	}
	return service.repository.GroupMenuIDs(ctx, groupID)
}

func (service *AccessService) ReplaceGroupMenus(ctx context.Context, actorUserID, groupID uint, menuIDs []uint) error {
	if actorUserID == 0 {
		return ErrPersonnelActorRequired
	}
	if groupID == 0 {
		return ErrUserGroupNotFound
	}
	ids, err := normalizedGrantIDs(menuIDs)
	if err != nil {
		return err
	}
	if service.menuValidator == nil {
		return ErrAccessMenuValidatorUnavailable
	}
	if err := service.menuValidator.ValidateEnabledMenuIDs(ctx, ids); err != nil {
		return err
	}
	return service.repository.ReplaceGroupMenus(ctx, actorUserID, groupID, ids)
}

// DirectMenuIDsForUser is the workbench-facing grant query. It returns menu
// IDs from enabled assigned roles only; workbench owns menu visibility/tree rules.
func (service *AccessService) DirectMenuIDsForUser(ctx context.Context, userID uint) ([]uint, error) {
	if userID == 0 {
		return nil, ErrAccessUserNotFound
	}
	return service.repository.DirectMenuIDsForUser(ctx, userID)
}

func (service *AccessService) CountMenuGrants(ctx context.Context, menuID uint) (int64, error) {
	if menuID == 0 {
		return 0, nil
	}
	return service.repository.CountMenuGrants(ctx, menuID)
}

func (service *AccessService) EffectiveActionCodes(ctx context.Context, userID uint) ([]string, error) {
	if userID == 0 {
		return nil, ErrAccessUserNotFound
	}
	return service.repository.EffectiveActionCodes(ctx, userID)
}

func (service *AccessService) Authorize(ctx context.Context, userID uint, method, routeTemplate string) (AuthorizationResult, error) {
	method = strings.ToUpper(strings.TrimSpace(method))
	routeTemplate = strings.TrimSpace(routeTemplate)
	if userID == 0 || !validHTTPMethod(method) || routeTemplate == "" {
		return AuthorizationForbidden, nil
	}
	result, err := service.repository.Authorize(ctx, userID, method, routeTemplate)
	if err != nil || result != AuthorizationForbidden {
		return result, err
	}
	requested, found := accesscatalog.ActionForRoute(method, routeTemplate)
	if !found {
		return result, nil
	}
	granting := accesscatalog.GrantingActions(requested)
	if len(granting) <= 1 {
		return result, nil
	}
	codes, err := service.repository.EffectiveActionCodes(ctx, userID)
	if err != nil {
		return AuthorizationForbidden, err
	}
	effective := make(map[string]struct{}, len(codes))
	for _, code := range codes {
		effective[code] = struct{}{}
	}
	for _, action := range granting[1:] {
		if _, allowed := effective[string(action)]; allowed {
			return AuthorizationAllowed, nil
		}
	}
	return result, nil
}

// Bootstrap creates the system-managed access catalog and, when configured,
// binds the specified PM user to its administration role. It is idempotent so
// every API process may validate the state safely at startup.
func (service *AccessService) Bootstrap(ctx context.Context, spec BootstrapSpec) error {
	if len(spec.ServiceInterfaces) == 0 {
		return ErrAccessRouteNotRegistered
	}
	seenCodes := make(map[string]struct{}, len(spec.ServiceInterfaces))
	seenRoutes := make(map[string]struct{}, len(spec.ServiceInterfaces))
	for index, input := range spec.ServiceInterfaces {
		normalized, err := normalizeServiceInterface(input)
		if err != nil {
			return err
		}
		if _, exists := seenCodes[normalized.Code]; exists {
			return ErrAccessCodeConflict
		}
		key := normalized.HTTPMethod + "\x00" + normalized.RouteTemplate
		if _, exists := seenRoutes[key]; exists {
			return ErrAccessRouteConflict
		}
		seenCodes[normalized.Code] = struct{}{}
		seenRoutes[key] = struct{}{}
		spec.ServiceInterfaces[index] = normalized
		if service.routeValidator != nil {
			if err := service.routeValidator.ValidateServiceInterface(ctx, normalized.HTTPMethod, normalized.RouteTemplate); err != nil {
				return err
			}
		}
	}
	return service.repository.Bootstrap(ctx, spec)
}
