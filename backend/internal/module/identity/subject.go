package identity

import (
	"context"
	"errors"
)

const (
	SubjectUser             SubjectKind = "user"
	SubjectRole             SubjectKind = "role"
	SubjectUserGroup        SubjectKind = "user_group"
	SubjectOrganizationUnit SubjectKind = "organization_unit"
	SubjectProjectSet       SubjectKind = "project_set"
)

const (
	DefaultSubjectPageSize = 20
	MaximumSubjectPageSize = 100
)

var (
	ErrSubjectInvalid         = errors.New("resource access subject is invalid")
	ErrSubjectKindUnavailable = errors.New("resource access subject kind is unavailable")
)

// SubjectKind is the canonical resource-sharing subject discriminator.
// Authorization keys must never be inferred from display names or external
// identity-provider identifiers.
type SubjectKind string

func (kind SubjectKind) Valid() bool {
	switch kind {
	case SubjectUser, SubjectRole, SubjectUserGroup, SubjectOrganizationUnit, SubjectProjectSet:
		return true
	default:
		return false
	}
}

// SubjectRef is the stable policy-facing reference. ID is intentionally
// opaque to consumers. PM uint primary keys are encoded as decimal strings by
// their owning module, but callers must not reinterpret them as upstream IDs.
type SubjectRef struct {
	Kind               SubjectKind `json:"kind"`
	ID                 string      `json:"id"`
	IncludeDescendants bool        `json:"include_descendants,omitempty"`
}

// SubjectCandidate is the minimum projection exposed to a sharing picker.
// It deliberately excludes email, external IDs, membership lists and grants.
type SubjectCandidate struct {
	Kind          SubjectKind `json:"kind"`
	ID            string      `json:"id"`
	DisplayName   string      `json:"display_name"`
	SecondaryText string      `json:"secondary_text,omitempty"`
}

type SubjectSearch struct {
	Kind     SubjectKind
	Query    string
	Page     int
	PageSize int
}

type SubjectPage struct {
	Items    []SubjectCandidate `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Total    int64              `json:"total"`
	HasMore  bool               `json:"has_more"`
}

type SubjectMatchRequest struct {
	WorkspaceID uint
	UserID      uint
	Subjects    []SubjectRef
}

type SubjectMatch struct {
	Subject SubjectRef `json:"subject"`
	Matched bool       `json:"matched"`
}

type SubjectMatchResult struct {
	Matches []SubjectMatch `json:"matches"`
}

type SubjectKindCapability struct {
	Available bool `json:"available"`
}

// SubjectDirectory is the only cross-module subject contract resource access
// should consume. ResolveSubjects validates policy references and returns the
// same minimum projection used by SearchSubjects.
type SubjectDirectory interface {
	SearchSubjects(context.Context, uint, SubjectSearch) (SubjectPage, error)
	ResolveSubjects(context.Context, uint, []SubjectRef) ([]SubjectCandidate, error)
	MatchSubjects(context.Context, SubjectMatchRequest) (SubjectMatchResult, error)
}

// SubjectCapabilityProvider is optional so narrow test and downstream
// adapters can implement SubjectDirectory without inventing availability.
// Consumers must treat a missing provider or missing kind as unavailable.
type SubjectCapabilityProvider interface {
	SubjectCapabilities() map[SubjectKind]SubjectKindCapability
}

// OrganizationUnitProvider is the merge seam for the separately owned
// organization hierarchy. A nil provider is supported deliberately and must
// produce ErrSubjectKindUnavailable rather than an empty result.
type OrganizationUnitProvider interface {
	SearchOrganizationUnits(context.Context, uint, SubjectSearch) (SubjectPage, error)
	ResolveOrganizationUnits(context.Context, uint, []SubjectRef) ([]SubjectCandidate, error)
	MatchOrganizationUnits(context.Context, SubjectMatchRequest) (SubjectMatchResult, error)
}

// NativeSubjectRecord is the identity store's private-key projection. The
// public service converts ID to an opaque string before exposing it.
type NativeSubjectRecord struct {
	ID            uint
	DisplayName   string
	SecondaryText string
}

type NativeSubjectPage struct {
	Items    []NativeSubjectRecord
	Page     int
	PageSize int
	Total    int64
}

// NativeSubjectStore owns user, role and user-group facts. Project sets and
// organization units enter through their owning modules' public ports.
type NativeSubjectStore interface {
	ResolveSubjectFactUserID(context.Context, uint, uint) (uint, error)
	HasActiveWorkspaceUser(context.Context, uint, uint) (bool, error)
	SearchNativeSubjects(context.Context, uint, SubjectKind, string, int, int) (NativeSubjectPage, error)
	ResolveNativeSubjects(context.Context, uint, SubjectKind, []uint) ([]NativeSubjectRecord, error)
	MatchNativeSubjects(context.Context, uint, uint, SubjectKind, []uint) (map[uint]bool, error)
}
