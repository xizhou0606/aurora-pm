package toolruntime

import (
	"context"
	"errors"
	"testing"
	"time"
)

type authorizationMemoryRepository struct {
	requests       map[string]AuthorizationRequest
	grants         []Grant
	configurations map[uint]WorkspaceConfiguration
}

func (repository *authorizationMemoryRepository) FindByWorkspace(_ context.Context, workspaceID uint) (WorkspaceConfiguration, error) {
	configuration, ok := repository.configurations[workspaceID]
	if !ok {
		return WorkspaceConfiguration{}, ErrWorkspaceConfigurationNotFound
	}
	return configuration, nil
}

func (repository *authorizationMemoryRepository) Save(_ context.Context, configuration WorkspaceConfiguration) (WorkspaceConfiguration, error) {
	if repository.configurations == nil {
		repository.configurations = map[uint]WorkspaceConfiguration{}
	}
	if current, ok := repository.configurations[configuration.WorkspaceID]; ok {
		configuration.CreatedAt = current.CreatedAt
		configuration.Version = current.Version + 1
	}
	repository.configurations[configuration.WorkspaceID] = configuration
	return configuration, nil
}

func (repository *authorizationMemoryRepository) Create(_ context.Context, request *AuthorizationRequest) error {
	if repository.requests == nil {
		repository.requests = map[string]AuthorizationRequest{}
	}
	repository.requests[request.ID] = *request
	return nil
}

func (repository *authorizationMemoryRepository) FindByID(_ context.Context, id string) (AuthorizationRequest, error) {
	request, ok := repository.requests[id]
	if !ok {
		return AuthorizationRequest{}, ErrAuthorizationRequestNotFound
	}
	return request, nil
}

func (repository *authorizationMemoryRepository) Decide(_ context.Context, command AuthorizationDecisionCommand) (AuthorizationRequest, error) {
	request, ok := repository.requests[command.RequestID]
	if !ok || request.PrincipalID != command.PrincipalID || request.WorkspaceID != command.WorkspaceID {
		return AuthorizationRequest{}, ErrAuthorizationRequestNotFound
	}
	if request.Decision != AuthorizationPending {
		return AuthorizationRequest{}, ErrAuthorizationRequestDecided
	}
	request.Decision, request.DecidedAt, request.UpdatedAt = command.Decision, &command.DecidedAt, command.DecidedAt
	if command.Grant != nil {
		command.Grant.ID = uint(len(repository.grants) + 1)
		repository.grants = append(repository.grants, *command.Grant)
		request.GrantID = &command.Grant.ID
	}
	repository.requests[request.ID] = request
	return request, nil
}

func (repository *authorizationMemoryRepository) ListForPrincipalWorkspace(_ context.Context, principalID, workspaceID uint) ([]Grant, error) {
	result := make([]Grant, 0)
	for _, grant := range repository.grants {
		if grant.PrincipalID == principalID && grant.WorkspaceID == workspaceID {
			result = append(result, grant)
		}
	}
	return result, nil
}

func TestAuthorizationServiceResolvesToolNamesToCanonicalGrantScopes(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	repository := &authorizationMemoryRepository{}
	service := authorizationTestService(t, repository, func() time.Time { return now })

	request, err := service.Start(context.Background(), managementTestPrincipal(), AuthorizationRequestInput{
		Channel: DeliveryCLI,
	})
	if err != nil {
		t.Fatal(err)
	}
	if request.Decision != AuthorizationPending || request.ClientID != "pm-cli" || request.GrantScopes[0] != "project:read" || request.GrantScopes[1] != "task:read" {
		t.Fatalf("request was not resolved from the canonical catalog: %+v", request)
	}
	if !request.RequestExpiresAt.Equal(now.Add(DefaultAuthorizationRequestTTL)) {
		t.Fatalf("request expiry=%s", request.RequestExpiresAt)
	}
	if _, err := service.Decide(context.Background(), managementTestPrincipal(), request.ID, AuthorizationAuthorize); err != nil {
		t.Fatal(err)
	}
	if len(repository.grants) != 1 || repository.grants[0].AllowedScopes[1] != "task:read" ||
		repository.grants[0].AllowedTools[0] != "project.list" || repository.grants[0].AllowedTools[1] != "task.list" {
		t.Fatalf("authorized grant=%+v", repository.grants)
	}
	if _, err := service.Decide(context.Background(), managementTestPrincipal(), request.ID, AuthorizationAuthorize); !errors.Is(err, ErrAuthorizationRequestDecided) {
		t.Fatalf("replayed decision err=%v", err)
	}
}

func TestSecureAuthorizationRequestIDsAreOpaqueAndUnique(t *testing.T) {
	seen := make(map[string]struct{}, 128)
	for index := 0; index < 128; index++ {
		id, err := secureAuthorizationRequestID()
		if err != nil || !authorizationRequestIDPattern.MatchString(id) {
			t.Fatalf("id=%q err=%v", id, err)
		}
		if _, duplicate := seen[id]; duplicate {
			t.Fatalf("duplicate authorization request id %q", id)
		}
		seen[id] = struct{}{}
	}
}

func TestAuthorizationServiceRejectsUnknownDraftAndDuplicateTools(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	service := authorizationTestService(t, &authorizationMemoryRepository{}, func() time.Time { return now })
	tests := []struct {
		name  string
		tools []string
		want  error
	}{
		{name: "unknown", tools: []string{"missing.list"}, want: ErrAuthorizationCatalogInvalid},
		{name: "draft", tools: []string{"workspace.member.list"}, want: ErrAuthorizationCatalogInvalid},
		{name: "duplicate", tools: []string{"project.list", " project.list "}, want: ErrWorkspaceConfigurationInvalid},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.SaveWorkspaceConfiguration(context.Background(), managementTestPrincipal(), WorkspaceConfigurationInput{
				Mode: WorkspaceConfigurationWindow, ToolNames: test.tools, EffectiveFrom: now, ExpiresAt: now.Add(time.Hour),
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("err=%v want=%v", err, test.want)
			}
		})
	}
}

