package quality

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

const (
	TestReportKindFile = "file"
	TestReportKindLink = "link"
)

var (
	ErrTestReportKindInvalid      = errors.New("plan unit test report kind is invalid")
	ErrTestReportNameRequired     = errors.New("plan unit test report name is required")
	ErrTestReportPlanUnitRequired = errors.New("plan unit id is required")
	ErrTestReportURLInvalid       = errors.New("plan unit test report url is invalid")
	ErrTestReportNotFound         = errors.New("plan unit test report not found")
	ErrPlanUnitNotFound           = errors.New("plan unit not found")
)

type TestReport struct {
	ID             uint
	WorkspaceID    uint
	PlanUnitID     uint
	ProjectSetID   uint
	Kind           string
	Name           string
	ContentType    string
	Size           int64
	ObjectKey      string
	URL            string
	UploaderUserID uint
	CreatedAt      time.Time
	PlanUnitTitle  string
	ProjectName    string
}

type TestReportInput struct {
	WorkspaceID    uint
	PlanUnitID     uint
	ProjectSetID   uint
	Kind           string
	Name           string
	ContentType    string
	Size           int64
	ObjectKey      string
	URL            string
	UploaderUserID uint
}

type TestReportFilter struct {
	Query      string
	ProjectID  uint
	PlanUnitID uint
	// AccessScope restricts the report listing to the caller's accessible
	// projects in SQL. It is a raw SQL fragment + args produced by the project
	// access service (BuildMemberFilter against reports.project_set_id); an
	// empty SQL means the admin fast path (no restriction). The repository
	// only applies it, never builds it.
	AccessScope TestReportAccessScope
}

// TestReportAccessScope is the SQL-level project access restriction injected
// by the HTTP adapter. Unset (zero) means NO restriction was provided.
type TestReportAccessScope struct {
	SQL  string
	Args []any
}

type TestReportRepository interface {
	Create(TestReportInput) (TestReport, error)
	List(workspaceID uint, filter TestReportFilter) ([]TestReport, error)
	Get(workspaceID, reportID uint) (TestReport, error)
}

type TestReportService struct {
	repository TestReportRepository
}

func NewTestReportService(repository TestReportRepository) *TestReportService {
	return &TestReportService{repository: repository}
}

func ParseTestReportKind(raw string) (string, error) {
	kind := strings.TrimSpace(raw)
	switch kind {
	case TestReportKindFile, TestReportKindLink:
		return kind, nil
	default:
		return "", ErrTestReportKindInvalid
	}
}

func ValidReportURL(raw string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
	return err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https")
}

func (service *TestReportService) Create(input TestReportInput) (TestReport, error) {
	if input.PlanUnitID == 0 {
		return TestReport{}, ErrTestReportPlanUnitRequired
	}
	kind, err := ParseTestReportKind(input.Kind)
	if err != nil {
		return TestReport{}, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return TestReport{}, ErrTestReportNameRequired
	}
	input.Kind = kind
	input.Name = name
	input.ContentType = strings.TrimSpace(input.ContentType)
	input.ObjectKey = strings.TrimSpace(input.ObjectKey)
	input.URL = strings.TrimSpace(input.URL)
	if kind == TestReportKindLink && !ValidReportURL(input.URL) {
		return TestReport{}, ErrTestReportURLInvalid
	}
	return service.repository.Create(input)
}

func (service *TestReportService) List(workspaceID uint, filter TestReportFilter) ([]TestReport, error) {
	return service.repository.List(workspaceID, filter)
}

func (service *TestReportService) Get(workspaceID, reportID uint) (TestReport, error) {
	return service.repository.Get(workspaceID, reportID)
}
