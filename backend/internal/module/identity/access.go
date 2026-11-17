package identity

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	accessCodeMaxLength        = 96
	accessNameMaxLength        = 128
	accessDescriptionMaxLength = 1000
	accessRouteMaxLength       = 512
)

var (
	ErrAccessRoleNotFound                = errors.New("access role not found")
	ErrAccessUserNotFound                = errors.New("access user not found")
	ErrAccessServiceInterfaceNotFound    = errors.New("access service interface not found")
	ErrAccessRoleCodeInvalid             = errors.New("access role code is invalid")
	ErrAccessRoleNameConflict            = errors.New("access role name already exists")
	ErrAccessServiceInterfaceCodeInvalid = errors.New("access service interface code is invalid")
	ErrAccessActionCodeInvalid           = errors.New("access action code is invalid")
	ErrAccessActionNotFound              = errors.New("access action code is not registered")
	ErrAccessNameInvalid                 = errors.New("access resource name is invalid")
	ErrAccessDescriptionInvalid          = errors.New("access resource description is invalid")
	ErrAccessRouteInvalid                = errors.New("access service interface route is invalid")
	ErrAccessMethodInvalid               = errors.New("access service interface method is invalid")
	ErrAccessDuplicateID                 = errors.New("access grant IDs contain a duplicate or invalid value")
	ErrAccessResourceInUse               = errors.New("access resource is still in use")
	ErrAccessSystemManaged               = errors.New("access system resource cannot be changed this way")
	ErrAccessMenuValidatorUnavailable    = errors.New("access menu validator is unavailable")
	ErrAccessRouteValidatorUnavailable   = errors.New("access route validator is unavailable")
	ErrAccessRouteNotRegistered          = errors.New("access service interface route is not registered")
	ErrAccessBootstrapRequired           = errors.New("access bootstrap administrator is required")
	ErrAccessCodeConflict                = errors.New("access resource code already exists")
	ErrAccessRouteConflict               = errors.New("access service interface route already exists")
	ErrAccessBulkEmpty                   = errors.New("access service interface batch is empty")
)

var accessCodePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]*$`)

// Role is a global PM role. It never carries a workspace, team, ownership, or
// resource-scope dimension.
type Role struct {
	ID            uint      `json:"id"`
	Code          string    `json:"code"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	Enabled       bool      `json:"enabled"`
	SystemManaged bool      `json:"system_managed"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type RoleInput struct {
	Code        string
	Name        string
	Description string
	Enabled     bool
}

type RoleUpdate struct {
	Name        *string
	Description *string
	Enabled     *bool
}

// UserSummary is the minimum PM-owned identity data required to bind global
// roles. It deliberately excludes passwords, provider credentials, legacy
// role fields, and any workspace relation.
type UserSummary struct {
	ID          uint   `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email,omitempty"`
	Active      bool   `json:"active"`
}

// ServiceInterface is the registered server-side permission key. The method
// and canonical Gin route template are immutable after registration.
type ServiceInterface struct {
	ID            uint      `json:"id"`
	Code          string    `json:"code"`
	ActionCode    string    `json:"action_code"`
	HTTPMethod    string    `json:"http_method"`
	RouteTemplate string    `json:"route_template"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	Enabled       bool      `json:"enabled"`
	SystemManaged bool      `json:"system_managed"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ServiceInterfaceInput struct {
	Code          string
	ActionCode    string
	HTTPMethod    string
	RouteTemplate string
	Name          string
	Description   string
	Enabled       bool
}

type ServiceInterfaceUpdate struct {
	Name        *string
	Description *string
	Enabled     *bool
}

const (
	AccessAdministratorRoleCode  = "access-administrator"
	LegacySuperAdminRole         = "super_admin"
	AccessManagementActionPrefix = "pm.access-management."
)

// BootstrapSpec creates the small system-managed access-administration
// catalog. Its user ID is a PM users.id, never an upstream principal or a
// workspace-scoped identity.
type BootstrapSpec struct {
	AdministratorUserID             uint
	ServiceInterfaces               []ServiceInterfaceInput
	DefaultGroupActionCodes         []string
	DefaultGroupExcludedActionCodes []string
}

type AuthorizationResult uint8

const (
	AuthorizationAllowed AuthorizationResult = iota
	AuthorizationInterfaceUnregistered
	AuthorizationForbidden
)

func normalizeAccessCode(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if len(value) == 0 || len(value) > accessCodeMaxLength || !accessCodePattern.MatchString(value) {
		return "", ErrAccessRoleCodeInvalid
	}
	return value, nil
}

func normalizeAccessName(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" || len([]rune(value)) > accessNameMaxLength {
		return "", ErrAccessNameInvalid
	}
	return value, nil
}

