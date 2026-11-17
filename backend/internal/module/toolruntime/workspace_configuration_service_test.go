package toolruntime

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestWorkspaceConfigurationSaveDerivesScopesAndIsWorkspaceScoped(t *testing.T) {
	now := time.Date(2026, 8, 27, 4, 0, 0, 0, time.UTC)
	repository := &authorizationMemoryRepository{}
	service := authorizationTestService(t, repository, func() time.Time { return now })
	configuration, err := service.SaveWorkspaceConfiguration(context.Background(), managementTestPrincipal(), WorkspaceConfigurationInput{
		Mode: WorkspaceConfigurationWindow, ToolNames: []string{"task.list", "project.list"}, EffectiveFrom: now.Add(time.Hour), ExpiresAt: now.Add(24 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if !equalTools(configuration.AllowedTools, []string{"project.list", "task.list"}) ||
		!equalScopes(configuration.AllowedScopes, []string{"project:read", "task:read"}) || configuration.UpdatedBy != 7 {
		t.Fatalf("configuration=%+v", configuration)
	}
	other := managementTestPrincipal()
	other.WorkspaceID = 12
	if _, err := service.GetWorkspaceConfiguration(context.Background(), other); !errors.Is(err, ErrWorkspaceConfigurationNotFound) {
		t.Fatalf("cross-workspace err=%v", err)
	}
}

func TestAuthorizationStartUsesOnlyCurrentWorkspaceConfiguration(t *testing.T) {
	now := time.Date(2026, 8, 27, 4, 0, 0, 0, time.UTC)
	repository := &authorizationMemoryRepository{}
	service := authorizationTestService(t, repository, func() time.Time { return now })
	repository.configurations[11] = testWorkspaceConfiguration(
		11, []string{"project.list"}, []string{"project:read"}, now.Add(time.Hour), now.Add(24*time.Hour),
	)

	request, err := service.Start(context.Background(), managementTestPrincipal(), AuthorizationRequestInput{Channel: DeliveryCLI})
	if err != nil {
		t.Fatal(err)
	}
	if !equalTools(request.ToolNames, []string{"project.list"}) || !request.EffectiveFrom.Equal(now.Add(time.Hour)) {
		t.Fatalf("request=%+v", request)
	}
	_, err = service.Start(context.Background(), managementTestPrincipal(), AuthorizationRequestInput{
		Channel: DeliveryCLI, ToolNames: []string{"project.search"},
		EffectiveFrom: now.Add(time.Hour), ExpiresAt: now.Add(24 * time.Hour),
	})
	if !errors.Is(err, ErrAuthorizationRequestInvalid) {
		t.Fatalf("widened start err=%v", err)
	}

	repository.configurations[11] = testWorkspaceConfiguration(
		11, []string{"project.search"}, []string{"project:read"}, now.Add(time.Hour), now.Add(24*time.Hour),
	)
	if _, err := service.Decide(context.Background(), managementTestPrincipal(), request.ID, AuthorizationAuthorize); !errors.Is(err, ErrAuthorizationRequestInvalid) {
		t.Fatalf("stale request decision err=%v", err)
	}
	if len(repository.grants) != 0 {
		t.Fatalf("stale request created grants=%+v", repository.grants)
	}
}

func TestWorkspaceConfigurationRejectsEmptyAndInvalidWindows(t *testing.T) {
	now := time.Date(2026, 8, 27, 4, 0, 0, 0, time.UTC)
	service := authorizationTestService(t, &authorizationMemoryRepository{}, func() time.Time { return now })
	for name, input := range map[string]WorkspaceConfigurationInput{
		"empty tools": {Mode: WorkspaceConfigurationWindow, EffectiveFrom: now, ExpiresAt: now.Add(time.Hour)},
		"expired":     {Mode: WorkspaceConfigurationWindow, ToolNames: []string{"project.list"}, EffectiveFrom: now.Add(-time.Hour), ExpiresAt: now},
		"reversed":    {Mode: WorkspaceConfigurationWindow, ToolNames: []string{"project.list"}, EffectiveFrom: now.Add(time.Hour), ExpiresAt: now.Add(time.Minute)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := service.SaveWorkspaceConfiguration(context.Background(), managementTestPrincipal(), input); err == nil {
				t.Fatal("expected invalid configuration")
			}
		})
	}
}

func TestWorkspaceConfigurationSupportsPermanentAndDisabledModes(t *testing.T) {
	now := time.Date(2026, 8, 27, 4, 0, 0, 0, time.UTC)
	repository := &authorizationMemoryRepository{}
	service := authorizationTestService(t, repository, func() time.Time { return now })

	permanent, err := service.SaveWorkspaceConfiguration(context.Background(), managementTestPrincipal(), WorkspaceConfigurationInput{
		Mode: WorkspaceConfigurationPermanent, ToolNames: []string{"project.list"},
	})
	if err != nil || permanent.Mode != WorkspaceConfigurationPermanent ||
		!permanent.EffectiveFrom.IsZero() || !permanent.ExpiresAt.IsZero() || !permanent.RuntimeAllows(now) {
		t.Fatalf("permanent=%+v err=%v", permanent, err)
	}
	request, err := service.Start(context.Background(), managementTestPrincipal(), AuthorizationRequestInput{Channel: DeliveryCLI})
	if err != nil || request.ConfigurationMode != WorkspaceConfigurationPermanent || request.ConfigurationVersion != permanent.Version {
		t.Fatalf("permanent request=%+v err=%v", request, err)
	}
	if _, err := service.Decide(context.Background(), managementTestPrincipal(), request.ID, AuthorizationAuthorize); err != nil {
		t.Fatal(err)
	}
	if len(repository.grants) != 1 || repository.grants[0].Status(now) != GrantActive ||
		!repository.grants[0].ExpiresAt.Equal(now.Add(DefaultCLIGrantTTLHours*time.Hour)) {
		t.Fatalf("permanent grants=%+v", repository.grants)
	}

	disabled, err := service.SaveWorkspaceConfiguration(context.Background(), managementTestPrincipal(), WorkspaceConfigurationInput{
		Mode: WorkspaceConfigurationDisabled, ToolNames: []string{"project.list"},
	})
	if err != nil || disabled.Mode != WorkspaceConfigurationDisabled ||
		!equalTools(disabled.AllowedTools, []string{"project.list"}) ||
		!equalScopes(disabled.AllowedScopes, []string{"project:read"}) || disabled.RuntimeAllows(now) {
		t.Fatalf("disabled=%+v err=%v", disabled, err)
	}
	if _, err := service.Start(context.Background(), managementTestPrincipal(), AuthorizationRequestInput{Channel: DeliveryCLI}); !errors.Is(err, ErrWorkspaceConfigurationInvalid) {
		t.Fatalf("disabled start err=%v", err)
	}
}

func TestCLIGrantTTLIsConfigurableAndCappedByToolsWindow(t *testing.T) {
	now := time.Date(2026, 8, 27, 4, 0, 0, 0, time.UTC)
	repository := &authorizationMemoryRepository{}
	service := authorizationTestService(t, repository, func() time.Time { return now })

	configuration, err := service.SaveWorkspaceConfiguration(context.Background(), managementTestPrincipal(), WorkspaceConfigurationInput{
		Mode: WorkspaceConfigurationPermanent, ToolNames: []string{"project.list"}, CLIGrantTTLHours: 30,
	})
	if err != nil || configuration.EffectiveCLIGrantTTLHours() != 30 {
		t.Fatalf("configuration=%+v err=%v", configuration, err)
	}
	cli, err := service.Start(context.Background(), managementTestPrincipal(), AuthorizationRequestInput{Channel: DeliveryCLI})
	if err != nil || !cli.EffectiveFrom.Equal(now) || !cli.ExpiresAt.Equal(now.Add(30*time.Hour)) {
		t.Fatalf("cli request=%+v err=%v", cli, err)
	}
	mcp, err := service.Start(context.Background(), managementTestPrincipal(), AuthorizationRequestInput{Channel: DeliveryMCP})
	if err != nil || mcp.EffectiveFrom.Year() != 1970 || mcp.ExpiresAt.Year() != 9999 {
		t.Fatalf("mcp request=%+v err=%v", mcp, err)
	}

	windowEnd := now.Add(48 * time.Hour)
	configuration, err = service.SaveWorkspaceConfiguration(context.Background(), managementTestPrincipal(), WorkspaceConfigurationInput{
		Mode: WorkspaceConfigurationWindow, ToolNames: []string{"project.list"},
		EffectiveFrom: now.Add(time.Hour), ExpiresAt: windowEnd, CLIGrantTTLHours: 72,
	})
	if err != nil {
		t.Fatal(err)
	}
	cli, err = service.Start(context.Background(), managementTestPrincipal(), AuthorizationRequestInput{Channel: DeliveryCLI})
	if err != nil || !cli.EffectiveFrom.Equal(now.Add(time.Hour)) || !cli.ExpiresAt.Equal(windowEnd) {
		t.Fatalf("capped cli request=%+v configuration=%+v err=%v", cli, configuration, err)
	}
}

func TestWorkspaceConfigurationModeValidationFailsClosed(t *testing.T) {
	now := time.Date(2026, 8, 27, 4, 0, 0, 0, time.UTC)
	service := authorizationTestService(t, &authorizationMemoryRepository{}, func() time.Time { return now })
	tests := map[string]WorkspaceConfigurationInput{
		"unknown mode":    {Mode: "unknown", ToolNames: []string{"project.list"}},
		"disabled window": {Mode: WorkspaceConfigurationDisabled, EffectiveFrom: now, ExpiresAt: now.Add(time.Hour)},
		"permanent empty": {Mode: WorkspaceConfigurationPermanent},
		"permanent times": {Mode: WorkspaceConfigurationPermanent, ToolNames: []string{"project.list"}, EffectiveFrom: now, ExpiresAt: now.Add(time.Hour)},
		"window missing":  {Mode: WorkspaceConfigurationWindow, ToolNames: []string{"project.list"}},
		"ttl too large":   {Mode: WorkspaceConfigurationPermanent, ToolNames: []string{"project.list"}, CLIGrantTTLHours: MaxCLIGrantTTLHours + 1},
	}
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := service.SaveWorkspaceConfiguration(context.Background(), managementTestPrincipal(), input); !errors.Is(err, ErrWorkspaceConfigurationInvalid) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestAuthorizationDecisionRejectsAnyConfigurationVersionChange(t *testing.T) {
	now := time.Date(2026, 8, 27, 4, 0, 0, 0, time.UTC)
	repository := &authorizationMemoryRepository{}
	service := authorizationTestService(t, repository, func() time.Time { return now })
	request, err := service.Start(context.Background(), managementTestPrincipal(), AuthorizationRequestInput{Channel: DeliveryMCP})
	if err != nil {
		t.Fatal(err)
	}
	current := repository.configurations[11]
	current.UpdatedAt = now
	if _, err := repository.Save(context.Background(), current); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Decide(context.Background(), managementTestPrincipal(), request.ID, AuthorizationAuthorize); !errors.Is(err, ErrAuthorizationRequestInvalid) {
		t.Fatalf("same-values new-version decision err=%v", err)
	}
	if len(repository.grants) != 0 {
		t.Fatalf("stale request created grants=%+v", repository.grants)
	}
}
