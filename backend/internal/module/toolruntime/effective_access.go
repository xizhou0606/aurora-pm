package toolruntime

import (
	"context"
	"errors"
	"sort"
	"time"
)

type EffectiveAccessStatus string

const (
	EffectiveAccessUnconfigured EffectiveAccessStatus = "unconfigured"
	EffectiveAccessDisabled     EffectiveAccessStatus = "disabled"
	EffectiveAccessScheduled    EffectiveAccessStatus = "scheduled"
	EffectiveAccessActive       EffectiveAccessStatus = "active"
	EffectiveAccessExpired      EffectiveAccessStatus = "expired"
)

// EffectiveAccess is the self-service projection for an admitted PM user.
// It reports the workspace-owned rule together with the exact Tool union that
// is usable through at least one currently-active Grant owned by the caller.
type EffectiveAccess struct {
	Configured       bool
	Mode             WorkspaceConfigurationMode
	Status           EffectiveAccessStatus
	Tools            []Definition
	EffectiveFrom    time.Time
	ExpiresAt        time.Time
	CLIGrantTTLHours uint16
}

func (service *AuthorizationService) GetEffectiveAccess(
	ctx context.Context,
	principal ManagementPrincipal,
) (EffectiveAccess, error) {
	if err := validateManagementPrincipal(principal); err != nil {
		return EffectiveAccess{}, err
	}
	configuration, err := service.configurations.FindByWorkspace(ctx, principal.WorkspaceID)
	if errors.Is(err, ErrWorkspaceConfigurationNotFound) {
		return EffectiveAccess{
			Mode: WorkspaceConfigurationDisabled, Status: EffectiveAccessUnconfigured,
		}, nil
	}
	if err != nil {
		return EffectiveAccess{}, err
	}
	if err := configuration.Validate(); err != nil {
		return EffectiveAccess{}, err
	}

	now := service.now().UTC()
	result := EffectiveAccess{
		Configured:       true,
		Mode:             configuration.Mode,
		Status:           effectiveConfigurationStatus(configuration, now),
		CLIGrantTTLHours: configuration.EffectiveCLIGrantTTLHours(),
	}
	if configuration.Mode == WorkspaceConfigurationWindow {
		result.EffectiveFrom = configuration.EffectiveFrom.UTC()
		result.ExpiresAt = configuration.ExpiresAt.UTC()
	}
	if result.Status != EffectiveAccessActive {
		return result, nil
	}

	grants, err := service.grants.ListForPrincipalWorkspace(ctx, principal.PrincipalID, principal.WorkspaceID)
	if err != nil {
		return EffectiveAccess{}, err
	}
	configuredTools := make(map[string]struct{}, len(configuration.AllowedTools))
	for _, tool := range configuration.AllowedTools {
		configuredTools[tool] = struct{}{}
	}
	available := make(map[string]struct{})
	for _, grant := range grants {
		if grant.Status(now) != GrantActive {
			continue
		}
		toolNames, validationErr := service.validateGrantEntitlements(grant)
		if validationErr != nil {
			return EffectiveAccess{}, validationErr
		}
		for _, tool := range toolNames {
			if _, configured := configuredTools[tool]; configured {
				available[tool] = struct{}{}
			}
		}
	}

	names := make([]string, 0, len(available))
	for name := range available {
		names = append(names, name)
	}
	sort.Strings(names)
	result.Tools = make([]Definition, 0, len(names))
	for _, name := range names {
		definition, ok := service.catalog.Find(name)
		if !ok || definition.Lifecycle == LifecycleDraft {
			return EffectiveAccess{}, ErrAuthorizationCatalogInvalid
		}
		result.Tools = append(result.Tools, definition)
	}
	return result, nil
}

func effectiveConfigurationStatus(
	configuration WorkspaceConfiguration,
	now time.Time,
) EffectiveAccessStatus {
	switch configuration.Mode {
	case WorkspaceConfigurationDisabled:
		return EffectiveAccessDisabled
	case WorkspaceConfigurationPermanent:
		return EffectiveAccessActive
	case WorkspaceConfigurationWindow:
		if now.Before(configuration.EffectiveFrom.UTC()) {
			return EffectiveAccessScheduled
		}
		if !now.Before(configuration.ExpiresAt.UTC()) {
			return EffectiveAccessExpired
		}
		return EffectiveAccessActive
	default:
		return EffectiveAccessUnconfigured
	}
}