func normalizeRoleName(raw string) (string, error) {
	return normalizeAccessName(raw)
}

func newRoleCode() (string, error) {
	var entropy [32]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", err
	}
	digest := sha256.Sum256(entropy[:])
	return hex.EncodeToString(digest[:]), nil
}

func normalizeAccessDescription(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if len([]rune(value)) > accessDescriptionMaxLength {
		return "", ErrAccessDescriptionInvalid
	}
	return value, nil
}

func normalizeServiceInterface(input ServiceInterfaceInput) (ServiceInterfaceInput, error) {
	code, err := normalizeAccessCode(input.Code)
	if err != nil {
		return ServiceInterfaceInput{}, ErrAccessServiceInterfaceCodeInvalid
	}
	actionCode := strings.TrimSpace(input.ActionCode)
	if actionCode == "" {
		actionCode = code
	}
	if len(actionCode) > accessCodeMaxLength || !accessCodePattern.MatchString(actionCode) {
		return ServiceInterfaceInput{}, ErrAccessActionCodeInvalid
	}
	method := strings.ToUpper(strings.TrimSpace(input.HTTPMethod))
	if !validHTTPMethod(method) {
		return ServiceInterfaceInput{}, ErrAccessMethodInvalid
	}
	route := strings.TrimSpace(input.RouteTemplate)
	if route == "" || len(route) > accessRouteMaxLength || !strings.HasPrefix(route, "/") ||
		strings.HasPrefix(route, "//") || strings.ContainsAny(route, "?#") {
		return ServiceInterfaceInput{}, ErrAccessRouteInvalid
	}
	name, err := normalizeAccessName(input.Name)
	if err != nil {
		return ServiceInterfaceInput{}, err
	}
	description, err := normalizeAccessDescription(input.Description)
	if err != nil {
		return ServiceInterfaceInput{}, err
	}
	return ServiceInterfaceInput{
		Code: code, ActionCode: actionCode, HTTPMethod: method, RouteTemplate: route, Name: name,
		Description: description, Enabled: input.Enabled,
	}, nil
}

func validHTTPMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead:
		return true
	default:
		return false
	}
}

func normalizedGrantIDs(ids []uint) ([]uint, error) {
	result := make([]uint, len(ids))
	seen := make(map[uint]struct{}, len(ids))
	for index, id := range ids {
		if id == 0 {
			return nil, ErrAccessDuplicateID
		}
		if _, exists := seen[id]; exists {
			return nil, ErrAccessDuplicateID
		}
		seen[id] = struct{}{}
		result[index] = id
	}
	return result, nil
}

func normalizedActionCodes(codes []string) ([]string, error) {
	result := make([]string, 0, len(codes))
	seen := make(map[string]struct{}, len(codes))
	for _, raw := range codes {
		code := strings.TrimSpace(raw)
		if code == "" || len(code) > accessCodeMaxLength || !accessCodePattern.MatchString(code) {
			return nil, ErrAccessActionCodeInvalid
		}
		if _, exists := seen[code]; exists {
			return nil, ErrAccessActionCodeInvalid
		}
		seen[code] = struct{}{}
		result = append(result, code)
	}
	return result, nil
}
