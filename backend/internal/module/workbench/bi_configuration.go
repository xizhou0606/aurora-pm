package workbench

import (
	"context"
	"errors"
	"time"
)

var ErrBIConfigurationReadInvalid = errors.New("BI configuration read query is invalid")

type BITeamMetricSettingsSnapshot struct {
	Configured                    bool
	RequirementWorkflowTemplateID uint
	ApprovalNodeID                string
}

type BITeamScoreProfileSnapshot struct {
	ID          uint
	WorkspaceID uint
	Version     int
	Status      string
	ActivatedAt *time.Time
	Items       []BITeamScoreProfileItemSnapshot
}

type BITeamScoreProfileItemSnapshot struct {
	MetricKey      string
	RoleKey        string
	Enabled        bool
	WeightBPS      int
	MissingPolicy  string
	MinimumSamples int
	ParametersJSON string
	SortOrder      int
}

type BIConfigurationReadStore interface {
	GetBITeamMetricSettings(context.Context, uint) (BITeamMetricSettingsSnapshot, error)
	GetBITeamScoreProfile(context.Context, uint) (BITeamScoreProfileSnapshot, error)
}

type BIConfigurationReadService struct{ store BIConfigurationReadStore }

func NewBIConfigurationReadService(store BIConfigurationReadStore) (*BIConfigurationReadService, error) {
	if store == nil {
		return nil, ErrBIConfigurationReadInvalid
	}
	return &BIConfigurationReadService{store: store}, nil
}

func (service *BIConfigurationReadService) GetBITeamMetricSettings(ctx context.Context, workspaceID uint) (BITeamMetricSettingsSnapshot, error) {
	if workspaceID == 0 {
		return BITeamMetricSettingsSnapshot{}, ErrBIConfigurationReadInvalid
	}
	return service.store.GetBITeamMetricSettings(ctx, workspaceID)
}

func (service *BIConfigurationReadService) GetBITeamScoreProfile(ctx context.Context, workspaceID uint) (BITeamScoreProfileSnapshot, error) {
	if workspaceID == 0 {
		return BITeamScoreProfileSnapshot{}, ErrBIConfigurationReadInvalid
	}
	return service.store.GetBITeamScoreProfile(ctx, workspaceID)
}
