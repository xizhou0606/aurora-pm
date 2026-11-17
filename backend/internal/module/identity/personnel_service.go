package identity

import (
	"context"
	"strings"
)

type PersonnelService struct {
	repository PersonnelRepository
}

func NewPersonnelService(repository PersonnelRepository) *PersonnelService {
	return &PersonnelService{repository: repository}
}

func (service *PersonnelService) ListUsers(ctx context.Context, filter PersonnelUserFilter) (PersonnelUserPage, error) {
	filter.Query = strings.TrimSpace(filter.Query)
	if len([]rune(filter.Query)) > 200 || (filter.Status != "" && !filter.Status.Valid()) {
		return PersonnelUserPage{}, ErrPersonnelQueryInvalid
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	return service.repository.ListPersonnelUsers(ctx, filter)
}

func (service *PersonnelService) User(ctx context.Context, userID uint) (PersonnelUser, error) {
	if userID == 0 {
		return PersonnelUser{}, ErrPersonnelUserNotFound
	}
	return service.repository.FindPersonnelUser(ctx, userID)
}

func (service *PersonnelService) ListGroups(ctx context.Context, includeDisabled bool) ([]UserGroup, error) {
	return service.repository.ListUserGroups(ctx, includeDisabled)
}

func (service *PersonnelService) CreateGroup(ctx context.Context, actorUserID uint, input UserGroupInput) (UserGroup, error) {
	if actorUserID == 0 {
		return UserGroup{}, ErrPersonnelActorRequired
	}
	normalized, err := normalizeUserGroupInput(input)
	if err != nil {
		return UserGroup{}, err
	}
	if normalized.Code == DefaultUserGroupCode {
		return UserGroup{}, ErrDefaultUserGroupProtected
	}
	return service.repository.CreateUserGroup(ctx, actorUserID, normalized)
}

func (service *PersonnelService) UpdateGroup(ctx context.Context, actorUserID, groupID uint, input UserGroupUpdate) (UserGroup, error) {
	if actorUserID == 0 {
		return UserGroup{}, ErrPersonnelActorRequired
	}
	if groupID == 0 {
		return UserGroup{}, ErrUserGroupNotFound
	}
	if input.Name != nil {
		value := strings.TrimSpace(*input.Name)
		if value == "" || len([]rune(value)) > groupNameMaxLength {
			return UserGroup{}, ErrUserGroupNameInvalid
		}
		input.Name = &value
	}
	if input.Description != nil {
		value := strings.TrimSpace(*input.Description)
		if len([]rune(value)) > groupDescriptionMaxLength {
			return UserGroup{}, ErrUserGroupDescriptionInvalid
		}
		input.Description = &value
	}
	return service.repository.UpdateUserGroup(ctx, actorUserID, groupID, input)
}

func (service *PersonnelService) DeleteGroup(ctx context.Context, actorUserID, groupID uint) error {
	if actorUserID == 0 {
		return ErrPersonnelActorRequired
	}
	if groupID == 0 {
		return ErrUserGroupNotFound
	}
	return service.repository.DeleteUserGroup(ctx, actorUserID, groupID)
}

func (service *PersonnelService) ReplaceUserGroup(ctx context.Context, actorUserID, userID, groupID uint) error {
	if actorUserID == 0 {
		return ErrPersonnelActorRequired
	}
	if userID == 0 {
		return ErrPersonnelUserNotFound
	}
	if groupID == 0 {
		return ErrUserGroupNotFound
	}
	return service.repository.ReplacePersonnelUserGroup(ctx, actorUserID, userID, groupID)
}

func (service *PersonnelService) ReplaceUserRoles(ctx context.Context, actorUserID, userID uint, roleIDs []uint) error {
	if actorUserID == 0 {
		return ErrPersonnelActorRequired
	}
	if userID == 0 {
		return ErrPersonnelUserNotFound
	}
	ids, err := normalizedGrantIDs(roleIDs)
	if err != nil {
		return err
	}
	return service.repository.ReplacePersonnelUserRoles(ctx, actorUserID, userID, ids)
}

func (service *PersonnelService) ReplaceUserAccess(ctx context.Context, actorUserID, userID, groupID uint, roleIDs []uint) error {
	if actorUserID == 0 {
		return ErrPersonnelActorRequired
	}
	if userID == 0 {
		return ErrPersonnelUserNotFound
	}
	if groupID == 0 {
		return ErrUserGroupNotFound
	}
	ids, err := normalizedGrantIDs(roleIDs)
	if err != nil {
		return err
	}
	return service.repository.ReplacePersonnelUserAccess(ctx, actorUserID, userID, groupID, ids)
}

func (service *PersonnelService) TransitionUserStatus(ctx context.Context, actorUserID, userID uint, status AccountStatus, reason string) (PersonnelUser, error) {
	if actorUserID == 0 {
		return PersonnelUser{}, ErrPersonnelActorRequired
	}
	if userID == 0 {
		return PersonnelUser{}, ErrPersonnelUserNotFound
	}
	if !status.Valid() {
		return PersonnelUser{}, ErrInvalidAccountTransition
	}
	reason = strings.TrimSpace(reason)
	if (status == AccountRejected || status == AccountBanned) && reason == "" {
		return PersonnelUser{}, ErrAccountReasonRequired
	}
	return service.repository.TransitionPersonnelUserStatus(ctx, actorUserID, userID, status, reason)
}

func (service *PersonnelService) AuditEvents(ctx context.Context, filter PersonnelAuditFilter) ([]PersonnelAuditEvent, error) {
	if filter.Limit <= 0 || filter.Limit > 500 {
		filter.Limit = 100
	}
	return service.repository.ListPersonnelAuditEvents(ctx, filter)
}

func RolePermissionSubject(roleID uint) PermissionSubjectRef {
	return PermissionSubjectRef{Kind: PermissionSubjectRole, ID: roleID}
}

func GroupPermissionSubject(groupID uint) PermissionSubjectRef {
	return PermissionSubjectRef{Kind: PermissionSubjectGroup, ID: groupID}
}
