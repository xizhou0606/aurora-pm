package toolruntimegorm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	toolruntime "github.com/xizhou0606/aurora-pm/backend/internal/module/toolruntime"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestAuthorizationDecisionAtomicallyCreatesOneScopedGrant(t *testing.T) {
	db := authorizationTestDB(t)
	requests := NewAuthorizationRequestRepository(db)
	grants := NewGrantRepository(db)
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	request := persistedAuthorizationRequest(now, "12345678901234567890123456789012", 7, 11)
	if err := requests.Create(context.Background(), &request); err != nil {
		t.Fatal(err)
	}
	grant, err := toolruntime.NewGrant(toolruntime.GrantRequest{
		PrincipalID: 7, WorkspaceID: 11, ClientID: "pm-cli", Channel: toolruntime.DeliveryCLI,
		AllowedTools:  []string{"project.list"},
		AllowedScopes: []string{"project:read"}, EffectiveFrom: now, ExpiresAt: now.Add(time.Hour),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	decided, err := requests.Decide(context.Background(), toolruntime.AuthorizationDecisionCommand{
		RequestID: request.ID, PrincipalID: 7, WorkspaceID: 11,
		Decision: toolruntime.AuthorizationAuthorize, DecidedAt: now.Add(time.Minute), Grant: &grant,
	})
	if err != nil || decided.GrantID == nil || decided.Decision != toolruntime.AuthorizationAuthorize {
		t.Fatalf("decided=%+v err=%v", decided, err)
	}
	rows, err := grants.ListForPrincipalWorkspace(context.Background(), 7, 11)
	if err != nil || len(rows) != 1 || rows[0].WorkspaceID != 11 || rows[0].PrincipalID != 7 {
		t.Fatalf("scoped grants=%+v err=%v", rows, err)
	}
	otherWorkspace, _ := grants.ListForPrincipalWorkspace(context.Background(), 7, 12)
	otherPrincipal, _ := grants.ListForPrincipalWorkspace(context.Background(), 8, 11)
	if len(otherWorkspace) != 0 || len(otherPrincipal) != 0 {
		t.Fatalf("grant isolation failed workspace=%+v principal=%+v", otherWorkspace, otherPrincipal)
	}

	_, err = requests.Decide(context.Background(), toolruntime.AuthorizationDecisionCommand{
		RequestID: request.ID, PrincipalID: 7, WorkspaceID: 11,
		Decision: toolruntime.AuthorizationAuthorize, DecidedAt: now.Add(2 * time.Minute), Grant: &grant,
	})
	if !errors.Is(err, toolruntime.ErrAuthorizationRequestDecided) {
		t.Fatalf("replayed decision err=%v", err)
	}
	var count int64
	if err := db.Model(&ExternalAuthorizationGrant{}).Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("replay created %d grants err=%v", count, err)
	}
}

func TestAuthorizationDecisionFailsClosedForOwnerExpiryAndDeny(t *testing.T) {
	db := authorizationTestDB(t)
	repository := NewAuthorizationRequestRepository(db)
	now := time.Date(2026, 8, 26, 12, 0, 0, 0, time.UTC)
	ownerBound := persistedAuthorizationRequest(now, "owner_bound_request_12345678901234", 7, 11)
	if err := repository.Create(context.Background(), &ownerBound); err != nil {
		t.Fatal(err)
	}
	_, err := repository.Decide(context.Background(), toolruntime.AuthorizationDecisionCommand{
		RequestID: ownerBound.ID, PrincipalID: 7, WorkspaceID: 99,
		Decision: toolruntime.AuthorizationDeny, DecidedAt: now.Add(time.Minute),
	})
	if !errors.Is(err, toolruntime.ErrAuthorizationRequestNotFound) {
		t.Fatalf("cross-workspace decision err=%v", err)
	}

	expired := persistedAuthorizationRequest(now, "expired_request_12345678901234567", 7, 11)
	if err := repository.Create(context.Background(), &expired); err != nil {
		t.Fatal(err)
	}
	_, err = repository.Decide(context.Background(), toolruntime.AuthorizationDecisionCommand{
		RequestID: expired.ID, PrincipalID: 7, WorkspaceID: 11,
		Decision: toolruntime.AuthorizationDeny, DecidedAt: expired.RequestExpiresAt,
	})
	if !errors.Is(err, toolruntime.ErrAuthorizationRequestExpired) {
		t.Fatalf("expired decision err=%v", err)
	}

	denied := persistedAuthorizationRequest(now, "denied_request_123456789012345678", 7, 11)
	if err := repository.Create(context.Background(), &denied); err != nil {
		t.Fatal(err)
	}
	result, err := repository.Decide(context.Background(), toolruntime.AuthorizationDecisionCommand{
		RequestID: denied.ID, PrincipalID: 7, WorkspaceID: 11,
		Decision: toolruntime.AuthorizationDeny, DecidedAt: now.Add(time.Minute),
	})
	if err != nil || result.Decision != toolruntime.AuthorizationDeny || result.GrantID != nil {
		t.Fatalf("denied=%+v err=%v", result, err)
	}
	var grantCount int64
	if err := db.Model(&ExternalAuthorizationGrant{}).Count(&grantCount).Error; err != nil || grantCount != 0 {
		t.Fatalf("deny created %d grants err=%v", grantCount, err)
	}
}

func authorizationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	name := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&ExternalAuthorizationGrant{}, &ExternalAuthorizationRequest{}); err != nil {
		t.Fatal(err)
	}
	return db
}

func persistedAuthorizationRequest(now time.Time, id string, principalID, workspaceID uint) toolruntime.AuthorizationRequest {
	return toolruntime.AuthorizationRequest{
		ID: id, PrincipalID: principalID, WorkspaceID: workspaceID, WorkspaceName: "研发",
		ClientID: "pm-cli", ClientName: "PM CLI", Channel: toolruntime.DeliveryCLI,
		ConfigurationMode: toolruntime.WorkspaceConfigurationWindow, ConfigurationVersion: 1,
		ToolNames: []string{"project.list"}, GrantScopes: []string{"project:read"},
		EffectiveFrom: now, ExpiresAt: now.Add(time.Hour), RequestExpiresAt: now.Add(10 * time.Minute),
		Decision: toolruntime.AuthorizationPending, CreatedAt: now, UpdatedAt: now,
	}
}
