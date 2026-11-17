package identity

import (
	"errors"
	"strings"
	"time"
)

const (
	DefaultUserGroupCode = "default-users"

	groupCodeMaxLength        = 96
	groupNameMaxLength        = 128
	groupDescriptionMaxLength = 1000
)

type AccountStatus string

const (
	AccountPendingApproval AccountStatus = "pending_approval"
	AccountActive          AccountStatus = "active"
	AccountRejected        AccountStatus = "rejected"
	AccountBanned          AccountStatus = "banned"
)

func (status AccountStatus) Valid() bool {
	switch status {
	case AccountPendingApproval, AccountActive, AccountRejected, AccountBanned:
		return true
	default:
		return false
	}
}

// EffectiveAccountStatus preserves pre-0036 rows during rolling deployment:
// an empty/unknown status falls back to the legacy active flag. New code should
// persist an explicit status and use this helper at authentication boundaries.
func EffectiveAccountStatus(raw string, legacyActive bool) AccountStatus {
	status := AccountStatus(strings.TrimSpace(raw))
	if status.Valid() {
		return status
	}
	if legacyActive {
		return AccountActive
	}
	return AccountBanned
}

func (status AccountStatus) AllowsBusinessAccess() bool {
	return status == AccountActive
}

var (
	ErrPersonnelUserNotFound       = errors.New("personnel user not found")
	ErrUserGroupNotFound           = errors.New("user group not found")
	ErrUserGroupCodeInvalid        = errors.New("user group code is invalid")
	ErrUserGroupNameInvalid        = errors.New("user group name is invalid")
	ErrUserGroupDescriptionInvalid = errors.New("user group description is invalid")
	ErrUserGroupCodeConflict       = errors.New("user group code already exists")
	ErrUserGroupInUse              = errors.New("user group is still assigned")
	ErrDefaultUserGroupProtected   = errors.New("default user group cannot be deleted or disabled")
	ErrInvalidAccountTransition    = errors.New("invalid account status transition")
	ErrAccountReasonRequired       = errors.New("account status reason is required")
	ErrLastAdministrator           = errors.New("last system administrator cannot lose recovery access")
	ErrPersonnelActorRequired      = errors.New("personnel mutation actor is required")
	ErrPersonnelQueryInvalid       = errors.New("personnel query is invalid")
)

type UserGroup struct {
	ID            uint      `json:"id"`
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	Enabled       bool      `json:"enabled"`
	SystemManaged bool      `json:"system_managed"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type UserGroupInput struct {
	Code        string
	Name        string
	Description string
	Enabled     bool
}

type UserGroupUpdate struct {
	Name        *string
	Description *string
	Enabled     *bool
}

type PersonnelUser struct {
	ID             uint          `json:"id"`
	Username       string        `json:"username"`
	DisplayName    string        `json:"display_name"`
	Email          string        `json:"email,omitempty"`
	AvatarURL      string        `json:"avatar_url,omitempty"`
	Status         AccountStatus `json:"status"`
	StatusReason   string        `json:"status_reason,omitempty"`
	SessionVersion uint64        `json:"session_version"`
	GroupID        uint          `json:"group_id"`
	RoleIDs        []uint        `json:"role_ids"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

type PersonnelUserFilter struct {
	Status   AccountStatus
	Query    string
	GroupID  uint
	RoleID   uint
	Page     int
	PageSize int
}

type PersonnelUserPage struct {
	Items    []PersonnelUser `json:"items"`
	Page     int             `json:"page"`
	PageSize int             `json:"page_size"`
	Total    int64           `json:"total"`
	HasMore  bool            `json:"has_more"`
}

type PersonnelAuditEvent struct {
	ID             uint      `json:"id"`
	EventType      string    `json:"event_type"`
	ActorUserID    uint      `json:"actor_user_id"`
	SubjectUserID  *uint     `json:"subject_user_id,omitempty"`
	SubjectGroupID *uint     `json:"subject_group_id,omitempty"`
	SubjectRoleID  *uint     `json:"subject_role_id,omitempty"`
	BeforeJSON     string    `json:"before_json,omitempty"`
	AfterJSON      string    `json:"after_json,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

type PersonnelAuditFilter struct {
	SubjectUserID  uint
	SubjectGroupID uint
	SubjectRoleID  uint
	Limit          int
}

// PermissionSubjectRef is the typed bridge consumed by the catalog/grant
// layer. IDs are database identities; permission action codes remain owned by
// the canonical permission catalog and never appear in personnel code.
type PermissionSubjectKind uint8

const (
	PermissionSubjectRole PermissionSubjectKind = iota + 1
	PermissionSubjectGroup
)

type PermissionSubjectRef struct {
	Kind PermissionSubjectKind
	ID   uint
}

func normalizeUserGroupInput(input UserGroupInput) (UserGroupInput, error) {
	code := strings.TrimSpace(input.Code)
	if len(code) == 0 || len(code) > groupCodeMaxLength || !accessCodePattern.MatchString(code) {
		return UserGroupInput{}, ErrUserGroupCodeInvalid
	}
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > groupNameMaxLength {
		return UserGroupInput{}, ErrUserGroupNameInvalid
	}
	description := strings.TrimSpace(input.Description)
	if len([]rune(description)) > groupDescriptionMaxLength {
		return UserGroupInput{}, ErrUserGroupDescriptionInvalid
	}
	return UserGroupInput{Code: code, Name: name, Description: description, Enabled: input.Enabled}, nil
}

func ValidAccountTransition(from, to AccountStatus) bool {
	switch from {
	case AccountPendingApproval:
		return to == AccountActive || to == AccountRejected
	case AccountActive:
		return to == AccountBanned
	case AccountRejected:
		return to == AccountPendingApproval
	case AccountBanned:
		return to == AccountActive
	default:
		return false
	}
}