func TestAuthorizationServiceFailsClosedForOwnerAndTimeWindows(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	repository := &authorizationMemoryRepository{}
	clock := now
	service := authorizationTestService(t, repository, func() time.Time { return clock })

	repository.configurations[11] = testWorkspaceConfiguration(11, []string{"project.list"}, []string{"project:read"}, now.Add(-time.Hour), now)
	if _, err := service.Start(context.Background(), managementTestPrincipal(), AuthorizationRequestInput{Channel: DeliveryMCP}); !errors.Is(err, ErrWorkspaceConfigurationInvalid) {
		t.Fatalf("expired start err=%v", err)
	}
	repository.configurations[11] = testWorkspaceConfiguration(11, []string{"project.list"}, []string{"project:read"}, now.Add(-time.Hour), now.Add(5*time.Minute))
	request, err := service.Start(context.Background(), managementTestPrincipal(), AuthorizationRequestInput{Channel: DeliveryMCP})
	if err != nil {
		t.Fatal(err)
	}
	other := managementTestPrincipal()
	other.WorkspaceID = 99
	if _, err := service.Get(context.Background(), other, request.ID); !errors.Is(err, ErrAuthorizationRequestNotFound) {
		t.Fatalf("cross-workspace read err=%v", err)
	}
	clock = now.Add(6 * time.Minute)
	if _, err := service.Decide(context.Background(), managementTestPrincipal(), request.ID, AuthorizationAuthorize); !errors.Is(err, ErrWorkspaceConfigurationInvalid) {
		t.Fatalf("late authorize err=%v", err)
	}
	if len(repository.grants) != 0 {
		t.Fatalf("late authorization created grants=%+v", repository.grants)
	}
	if _, err := service.Decide(context.Background(), managementTestPrincipal(), request.ID, AuthorizationDeny); err != nil {
		t.Fatalf("deny within request lifetime must remain possible: %v", err)
	}
}

func TestAuthorizationServiceListsExactToolsWhenScopesAreShared(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	repository := &authorizationMemoryRepository{}
	service := authorizationTestService(t, repository, func() time.Time { return now })
	grant, err := NewGrant(GrantRequest{
		PrincipalID: 7, WorkspaceID: 11, ClientID: "pm-cli", Channel: DeliveryCLI,
		AllowedTools: []string{"project.list"}, AllowedScopes: []string{"project:read"},
		EffectiveFrom: now, ExpiresAt: now.Add(time.Hour),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	grant.ID = 1
	repository.grants = []Grant{grant}
	summaries, err := service.ListGrants(context.Background(), managementTestPrincipal())
	if err != nil || len(summaries) != 1 || len(summaries[0].ToolNames) != 1 || summaries[0].ToolNames[0] != "project.list" {
		t.Fatalf("summaries=%+v err=%v", summaries, err)
	}
}

func authorizationTestService(t *testing.T, repository *authorizationMemoryRepository, now func() time.Time) *AuthorizationService {
	t.Helper()
	project := validDefinition()
	task := validDefinition()
	task.Name, task.Authorization.GrantScope, task.AuditEvent = "task.list", "task:read", "tool.task.list"
	search := validDefinition()
	search.Name, search.AuditEvent = "project.search", "tool.project.search"
	draft := validDefinition()
	draft.Name, draft.Authorization.GrantScope, draft.AuditEvent = "workspace.member.list", "workspace:read", "tool.workspace.member.list"
	draft.Lifecycle = LifecycleDraft
	catalog, err := NewCatalog([]Definition{project, search, task, draft})
	if err != nil {
		t.Fatal(err)
	}
	if repository.configurations == nil {
		repository.configurations = map[uint]WorkspaceConfiguration{}
	}
	repository.configurations[11] = WorkspaceConfiguration{
		WorkspaceID: 11, Mode: WorkspaceConfigurationWindow, AllowedTools: []string{"project.list", "task.list"},
		AllowedScopes: []string{"project:read", "task:read"}, EffectiveFrom: now().UTC().Add(-time.Hour),
		ExpiresAt: now().UTC().Add(time.Hour), UpdatedBy: 7, CreatedAt: now().UTC().Add(-time.Hour),
		UpdatedAt: now().UTC().Add(-time.Hour), Version: 1,
	}
	service, err := NewAuthorizationService(catalog, repository, repository, repository)
	if err != nil {
		t.Fatal(err)
	}
	service.now = now
	service.newID = func() (string, error) { return "12345678901234567890123456789012", nil }
	return service
}

func testWorkspaceConfiguration(workspaceID uint, tools, scopes []string, effectiveFrom, expiresAt time.Time) WorkspaceConfiguration {
	return WorkspaceConfiguration{
		WorkspaceID: workspaceID, Mode: WorkspaceConfigurationWindow, AllowedTools: tools, AllowedScopes: scopes,
		EffectiveFrom: effectiveFrom, ExpiresAt: expiresAt, UpdatedBy: 7,
		CreatedAt: effectiveFrom, UpdatedAt: effectiveFrom, Version: 1,
	}
}

func managementTestPrincipal() ManagementPrincipal {
	return ManagementPrincipal{PrincipalID: 7, WorkspaceID: 11, WorkspaceName: "研发"}
}
