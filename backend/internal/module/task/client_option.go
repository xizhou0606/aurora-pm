package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const ClientOptionNameMaxRunes = 64

var (
	ErrClientOptionInvalid   = errors.New("task client option is invalid")
	ErrClientOptionDuplicate = errors.New("task client option name already exists")
	ErrClientOptionNotFound  = errors.New("task client option not found")
)

var DefaultClientOptionNames = []string{"桌面端", "H5", "App"}

type ClientOption struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	UsageCount int64  `json:"usage_count"`
}

type ClientOptionRecord struct {
	ID             uint
	Name           string
	NormalizedName string
}

type ClientOptionRepository interface {
	EnsureDefaults(context.Context, uint, uint, []string) error
	List(context.Context, uint) ([]ClientOptionRecord, error)
	UsageCounts(context.Context, uint) (map[uint]int64, error)
	Create(context.Context, uint, uint, string, string) (ClientOptionRecord, error)
	Update(context.Context, uint, uint, string, string) (ClientOptionRecord, error)
	Delete(context.Context, uint, uint) error
}

type ClientOptionInUseError struct{ UsageCount int64 }

func (err *ClientOptionInUseError) Error() string {
	return fmt.Sprintf("task client option is used by %d tasks", err.UsageCount)
}

type ClientOptionService struct{ repository ClientOptionRepository }

func NewClientOptionService(repository ClientOptionRepository) (*ClientOptionService, error) {
	if repository == nil {
		return nil, ErrClientOptionInvalid
	}
	return &ClientOptionService{repository: repository}, nil
}

func NormalizeClientOptionName(raw string) (string, string, error) {
	name := strings.Join(strings.Fields(strings.TrimSpace(raw)), " ")
	if name == "" || utf8.RuneCountInString(name) > ClientOptionNameMaxRunes {
		return "", "", ErrClientOptionInvalid
	}
	for _, value := range name {
		if unicode.IsLetter(value) || unicode.IsNumber(value) || value == ' ' || value == '-' || value == '_' || value == '.' || value == '/' {
			continue
		}
		return "", "", ErrClientOptionInvalid
	}
	return name, strings.ToLower(name), nil
}

func (service *ClientOptionService) List(ctx context.Context, workspaceID, actorUserID uint) ([]ClientOption, error) {
	if workspaceID == 0 {
		return nil, ErrClientOptionInvalid
	}
	if err := service.repository.EnsureDefaults(ctx, workspaceID, actorUserID, DefaultClientOptionNames); err != nil {
		return nil, err
	}
	rows, err := service.repository.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	usage, err := service.repository.UsageCounts(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	result := make([]ClientOption, 0, len(rows))
	for _, row := range rows {
		result = append(result, ClientOption{ID: row.ID, Name: row.Name, UsageCount: usage[row.ID]})
	}
	return result, nil
}

func (service *ClientOptionService) Create(ctx context.Context, workspaceID, actorUserID uint, rawName string) (ClientOption, error) {
	name, normalized, err := NormalizeClientOptionName(rawName)
	if err != nil || workspaceID == 0 || actorUserID == 0 {
		return ClientOption{}, ErrClientOptionInvalid
	}
	row, err := service.repository.Create(ctx, workspaceID, actorUserID, name, normalized)
	if err != nil {
		return ClientOption{}, err
	}
	return ClientOption{ID: row.ID, Name: row.Name}, nil
}

func (service *ClientOptionService) Update(ctx context.Context, workspaceID, id uint, rawName string) (ClientOption, error) {
	name, normalized, err := NormalizeClientOptionName(rawName)
	if err != nil || workspaceID == 0 || id == 0 {
		return ClientOption{}, ErrClientOptionInvalid
	}
	row, err := service.repository.Update(ctx, workspaceID, id, name, normalized)
	if err != nil {
		return ClientOption{}, err
	}
	return ClientOption{ID: row.ID, Name: row.Name}, nil
}

func (service *ClientOptionService) Delete(ctx context.Context, workspaceID, id uint) error {
	if workspaceID == 0 || id == 0 {
		return ErrClientOptionInvalid
	}
	usage, err := service.repository.UsageCounts(ctx, workspaceID)
	if err != nil {
		return err
	}
	if usage[id] > 0 {
		return &ClientOptionInUseError{UsageCount: usage[id]}
	}
	return service.repository.Delete(ctx, workspaceID, id)
}
