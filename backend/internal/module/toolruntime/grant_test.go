package toolruntime

import (
	"errors"
	"testing"
	"time"
)

func TestGrantStatusUsesEffectiveWindowAndRevocation(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	grant := testGrant(t, now)

	if status := grant.Status(now.Add(-time.Second)); status != GrantScheduled {
		t.Fatalf("before effective status=%s", status)
	}
	if status := grant.Status(now); status != GrantActive {
		t.Fatalf("effective boundary status=%s", status)
	}
	if status := grant.Status(grant.ExpiresAt); status != GrantExpired {
		t.Fatalf("expiry boundary status=%s", status)
	}
	if err := grant.Revoke(now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if status := grant.Status(now); status != GrantRevoked {
		t.Fatalf("revocation must take precedence, status=%s", status)
	}
}

func TestGrantAuthorizeChecksEveryDelegationFactFailClosed(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	grant := testGrant(t, now)
	valid := AccessRequest{
		GrantID: 9, PrincipalID: 41, WorkspaceID: 7, ClientID: "codex-local",
		Channel: DeliveryCLI, Audience: AudienceCLI, Tool: "project.list", Scope: "project:read",
	}
	if err := grant.Authorize(valid, now); err != nil {
		t.Fatalf("valid request denied: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*AccessRequest)
		when   time.Time
		want   error
	}{
		{"grant id", func(value *AccessRequest) { value.GrantID++ }, now, ErrGrantIDMismatch},
		{"principal", func(value *AccessRequest) { value.PrincipalID++ }, now, ErrGrantPrincipalMismatch},
		{"workspace", func(value *AccessRequest) { value.WorkspaceID++ }, now, ErrGrantWorkspaceMismatch},
		{"client", func(value *AccessRequest) { value.ClientID = "other" }, now, ErrGrantClientMismatch},
		{"channel", func(value *AccessRequest) { value.Channel = DeliveryMCP }, now, ErrGrantChannelMismatch},
		{"audience", func(value *AccessRequest) { value.Audience = AudienceMCP }, now, ErrGrantAudienceMismatch},
		{"tool", func(value *AccessRequest) { value.Tool = "project.delete" }, now, ErrGrantToolDenied},
		{"malformed tool", func(value *AccessRequest) { value.Tool = "*" }, now, ErrGrantToolDenied},
		{"scope", func(value *AccessRequest) { value.Scope = "task:write" }, now, ErrGrantScopeDenied},
		{"malformed scope", func(value *AccessRequest) { value.Scope = "*" }, now, ErrGrantScopeDenied},
		{"scheduled", func(*AccessRequest) {}, now.Add(-time.Second), ErrGrantNotActive},
		{"expired", func(*AccessRequest) {}, grant.ExpiresAt, ErrGrantNotActive},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := valid
			test.mutate(&request)
			err := grant.Authorize(request, test.when)
			if !errors.Is(err, ErrGrantDenied) || !errors.Is(err, test.want) {
				t.Fatalf("err=%v, want denial and %v", err, test.want)
			}
		})
	}
}

func TestRequireAuthorizationPageForNewChangedOrInactiveGrant(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	grant := testGrant(t, now)
	request := grantRequest(grant)

	required, err := RequireAuthorizationPage(nil, request, now)
	if err != nil || !required {
		t.Fatalf("new grant required=%v err=%v", required, err)
	}
	required, err = RequireAuthorizationPage(&grant, request, now)
	if err != nil || required {
		t.Fatalf("unchanged active grant required=%v err=%v", required, err)
	}

	changes := map[string]func(*GrantRequest){
		"tool":      func(value *GrantRequest) { value.AllowedTools = append(value.AllowedTools, "task.list") },
		"scope":     func(value *GrantRequest) { value.AllowedScopes = append(value.AllowedScopes, "task:read") },
		"workspace": func(value *GrantRequest) { value.WorkspaceID++ },
		"effective": func(value *GrantRequest) { value.EffectiveFrom = value.EffectiveFrom.Add(time.Minute) },
		"expiry":    func(value *GrantRequest) { value.ExpiresAt = value.ExpiresAt.Add(time.Hour) },
		"channel":   func(value *GrantRequest) { value.Channel = DeliveryMCP },
		"client":    func(value *GrantRequest) { value.ClientID = "another-client" },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			changed := request
			changed.AllowedTools = append([]string(nil), request.AllowedTools...)
			changed.AllowedScopes = append([]string(nil), request.AllowedScopes...)
			change(&changed)
			required, err := RequireAuthorizationPage(&grant, changed, now)
			if err != nil || !required {
				t.Fatalf("required=%v err=%v", required, err)
			}
		})
	}

	reordered := request
	reordered.AllowedTools = []string{"project.update", "project.list", "project.list"}
	reordered.AllowedScopes = []string{"project:write", "project:read", "project:read"}
	required, err = RequireAuthorizationPage(&grant, reordered, now)
	if err != nil || required {
		t.Fatalf("scope order/duplicates should not widen grant: required=%v err=%v", required, err)
	}
	required, err = RequireAuthorizationPage(&grant, request, grant.ExpiresAt)
	if err != nil || !required {
		t.Fatalf("inactive grant must require page: required=%v err=%v", required, err)
	}
}

func TestNewGrantRejectsInvalidDelegations(t *testing.T) {
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	base := GrantRequest{
		PrincipalID: 1, WorkspaceID: 2, ClientID: "client", Channel: DeliveryCLI,
		AllowedTools: []string{"project.list"}, AllowedScopes: []string{"project:read"},
		EffectiveFrom: now, ExpiresAt: now.Add(time.Hour),
	}
	tests := map[string]func(*GrantRequest){
		"principal": func(value *GrantRequest) { value.PrincipalID = 0 },
		"workspace": func(value *GrantRequest) { value.WorkspaceID = 0 },
		"client":    func(value *GrantRequest) { value.ClientID = " " },
		"channel":   func(value *GrantRequest) { value.Channel = "web" },
		"tool":      func(value *GrantRequest) { value.AllowedTools = []string{"project"} },
		"scope":     func(value *GrantRequest) { value.AllowedScopes = []string{"admin"} },
		"window":    func(value *GrantRequest) { value.ExpiresAt = value.EffectiveFrom },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			request := base
			mutate(&request)
			if _, err := NewGrant(request, now); !errors.Is(err, ErrGrantInvalid) {
				t.Fatalf("err=%v, want ErrGrantInvalid", err)
			}
		})
	}
	if _, err := NewGrant(base, time.Time{}); !errors.Is(err, ErrGrantInvalid) {
		t.Fatalf("zero created_at err=%v", err)
	}
}

func testGrant(t *testing.T, now time.Time) Grant {
	t.Helper()
	grant, err := NewGrant(GrantRequest{
		PrincipalID: 41, WorkspaceID: 7, ClientID: "codex-local", Channel: DeliveryCLI,
		AllowedTools:  []string{"project.update", "project.list"},
		AllowedScopes: []string{"project:write", "project:read"},
		EffectiveFrom: now, ExpiresAt: now.Add(2 * time.Hour),
	}, now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	grant.ID = 9
	return grant
}

func grantRequest(grant Grant) GrantRequest {
	return GrantRequest{
		PrincipalID: grant.PrincipalID, WorkspaceID: grant.WorkspaceID,
		ClientID: grant.ClientID, Channel: grant.Channel,
		AllowedTools:  append([]string(nil), grant.AllowedTools...),
		AllowedScopes: append([]string(nil), grant.AllowedScopes...),
		EffectiveFrom: grant.EffectiveFrom, ExpiresAt: grant.ExpiresAt,
	}
}
