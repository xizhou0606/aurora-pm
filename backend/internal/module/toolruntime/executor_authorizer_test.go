package toolruntime

import (
	"context"
	"errors"
	"testing"
	"time"
)

type authorizerGrantRepository struct {
	grant              Grant
	err                error
	reads              int
	configuration      WorkspaceConfiguration
	configurationError error
	configurationReads int
}

func (*authorizerGrantRepository) Create(context.Context, *Grant) error { return nil }

func (repository *authorizerGrantRepository) FindByID(context.Context, uint) (Grant, error) {
	repository.reads++
	return repository.grant, repository.err
}

func (*authorizerGrantRepository) Revoke(context.Context, uint, time.Time) (Grant, error) {
	return Grant{}, nil
}

func (repository *authorizerGrantRepository) FindByWorkspace(context.Context, uint) (WorkspaceConfiguration, error) {
	repository.configurationReads++
	return repository.configuration, repository.configurationError
}

func (*authorizerGrantRepository) Save(context.Context, WorkspaceConfiguration) (WorkspaceConfiguration, error) {
	return WorkspaceConfiguration{}, nil
}

func activeAuthorizerGrant(t *testing.T, now time.Time) Grant {
	t.Helper()
	grant, err := NewGrant(GrantRequest{
		PrincipalID: 7, WorkspaceID: 11, ClientID: "pm-cli", Channel: DeliveryCLI,
		AllowedTools:  []string{"quality.list", "project.list"},
		AllowedScopes: []string{"quality:read", "project:read"},
		EffectiveFrom: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour),
	}, now.Add(-2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	grant.ID = 13
	return grant
}

func TestGrantAuthorizerLoadsOnceAndReturnsVerifiedScopes(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	repository := &authorizerGrantRepository{
		grant:         activeAuthorizerGrant(t, now),
		configuration: testWorkspaceConfiguration(11, []string{"project.list", "quality.list"}, []string{"project:read", "quality:read"}, now.Add(-time.Hour), now.Add(time.Hour)),
	}
	authorizer, err := NewGrantAuthorizer(repository, func() time.Time { return now }, repository)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := authorizer.AuthorizeGrant(context.Background(), executionAuthorization())
	if err != nil {
		t.Fatal(err)
	}
	if repository.reads != 1 || repository.configurationReads != 1 || !grant.Allows("project.list", "project:read") ||
		!grant.Allows("quality.list", "quality:read") || !grant.Allows(PMCurrentUserToolName, PMCurrentUserReadScope) ||
		grant.Allows("project.list", "project:write") ||
		grant.Allows("project.search", "project:read") {
		t.Fatalf("grant reads=%d config reads=%d tools=%v scopes=%v", repository.reads, repository.configurationReads, grant.tools, grant.scopes)
	}
}

func TestGrantAuthorizerMapsInactiveWindowSeparatelyAndOtherFailuresClosed(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	inactive := activeAuthorizerGrant(t, now)
	inactive.EffectiveFrom = now.Add(time.Hour)
	inactive.ExpiresAt = now.Add(2 * time.Hour)
	for name, fixture := range map[string]struct {
		repository *authorizerGrantRepository
		auth       ExecutionAuthorization
		want       error
	}{
		"scheduled grant": {repository: &authorizerGrantRepository{
			grant: inactive, configuration: testWorkspaceConfiguration(11, []string{"project.list"}, []string{"project:read"}, now.Add(-time.Hour), now.Add(time.Hour)),
		}, auth: executionAuthorization(), want: ErrToolUATBlocked},
		"repository failure": {repository: &authorizerGrantRepository{err: errors.New("private database failure")}, auth: executionAuthorization(), want: ErrAuthorizationDenied},
		"workspace mismatch": {repository: &authorizerGrantRepository{grant: activeAuthorizerGrant(t, now)}, auth: func() ExecutionAuthorization { value := executionAuthorization(); value.WorkspaceID = 99; return value }(), want: ErrAuthorizationDenied},
		"audience mismatch": {repository: &authorizerGrantRepository{grant: activeAuthorizerGrant(t, now)}, auth: func() ExecutionAuthorization {
			value := executionAuthorization()
			value.Audience = AudienceMCP
			return value
		}(), want: ErrAuthorizationDenied},
	} {
		t.Run(name, func(t *testing.T) {
			if fixture.repository.configuration.WorkspaceID == 0 && fixture.repository.err == nil {
				fixture.repository.configuration = testWorkspaceConfiguration(11, []string{"project.list", "quality.list"}, []string{"project:read", "quality:read"}, now.Add(-time.Hour), now.Add(time.Hour))
			}
			authorizer, err := NewGrantAuthorizer(fixture.repository, func() time.Time { return now }, fixture.repository)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := authorizer.AuthorizeGrant(context.Background(), fixture.auth); !errors.Is(err, fixture.want) {
				t.Fatalf("err=%v, want %v", err, fixture.want)
			}
		})
	}
}

func TestNewGrantAuthorizerRejectsMissingDependencies(t *testing.T) {
	repository := &authorizerGrantRepository{}
	if _, err := NewGrantAuthorizer(nil, time.Now, repository); !errors.Is(err, ErrExecutorInvalid) {
		t.Fatalf("nil repository err=%v", err)
	}
	if _, err := NewGrantAuthorizer(repository, nil, repository); !errors.Is(err, ErrExecutorInvalid) {
		t.Fatalf("nil clock err=%v", err)
	}
	if _, err := NewGrantAuthorizer(repository, time.Now, nil); !errors.Is(err, ErrExecutorInvalid) {
		t.Fatalf("missing configuration repository err=%v", err)
	}
}

func TestGrantAuthorizerIntersectsCurrentWorkspaceConfigurationExactly(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	repository := &authorizerGrantRepository{
		grant:         activeAuthorizerGrant(t, now),
		configuration: testWorkspaceConfiguration(11, []string{"project.list"}, []string{"project:read"}, now.Add(-time.Hour), now.Add(time.Hour)),
	}
	authorizer, err := NewGrantAuthorizer(repository, func() time.Time { return now }, repository)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := authorizer.AuthorizeGrant(context.Background(), executionAuthorization())
	if err != nil {
		t.Fatal(err)
	}
	if !grant.Allows("project.list", "project:read") || grant.Allows("quality.list", "quality:read") {
		t.Fatalf("intersection tools=%v scopes=%v", grant.tools, grant.scopes)
	}
	repository.configuration.EffectiveFrom = now.Add(time.Minute)
	repository.configuration.ExpiresAt = now.Add(time.Hour)
	if _, err := authorizer.AuthorizeGrant(context.Background(), executionAuthorization()); !errors.Is(err, ErrToolUATBlocked) {
		t.Fatalf("scheduled configuration err=%v", err)
	}
}

func TestGrantAuthorizerAppliesPermanentAndDisabledConfigurationImmediately(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	permanent := testWorkspaceConfiguration(
		11, []string{"project.list"}, []string{"project:read"}, now.Add(-time.Hour), now.Add(time.Hour),
	)
	permanent.Mode = WorkspaceConfigurationPermanent
	permanent.EffectiveFrom, permanent.ExpiresAt = time.Time{}, time.Time{}
	repository := &authorizerGrantRepository{grant: activeAuthorizerGrant(t, now), configuration: permanent}
	authorizer, err := NewGrantAuthorizer(repository, func() time.Time { return now }, repository)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := authorizer.AuthorizeGrant(context.Background(), executionAuthorization())
	if err != nil || !grant.Allows("project.list", "project:read") || grant.Allows("quality.list", "quality:read") {
		t.Fatalf("permanent grant=%+v err=%v", grant, err)
	}

	repository.configuration.Mode = WorkspaceConfigurationDisabled
	repository.configuration.AllowedTools = nil
	repository.configuration.AllowedScopes = nil
	if _, err := authorizer.AuthorizeGrant(context.Background(), executionAuthorization()); !errors.Is(err, ErrToolUATBlocked) {
		t.Fatalf("disabled configuration err=%v", err)
	}
}

func TestNewAuthorizedGrantCanonicalizesAndRejectsInvalidEntitlements(t *testing.T) {
	grant, err := NewAuthorizedGrant(
		[]string{" project.list ", "quality.list", "project.list"},
		[]string{" project:read ", "quality:read", "project:read"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !grant.Allows("project.list", "project:read") || !grant.Allows("quality.list", "quality:read") ||
		grant.Allows("project.list", "project:write") || grant.Allows("project.search", "project:read") {
		t.Fatalf("tools=%v scopes=%v", grant.tools, grant.scopes)
	}
	for name, fixture := range map[string]struct{ tools, scopes []string }{
		"empty tools":    {tools: nil, scopes: []string{"project:read"}},
		"invalid tools":  {tools: []string{"PROJECT_LIST"}, scopes: []string{"project:read"}},
		"empty scopes":   {tools: []string{"project.list"}, scopes: nil},
		"invalid scopes": {tools: []string{"project.list"}, scopes: []string{"PROJECT_READ"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := NewAuthorizedGrant(fixture.tools, fixture.scopes); !errors.Is(err, ErrAuthorizationDenied) {
				t.Fatalf("err=%v, want authorization denied", err)
			}
		})
	}
}
