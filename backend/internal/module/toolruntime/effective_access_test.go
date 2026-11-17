package toolruntime

import (
	"context"
	"testing"
	"time"
)

func TestEffectiveAccessIntersectsCurrentUserGrantWithWorkspaceRule(t *testing.T) {
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	repository := &authorizationMemoryRepository{}
	service := authorizationTestService(t, repository, func() time.Time { return now })
	grant, err := NewGrant(GrantRequest{
		PrincipalID: 7, WorkspaceID: 11, ClientID: "pm-cli", Channel: DeliveryCLI,
		AllowedTools: []string{"project.list"}, AllowedScopes: []string{"project:read"},
		EffectiveFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour),
	}, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	grant.ID = 1
	repository.grants = []Grant{grant}

	access, err := service.GetEffectiveAccess(context.Background(), managementTestPrincipal())
	if err != nil {
		t.Fatal(err)
	}
	if !access.Configured || access.Status != EffectiveAccessActive || access.Mode != WorkspaceConfigurationWindow ||
		len(access.Tools) != 1 || access.Tools[0].Name != "project.list" {
		t.Fatalf("effective access=%+v", access)
	}
}

func TestEffectiveAccessFailsClosedOutsideTheWorkspaceRuleWindow(t *testing.T) {
	now := time.Date(2026, 8, 27, 8, 0, 0, 0, time.UTC)
	repository := &authorizationMemoryRepository{}
	service := authorizationTestService(t, repository, func() time.Time { return now })
	configuration := repository.configurations[11]
	configuration.EffectiveFrom = now.Add(time.Hour)
	configuration.ExpiresAt = now.Add(2 * time.Hour)
	repository.configurations[11] = configuration

	access, err := service.GetEffectiveAccess(context.Background(), managementTestPrincipal())
	if err != nil {
		t.Fatal(err)
	}
	if access.Status != EffectiveAccessScheduled || len(access.Tools) != 0 {
		t.Fatalf("scheduled effective access=%+v", access)
	}

	principal := managementTestPrincipal()
	principal.WorkspaceID = 12
	access, err = service.GetEffectiveAccess(context.Background(), principal)
	if err != nil {
		t.Fatal(err)
	}
	if access.Configured || access.Status != EffectiveAccessUnconfigured || access.Mode != WorkspaceConfigurationDisabled || len(access.Tools) != 0 {
		t.Fatalf("unconfigured effective access=%+v", access)
	}
}
