package quality

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type SceneOptionKind string

const (
	SceneOptionEnvironment     SceneOptionKind = "environment"
	SceneOptionOperatingSystem SceneOptionKind = "operating_system"
	SceneOptionBrowser         SceneOptionKind = "browser"

	DefaultEnvironmentName  = "test"
	SceneOptionNameMaxRunes = 64
)

var (
	ErrSceneOptionKindInvalid   = errors.New("quality scene option kind is invalid")
	ErrSceneOptionNameInvalid   = errors.New("quality scene option name is invalid")
	ErrSceneOptionNameDuplicate = errors.New("quality scene option name already exists")
	ErrSceneOptionNotFound      = errors.New("quality scene option not found")
)

type SceneOptionInUseError struct {
	UsageCount int64
}

func (err *SceneOptionInUseError) Error() string {
	return fmt.Sprintf("quality scene option is used by %d bugs", err.UsageCount)
}

func ParseSceneOptionKind(raw string) (SceneOptionKind, error) {
	kind := SceneOptionKind(strings.TrimSpace(raw))
	switch kind {
	case SceneOptionEnvironment, SceneOptionOperatingSystem, SceneOptionBrowser:
		return kind, nil
	default:
		return "", ErrSceneOptionKindInvalid
	}
}

func (kind SceneOptionKind) LegacyColumn() string {
	switch kind {
	case SceneOptionEnvironment:
		return "found_environment"
	case SceneOptionOperatingSystem:
		return "operating_system"
	case SceneOptionBrowser:
		return "browser"
	default:
		return ""
	}
}

func (kind SceneOptionKind) BuiltInNames() []string {
	if kind == SceneOptionEnvironment {
		return []string{DefaultEnvironmentName}
	}
	return nil
}

type SceneOption struct {
	ID         uint            `json:"id,omitempty"`
	Kind       SceneOptionKind `json:"kind"`
	Name       string          `json:"name"`
	BuiltIn    bool            `json:"built_in"`
	Legacy     bool            `json:"legacy"`
	UsageCount int64           `json:"usage_count"`
}

type SceneOptionRecord struct {
	ID             uint
	Name           string
	NormalizedName string
}

type SceneOptionRepository interface {
	ListCustom(workspaceID uint, kind SceneOptionKind) ([]SceneOptionRecord, error)
	ListLegacyNames(workspaceID uint, kind SceneOptionKind) ([]string, error)
	UsageCounts(workspaceID uint, kind SceneOptionKind) (map[string]int64, error)
	NameExists(workspaceID uint, kind SceneOptionKind, normalizedName string) (bool, error)
	Create(workspaceID, createdByUserID uint, kind SceneOptionKind, name, normalizedName string) (SceneOptionRecord, error)
	FindCustom(workspaceID, id uint, kind SceneOptionKind) (SceneOptionRecord, error)
	DeleteCustom(workspaceID, id uint, kind SceneOptionKind) error
}

type SceneOptionService struct {
	repository SceneOptionRepository
}

func NewSceneOptionService(repository SceneOptionRepository) *SceneOptionService {
	return &SceneOptionService{repository: repository}
}

func NormalizeSceneOptionName(raw string) (string, string, error) {
	name := strings.TrimSpace(raw)
	if name == "" || utf8.RuneCountInString(name) > SceneOptionNameMaxRunes {
		return "", "", ErrSceneOptionNameInvalid
	}
	for _, value := range name {
		if unicode.IsLetter(value) || unicode.IsNumber(value) || value == ' ' ||
			value == '-' || value == '_' || value == '.' || value == '/' {
			continue
		}
		return "", "", ErrSceneOptionNameInvalid
	}
	name = strings.Join(strings.Fields(name), " ")
	if name == "" {
		return "", "", ErrSceneOptionNameInvalid
	}
	return name, strings.ToLower(name), nil
}

func (service *SceneOptionService) List(workspaceID uint, kind SceneOptionKind) ([]SceneOption, error) {
	if _, err := ParseSceneOptionKind(string(kind)); err != nil {
		return nil, err
	}
	custom, err := service.repository.ListCustom(workspaceID, kind)
	if err != nil {
		return nil, err
	}
	legacy, err := service.repository.ListLegacyNames(workspaceID, kind)
	if err != nil {
		return nil, err
	}
	usageCounts, err := service.repository.UsageCounts(workspaceID, kind)
	if err != nil {
		return nil, err
	}

	result := make([]SceneOption, 0, len(custom)+len(legacy)+1)
	seen := make(map[string]struct{}, len(custom)+len(legacy)+1)
	for _, name := range kind.BuiltInNames() {
		normalized := strings.ToLower(name)
		seen[normalized] = struct{}{}
		result = append(result, SceneOption{
			Kind: kind, Name: name, BuiltIn: true, UsageCount: usageCounts[normalized],
		})
	}
	for _, item := range custom {
		normalized := strings.TrimSpace(item.NormalizedName)
		if normalized == "" {
			normalized = strings.ToLower(strings.TrimSpace(item.Name))
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, SceneOption{
			ID: item.ID, Kind: kind, Name: item.Name, UsageCount: usageCounts[normalized],
		})
	}
	for _, raw := range legacy {
		name := strings.TrimSpace(raw)
		normalized := strings.ToLower(name)
		if name == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, SceneOption{
			Kind: kind, Name: name, Legacy: true, UsageCount: usageCounts[normalized],
		})
	}
	return result, nil
}

func (service *SceneOptionService) Create(
	workspaceID, createdByUserID uint,
	kind SceneOptionKind,
	rawName string,
) (SceneOption, error) {
	if _, err := ParseSceneOptionKind(string(kind)); err != nil {
		return SceneOption{}, err
	}
	name, normalized, err := NormalizeSceneOptionName(rawName)
	if err != nil {
		return SceneOption{}, err
	}
	for _, builtIn := range kind.BuiltInNames() {
		if normalized == strings.ToLower(builtIn) {
			return SceneOption{}, ErrSceneOptionNameDuplicate
		}
	}
	exists, err := service.repository.NameExists(workspaceID, kind, normalized)
	if err != nil {
		return SceneOption{}, err
	}
	if exists {
		return SceneOption{}, ErrSceneOptionNameDuplicate
	}
	record, err := service.repository.Create(workspaceID, createdByUserID, kind, name, normalized)
	if err != nil {
		return SceneOption{}, err
	}
	return SceneOption{ID: record.ID, Kind: kind, Name: record.Name}, nil
}

func (service *SceneOptionService) Delete(workspaceID, id uint, kind SceneOptionKind) error {
	if _, err := ParseSceneOptionKind(string(kind)); err != nil {
		return err
	}
	record, err := service.repository.FindCustom(workspaceID, id, kind)
	if err != nil {
		return err
	}
	usageCounts, err := service.repository.UsageCounts(workspaceID, kind)
	if err != nil {
		return err
	}
	usageCount := usageCounts[strings.ToLower(strings.TrimSpace(record.NormalizedName))]
	if usageCount > 0 {
		return &SceneOptionInUseError{UsageCount: usageCount}
	}
	return service.repository.DeleteCustom(workspaceID, id, kind)
}

// Compatibility aliases keep the initial environment-only API source stable
// while callers migrate to the generic quality scene option catalog.
const EnvironmentNameMaxRunes = SceneOptionNameMaxRunes

var (
	ErrEnvironmentNameInvalid   = ErrSceneOptionNameInvalid
	ErrEnvironmentNameDuplicate = ErrSceneOptionNameDuplicate
)

func NormalizeEnvironmentName(raw string) (string, string, error) {
	return NormalizeSceneOptionName(raw)
}
