package toolruntime

import (
	"context"
	"errors"
	"time"
)

var (
	ErrWorkspaceConfigurationInvalid  = errors.New("external tooling workspace configuration is invalid")
	ErrWorkspaceConfigurationNotFound = errors.New("external tooling workspace configuration was not found")
)

type WorkspaceConfigurationMode string

const (
	WorkspaceConfigurationPermanent WorkspaceConfigurationMode = "permanent"
	WorkspaceConfigurationDisabled  WorkspaceConfigurationMode = "disabled"
	WorkspaceConfigurationWindow    WorkspaceConfigurationMode = "window"
	DefaultCLIGrantTTLHours                                    = 168
	MaxCLIGrantTTLHours                                        = 8760
)

var (
	permanentGrantEffectiveFrom = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	permanentGrantExpiresAt     = time.Date(9999, time.December, 31, 23, 59, 59, 0, time.UTC)
)

// WorkspaceConfiguration is the workspace-owned, server-authoritative Tool
// and UAT rule baseline. Grant entitlements can only narrow this baseline.
type WorkspaceConfiguration struct {
	WorkspaceID      uint
	Mode             WorkspaceConfigurationMode
	AllowedTools     []string
	AllowedScopes    []string
	EffectiveFrom    time.Time
	ExpiresAt        time.Time
	CLIGrantTTLHours uint16
	UpdatedBy        uint
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Version          uint64
}

type WorkspaceConfigurationInput struct {
	Mode             WorkspaceConfigurationMode
	ToolNames        []string
	EffectiveFrom    time.Time
	ExpiresAt        time.Time
	CLIGrantTTLHours uint16
}

type WorkspaceConfigurationRepository interface {
	FindByWorkspace(ctx context.Context, workspaceID uint) (WorkspaceConfiguration, error)
	Save(ctx context.Context, configuration WorkspaceConfiguration) (WorkspaceConfiguration, error)
}

func (configuration WorkspaceConfiguration) Validate() error {
	if configuration.WorkspaceID == 0 || configuration.UpdatedBy == 0 || configuration.Version == 0 ||
		configuration.CreatedAt.IsZero() || configuration.UpdatedAt.IsZero() ||
		configuration.UpdatedAt.UTC().Before(configuration.CreatedAt.UTC()) ||
		!equalTools(configuration.AllowedTools, canonicalTools(configuration.AllowedTools)) ||
		!equalScopes(configuration.AllowedScopes, canonicalScopes(configuration.AllowedScopes)) ||
		configuration.CLIGrantTTLHours > MaxCLIGrantTTLHours {
		return ErrWorkspaceConfigurationInvalid
	}
	switch configuration.Mode {
	case WorkspaceConfigurationDisabled:
		if (len(configuration.AllowedTools) == 0) != (len(configuration.AllowedScopes) == 0) ||
			!configuration.EffectiveFrom.IsZero() || !configuration.ExpiresAt.IsZero() {
			return ErrWorkspaceConfigurationInvalid
		}
	case WorkspaceConfigurationPermanent:
		if len(configuration.AllowedTools) == 0 || len(configuration.AllowedScopes) == 0 ||
			!configuration.EffectiveFrom.IsZero() || !configuration.ExpiresAt.IsZero() {
			return ErrWorkspaceConfigurationInvalid
		}
	case WorkspaceConfigurationWindow:
		if len(configuration.AllowedTools) == 0 || len(configuration.AllowedScopes) == 0 ||
			configuration.EffectiveFrom.IsZero() || configuration.ExpiresAt.IsZero() ||
			!configuration.EffectiveFrom.UTC().Before(configuration.ExpiresAt.UTC()) {
			return ErrWorkspaceConfigurationInvalid
		}
	default:
		return ErrWorkspaceConfigurationInvalid
	}
	for _, tool := range configuration.AllowedTools {
		if !toolNamePattern.MatchString(tool) {
			return ErrWorkspaceConfigurationInvalid
		}
	}
	for _, scope := range configuration.AllowedScopes {
		if !grantScopePattern.MatchString(scope) {
			return ErrWorkspaceConfigurationInvalid
		}
	}
	return nil
}

func (configuration WorkspaceConfiguration) RuntimeAllows(now time.Time) bool {
	if configuration.Validate() != nil || now.IsZero() {
		return false
	}
	if configuration.Mode == WorkspaceConfigurationPermanent {
		return true
	}
	if configuration.Mode == WorkspaceConfigurationDisabled {
		return false
	}
	current := now.UTC()
	return !current.Before(configuration.EffectiveFrom.UTC()) && current.Before(configuration.ExpiresAt.UTC())
}

// GrantWindow translates the server-owned UAT rule into the existing finite
// Grant representation. Permanent configuration never exposes these sentinel
// values through the management API; they are only an internal persistence
// boundary for grants.
func (configuration WorkspaceConfiguration) GrantWindow(channel DeliveryChannel, requestedAt time.Time) (time.Time, time.Time, bool) {
	if configuration.Validate() != nil || configuration.Mode == WorkspaceConfigurationDisabled {
		return time.Time{}, time.Time{}, false
	}
	var effectiveFrom, expiresAt time.Time
	if configuration.Mode == WorkspaceConfigurationPermanent {
		effectiveFrom, expiresAt = permanentGrantEffectiveFrom, permanentGrantExpiresAt
	} else {
		effectiveFrom, expiresAt = configuration.EffectiveFrom.UTC(), configuration.ExpiresAt.UTC()
	}
	if channel != DeliveryCLI {
		return effectiveFrom, expiresAt, true
	}
	if requestedAt.IsZero() {
		return time.Time{}, time.Time{}, false
	}
	requestedAt = requestedAt.UTC()
	if requestedAt.After(effectiveFrom) {
		effectiveFrom = requestedAt
	}
	cliExpiresAt := effectiveFrom.Add(time.Duration(configuration.EffectiveCLIGrantTTLHours()) * time.Hour)
	if cliExpiresAt.Before(expiresAt) {
		expiresAt = cliExpiresAt
	}
	return effectiveFrom, expiresAt, effectiveFrom.Before(expiresAt)
}

func (configuration WorkspaceConfiguration) EffectiveCLIGrantTTLHours() uint16 {
	if configuration.CLIGrantTTLHours == 0 {
		return DefaultCLIGrantTTLHours
	}
	return configuration.CLIGrantTTLHours
}
