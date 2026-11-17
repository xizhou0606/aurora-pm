package requirement

import (
	"errors"
	"strings"
	"time"
)

const (
	MaterialKindDesign   = "design"
	MaterialKindTestCase = "test_case"

	MaterialSourceRequirement    = "requirement"
	MaterialSourceWorkflow       = "workflow"
	MaterialSourceTestingLibrary = "testing_library"
	MaterialSourceAIGenerated    = "ai_generated"
)

var (
	ErrMaterialKindInvalid   = errors.New("requirement material kind is invalid")
	ErrMaterialSourceInvalid = errors.New("requirement material source is invalid")
	ErrMaterialNameRequired  = errors.New("requirement material name is required")
	ErrMaterialNotFound      = errors.New("requirement material not found")
	ErrRequirementNotFound   = errors.New("requirement not found")
)

type Material struct {
	ID             uint
	WorkspaceID    uint
	RequirementID  uint
	Kind           string
	Source         string
	Name           string
	ContentType    string
	Size           int64
	ObjectKey      string
	URL            string
	AIRunID        *uint
	UploaderUserID uint
	CreatedAt      time.Time
}

type MaterialInput struct {
	WorkspaceID    uint
	RequirementID  uint
	Kind           string
	Source         string
	Name           string
	ContentType    string
	Size           int64
	ObjectKey      string
	URL            string
	AIRunID        *uint
	UploaderUserID uint
}

type LibraryMaterial struct {
	Material
	RequirementTitle string
	ProjectSetID     uint
	PlanUnitID       uint
	ProjectName      string
	PlanUnitTitle    string
}

type LibraryFilter struct {
	Query      string
	ProjectID  uint
	PlanUnitID uint
	// AccessScope restricts the library listing to the caller's accessible
	// projects in SQL. It is a raw SQL fragment + args produced by the project
	// access service (BuildMemberFilter against requirements.project_new_id);
	// an empty SQL means the admin fast path (no restriction). The field is
	// transport-owned: the repository only applies it, never builds it.
	AccessScope LibraryAccessScope
}

// LibraryAccessScope is the SQL-level project access restriction injected by
// the HTTP adapter. Unset (zero) means NO restriction was provided; callers
// that require enforcement must supply a scope explicitly.
type LibraryAccessScope struct {
	SQL  string
	Args []any
}

type MaterialRepository interface {
	Create(MaterialInput) (Material, error)
	List(workspaceID, requirementID uint, kind string) ([]Material, error)
	Get(workspaceID, requirementID, materialID uint) (Material, error)
	Delete(workspaceID, requirementID, materialID uint) error
	Count(workspaceID, requirementID uint, kind string) (int64, error)
	ListTestCases(workspaceID uint, filter LibraryFilter) ([]LibraryMaterial, error)
	ListForRequirements(workspaceID uint, requirementIDs []uint, kind string) ([]Material, error)
	GetByObjectKey(workspaceID uint, objectKey string) (Material, error)
}

type MaterialService struct {
	repository MaterialRepository
}

func NewMaterialService(repository MaterialRepository) *MaterialService {
	return &MaterialService{repository: repository}
}

func ParseMaterialKind(raw string) (string, error) {
	kind := strings.TrimSpace(raw)
	switch kind {
	case MaterialKindDesign, MaterialKindTestCase:
		return kind, nil
	default:
		return "", ErrMaterialKindInvalid
	}
}

func ParseMaterialSource(raw string) (string, error) {
	source := strings.TrimSpace(raw)
	switch source {
	case MaterialSourceRequirement, MaterialSourceWorkflow, MaterialSourceTestingLibrary, MaterialSourceAIGenerated:
		return source, nil
	default:
		return "", ErrMaterialSourceInvalid
	}
}

func (service *MaterialService) Create(input MaterialInput) (Material, error) {
	kind, err := ParseMaterialKind(input.Kind)
	if err != nil {
		return Material{}, err
	}
	source, err := ParseMaterialSource(input.Source)
	if err != nil {
		return Material{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Material{}, ErrMaterialNameRequired
	}
	input.Kind = kind
	input.Source = source
	input.Name = name
	input.ContentType = strings.TrimSpace(input.ContentType)
	input.ObjectKey = strings.TrimSpace(input.ObjectKey)
	input.URL = strings.TrimSpace(input.URL)
	return service.repository.Create(input)
}

func (service *MaterialService) List(workspaceID, requirementID uint, kind string) ([]Material, error) {
	if kind != "" {
		if _, err := ParseMaterialKind(kind); err != nil {
			return nil, err
		}
	}
	return service.repository.List(workspaceID, requirementID, kind)
}

func (service *MaterialService) Get(workspaceID, requirementID, materialID uint) (Material, error) {
	return service.repository.Get(workspaceID, requirementID, materialID)
}

func (service *MaterialService) Delete(workspaceID, requirementID, materialID uint) error {
	return service.repository.Delete(workspaceID, requirementID, materialID)
}

func (service *MaterialService) HasKind(workspaceID, requirementID uint, kind string) (bool, error) {
	if _, err := ParseMaterialKind(kind); err != nil {
		return false, err
	}
	count, err := service.repository.Count(workspaceID, requirementID, kind)
	return count > 0, err
}

func (service *MaterialService) ListTestCases(workspaceID uint, filter LibraryFilter) ([]LibraryMaterial, error) {
	return service.repository.ListTestCases(workspaceID, filter)
}

func (service *MaterialService) ListForRequirements(workspaceID uint, requirementIDs []uint, kind string) ([]Material, error) {
	if kind != "" {
		if _, err := ParseMaterialKind(kind); err != nil {
			return nil, err
		}
	}
	return service.repository.ListForRequirements(workspaceID, requirementIDs, kind)
}
