package toolruntime

import (
	"context"
	"errors"
	"time"
)

func (service *AuthorizationService) GetWorkspaceConfiguration(
	ctx context.Context,
	principal ManagementPrincipal,
) (WorkspaceConfiguration, error) {
	if err := validateManagementPrincipal(principal); err != nil {
		return WorkspaceConfiguration{}, ErrWorkspaceConfigurationNotFound
	}
	configuration, err := service.configurations.FindByWorkspace(ctx, principal.WorkspaceID)
	if err != nil {
		return WorkspaceConfiguration{}, err
	}
	if configuration.WorkspaceID != principal.WorkspaceID {
		return WorkspaceConfiguration{}, ErrWorkspaceConfigurationNotFound
	}
	if err := service.validateConfiguredCatalog(configuration); err != nil {
		return WorkspaceConfiguration{}, err
	}
	return configuration, nil
}

func (service *AuthorizationService) SaveWorkspaceConfiguration(
	ctx context.Context,
	principal ManagementPrincipal,
	input WorkspaceConfigurationInput,
) (WorkspaceConfiguration, error) {
	if err := validateManagementPrincipal(principal); err != nil {
		return WorkspaceConfiguration{}, ErrWorkspaceConfigurationInvalid
	}
	now := service.now().UTC()
	if now.IsZero() {
		return WorkspaceConfiguration{}, ErrWorkspaceConfigurationInvalid
	}
	cliGrantTTLHours := input.CLIGrantTTLHours
	if cliGrantTTLHours == 0 {
		cliGrantTTLHours = DefaultCLIGrantTTLHours
	}
	if cliGrantTTLHours > MaxCLIGrantTTLHours {
		return WorkspaceConfiguration{}, ErrWorkspaceConfigurationInvalid
	}
	var tools, scopes []string
	switch input.Mode {
	case WorkspaceConfigurationDisabled:
		if !input.EffectiveFrom.IsZero() || !input.ExpiresAt.IsZero() {
			return WorkspaceConfiguration{}, ErrWorkspaceConfigurationInvalid
		}
		if len(input.ToolNames) == 0 {
			tools, scopes = []string{}, []string{}
		}
	case WorkspaceConfigurationPermanent:
		if !input.EffectiveFrom.IsZero() || !input.ExpiresAt.IsZero() {
			return WorkspaceConfiguration{}, ErrWorkspaceConfigurationInvalid
		}
	case WorkspaceConfigurationWindow:
		if input.EffectiveFrom.IsZero() || input.ExpiresAt.IsZero() ||
			!input.EffectiveFrom.UTC().Before(input.ExpiresAt.UTC()) || !now.Before(input.ExpiresAt.UTC()) {
			return WorkspaceConfiguration{}, ErrWorkspaceConfigurationInvalid
		}
	default:
		return WorkspaceConfiguration{}, ErrWorkspaceConfigurationInvalid
	}
	if input.Mode != WorkspaceConfigurationDisabled || len(input.ToolNames) > 0 {
		var err error
		tools, scopes, err = service.resolveTools(input.ToolNames)
		if err != nil {
			if errors.Is(err, ErrAuthorizationRequestInvalid) {
				return WorkspaceConfiguration{}, ErrWorkspaceConfigurationInvalid
			}
			return WorkspaceConfiguration{}, err
		}
	}
	configuration, err := service.configurations.Save(ctx, WorkspaceConfiguration{
		WorkspaceID: principal.WorkspaceID, Mode: input.Mode, AllowedTools: tools, AllowedScopes: scopes,
		EffectiveFrom: input.EffectiveFrom.UTC(), ExpiresAt: input.ExpiresAt.UTC(), CLIGrantTTLHours: cliGrantTTLHours,
		UpdatedBy: principal.PrincipalID,
		CreatedAt: now, UpdatedAt: now, Version: 1,
	})
	if err != nil {
		return WorkspaceConfiguration{}, err
	}
	if configuration.WorkspaceID != principal.WorkspaceID || configuration.Validate() != nil ||
		!equalTools(configuration.AllowedTools, tools) || !equalScopes(configuration.AllowedScopes, scopes) ||
		configuration.EffectiveCLIGrantTTLHours() != cliGrantTTLHours {
		return WorkspaceConfiguration{}, ErrWorkspaceConfigurationInvalid
	}
	return configuration, nil
}

// configurationForAuthorization permits a future window so a scheduled Grant
// can be approved, but rejects disabled or expired baselines.
func (service *AuthorizationService) configurationForAuthorization(
	ctx context.Context,
	workspaceID uint,
	now time.Time,
) (WorkspaceConfiguration, error) {
	configuration, err := service.configurations.FindByWorkspace(ctx, workspaceID)
	if err != nil {
		return WorkspaceConfiguration{}, err
	}
	if configuration.WorkspaceID != workspaceID || configuration.Validate() != nil ||
		configuration.Mode == WorkspaceConfigurationDisabled ||
		(configuration.Mode == WorkspaceConfigurationWindow && !now.UTC().Before(configuration.ExpiresAt.UTC())) {
		return WorkspaceConfiguration{}, ErrWorkspaceConfigurationInvalid
	}
	if err := service.validateConfiguredCatalog(configuration); err != nil {
		return WorkspaceConfiguration{}, err
	}
	return configuration, nil
}

func (service *AuthorizationService) validateConfiguredCatalog(configuration WorkspaceConfiguration) error {
	if configuration.Mode == WorkspaceConfigurationDisabled {
		if len(configuration.AllowedTools) == 0 && len(configuration.AllowedScopes) == 0 {
			return nil
		}
	}
	tools, scopes, err := service.resolveTools(configuration.AllowedTools)
	if err != nil || !equalTools(tools, configuration.AllowedTools) || !equalScopes(scopes, configuration.AllowedScopes) {
		return ErrAuthorizationCatalogInvalid
	}
	return nil
}
